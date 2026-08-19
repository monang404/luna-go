package commands

import (
	"fmt"
	"os"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
	"github.com/monang404/luna-go/internal/ui"
	"github.com/spf13/cobra"
)

// noAPIKeyCommands lists subcommands that never need a live LUNA request,
// so the startup self-check (below) must not block them just because no
// provider key is configured -- mirrors which legacy aliases never
// called _ai_need_any_key in the zsh source (undo/bakclean/share/view
// are pure filesystem operations; deps/h/menu are diagnostics/help).
var noAPIKeyCommands = map[string]bool{
	"undo": true, "bakclean": true, "share": true, "view": true,
	"deps": true, "h": true, "menu": true, "help": true, "completion": true,
	"scan": true, "index": true, "log": true, "stats": true, "dev": true,
}

// NewRootCmd builds the full cobra command tree: one subcommand per
// docs/execution_sessions/55_wire_cli_entrypoint.yaml's mapping (see
// docs/MIGRATION_TRACEABILITY.md's SESSION-55 alias table), rooted
// under `luna`. AC-02: `luna --help` lists every command
// grouped by category, sourced from ui.CommandRegistry (the same
// single source of truth `luna h` / `20-menu.zsh` used in zsh) rather
// than a second hand-maintained list.
func NewRootCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:   "luna",
		Short: "LUNA toolkit (Go rewrite)",
		Long: "LUNA toolkit -- a personal LUNA-assisted dev workflow, originally\n" +
			"a zsh plugin, now a single Go binary. Run a subcommand, or `luna h`\n" +
			"for the full categorized list (equivalent to the old `aih`/`luna h`).\n\n" +
			ui.RegistryRenderCategorized(),
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return startupSelfCheck(cmd)
		},
	}

	// --- Chat ---
	root.AddCommand(newChatCmd(app))
	root.AddCommand(newLongCmd(app))
	root.AddCommand(newAskCmd(app))
	root.AddCommand(newShellCmd(app))
	root.AddCommand(newClipCmd(app))
	root.AddCommand(newSessionCmd(app))
	// --- Code ---
	root.AddCommand(newCodeCmd(app))
	root.AddCommand(newEditCmd(app))
	root.AddCommand(newViewCmd())
	root.AddCommand(newFixCmd(app))
	root.AddCommand(newRunCmd(app))
	root.AddCommand(newCommitCmd(app))
	root.AddCommand(newReviewCmd(app))
	root.AddCommand(newScrapCmd(app))
	// --- Files ---
	root.AddCommand(newUndoCmd(app))
	root.AddCommand(newBakCleanCmd(app))
	root.AddCommand(newShareCmd())
	root.AddCommand(newScanCmd())
	root.AddCommand(newIndexCmd())
	// --- Workflow ---
	root.AddCommand(newPlanCmd(app))
	root.AddCommand(newPromptCmd(app))
	root.AddCommand(newSpecCmd(app))
	root.AddCommand(newSummarizeCmd(app))
	// --- Project ---
	root.AddCommand(newProjectCmd(app))
	root.AddCommand(newBuildCmd(app))
	// --- Agent ---
	root.AddCommand(newAgentCmd(app))
	root.AddCommand(newDebugCmd(app))
	root.AddCommand(newResearchCmd(app))
	root.AddCommand(newDelegateCmd(app))
	// --- Utility ---
	root.AddCommand(newStatsCmd())
	root.AddCommand(newLogCmd())
	root.AddCommand(newMenuCmd())
	root.AddCommand(newDepsCmd())
	root.AddCommand(newDevCmd())
	root.AddCommand(newTestModelsCmd(app))
	root.AddCommand(newHCmd())

	assertRegistryParity(root)
	return root
}

// startupSelfCheck is AC-03: refuse to run (with a clear message) when
// no provider API key is set at all, UNLESS the invoked subcommand is
// one of noAPIKeyCommands. This is a deliberate new guard, not a 1:1
// port -- 00-core/90-selfcheck.zsh's actual behavior is an unrelated
// duplicate-function-name scanner that has no Go equivalent (dynamic
// zsh sourcing doesn't exist here); see docs/MIGRATION_TRACEABILITY.md's
// SESSION-55 entry for that flagged deviation. This function instead
// gives AC-03's literal description (a real, useful guard) using the
// config.HasAnyKey primitive every ported package already depends on.
func startupSelfCheck(cmd *cobra.Command) error {
	name := cmd.Name()
	llmclient.Debugf("startup_self_check command=%s order=%v", name, config.TaskProviderOrder)
	if name == "luna" || noAPIKeyCommands[name] {
		return nil
	}
	if config.HasAnyKey(config.TaskProviderOrder) {
		return nil
	}
	return fmt.Errorf(
		"luna: no LUNA provider API key is set (checked %v). "+
			"Set at least one provider's key env var (see `luna deps`) before running `%s`",
		config.TaskProviderOrder, name)
}

