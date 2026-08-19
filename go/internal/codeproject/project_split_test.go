package codeproject

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSplitFiles_Basic(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{Runner: &fakeRunner{}}
	raw := "### FILE: main.py\nprint('hi')\n### FILE: utils.py\ndef f():\n    pass\n"

	res, err := svc.SplitFiles(context.Background(), dir, raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Written) != 2 {
		t.Fatalf("expected 2 files, got %d: %v", len(res.Written), res.Written)
	}
	main, _ := os.ReadFile(filepath.Join(dir, "main.py"))
	if string(main) != "print('hi')\n" {
		t.Errorf("main.py content = %q", main)
	}
	utils, _ := os.ReadFile(filepath.Join(dir, "utils.py"))
	if string(utils) != "def f():\n    pass\n" {
		t.Errorf("utils.py content = %q", utils)
	}
}

func TestSplitFiles_NoMarkers_Noop(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{Runner: &fakeRunner{}}
	res, err := svc.SplitFiles(context.Background(), dir, "no markers here at all", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Written) != 0 {
		t.Error("expected no files written when hasMarkers=false")
	}
}

func TestSplitFiles_RejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{Runner: &fakeRunner{}}
	raw := "### FILE: /etc/passwd\nmalicious content\n"

	res, err := svc.SplitFiles(context.Background(), dir, raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Written) != 0 {
		t.Fatal("must not write to an absolute path")
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(res.Warnings))
	}
	if _, err := os.Stat("/etc/passwd.written-by-test"); err == nil {
		t.Fatal("sanity check file should not exist")
	}
}

func TestSplitFiles_RejectsPathTraversal(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{Runner: &fakeRunner{}}
	raw := "### FILE: ../../etc/passwd\nmalicious\n"

	res, err := svc.SplitFiles(context.Background(), dir, raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Written) != 0 {
		t.Fatal("must not write outside projectDir via ..")
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(res.Warnings), res.Warnings)
	}
}

func TestSplitFiles_AllowsSafeSubdirectory(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{Runner: &fakeRunner{}}
	raw := "### FILE: pkg/sub/module.py\nx = 1\n"

	res, err := svc.SplitFiles(context.Background(), dir, raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Written) != 1 {
		t.Fatalf("expected 1 file written, got %d", len(res.Written))
	}
	if _, err := os.Stat(filepath.Join(dir, "pkg", "sub", "module.py")); err != nil {
		t.Errorf("expected nested file to exist: %v", err)
	}
}

func TestSplitFiles_UnmarkedLinesBeforeFirstMarkerAreIgnored(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{Runner: &fakeRunner{}}
	raw := "some preamble text\nmore text\n### FILE: main.py\nreal content\n"

	res, err := svc.SplitFiles(context.Background(), dir, raw, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "main.py"))
	if string(got) != "real content\n" {
		t.Errorf("preamble leaked into file content: %q", got)
	}
	_ = res
}
