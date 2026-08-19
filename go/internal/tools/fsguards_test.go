package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSecretFile_NamePatterns(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name   string
		secret bool
	}{
		{".env", true},
		{".env.local", true},
		{"my_secret_config.yaml", true},
		{"aws_credential_store.json", true},
		{"server.pem", true},
		{"api.key", true},
		{"access_token.txt", true},
		{"id_rsa", true},
		{"id_ed25519", true},
		{"backup.ppk", true},
		{".npmrc", true},
		{".pgpass", true},
		{".netrc", true},
		{".git-credentials", true},
		{"credentials.json", true},
		{"client_secret_123.json", true},
		{"bundle.pfx", true},
		{"bundle.p12", true},
		{"store.jks", true},
		{"store.keystore", true},
		{"main.go", false},
		{"README.md", false},
		{"config.yaml", false},
		{"index.js", false},
	}
	for _, c := range cases {
		p := filepath.Join(dir, c.name)
		got := IsSecretFile(p)
		if got != c.secret {
			t.Errorf("IsSecretFile(%q) = %v, want %v", c.name, got, c.secret)
		}
	}
}

func TestIsSecretFile_PrivateKeyContent(t *testing.T) {
	dir := t.TempDir()
	// A filename that matches no name pattern, but whose content is a
	// PEM private key -- the second-layer content check.
	p := filepath.Join(dir, "mysterious_file")
	if err := os.WriteFile(p, []byte("-----BEGIN RSA PRIVATE KEY-----\nMIIB...\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !IsSecretFile(p) {
		t.Error("expected file with PEM private key header to be detected as secret")
	}
}

func TestIsSecretFile_NonexistentPathStillMatchesByName(t *testing.T) {
	if !IsSecretFile("/does/not/exist/.env") {
		t.Error("expected .env to be flagged as secret purely by name, even if it doesn't exist yet")
	}
}

func TestIsBinaryFile_NullByteHeuristic(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bin.dat")
	if err := os.WriteFile(p, []byte{'h', 'i', 0x00, 'x'}, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if !IsBinaryFile(p) {
		t.Error("expected file containing a NUL byte to be detected as binary")
	}
}

func TestIsBinaryFile_TextFileIsNotBinary(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "text.txt")
	if err := os.WriteFile(p, []byte("hello world\nsecond line\n"), 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if IsBinaryFile(p) {
		t.Error("expected plain text file to not be detected as binary")
	}
}

func TestIsBinaryFile_NonexistentPath(t *testing.T) {
	if IsBinaryFile("/does/not/exist") {
		t.Error("expected nonexistent path to never be reported as binary")
	}
}

func TestBackupPath_HasExpectedShape(t *testing.T) {
	got := backupPath("/tmp/foo.txt")
	if len(got) <= len("/tmp/foo.txt.bak.") {
		t.Fatalf("expected backup path to have a timestamp suffix, got %q", got)
	}
}
