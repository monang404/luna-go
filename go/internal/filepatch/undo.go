package filepatch

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/monang404/luna-go/internal/aiops"
)

// Errors returned by Undo.
var (
	ErrUndoNoBackup = errors.New("filepatch: no backups found for file")
	ErrUndoDeclined = errors.New("filepatch: restore declined by user")
	ErrUndoTimedOut = errors.New("filepatch: restore confirmation timed out")
	ErrUndoNoChoice = errors.New("filepatch: no backup selected")
)

// ChooseFunc is the injectable replacement for aiundo -s/--select's
// interactive picker (gum choose, or numbered `read` fallback). options
// is ordered newest-first, matching `ls -t`. An empty return value (with
// a nil error) means the user cancelled the picker.
type ChooseFunc func(ctx context.Context, prompt string, options []string) (string, error)

// UndoResult reports what Undo did.
type UndoResult struct {
	RestoredFrom string
	// SafetyBackup is the ".bak.<ts>.before_undo" copy of the file's
	// state immediately before the restore, so "undo of an undo" stays
	// possible -- mirrors aiundo's own `$safety` copy.
	SafetyBackup string
}

// ListBackups mirrors the raw_matches glob (`"${file}.bak.*"(N)`),
// returned newest-first (matching `ls -t`).
func ListBackups(file string) ([]string, error) {
	matches, err := filepath.Glob(file + ".bak.*")
	if err != nil {
		return nil, err
	}
	sort.Slice(matches, func(i, j int) bool {
		ii, _ := os.Stat(matches[i])
		jj, _ := os.Stat(matches[j])
		if ii == nil || jj == nil {
			return false
		}
		return ii.ModTime().After(jj.ModTime())
	})
	return matches, nil
}

// Undo mirrors aiundo(file) / aiundo -s file: restore file from a
// backup. When selectMode is false, the newest backup is used directly
// (no confirm-picker involved, only the final "Restore?" gate). When
// selectMode is true, choose is consulted -- unless there is exactly one
// backup, in which case (matching the zsh source) the single candidate
// is used without prompting a picker over a one-item list.
func (s *Service) Undo(ctx context.Context, file string, selectMode bool, choose ChooseFunc) (UndoResult, error) {
	backups, err := ListBackups(file)
	if err != nil {
		return UndoResult{}, err
	}
	if len(backups) == 0 {
		return UndoResult{}, fmt.Errorf("%w: %s", ErrUndoNoBackup, file)
	}

	var chosen string
	switch {
	case !selectMode:
		chosen = backups[0]
	case len(backups) == 1:
		chosen = backups[0]
	default:
		if choose == nil {
			return UndoResult{}, ErrUndoNoChoice
		}
		picked, err := choose(ctx, fmt.Sprintf("Pilih backup buat restore %s (terbaru di atas):", file), backups)
		if err != nil {
			return UndoResult{}, err
		}
		if picked == "" {
			return UndoResult{}, ErrUndoNoChoice
		}
		chosen = picked
	}

	decision, err := s.Confirm(ctx, "Restore?")
	if err != nil {
		return UndoResult{}, err
	}
	switch decision {
	case aiops.Approved:
	case aiops.TimedOut:
		return UndoResult{}, ErrUndoTimedOut
	default:
		return UndoResult{}, ErrUndoDeclined
	}

	// State-before-undo safety copy, best-effort (matches zsh's `cp ...
	// 2>/dev/null` -- a failure here does not block the restore).
	safety := file + ".bak." + aiops.Timestamp() + ".before_undo"
	_ = copyFile(file, safety)

	if err := copyFile(chosen, file); err != nil {
		return UndoResult{SafetyBackup: safety}, fmt.Errorf("filepatch: restore failed: %w", err)
	}

	return UndoResult{RestoredFrom: chosen, SafetyBackup: safety}, nil
}
