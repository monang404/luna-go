package codeproject

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestFix_GeneratesAndApplies(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "buggy.py")
	os.WriteFile(file, []byte("prin('hi')\n"), 0o644)

	svc := &Service{Requester: &fakeCompleter{contents: []string{"print('hi')\n"}}, Confirm: approveConfirm, Runner: &fakeRunner{}}
	res, err := svc.Fix(context.Background(), file, "NameError: prin not defined", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Apply.Applied {
		t.Fatal("expected the fix to be applied")
	}
	got, _ := os.ReadFile(file)
	if string(got) != "print('hi')\n" {
		t.Errorf("file content = %q", got)
	}
}

func TestFix_InspectOnlyDoesNotApply(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "buggy.py")
	original := "prin('hi')\n"
	os.WriteFile(file, []byte(original), 0o644)

	svc := &Service{Requester: &fakeCompleter{contents: []string{"print('hi')\n"}}, Confirm: approveConfirm, Runner: &fakeRunner{}}
	res, err := svc.Fix(context.Background(), file, "err", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.FixedPath != file+".fixed" {
		t.Errorf("FixedPath = %q", res.FixedPath)
	}
	got, _ := os.ReadFile(file)
	if string(got) != original {
		t.Error("--inspect must not modify the original file")
	}
	if _, err := os.Stat(res.FixedPath); err != nil {
		t.Error("expected .fixed file to exist")
	}
}

func TestFix_GenerationFailure_RemovesStaleFixed(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "buggy.py")
	os.WriteFile(file, []byte("code\n"), 0o644)
	os.WriteFile(file+".fixed", []byte("stale\n"), 0o644)

	svc := &Service{Requester: &fakeCompleter{err: context.DeadlineExceeded}, Confirm: approveConfirm, Runner: &fakeRunner{}}
	_, err := svc.Fix(context.Background(), file, "err", false)
	if err == nil {
		t.Fatal("expected error on generation failure")
	}
	if _, statErr := os.Stat(file + ".fixed"); !os.IsNotExist(statErr) {
		t.Error("stale .fixed file should be removed on generation failure")
	}
}

func TestFixApply_NoFixedFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.py")
	os.WriteFile(file, []byte("x\n"), 0o644)

	svc := &Service{Confirm: approveConfirm}
	_, err := svc.FixApply(context.Background(), file, "")
	if err == nil {
		t.Fatal("expected error when no .fixed file exists")
	}
}

func TestFixApply_NoChange(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.py")
	os.WriteFile(file, []byte("same\n"), 0o644)
	os.WriteFile(file+".fixed", []byte("same\n"), 0o644)

	svc := &Service{Confirm: approveConfirm}
	res, err := svc.FixApply(context.Background(), file, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.NoChange {
		t.Error("expected NoChange=true")
	}
}

func TestFixApply_Declined(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "f.py")
	original := "old\n"
	os.WriteFile(file, []byte(original), 0o644)
	os.WriteFile(file+".fixed", []byte("new\n"), 0o644)

	svc := &Service{Confirm: declineConfirm}
	_, err := svc.FixApply(context.Background(), file, "")
	if err == nil {
		t.Fatal("expected error on decline")
	}
	got, _ := os.ReadFile(file)
	if string(got) != original {
		t.Error("file must be untouched when declined")
	}
}
