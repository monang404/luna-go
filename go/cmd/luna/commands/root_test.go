package commands

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/monang404/luna-go/internal/ui"
)

// clearProviderKeys unsets every provider key env var for the duration
// of a test, restoring the original values on cleanup -- so
// startupSelfCheck tests don't depend on (or pollute) whatever the CI
// environment happens to have set.
func clearProviderKeys(t *testing.T) {
	t.Helper()
	keys := []string{"GROQ_API_KEY", "GEMINI_API_KEY", "CEREBRAS_API_KEY", "DEEPSEEK_API_KEY"}
	saved := map[string]string{}
	for _, k := range keys {
		saved[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	t.Cleanup(func() {
		for k, v := range saved {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	})
}

func newTestRoot(t *testing.T) *App {
	t.Helper()
	// NewApp is side-effect free at construction time (Requester only
	// touches the network on Complete, dispatcher registration is pure
	// in-memory bookkeeping) -- safe to build fresh per test.
	return NewApp()
}

// TestRegistryParity is AC-01 as an executable test: every
// ui.CommandRegistry entry (the categorized `luna h` listing, ported
// verbatim from 20-menu.zsh) must resolve to a real cobra subcommand or
// alias, with "update" as the one documented exception (no source alias
// function exists to port).
func TestRegistryParity(t *testing.T) {
	app := newTestRoot(t)
	root := NewRootCmd(app) // panics on mismatch -- see assertRegistryParity

	known := map[string]bool{}
	for _, c := range root.Commands() {
		known[c.Name()] = true
		for _, a := range c.Aliases {
			known[a] = true
		}
	}
	for _, name := range ui.RegistryNames() {
		if name == "update" {
			continue
		}
		if !known[name] {
			t.Errorf("registry command %q has no cobra subcommand", name)
		}
	}
}

// TestLegacyAliasesWired checks the other direction of AC-01: every
// legacy alias name this session's YAML explicitly calls out
// (aiask, aiagent, aipatch, aicommit, aiundo, ...) is reachable as a
// cobra alias, not just its new canonical name.
func TestLegacyAliasesWired(t *testing.T) {
	app := newTestRoot(t)
	root := NewRootCmd(app)

	known := map[string]bool{}
	for _, c := range root.Commands() {
		for _, a := range c.Aliases {
			known[a] = true
		}
	}

	legacy := []string{
		"aiask", "aiagent", "aipatch", "aicommit", "aiundo",
		"aic", "aicl", "aish", "aiclip", "aicode", "aicat", "aifix",
		"airun", "aiscrap", "aireview", "aibakclean", "aishare",
		"aiscan", "aiindex", "aiplan", "aiprompt", "aispec",
		"aisummarize", "aiproject", "aibuild", "aidebug", "airesearch",
		"aidelegate", "aistats", "aihist", "aidev", "ai_check_deps",
		"ai_testmodels", "aih",
	}
	for _, alias := range legacy {
		if !known[alias] {
			t.Errorf("legacy alias %q has no cobra alias in the new tree", alias)
		}
	}
}

func TestHelpListsCategories(t *testing.T) {
	app := newTestRoot(t)
	root := NewRootCmd(app)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetArgs([]string{"--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--help returned error: %v", err)
	}
	out := buf.String()
	for _, category := range []string{"Chat:", "Code:", "Files:", "Workflow:", "Project:", "Agent:", "Utility:"} {
		if !strings.Contains(out, category) {
			t.Errorf("--help output missing category %q\noutput:\n%s", category, out)
		}
	}
}

func TestStartupSelfCheckBlocksWithoutKey(t *testing.T) {
	clearProviderKeys(t)
	app := newTestRoot(t)
	root := NewRootCmd(app)
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	root.SetArgs([]string{"ask", "hello"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected startupSelfCheck to block `ask` with no provider key set, got nil error")
	}
	if !strings.Contains(err.Error(), "no LUNA provider API key is set") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestStartupSelfCheckAllowsNoKeyCommands(t *testing.T) {
	clearProviderKeys(t)
	app := newTestRoot(t)

	for name := range noAPIKeyCommands {
		if name == "help" || name == "completion" {
			continue // cobra built-ins, not our subcommands
		}
		root := NewRootCmd(app)
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{name, "--help"})
		if err := root.Execute(); err != nil {
			t.Errorf("command %q should not be blocked by the API-key self-check, got: %v", name, err)
		}
	}
}

func TestUndoReachesRealFilepatchLogic(t *testing.T) {
	// Regression guard for the wiring itself: `undo` on a file with no
	// backups must fail with filepatch's own "no backups found" error,
	// not a nil-pointer panic or a self-check block -- proof that
	// newUndoCmd is actually calling app.Patch.Undo, not a stub.
	clearProviderKeys(t)
	dir := t.TempDir()
	f := dir + "/sample.txt"
	if err := os.WriteFile(f, []byte("hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := newTestRoot(t)
	root := NewRootCmd(app)
	buf := &bytes.Buffer{}
	root.SetOut(buf)
	root.SetErr(buf)
	root.SetArgs([]string{"undo", f})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error (no backups exist for a fresh file)")
	}
	if !strings.Contains(err.Error(), "no backups found") {
		t.Errorf("expected filepatch's own error, got: %v", err)
	}
}
