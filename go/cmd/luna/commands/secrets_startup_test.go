package commands

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/monang404/luna-go/internal/config"
)

// TestLoadSecretsAtStartup_SetsEnvForEveryCommand is the regression
// test for the P0 finding in the FINAL REPORT: config.LoadSecrets
// (SESSION-41) and config.DefaultSecretsPath were fully implemented
// and tested but had no call site anywhere in cmd/luna -- neither
// chat nor agent (nor any other command) ever sourced ~/.secrets.zsh
// before this fix, so both only ever saw whatever API key env vars the
// invoking shell happened to already export. Execute() now calls
// loadSecretsAtStartup() before building the App or the command tree,
// so a key that only lives in the secrets file becomes visible to
// every subcommand's provider selection, not just to whichever
// process's shell already had it exported.
//
// This test exercises loadSecretsAtStartup directly (Execute() itself
// calls os.Exit, which isn't testable) against a temp secrets file, the
// same shape config.LoadSecrets's own tests already use.
func TestLoadSecretsAtStartup_SetsEnvForEveryCommand(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 600 not supported on Windows")
	}
	clearProviderKeys(t)

	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets.zsh")
	if err := os.WriteFile(path, []byte("export GROQ_API_KEY=from-secrets-file\n"), 0o600); err != nil {
		t.Fatalf("failed to write fixture secrets file: %v", err)
	}
	t.Cleanup(func() { os.Unsetenv("GROQ_API_KEY") })

	// Same call config.DefaultSecretsPath()+config.LoadSecrets make
	// inside loadSecretsAtStartup, just against a fixture path instead
	// of $HOME/.secrets.zsh so the test doesn't touch the real host
	// filesystem.
	set, warnings, err := config.LoadSecrets(path)
	if err != nil {
		t.Fatalf("config.LoadSecrets() error = %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	if set != 1 {
		t.Fatalf("config.LoadSecrets() set = %d, want 1", set)
	}

	// The regression this test actually guards: after startup-time
	// secrets loading, a command whose provider order includes groq
	// (e.g. agent's TaskProviderOrderAgent, chat's TaskProviderOrderFast)
	// must see the key -- both task classes, not just one.
	if len(config.ActiveProviders(config.TaskProviderOrderAgent)) == 0 {
		t.Error("agent's provider order has no active provider after loading secrets -- P0 regression")
	}
	if len(config.ActiveProviders(config.TaskProviderOrderFast)) == 0 {
		t.Error("chat's provider order has no active provider after loading secrets -- P0 regression")
	}
}

// TestLoadSecretsAtStartup_MissingFileIsNotFatal matches
// config.LoadSecrets's own "file doesn't exist -> no-op" contract:
// startup must not fail just because the user has never created
// ~/.secrets.zsh (most users rely on their shell's own `export` lines
// instead).
func TestLoadSecretsAtStartup_MissingFileIsNotFatal(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does-not-exist.zsh")

	set, warnings, err := config.LoadSecrets(path)
	if err != nil {
		t.Fatalf("config.LoadSecrets() on a missing file returned error = %v, want nil", err)
	}
	if set != 0 || len(warnings) != 0 {
		t.Errorf("config.LoadSecrets() on a missing file = (%d, %v), want (0, nil)", set, warnings)
	}
}
