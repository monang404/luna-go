package filepatch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSecretFile(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		want bool
	}{
		{".env", true},
		{".env.production", true},
		{"my_secret.txt", true},
		{"credentials.json", true},
		{"id_rsa", true},
		{"server.pem", true},
		{"api.key", true},
		{".npmrc", true},
		{"client_secret_123.json", true},
		{"main.py", false},
		{"README.md", false},
	}
	for _, c := range cases {
		p := filepath.Join(dir, c.name)
		if err := os.WriteFile(p, []byte("hello"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := IsSecretFile(p); got != c.want {
			t.Errorf("IsSecretFile(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestIsSecretFile_ContentLayer(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "weird_name.txt")
	os.WriteFile(p, []byte("-----BEGIN RSA PRIVATE KEY-----\nabc\n"), 0o644)
	if !IsSecretFile(p) {
		t.Error("expected content-based PEM header detection to flag file as secret")
	}
}

func TestIsBinaryFile(t *testing.T) {
	dir := t.TempDir()

	text := filepath.Join(dir, "a.txt")
	os.WriteFile(text, []byte("hello world\nline2\n"), 0o644)
	if IsBinaryFile(text) {
		t.Error("text file misclassified as binary")
	}

	bin := filepath.Join(dir, "a.bin")
	os.WriteFile(bin, []byte("hello\x00world"), 0o644)
	if !IsBinaryFile(bin) {
		t.Error("NUL-containing file not classified as binary")
	}

	if IsBinaryFile(filepath.Join(dir, "nope.txt")) {
		t.Error("nonexistent file should not be classified as binary")
	}
}
