package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigYAML_ParsesFlatMapping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := "# comment, should be skipped\n" +
		"\n" +
		"GROQ_API_KEY: abc123\n" +
		"GEMINI_API_KEY: \"quoted value\"\n" +
		"CEREBRAS_API_KEY: 'single-quoted'\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GROQ_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("CEREBRAS_API_KEY", "")

	set, warnings, err := LoadConfigYAML(path)
	if err != nil {
		t.Fatalf("LoadConfigYAML() error = %v", err)
	}
	if set != 3 {
		t.Errorf("LoadConfigYAML() set = %d, want 3", set)
	}
	if len(warnings) != 0 {
		t.Errorf("LoadConfigYAML() warnings = %v, want none (file is 0600)", warnings)
	}

	if got := os.Getenv("GROQ_API_KEY"); got != "abc123" {
		t.Errorf("GROQ_API_KEY = %q, want abc123", got)
	}
	if got := os.Getenv("GEMINI_API_KEY"); got != "quoted value" {
		t.Errorf("GEMINI_API_KEY = %q, want %q", got, "quoted value")
	}
	if got := os.Getenv("CEREBRAS_API_KEY"); got != "single-quoted" {
		t.Errorf("CEREBRAS_API_KEY = %q, want single-quoted", got)
	}
}

func TestLoadConfigYAML_EnvVarWinsOverFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("GROQ_API_KEY: from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GROQ_API_KEY", "from-shell")

	set, _, err := LoadConfigYAML(path)
	if err != nil {
		t.Fatalf("LoadConfigYAML() error = %v", err)
	}
	if set != 0 {
		t.Errorf("LoadConfigYAML() set = %d, want 0 (env var already set, file should be skipped)", set)
	}
	if got := os.Getenv("GROQ_API_KEY"); got != "from-shell" {
		t.Errorf("GROQ_API_KEY = %q, want from-shell (pre-existing env var must win)", got)
	}
}

func TestLoadConfigYAML_MissingFileIsNotError(t *testing.T) {
	set, warnings, err := LoadConfigYAML(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if err != nil {
		t.Fatalf("LoadConfigYAML() on missing file error = %v, want nil", err)
	}
	if set != 0 || warnings != nil {
		t.Errorf("LoadConfigYAML() on missing file = (%d, %v), want (0, nil)", set, warnings)
	}
}

func TestDefaultConfigYAMLPath_UnderDotLuna(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skipf("no home dir available in this environment: %v", err)
	}
	want := filepath.Join(home, ".luna", "config.yaml")
	if got := DefaultConfigYAMLPath(); got != want {
		t.Errorf("DefaultConfigYAMLPath() = %q, want %q", got, want)
	}
}
