package tools

import (
	"bytes"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// This file ports 30-luna/35-files/00-guards.zsh's two guard predicates
// (_ai_is_secret_file, _ai_is_binary_file) plus 30-luna/10-core/25-quick_chat.zsh's
// _ai_ts timestamp helper. Every read/write/patch/delete tool in this
// session calls IsSecretFile before touching file content (and
// read_file additionally calls IsBinaryFile); both guards are shared
// verbatim rather than re-implemented per tool, mirroring the zsh
// source's own single-definition-many-callers shape.

// secretExactNames mirrors the zsh case arms that match a whole
// (lowercased) basename exactly: id_rsa|id_dsa|id_ecdsa|id_ed25519,
// .npmrc|.pgpass|.netrc|.git-credentials, credentials.json.
var secretExactNames = map[string]bool{
	"id_rsa": true, "id_dsa": true, "id_ecdsa": true, "id_ed25519": true,
	".npmrc": true, ".pgpass": true, ".netrc": true, ".git-credentials": true,
	"credentials.json": true,
}

// secretContainsSubstrings mirrors the `*secret*|*credential*|*token*`
// glob arms -- a substring match anywhere in the lowercased basename.
var secretContainsSubstrings = []string{"secret", "credential", "token"}

// secretSuffixes mirrors the `*.pem|*.key|*.ppk|*.pfx|*.p12|*.jks|*.keystore`
// glob arms -- a suffix match on the lowercased basename.
var secretSuffixes = []string{".pem", ".key", ".ppk", ".pfx", ".p12", ".jks", ".keystore"}

// privateKeyHeader mirrors the second-layer content check:
// `grep -qE 'BEGIN (RSA |EC |OPENSSH |DSA |ENCRYPTED )?PRIVATE KEY'`.
var privateKeyHeader = regexp.MustCompile(`BEGIN (RSA |EC |OPENSSH |DSA |ENCRYPTED )?PRIVATE KEY`)

// IsSecretFile mirrors _ai_is_secret_file: true if path's basename
// matches one of the known secret-file name patterns, OR (second layer,
// only if the file actually exists and is readable) its first 2000
// bytes contain a PEM private-key header. A nonexistent path can still
// be flagged by the name-pattern layer alone (e.g. write_file checking
// a not-yet-created destination), matching the zsh source's own
// `[ -f "$1" ] && ...` guard around the content-layer check only.
func IsSecretFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))

	// .env|.env.* -- exact "​.env" or a ".env." prefix.
	if base == ".env" || strings.HasPrefix(base, ".env.") {
		return true
	}
	if secretExactNames[base] {
		return true
	}
	// client_secret*.json -- prefix+suffix combination.
	if strings.HasPrefix(base, "client_secret") && strings.HasSuffix(base, ".json") {
		return true
	}
	for _, s := range secretContainsSubstrings {
		if strings.Contains(base, s) {
			return true
		}
	}
	for _, suf := range secretSuffixes {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}

	if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
		buf := make([]byte, 2000)
		f, err := os.Open(path)
		if err == nil {
			n, _ := f.Read(buf)
			f.Close()
			if privateKeyHeader.Match(buf[:n]) {
				return true
			}
		}
	}
	return false
}

// IsBinaryFile mirrors _ai_is_binary_file: prefers `file --mime-encoding
// -b` when available (returns true for a reported encoding of exactly
// "binary"), else falls back to a null-byte-in-the-first-8000-bytes
// heuristic (the zsh source's own fallback, ported literally: read up
// to 8000 bytes, strip NUL bytes, and if the stripped length differs
// from the raw length, a NUL was present). A nonexistent path is never
// binary (mirrors `[ -f "$1" ] || return 1`).
func IsBinaryFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}

	if fileBin, err := exec.LookPath("file"); err == nil {
		out, err := exec.Command(fileBin, "--mime-encoding", "-b", "--", path).Output()
		if err == nil {
			return strings.TrimSpace(string(out)) == "binary"
		}
		// `file` present but failed on this path -- fall through to the
		// NUL-byte heuristic rather than guessing, same as the zsh
		// source's own fallthrough on a non-zero exit.
	}

	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 8000)
	n, _ := f.Read(buf)
	if n == 0 {
		return false
	}
	return bytes.IndexByte(buf[:n], 0) >= 0
}

// timestampSuffix mirrors _ai_ts: `date +%Y%m%d_%H%M%S` plus a random
// 4-hex-digit suffix (zsh's `$RANDOM` is 0-32767; %04x here covers the
// same "short, human-glanceable, collision-unlikely-within-one-second"
// intent rather than reproducing zsh's exact RNG range byte-for-byte).
func timestampSuffix() string {
	return fmt.Sprintf("%s_%04x", time.Now().Format("20060102_150405"), rand.Intn(0x10000))
}

// backupPath mirrors the `$fs_path.bak.$(_ai_ts)` pattern used
// identically by edit_file, patch_file, and delete_file.
func backupPath(path string) string {
	return path + ".bak." + timestampSuffix()
}

// firstNChars mirrors _ai_head_c (00-core/compat.zsh): the first n
// *bytes* of s, not runes -- matching `head -c N`'s own byte-oriented
// truncation (the zsh source's primary fallback path). SESSION-48's
// git_diff/web_fetch/exec_process/run_test/run_command all cap their
// output through this instead of the line-oriented firstNLines above,
// exactly like the zsh source uses `_ai_head_c` (byte cap) rather than
// `_ai_head_n` (line cap) for those five call sites.
func firstNChars(s string, n int) string {
	if n <= 0 || len(s) <= n {
		return s
	}
	return s[:n]
}
