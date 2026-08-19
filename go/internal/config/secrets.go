package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LoadSecrets is a minimal, dependency-free port of the "source
// ~/.secrets.zsh" step in 00-core/secrets-guard.zsh (00-core/ itself is
// not being ported -- see internal/env's doc.go -- but the parsing logic
// this session needs is small enough to own here rather than block on
// that).
//
// It parses lines of the form `export KEY=VALUE` or plain `KEY=VALUE`
// (VALUE optionally single- or double-quoted) from the given file and
// calls os.Setenv for each. Comment lines (#) and blank lines are skipped.
//
// This is intentionally NOT a shell interpreter: command substitution,
// variable expansion inside VALUE, multi-line values, and other shell
// syntax are not supported. That's a deliberate scope cut for SESSION-41
// (see the session's why_not_more) -- secrets files in practice are just a
// flat list of `export FOO=bar` lines, and a fuller shell-compatible
// parser is deferred until/unless something actually needs it.
//
// If path doesn't exist, LoadSecrets returns (0, nil, nil) -- matching
// zsh's `if [ -f "$HOME/.secrets.zsh" ]; then ... fi` silently skipping a
// missing file rather than erroring.
func LoadSecrets(path string) (set int, warnings []string, err error) {
	f, openErr := os.Open(path)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			return 0, nil, nil
		}
		return 0, nil, openErr
	}
	defer f.Close()

	// Permission check, matching secrets-guard.zsh's warning (600/400 OK,
	// anything looser gets flagged). Unlike the zsh version, this never
	// auto-chmod's -- that stays a caller/CLI decision, not something a
	// config-loading library function should do silently.
	if info, statErr := f.Stat(); statErr == nil {
		if perm := info.Mode().Perm(); perm != 0o600 && perm != 0o400 {
			warnings = append(warnings, fmt.Sprintf(
				"%s permission %04o (bukan 600), isinya API key. Jalanin: chmod 600 %s",
				path, perm, path))
		}
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))

		eq := strings.IndexByte(line, '=')
		if eq <= 0 {
			continue // no "=", or line starts with "=" -- not a valid assignment
		}
		key := strings.TrimSpace(line[:eq])
		val := unquote(strings.TrimSpace(line[eq+1:]))
		if key == "" {
			continue
		}

		if os.Getenv(key) != "" {
			continue // env var already set -- file is a fallback, never an override
		}
		if setErr := os.Setenv(key, val); setErr != nil {
			warnings = append(warnings, fmt.Sprintf("gagal set %s: %v", key, setErr))
			continue
		}
		set++
	}
	if scanErr := scanner.Err(); scanErr != nil {
		return set, warnings, scanErr
	}
	return set, warnings, nil
}

// DefaultSecretsPath returns $HOME/.secrets.zsh, matching
// secrets-guard.zsh's fixed location. Deprecated: new setups should use
// ~/.luna/config.yaml (DefaultConfigYAMLPath / LoadConfigYAML) instead --
// this remains only as a fallback so an existing ~/.secrets.zsh keeps
// working after upgrading.
func DefaultSecretsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".secrets.zsh")
}

// unquote strips a single matching pair of surrounding single or double
// quotes, if present.
func unquote(s string) string {
	if len(s) >= 2 {
		first, last := s[0], s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
