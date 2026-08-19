package codeproject

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
)

// fixSysPrompt mirrors aifix's inline sysprompt.
const fixSysPrompt = `Kamu programmer expert. Diberikan kode dan pesan error, perbaiki kodenya. Output HANYA kode yang sudah diperbaiki secara lengkap, tanpa penjelasan, tanpa markdown/backtick. WAJIB pakai baris baru SUNGGUHAN buat pisah tiap statement/baris kode di luar string.`

// Errors returned by FixApply/Fix.
var (
	ErrFixNoFixedFile = errors.New("codeproject: no <file>.fixed to apply -- run Fix first")
	ErrFixDeclined    = errors.New("codeproject: fix declined by user")
	ErrFixTimedOut    = errors.New("codeproject: fix confirmation timed out")
	ErrFixApplyFailed = errors.New("codeproject: failed to apply the fix")
	ErrFixUsage       = errors.New("codeproject: usage: file and error message are required")
	ErrFixGenFailed   = errors.New("codeproject: fix generation failed")
)

// FixApplyResult reports what FixApply did.
type FixApplyResult struct {
	NoChange   bool
	Applied    bool
	BackupPath string
	Diff       string
}

// FixApply mirrors _ai_fix_apply(file, label): apply a previously
// generated "<file>.fixed" onto file, gated by s.Confirm, with the same
// backup + restore-before-delete-on-failure discipline as
// filepatch.Patch/Code.overwriteExisting. This is intentionally the
// SAME apply contract as those two -- see the zsh source's own comment
// on why _ai_fix_apply exists as a standalone, reusable step (airun
// calls it too via Run below).
func (s *Service) FixApply(ctx context.Context, file, label string) (FixApplyResult, error) {
	if label == "" {
		label = file
	}
	fixed := file + ".fixed"
	if _, err := os.Stat(fixed); err != nil {
		return FixApplyResult{}, fmt.Errorf("%w: %s", ErrFixNoFixedFile, fixed)
	}

	original, err := os.ReadFile(file)
	if err != nil {
		return FixApplyResult{}, err
	}
	fixedContent, err := os.ReadFile(fixed)
	if err != nil {
		return FixApplyResult{}, err
	}
	if string(original) == string(fixedContent) {
		return FixApplyResult{NoChange: true}, nil
	}
	diff := aiops.UnifiedDiff(file, string(original), string(fixedContent))

	decision, err := s.Confirm(ctx, fmt.Sprintf("Terapkan perbaikan ke %s?", file))
	if err != nil {
		return FixApplyResult{Diff: diff}, err
	}
	switch decision {
	case aiops.Approved:
	case aiops.TimedOut:
		return FixApplyResult{Diff: diff}, ErrFixTimedOut
	default:
		return FixApplyResult{Diff: diff}, ErrFixDeclined
	}

	backup := aiops.BackupPath(file)
	if err := copyFile(file, backup); err != nil {
		return FixApplyResult{Diff: diff}, err
	}
	if err := os.WriteFile(file, fixedContent, 0o644); err != nil {
		_ = copyFile(backup, file)
		return FixApplyResult{Diff: diff, BackupPath: backup}, fmt.Errorf("%w: %v", ErrFixApplyFailed, err)
	}

	written, err := os.ReadFile(file)
	if err != nil || string(written) == string(original) {
		return FixApplyResult{Diff: diff, BackupPath: backup}, ErrFixApplyFailed
	}
	return FixApplyResult{Applied: true, BackupPath: backup, Diff: diff}, nil
}

// FixResult is Fix's return shape.
type FixResult struct {
	FixedPath string
	Apply     FixApplyResult // zero value when inspectOnly is true
}

// Fix mirrors aifix([--inspect]) file error_msg: ask the model to
// repair file given error_msg, write "<file>.fixed", best-effort
// sanitize it, then (unless inspectOnly) immediately run FixApply.
func (s *Service) Fix(ctx context.Context, file, errorMsg string, inspectOnly bool) (FixResult, error) {
	if err := needAnyKeyBig(); err != nil {
		return FixResult{}, err
	}
	if file == "" || errorMsg == "" {
		return FixResult{}, ErrFixUsage
	}
	code, err := os.ReadFile(file)
	if err != nil {
		return FixResult{}, err
	}

	userMsg := fmt.Sprintf("Kode:\n%s\n\nError:\n%s", string(code), errorMsg)
	res, err := s.Requester.Complete(ctx, fixSysPrompt, userMsg, config.TaskSmart, config.TaskProviderOrderBig, 0)
	fixedPath := file + ".fixed"
	if err != nil || res.Content == "" {
		os.Remove(fixedPath)
		return FixResult{}, fmt.Errorf("%w: %v", ErrFixGenFailed, err)
	}
	reply := stripFences(res.Content)
	if err := os.WriteFile(fixedPath, []byte(reply), 0o644); err != nil {
		return FixResult{}, err
	}
	aiops.SanitizePyCode(ctx, s.Runner, s.SanitizeScript, fixedPath)

	if inspectOnly {
		return FixResult{FixedPath: fixedPath}, nil
	}

	applyRes, err := s.FixApply(ctx, file, file+": "+errorMsg)
	return FixResult{FixedPath: fixedPath, Apply: applyRes}, err
}
