package codeproject

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestCode_NewFile(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	svc := &Service{Requester: &fakeCompleter{contents: []string{"print('hi')\n"}}, Confirm: approveConfirm, Runner: &fakeRunner{}, CodeDir: dir}

	res, err := svc.Code(context.Background(), "hello.py", "print hi")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Overwrote {
		t.Error("expected Overwrote=false for a new file")
	}
	got, _ := os.ReadFile(res.Output)
	if string(got) != "print('hi')\n" {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestCode_AutoNamedFile(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	svc := &Service{Requester: &fakeCompleter{contents: []string{"x = 1\n"}}, Confirm: approveConfirm, Runner: &fakeRunner{}, CodeDir: dir}

	res, err := svc.Code(context.Background(), "", "Buat variabel X")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if filepath.Dir(res.Output) != dir {
		t.Errorf("expected output under %s, got %s", dir, res.Output)
	}
	if filepath.Ext(res.Output) != ".py" {
		t.Errorf("expected .py extension, got %s", res.Output)
	}
}

func TestCode_ExistingFile_ApprovedOverwrite(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.py")
	os.WriteFile(target, []byte("old code\n"), 0o644)

	svc := &Service{Requester: &fakeCompleter{contents: []string{"new code\n"}}, Confirm: approveConfirm, Runner: &fakeRunner{}, CodeDir: dir}
	res, err := svc.Code(context.Background(), "existing.py", "update it")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Overwrote {
		t.Error("expected Overwrote=true")
	}
	if res.BackupPath == "" {
		t.Error("expected a backup path")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "new code\n" {
		t.Errorf("unexpected content: %q", got)
	}
	backupContent, _ := os.ReadFile(res.BackupPath)
	if string(backupContent) != "old code\n" {
		t.Errorf("backup content wrong: %q", backupContent)
	}
}

func TestCode_ExistingFile_DeclinedLeavesFileUntouched(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.py")
	os.WriteFile(target, []byte("old code\n"), 0o644)

	svc := &Service{Requester: &fakeCompleter{contents: []string{"new code\n"}}, Confirm: declineConfirm, Runner: &fakeRunner{}, CodeDir: dir}
	_, err := svc.Code(context.Background(), "existing.py", "update it")
	if err == nil {
		t.Fatal("expected error on decline")
	}
	got, _ := os.ReadFile(target)
	if string(got) != "old code\n" {
		t.Error("file should be untouched when declined")
	}
}

func TestCode_ExistingFile_NoChange(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	target := filepath.Join(dir, "existing.py")
	os.WriteFile(target, []byte("same code\n"), 0o644)

	svc := &Service{Requester: &fakeCompleter{contents: []string{"same code\n"}}, Confirm: approveConfirm, Runner: &fakeRunner{}, CodeDir: dir}
	res, err := svc.Code(context.Background(), "existing.py", "no real change")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.NoChange {
		t.Error("expected NoChange=true")
	}
}

func TestCode_NoProvider(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{Requester: &fakeCompleter{contents: []string{"x"}}, Confirm: approveConfirm, Runner: &fakeRunner{}, CodeDir: dir}
	_, err := svc.Code(context.Background(), "", "prompt")
	if err != ErrCodeNoProvider {
		t.Errorf("expected ErrCodeNoProvider, got %v", err)
	}
}

func TestCode_StripsCodeFences(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	svc := &Service{Requester: &fakeCompleter{contents: []string{"```python\nprint(1)\n```\n"}}, Confirm: approveConfirm, Runner: &fakeRunner{}, CodeDir: dir}
	res, err := svc.Code(context.Background(), "f.py", "print 1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, _ := os.ReadFile(res.Output)
	if string(got) != "print(1)\n" {
		t.Errorf("expected fences stripped, got %q", got)
	}
}
