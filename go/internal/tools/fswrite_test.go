package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteFileTool_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "new.txt")
	res, err := WriteFileTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "content": "hello"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "OK") {
		t.Fatalf("expected OK output, got: %q", res.Output)
	}
	got, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if string(got) != "hello\n" {
		t.Fatalf("expected trailing newline appended, got %q", string(got))
	}
}

func TestWriteFileTool_RefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "existing.txt", "old")
	if _, err := (WriteFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "content": "new"})); err == nil {
		t.Fatal("expected error when file already exists")
	}
}

func TestWriteFileTool_RejectsEmptyContent(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "new.txt")
	if _, err := (WriteFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "content": ""})); err == nil {
		t.Fatal("expected error for empty content (matches zsh source's -z check)")
	}
}

func TestWriteFileTool_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a", "b", "c.txt")
	if _, err := (WriteFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "content": "x"})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatalf("expected nested file to exist: %v", err)
	}
}

func TestWriteFileTool_RejectsSecretFilename(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, ".env")
	if _, err := (WriteFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "content": "x"})); err == nil {
		t.Fatal("expected .env to be rejected")
	}
}

func TestEditFileTool_ReplacesUniqueOccurrence(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "foo bar baz")
	res, err := EditFileTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "old_str": "bar", "new_str": "QUX"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "backup:") {
		t.Fatalf("expected backup path mentioned, got: %q", res.Output)
	}
	got, _ := os.ReadFile(p)
	if string(got) != "foo QUX baz" {
		t.Fatalf("expected replacement, got %q", string(got))
	}
	// Backup file should exist alongside the edited file.
	matches, _ := filepath.Glob(p + ".bak.*")
	if len(matches) != 1 {
		t.Fatalf("expected exactly one backup file, found %d: %v", len(matches), matches)
	}
	backupContent, _ := os.ReadFile(matches[0])
	if string(backupContent) != "foo bar baz" {
		t.Fatalf("expected backup to hold pre-edit content, got %q", string(backupContent))
	}
}

func TestEditFileTool_RejectsZeroOccurrences(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "foo bar")
	if _, err := (EditFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "old_str": "nope", "new_str": "x"})); err == nil {
		t.Fatal("expected error when old_str not found")
	}
}

func TestEditFileTool_RejectsMultipleOccurrences(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "foo foo foo")
	if _, err := (EditFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "old_str": "foo", "new_str": "x"})); err == nil {
		t.Fatal("expected error when old_str matches more than once")
	}
}

func TestEditFileTool_MissingFile(t *testing.T) {
	if _, err := (EditFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": "/no/such/file", "old_str": "a", "new_str": "b"})); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestMoveFileTool_MovesFile(t *testing.T) {
	dir := t.TempDir()
	src := writeTestFile(t, dir, "src.txt", "content")
	dest := filepath.Join(dir, "sub", "dest.txt")
	res, err := MoveFileTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": src, "dest": dest}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "OK") {
		t.Fatalf("expected OK output, got %q", res.Output)
	}
	if _, err := os.Stat(src); err == nil {
		t.Fatal("expected source file to no longer exist")
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "content" {
		t.Fatalf("expected dest file with original content, got %q err=%v", got, err)
	}
}

func TestMoveFileTool_RefusesExistingDest(t *testing.T) {
	dir := t.TempDir()
	src := writeTestFile(t, dir, "src.txt", "a")
	dest := writeTestFile(t, dir, "dest.txt", "b")
	if _, err := (MoveFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": src, "dest": dest})); err == nil {
		t.Fatal("expected error when destination already exists")
	}
}

func TestMoveFileTool_MissingSource(t *testing.T) {
	dir := t.TempDir()
	if _, err := (MoveFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": "/no/such/file", "dest": filepath.Join(dir, "d.txt")})); err == nil {
		t.Fatal("expected error for missing source")
	}
}
