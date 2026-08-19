package codeproject

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCheckCompleteness_MissingSpecFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.py"), []byte("if __name__ == '__main__':\n    pass\n"), 0o644)

	taskDesc := "[FILES]\n- main.py\n- utils.py\n- helpers.py\n"
	svc := &Service{}
	res := svc.CheckCompleteness(dir, taskDesc, filepath.Join(dir, "main.py"))
	if len(res.MissingFiles) != 2 {
		t.Fatalf("expected 2 missing files, got %d: %v", len(res.MissingFiles), res.MissingFiles)
	}
}

func TestCheckCompleteness_NoMainGuard(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.py")
	os.WriteFile(entry, []byte("def helper():\n    pass\n"), 0o644)

	svc := &Service{}
	res := svc.CheckCompleteness(dir, "buatkan aplikasi sederhana", entry)
	if !res.NoMainGuard {
		t.Error("expected NoMainGuard=true when no __main__ block present")
	}
}

func TestCheckCompleteness_AllGood(t *testing.T) {
	dir := t.TempDir()
	entry := filepath.Join(dir, "main.py")
	os.WriteFile(entry, []byte("if __name__ == '__main__':\n    pass\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "utils.py"), []byte("x = 1\n"), 0o644)

	taskDesc := "[FILES]\n- main.py\n- utils.py\n"
	svc := &Service{}
	res := svc.CheckCompleteness(dir, taskDesc, entry)
	if len(res.MissingFiles) != 0 {
		t.Errorf("expected no missing files, got %v", res.MissingFiles)
	}
	if res.NoMainGuard {
		t.Error("expected NoMainGuard=false")
	}
}

func TestCheckCompleteness_NoSpecSection(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{}
	res := svc.CheckCompleteness(dir, "buat aplikasi todo list", filepath.Join(dir, "main.py"))
	if len(res.MissingFiles) != 0 {
		t.Error("no [FILES] section means no missing-file check should fire")
	}
}
