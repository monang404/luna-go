package filepatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
)

// patchSysPrompt mirrors aipatch's sysprompt (35-files/10-aipatch.zsh):
// the model is asked to return the COMPLETE file content, not a diff --
// see that file's comment for why (small/free-tier models often produce
// diffs that don't apply cleanly; full-file + local `diff -u` is more
// reliable).
const patchSysPrompt = `Kamu programmer expert. Kamu dikasih ISI FILE LENGKAP dan instruksi perubahan. Balas HANYA isi file LENGKAP hasil perubahan -- bukan diff, bukan potongan, bukan penjelasan, bukan markdown/backtick. Output harus langsung siap ditimpa ke disk apa adanya. Bagian yang gak diminta berubah harus tetap PERSIS sama seperti aslinya.`

// Errors returned by Patch, distinguishing each guard so callers (and
// tests) can assert on the specific rejection reason -- mirroring each
// distinct message aipatch prints before returning 1.
var (
	ErrPatchUsage       = errors.New("filepatch: usage: file and instruction are required")
	ErrPatchNoSuchFile  = errors.New("filepatch: file not found")
	ErrPatchBinaryFile  = errors.New("filepatch: refusing to patch a binary file")
	ErrPatchSecretFile  = errors.New("filepatch: refusing to send a secret-looking file to the model without --force")
	ErrPatchFileTooBig  = errors.New("filepatch: file exceeds the size guard without --force")
	ErrPatchEmptyReply  = errors.New("filepatch: model returned no content")
	ErrPatchDeclined    = errors.New("filepatch: change declined by user")
	ErrPatchTimedOut    = errors.New("filepatch: confirmation timed out")
	ErrPatchApplyFailed = errors.New("filepatch: failed to apply change to disk")
)

// PatchResult reports what Patch actually did.
type PatchResult struct {
	// NoChange is true when the model proposed content identical to the
	// original file (aipatch's "LUNA gak mengusulkan perubahan apa pun."
	// no-op success path).
	NoChange bool
	// Applied is true once the new content has been written to disk.
	Applied bool
	// BackupPath is the ".bak.<ts>" copy of the original file, taken
	// before the overwrite (empty when NoChange is true, since nothing
	// was ever written).
	BackupPath string
	// Diff is the unified diff shown to the user for review/confirmation
	// (unstyled -- ANSI colorization is a terminal/UI concern, out of
	// scope for this package per SESSION-54 §3 internal/ui note).
	Diff string
}

// Service bundles the shared dependencies internal/filepatch's
// destructive operations need: an aiops.Requester for the LLM call, and
// an aiops.ConfirmFunc for the mandatory review gate.
type Service struct {
	Requester aiops.Completer
	Confirm   aiops.ConfirmFunc
	Limits    config.Limits
}

// NewService builds a Service with the given Completer/ConfirmFunc and
// the current environment's config.Limits.
func NewService(requester aiops.Completer, confirm aiops.ConfirmFunc) *Service {
	return &Service{Requester: requester, Confirm: confirm, Limits: config.LoadLimits()}
}

// Patch mirrors aipatch(file, instruction) / aipatch --force. It never
// writes to disk without first getting Approved from s.Confirm, and it
// always takes a backup before overwriting -- verifying the post-write
// state and restoring from backup if the write left the file missing or
// unexpectedly unchanged (RC-013 restore-before-delete discipline,
// mirrored exactly from the zsh source).
func (s *Service) Patch(ctx context.Context, file, instruction string, force bool) (PatchResult, error) {
	if file == "" || instruction == "" {
		return PatchResult{}, ErrPatchUsage
	}
	info, err := os.Stat(file)
	if err != nil || info.IsDir() {
		return PatchResult{}, fmt.Errorf("%w: %s", ErrPatchNoSuchFile, file)
	}
	if IsBinaryFile(file) {
		return PatchResult{}, fmt.Errorf("%w: %s", ErrPatchBinaryFile, file)
	}
	if !force && IsSecretFile(file) {
		return PatchResult{}, fmt.Errorf("%w: %s", ErrPatchSecretFile, file)
	}

	content, err := os.ReadFile(file)
	if err != nil {
		return PatchResult{}, err
	}
	maxChars := s.Limits.FileMaxChars
	if maxChars <= 0 {
		maxChars = 40000
	}
	if !force && len(content) > maxChars {
		return PatchResult{}, fmt.Errorf("%w: %s is %d chars, limit %d", ErrPatchFileTooBig, file, len(content), maxChars)
	}

	userMsg := fmt.Sprintf("Nama file: %s\nInstruksi: %s\n\nIsi file saat ini:\n%s", file, instruction, string(content))
	result, err := s.Requester.Complete(ctx, patchSysPrompt, userMsg, config.TaskSmart, config.TaskProviderOrderSmart, 0)
	if err != nil {
		return PatchResult{}, fmt.Errorf("filepatch: patch request failed: %w", err)
	}
	newContent := stripCodeFences(result.Content)
	if newContent == "" {
		return PatchResult{}, ErrPatchEmptyReply
	}

	if newContent == string(content) {
		return PatchResult{NoChange: true}, nil
	}

	diff := aiops.UnifiedDiff(file, string(content), newContent)

	decision, err := s.Confirm(ctx, fmt.Sprintf("Terapkan perubahan ke %s?", file))
	if err != nil {
		return PatchResult{Diff: diff}, err
	}
	switch decision {
	case aiops.Approved:
	case aiops.TimedOut:
		return PatchResult{Diff: diff}, ErrPatchTimedOut
	default:
		return PatchResult{Diff: diff}, ErrPatchDeclined
	}

	backup := aiops.BackupPath(file)
	if err := copyFile(file, backup); err != nil {
		return PatchResult{Diff: diff}, fmt.Errorf("filepatch: backup failed: %w", err)
	}
	if err := os.WriteFile(file, []byte(newContent), info.Mode().Perm()); err != nil {
		restoreErr := copyFile(backup, file)
		msg := fmt.Errorf("%w: %v", ErrPatchApplyFailed, err)
		if restoreErr != nil {
			return PatchResult{Diff: diff, BackupPath: backup}, msg
		}
		return PatchResult{Diff: diff, BackupPath: backup}, msg
	}

	written, err := os.ReadFile(file)
	if err != nil || string(written) == string(content) {
		// Post-write verification failed exactly like the zsh source's
		// post-mv `diff -q` check -- the file wasn't actually changed.
		return PatchResult{Diff: diff, BackupPath: backup}, ErrPatchApplyFailed
	}

	return PatchResult{Applied: true, BackupPath: backup, Diff: diff}, nil
}

// stripCodeFences mirrors `grep -vE '^```'`: drop any line that is
// exactly a markdown code-fence marker, keeping everything else
// (including the original trailing-newline structure) as-is.
func stripCodeFences(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

// copyFile copies src to dst, preserving src's permissions.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	perm := os.FileMode(0o644)
	if err == nil {
		perm = info.Mode().Perm()
	}
	return os.WriteFile(dst, data, perm)
}
