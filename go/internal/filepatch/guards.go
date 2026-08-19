package filepatch

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Ported from 30-luna/35-files/00-guards.zsh.

var secretFileNames = map[string]bool{
	"id_rsa": true, "id_dsa": true, "id_ecdsa": true, "id_ed25519": true,
	".npmrc": true, ".pgpass": true, ".netrc": true, ".git-credentials": true,
	"credentials.json": true,
}

var secretFileSuffixes = []string{".pem", ".key", ".ppk", ".pfx", ".p12", ".jks", ".keystore"}

var secretFileSubstrings = []string{"secret", "credential", "token"}

var privateKeyHeaderRE = regexp.MustCompile(`BEGIN (RSA |EC |OPENSSH |DSA |ENCRYPTED )?PRIVATE KEY`)

// IsSecretFile mirrors _ai_is_secret_file: a name-pattern check (env
// files, key/credential/token-ish names, well-known credential
// filenames) plus a content-based second layer (a leading PEM private
// key header, regardless of filename) so a secret isn't missed just
// because its name doesn't match any known pattern.
func IsSecretFile(path string) bool {
	name := strings.ToLower(filepath.Base(path))

	if name == ".env" || strings.HasPrefix(name, ".env.") {
		return true
	}
	if secretFileNames[name] {
		return true
	}
	for _, sub := range secretFileSubstrings {
		if strings.Contains(name, sub) {
			return true
		}
	}
	for _, suf := range secretFileSuffixes {
		if strings.HasSuffix(name, suf) {
			return true
		}
	}
	if strings.HasPrefix(name, "client_secret") && strings.HasSuffix(name, ".json") {
		return true
	}

	// Second layer: content, not just name -- a leading PEM private-key
	// header regardless of what the file happens to be called.
	if head, err := readHead(path, 2000); err == nil {
		if privateKeyHeaderRE.Match(head) {
			return true
		}
	}
	return false
}

// readHead reads up to n bytes from the start of path.
func readHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, n)
	read, err := f.Read(buf)
	if err != nil && read == 0 {
		return nil, err
	}
	return buf[:read], nil
}

// IsBinaryFile mirrors _ai_is_binary_file's fallback path (no `file`
// command dependency in Go): a NUL byte anywhere in the first 8000 bytes
// is treated as a strong signal of binary content, matching the zsh
// source's own fallback heuristic (raw byte count vs. NUL-stripped byte
// count differing). Returns false (not binary, matching the zsh
// source's `[ -f "$1" ] || return 1`) if the file doesn't exist.
func IsBinaryFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	head, err := readHead(path, 8000)
	if err != nil {
		return false
	}
	return bytes.IndexByte(head, 0) >= 0
}
