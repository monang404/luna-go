package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func requirePatchBinary(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("patch"); err != nil {
		t.Skip("'patch' binary not available in this environment")
	}
}

func TestPatchFileTool_AppliesUnifiedDiff(t *testing.T) {
	requirePatchBinary(t)
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "line1\nline2\nline3\n")

	diff := "--- a.txt\n+++ a.txt\n@@ -1,3 +1,3 @@\n line1\n-line2\n+LINE2\n line3\n"
	res, err := PatchFileTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "diff_content": diff}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "OK") {
		t.Fatalf("expected OK output, got %q", res.Output)
	}
	got, _ := os.ReadFile(p)
	if !strings.Contains(string(got), "LINE2") {
		t.Fatalf("expected patched content, got %q", string(got))
	}
	matches, _ := filepath.Glob(p + ".bak.*")
	if len(matches) != 1 {
		t.Fatalf("expected one backup file, found %d", len(matches))
	}
}

func TestPatchFileTool_RestoresBackupOnFailure(t *testing.T) {
	requirePatchBinary(t)
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "unrelated content\n")

	badDiff := "--- a.txt\n+++ a.txt\n@@ -1,3 +1,3 @@\n line1\n-line2\n+LINE2\n line3\n"
	if _, err := (PatchFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "diff_content": badDiff})); err == nil {
		t.Fatal("expected error when patch does not apply")
	}
	got, _ := os.ReadFile(p)
	if string(got) != "unrelated content\n" {
		t.Fatalf("expected original content restored after failed patch, got %q", string(got))
	}
	// Backup should have been cleaned up after a successful restore.
	matches, _ := filepath.Glob(p + ".bak.*")
	if len(matches) != 0 {
		t.Fatalf("expected backup to be removed after restore, found %v", matches)
	}
}

func TestPatchFileTool_RejectsOversizedDiff(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "x\n")
	huge := strings.Repeat("x", 200001)
	if _, err := (PatchFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "diff_content": huge})); err == nil {
		t.Fatal("expected error for oversized diff_content")
	}
}

func TestPatchFileTool_MissingFile(t *testing.T) {
	if _, err := (PatchFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": "/no/such/file", "diff_content": "x"})); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDeleteFileTool_BacksUpAndDeletes(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "important data")

	res, err := DeleteFileTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": p}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "OK") {
		t.Fatalf("expected OK output, got %q", res.Output)
	}
	if _, err := os.Stat(p); err == nil {
		t.Fatal("expected original file to be deleted")
	}
	matches, _ := filepath.Glob(p + ".bak.*")
	if len(matches) != 1 {
		t.Fatalf("expected exactly one backup file (AC-03), found %d", len(matches))
	}
	backupContent, _ := os.ReadFile(matches[0])
	if string(backupContent) != "important data" {
		t.Fatalf("expected backup to preserve original content, got %q", string(backupContent))
	}
}

func TestDeleteFileTool_MissingFile(t *testing.T) {
	if _, err := (DeleteFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": "/no/such/file"})); err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestDeleteFileTool_RejectsSecretFile(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "id_rsa", "-----BEGIN OPENSSH PRIVATE KEY-----\n")
	if _, err := (DeleteFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p})); err == nil {
		t.Fatal("expected id_rsa to be rejected as a secret file")
	}
}
