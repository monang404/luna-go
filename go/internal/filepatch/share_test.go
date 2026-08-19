package filepatch

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestShare_Success(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "out.txt")
	os.WriteFile(file, []byte("x"), 0o644)

	var shared string
	share := func(ctx context.Context, path string) error {
		shared = path
		return nil
	}
	if err := Share(context.Background(), file, share); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if shared != file {
		t.Errorf("share called with %q, want %q", shared, file)
	}
}

func TestShare_MissingFile(t *testing.T) {
	err := Share(context.Background(), "/nonexistent/x.txt", func(context.Context, string) error { return nil })
	if !errors.Is(err, ErrShareNoSuchFile) {
		t.Errorf("expected ErrShareNoSuchFile, got %v", err)
	}
}

func TestShare_Unavailable(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "out.txt")
	os.WriteFile(file, []byte("x"), 0o644)

	err := Share(context.Background(), file, nil)
	if !errors.Is(err, ErrShareUnavailable) {
		t.Errorf("expected ErrShareUnavailable, got %v", err)
	}
}
