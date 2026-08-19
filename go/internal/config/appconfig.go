package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// LoadConfigYAML is a minimal, dependency-free reader for ~/.luna/config.yaml
// (DefaultConfigYAMLPath). It supports flat `KEY: value` mappings only --
// no nesting, no lists, no multi-line scalars. That's a deliberate scope
// cut, same rationale as LoadSecrets's zsh-subset parser: a config file in
// practice is just a handful of "PROVIDER_API_KEY: value" lines, and a
// full YAML parser is deferred until/unless something actually needs one.
//
// Keys are written as-is into the process environment via os.Setenv, so
// they should match a provider's KeyVar exactly (see Providers in
// providers.go) -- e.g. GROQ_API_KEY, not groq_api_key or groq.api_key.
// Comment lines (#) and blank lines are skipped. Values may be bare,
// single-quoted, or double-quoted (quotes are stripped, matching
// LoadSecrets's unquote).
//
// Unlike LoadSecrets, a key already present in the environment is left
// untouched: config.yaml is a fallback, not an override. This matches the
// precedence every similar tool uses (Hermes Agent's ~/.hermes/.env,
// OpenClaw's ~/.openclaw/openclaw.json) -- a real env var the user already
// exported always wins over whatever sits in the file.
//
// If path doesn't exist, LoadConfigYAML returns (0, nil, nil), same
// no-op-on-missing-file contract as LoadSecrets.
func LoadConfigYAML(path string) (set int, warnings []string, err error) {
	f, openErr := os.Open(path)
	if openErr != nil {
		if os.IsNotExist(openErr) {
			return 0, nil, nil
		}
		return 0, nil, openErr
	}
	defer f.Close()

	// Permission check only makes sense on POSIX-style permission bits.
	// Windows' filesystem reports permissions through ACLs, not rwx bits,
	// so os.FileMode.Perm() there is not meaningful and a chmod-flavored
	// warning would fire even on a perfectly fine file -- skip the check
	// there entirely instead of emitting a warning nobody can act on.
	if runtime.GOOS != "windows" {
		if info, statErr := f.Stat(); statErr == nil {
			if perm := info.Mode().Perm(); perm != 0o600 && perm != 0o400 {
				warnings = append(warnings, fmt.Sprintf(
					"%s permission %04o (bukan 600), isinya API key. Jalanin: chmod 600 %s",
					path, perm, path))
			}
		}
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		colon := strings.IndexByte(line, ':')
		if colon <= 0 {
			continue // no ":", or line starts with ":" -- not a valid mapping entry
		}
		key := strings.TrimSpace(line[:colon])
		val := unquote(strings.TrimSpace(line[colon+1:]))
		if key == "" || val == "" {
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

// DefaultConfigDir returns $HOME/.luna -- the per-user state directory,
// following the same convention as Hermes Agent's ~/.hermes/ and
// OpenClaw's ~/.openclaw/: one dotfile folder per tool instead of loose
// dotfiles directly in $HOME. On Windows this resolves under
// %USERPROFILE% (os.UserHomeDir() and filepath.Join are both OS-aware),
// e.g. C:\Users\<user>\.luna.
func DefaultConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".luna")
}

// DefaultConfigYAMLPath returns $HOME/.luna/config.yaml.
func DefaultConfigYAMLPath() string {
	return filepath.Join(DefaultConfigDir(), "config.yaml")
}
