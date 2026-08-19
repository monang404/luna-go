package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("setup: write %s: %v", p, err)
	}
	return p
}

func argsJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	return b
}

func TestReadFileTool_HappyPath(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "line1\nline2\nline3\n")

	res, err := ReadFileTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": p}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "line1") || !strings.Contains(res.Output, "line3") {
		t.Fatalf("output missing expected lines: %q", res.Output)
	}
	if !strings.Contains(res.Output, "1  line1") {
		t.Fatalf("expected line-numbered output, got: %q", res.Output)
	}
}

func TestReadFileTool_OffsetLimitWindow(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "l1\nl2\nl3\nl4\nl5\n")

	res, err := ReadFileTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"path": p, "offset": 2, "limit": 2,
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if strings.Contains(res.Output, "l1") || strings.Contains(res.Output, "l5") {
		t.Fatalf("window leaked lines outside offset/limit: %q", res.Output)
	}
	if !strings.Contains(res.Output, "l2") || !strings.Contains(res.Output, "l3") {
		t.Fatalf("window missing expected lines: %q", res.Output)
	}
}

func TestReadFileTool_MissingPath(t *testing.T) {
	if _, err := (ReadFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": "/no/such/file"})); err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}

func TestReadFileTool_SecretFileRejected(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, ".env", "SECRET=1\n")
	if _, err := (ReadFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p})); err == nil {
		t.Fatal("expected .env to be rejected as a secret file")
	}
}

func TestReadFileTool_BinaryFileRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bin.dat")
	if err := os.WriteFile(p, []byte{0x00, 0x01, 0x02, 'h', 'i'}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := (ReadFileTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p})); err == nil {
		t.Fatal("expected binary file to be rejected")
	}
}

func TestListDirTool_HappyPath(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "hi")
	res, err := ListDirTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": dir}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "a.txt") {
		t.Fatalf("expected directory listing to contain a.txt, got: %q", res.Output)
	}
}

func TestListDirTool_DefaultsToCurrentDir(t *testing.T) {
	if _, err := (ListDirTool{}).Execute(context.Background(), json.RawMessage(`{}`)); err != nil {
		t.Fatalf("unexpected error with omitted path: %v", err)
	}
}

func TestListDirTool_NotADirectory(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "f.txt", "x")
	if _, err := (ListDirTool{}).Execute(context.Background(), argsJSON(t, map[string]string{"path": p})); err == nil {
		t.Fatal("expected error when path is a file, not a directory")
	}
}

func TestCountLinesTool_TotalOnly(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "a\nb\nc\n")
	res, err := CountLinesTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": p}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "Total baris: 3") {
		t.Fatalf("expected total baris 3, got: %q", res.Output)
	}
}

func TestCountLinesTool_WithPattern(t *testing.T) {
	dir := t.TempDir()
	p := writeTestFile(t, dir, "a.txt", "foo\nbar\nfoo\n")
	res, err := CountLinesTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": p, "pattern": "foo"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "Kemunculan 'foo': 2") {
		t.Fatalf("expected 2 occurrences of foo, got: %q", res.Output)
	}
}

func TestGrepSearchTool_FindsMatch(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "a.txt", "needle here\nhaystack\n")
	res, err := GrepSearchTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"pattern": "needle", "path": dir}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "needle here") {
		t.Fatalf("expected match in output, got: %q", res.Output)
	}
}

func TestGrepSearchTool_RequiresPattern(t *testing.T) {
	if _, err := (GrepSearchTool{}).Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for missing pattern")
	}
}

func TestGlobSearchTool_FindsFile(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, dir, "target_unique_name.go", "package x")
	wd, _ := os.Getwd()
	defer os.Chdir(wd)
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	res, err := GlobSearchTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"pattern": "target_unique_name"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(res.Output, "target_unique_name.go") {
		t.Fatalf("expected to find target file, got: %q", res.Output)
	}
}

func TestFirstNLines_CapsOutput(t *testing.T) {
	s := "a\nb\nc\nd\n"
	got := firstNLines(s, 2)
	if got != "a\nb" {
		t.Fatalf("expected 'a\\nb', got %q", got)
	}
}

func TestFirstNLines_Empty(t *testing.T) {
	if got := firstNLines("", 10); got != "" {
		t.Fatalf("expected empty string, got %q", got)
	}
}
