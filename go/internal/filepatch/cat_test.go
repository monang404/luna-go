package filepatch

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCat_WholeFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte("one\ntwo\nthree\n"), 0o644)

	out, err := Cat(p, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "1  one") || !strings.Contains(out, "3  three") {
		t.Errorf("unexpected output: %q", out)
	}
}

func TestCat_Range(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.txt")
	os.WriteFile(p, []byte("one\ntwo\nthree\nfour\n"), 0o644)

	out, err := Cat(p, 2, 3)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "one") || strings.Contains(out, "four") {
		t.Errorf("range should exclude lines outside [2,3]: %q", out)
	}
	if !strings.Contains(out, "two") || !strings.Contains(out, "three") {
		t.Errorf("range should include lines 2-3: %q", out)
	}
}

func TestCat_BinaryRejected(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "a.bin")
	os.WriteFile(p, []byte("a\x00b"), 0o644)

	if _, err := Cat(p, 0, 0); err == nil {
		t.Error("expected binary file to be rejected")
	}
}

func TestCat_MissingFile(t *testing.T) {
	if _, err := Cat("/nonexistent/path.txt", 0, 0); err == nil {
		t.Error("expected error for missing file")
	}
}
