package filepatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func touchWithAge(t *testing.T, path string, age time.Duration) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-age)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
}

func TestBakClean_NothingToDo(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(fakeCompleter{}, approveConfirm)
	res, err := svc.BakClean(context.Background(), dir, "", 14, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Removed {
		t.Error("expected Removed=false when nothing is old enough")
	}
}

func TestBakClean_RemovesOldBackupsAfterConfirm(t *testing.T) {
	dir := t.TempDir()
	oldBak := filepath.Join(dir, "file.py.bak.20200101_000000_0001")
	freshBak := filepath.Join(dir, "file.py.bak.20990101_000000_0001")
	touchWithAge(t, oldBak, 30*24*time.Hour)
	touchWithAge(t, freshBak, time.Hour)

	svc := NewService(fakeCompleter{}, approveConfirm)
	res, err := svc.BakClean(context.Background(), dir, "", 14, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Removed {
		t.Fatal("expected Removed=true")
	}
	if _, err := os.Stat(oldBak); !os.IsNotExist(err) {
		t.Error("old backup should have been removed")
	}
	if _, err := os.Stat(freshBak); err != nil {
		t.Error("fresh backup should NOT have been removed")
	}
}

func TestBakClean_DeclinedLeavesFilesAlone(t *testing.T) {
	dir := t.TempDir()
	oldBak := filepath.Join(dir, "file.py.bak.20200101_000000_0001")
	touchWithAge(t, oldBak, 30*24*time.Hour)

	svc := NewService(fakeCompleter{}, declineConfirm)
	_, err := svc.BakClean(context.Background(), dir, "", 14, nil)
	if err == nil {
		t.Fatal("expected error on decline")
	}
	if _, err := os.Stat(oldBak); err != nil {
		t.Error("declined cleanup must not remove any file")
	}
}

func TestBakClean_CacheDir(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "cache")
	os.MkdirAll(cacheDir, 0o755)
	oldCache := filepath.Join(cacheDir, "abc123.json")
	touchWithAge(t, oldCache, 30*24*time.Hour)

	svc := NewService(fakeCompleter{}, approveConfirm)
	res, err := svc.BakClean(context.Background(), dir, cacheDir, 14, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.OldCache) != 1 {
		t.Fatalf("expected 1 old cache file, got %d", len(res.OldCache))
	}
	if _, err := os.Stat(oldCache); !os.IsNotExist(err) {
		t.Error("old cache file should have been removed")
	}
}
