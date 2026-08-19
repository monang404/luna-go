package codeproject

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
)

// codeSysPrompt mirrors _AI_CODE_SYSPROMPT (30-code/05-code.zsh).
const codeSysPrompt = `Kamu programmer expert. Tulis kode yang: 1) Semua string tertutup benar. 2) Semua kurung tertutup. 3) Tidak ada syntax error. 4) Selalu tangani edge case & error input/dependency. 5) Langsung bisa dijalankan. 6) Tanpa backtick atau markdown. 7) WAJIB pakai baris baru SUNGGUHAN (tekan enter beneran) buat pisah tiap statement/baris kode di luar string.`

// projectMaxToks mirrors ${AI_PROJECT_MAX_TOKS:-3500}.
const projectMaxToks = 3500

// Errors returned by Code.
var (
	ErrCodeNoProvider  = errors.New("codeproject: no LUNA provider configured")
	ErrCodeGenFailed   = errors.New("codeproject: generation failed")
	ErrCodeDeclined    = errors.New("codeproject: overwrite declined by user")
	ErrCodeTimedOut    = errors.New("codeproject: overwrite confirmation timed out")
	ErrCodeApplyFailed = errors.New("codeproject: failed to overwrite target file")
)

// CodeResult is Code's return shape.
type CodeResult struct {
	Output     string
	NoChange   bool
	Overwrote  bool
	BackupPath string
	Diff       string
}

// Service bundles the shared dependencies codeproject's operations need.
type Service struct {
	Requester aiops.Completer
	Confirm   aiops.ConfirmFunc
	Runner    aiops.CommandRunner
	// SanitizeScript/ExtractScript are config.Paths.SanitizeScript/
	// ExtractScript -- external python3 helper scripts, invoked
	// best-effort via Runner (see aiops.SanitizePyCode).
	SanitizeScript string
	CodeDir        string
}

// NewService builds a Service from the current environment's config.
func NewService(requester aiops.Completer, confirm aiops.ConfirmFunc, runner aiops.CommandRunner) *Service {
	paths := config.LoadPaths()
	return &Service{
		Requester:      requester,
		Confirm:        confirm,
		Runner:         runner,
		SanitizeScript: paths.SanitizeScript,
		CodeDir:        paths.CodeDir,
	}
}

func needAnyKeyBig() error {
	if len(config.ActiveProviders(config.TaskProviderOrderBig)) == 0 {
		return ErrCodeNoProvider
	}
	return nil
}

// Code mirrors aicode(prompt) / aicode -o <name> <prompt>: generate a
// single Python file from a prompt. When the target doesn't exist yet,
// it's written directly. When it already exists, Code follows aipatch's
// review contract instead (diff shown via the returned Diff, s.Confirm
// gates the overwrite, a ".bak.<ts>" copy is taken first) -- matching
// the zsh source's bug #59 fix (no more silent overwrite of an existing
// file).
func (s *Service) Code(ctx context.Context, outputName, prompt string) (CodeResult, error) {
	if err := needAnyKeyBig(); err != nil {
		return CodeResult{}, err
	}
	if err := os.MkdirAll(s.CodeDir, 0o755); err != nil {
		return CodeResult{}, err
	}

	var output string
	if outputName != "" {
		output = filepath.Join(s.CodeDir, outputName)
	} else {
		slug := aiops.Slugify(prompt, 40)
		output = filepath.Join(s.CodeDir, fmt.Sprintf("%s_%s.py", slug, aiops.Timestamp()))
	}

	res, err := s.Requester.Complete(ctx, codeSysPrompt, prompt, config.TaskSmart, config.TaskProviderOrderBig, projectMaxToks)
	if err != nil || res.Content == "" {
		return CodeResult{}, fmt.Errorf("%w: %v", ErrCodeGenFailed, err)
	}
	newContent := stripFences(res.Content)

	if _, statErr := os.Stat(output); statErr == nil {
		return s.overwriteExisting(ctx, output, newContent)
	}

	if err := os.WriteFile(output, []byte(newContent), 0o644); err != nil {
		return CodeResult{}, err
	}
	aiops.SanitizePyCode(ctx, s.Runner, s.SanitizeScript, output)
	return CodeResult{Output: output, Overwrote: false}, nil
}

func (s *Service) overwriteExisting(ctx context.Context, output, newContent string) (CodeResult, error) {
	existing, err := os.ReadFile(output)
	if err != nil {
		return CodeResult{}, err
	}
	if newContent == string(existing) {
		return CodeResult{Output: output, NoChange: true}, nil
	}
	diff := aiops.UnifiedDiff(output, string(existing), newContent)

	decision, err := s.Confirm(ctx, fmt.Sprintf("Timpa %s dengan hasil di atas?", output))
	if err != nil {
		return CodeResult{Output: output, Diff: diff}, err
	}
	switch decision {
	case aiops.Approved:
	case aiops.TimedOut:
		return CodeResult{Output: output, Diff: diff}, ErrCodeTimedOut
	default:
		return CodeResult{Output: output, Diff: diff}, ErrCodeDeclined
	}

	backup := aiops.BackupPath(output)
	if err := copyFile(output, backup); err != nil {
		return CodeResult{Output: output, Diff: diff}, err
	}
	if err := os.WriteFile(output, []byte(newContent), 0o644); err != nil {
		_ = copyFile(backup, output)
		return CodeResult{Output: output, Diff: diff, BackupPath: backup}, fmt.Errorf("%w: %v", ErrCodeApplyFailed, err)
	}
	aiops.SanitizePyCode(ctx, s.Runner, s.SanitizeScript, output)
	return CodeResult{Output: output, Overwrote: true, BackupPath: backup, Diff: diff}, nil
}

// stripFences mirrors `printf '%s\n' "$raw" | grep -v '```'`.
func stripFences(s string) string {
	lines := splitLines(s)
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		if containsFence(l) {
			continue
		}
		out = append(out, l)
	}
	return joinLines(out)
}

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
