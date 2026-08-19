package filepatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUndo_NoBackups(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	os.WriteFile(file, []byte("current\n"), 0o644)

	svc := NewService(fakeCompleter{}, approveConfirm)
	_, err := svc.Undo(context.Background(), file, false, nil)
	if err == nil {
		t.Fatal("expected error when there are no backups")
	}
}

func TestUndo_LatestByDefault(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	os.WriteFile(file, []byte("current\n"), 0o644)

	older := file + ".bak.20200101_000000_0001"
	newer := file + ".bak.20200102_000000_0002"
	os.WriteFile(older, []byte("older content\n"), 0o644)
	time.Sleep(10 * time.Millisecond)
	os.WriteFile(newer, []byte("newer content\n"), 0o644)

	svc := NewService(fakeCompleter{}, approveConfirm)
	res, err := svc.Undo(context.Background(), file, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RestoredFrom != newer {
		t.Errorf("expected to restore from the newest backup %q, got %q", newer, res.RestoredFrom)
	}
	got, _ := os.ReadFile(file)
	if string(got) != "newer content\n" {
		t.Errorf("file content after restore = %q", got)
	}
	if res.SafetyBackup == "" {
		t.Error("expected a before_undo safety backup")
	}
	if _, err := os.Stat(res.SafetyBackup); err != nil {
		t.Errorf("safety backup should exist: %v", err)
	}
}

func TestUndo_SelectModeSingleBackup(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	os.WriteFile(file, []byte("current\n"), 0o644)
	backup := file + ".bak.20200101_000000_0001"
	os.WriteFile(backup, []byte("only backup\n"), 0o644)

	svc := NewService(fakeCompleter{}, approveConfirm)
	// choose() must NOT be called when there's only one candidate.
	called := false
	choose := func(ctx context.Context, prompt string, options []string) (string, error) {
		called = true
		return options[0], nil
	}
	res, err := svc.Undo(context.Background(), file, true, choose)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("choose() should not be invoked for a single-candidate list")
	}
	if res.RestoredFrom != backup {
		t.Errorf("expected restore from %q, got %q", backup, res.RestoredFrom)
	}
}

func TestUndo_SelectModeMultipleBackups(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	os.WriteFile(file, []byte("current\n"), 0o644)
	b1 := file + ".bak.20200101_000000_0001"
	b2 := file + ".bak.20200102_000000_0002"
	os.WriteFile(b1, []byte("b1\n"), 0o644)
	os.WriteFile(b2, []byte("b2\n"), 0o644)

	svc := NewService(fakeCompleter{}, approveConfirm)
	choose := func(ctx context.Context, prompt string, options []string) (string, error) {
		return b1, nil
	}
	res, err := svc.Undo(context.Background(), file, true, choose)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RestoredFrom != b1 {
		t.Errorf("expected chosen backup %q, got %q", b1, res.RestoredFrom)
	}
}

func TestUndo_Declined(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	original := "current\n"
	os.WriteFile(file, []byte(original), 0o644)
	os.WriteFile(file+".bak.x", []byte("backup\n"), 0o644)

	svc := NewService(fakeCompleter{}, declineConfirm)
	_, err := svc.Undo(context.Background(), file, false, nil)
	if err == nil {
		t.Fatal("expected error on decline")
	}
	got, _ := os.ReadFile(file)
	if string(got) != original {
		t.Error("file must not change when the user declines")
	}
}