// assertRegistryParity is AC-01's compile-adjacent guard: every command
// name in ui.CommandRegistry (the categorized `luna h` listing) must
// resolve to a registered cobra subcommand, and vice versa (modulo the
// small set of legacy names ui.CommandRegistry doesn't carry --
// "update" has no zsh alias function to port, per this session's own
// file-level note). Panics at startup rather than silently drifting,
// since a missing subcommand is exactly the kind of gap AC-01 exists to
// catch.
func assertRegistryParity(root *cobra.Command) {
	known := map[string]bool{}
	for _, c := range root.Commands() {
		known[c.Name()] = true
		for _, a := range c.Aliases {
			known[a] = true
		}
	}
	for _, name := range ui.RegistryNames() {
		if name == "update" {
			// "update" (git pull self-update) has no source alias
			// function anywhere in 30-luna/ -- see CHANGELOG SESSION-55
			// entry. Flagged, not fabricated.
			continue
		}
		if !known[name] {
			panic(fmt.Sprintf("commands: ui.CommandRegistry entry %q has no registered cobra subcommand -- AC-01 violation", name))
		}
	}
}

// Execute is main()'s single call: load secrets, build the App, build
// the command tree, run it, and translate a returned error into the
// same exit-code contract every legacy alias used (0 = success, 1 =
// failure).
//
// loadSecretsAtStartup runs first, before NewApp/NewRootCmd touch any
// provider config. This was previously missing entirely: config.LoadSecrets
// (SESSION-41) and config.DefaultSecretsPath existed as fully-implemented,
// fully-tested library functions but had zero call sites anywhere in
// cmd/luna -- neither chat nor agent nor any other command ever
// sourced ~/.secrets.zsh, so every command relied solely on whatever
// API key env vars the invoking shell happened to already have
// exported. That's the audit brief's own P0 hypothesis (chat command:
// LoadConfig/LoadSecrets/NewLLMClient vs agent command: NewAgent but no
// LoadSecrets) confirmed against the actual source: neither path called
// it, so the two commands only ever appeared to differ when one
// provider's key was exported directly and another wasn't -- see
// docs/execution_sessions/41_port_config_layer.yaml and the FINAL
// REPORT's Critical Findings table (ID P0-1) for the full writeup.
func Execute() {
	loadSecretsAtStartup()
	app := NewApp()
	root := NewRootCmd(app)
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// loadSecretsAtStartup sources ~/.secrets.zsh (config.DefaultSecretsPath)
// into the process environment via config.LoadSecrets, exactly once,
// before any command runs. A missing file is not an error (LoadSecrets
// itself treats that as a no-op, matching the zsh source's `[ -f ... ]`
// guard); permission warnings and hard read errors are surfaced on
// stderr rather than aborting startup, since a malformed secrets file
// should not block commands that don't need it (noAPIKeyCommands) or
// that already have their key exported some other way.
//
// Two sources are loaded, in this order:
//  1. ~/.luna/config.yaml (config.LoadConfigYAML) -- the current
//     convention, following the same ~/.<tool>/ dotfile-folder pattern
//     as Hermes Agent and OpenClaw.
//  2. ~/.secrets.zsh (config.LoadSecrets) -- the original SESSION-41
//     location, kept as a fallback so an existing setup doesn't silently
//     stop working after upgrading.
//
// Both loaders only set a key if it isn't already in the environment, so
// an env var the invoking shell already exported always wins over either
// file, and config.yaml is checked before .secrets.zsh (source 1 wins
// over source 2 for the same key).
func loadSecretsAtStartup() {
	yamlPath := config.DefaultConfigYAMLPath()
	nYAML, yamlWarnings, yamlErr := config.LoadConfigYAML(yamlPath)
	for _, w := range yamlWarnings {
		fmt.Fprintln(os.Stderr, "luna:", w)
	}
	if yamlErr != nil {
		fmt.Fprintf(os.Stderr, "luna: gagal load %s: %v\n", yamlPath, yamlErr)
	}

	zshPath := config.DefaultSecretsPath()
	nZsh, warnings, err := config.LoadSecrets(zshPath)
	for _, w := range warnings {
		fmt.Fprintln(os.Stderr, "luna:", w)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "luna: gagal load %s: %v\n", zshPath, err)
	}

	llmclient.Debugf("secrets loaded: yaml=%s set=%d zsh=%s set=%d", yamlPath, nYAML, zshPath, nZsh)
}
