package commands

import (
	"fmt"
	"os/exec"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/ui"
	"github.com/spf13/cobra"
)

// depsChecklist mirrors ai_check_deps's tool list (15-diagnostics.zsh).
// A best-effort port to the CLI layer -- this session's scope, unlike
// the request/permission/tool packages, since it is pure environment
// introspection with no internal/ package of its own to call into.
var depsChecklist = []string{"gum", "jq", "fzf", "fd", "bat", "curl", "tmux", "timeout"}

func newDepsCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "deps",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"ai_check_deps"},
		Short:      "Cek semua dependency & konfigurasi (legacy: ai_check_deps)",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintln(out, "Cek dependency LUNA environment:")
			for _, d := range depsChecklist {
				if _, err := exec.LookPath(d); err == nil {
					fmt.Fprintf(out, "  OK %s\n", d)
				} else {
					fmt.Fprintf(out, "  MISSING %s   -> pkg install %s\n", d, d)
				}
			}
			fmt.Fprintln(out, "")
			fmt.Fprintln(out, "Provider API key:")
			for name, p := range config.Providers() {
				status := "MISSING"
				if len(config.ActiveProviders([]string{name})) > 0 {
					status = "OK"
				}
				fmt.Fprintf(out, "  %-10s %-8s (env %s)\n", name, status, p.KeyVar)
			}
			return nil
		},
	}
}

func newTestModelsCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:        "testmodels",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"ai_testmodels"},
		Short:      "Test konektivitas ke semua provider (legacy: ai_testmodels)",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			providers := config.Providers()
			ctx := cmd.Context()
			for name := range providers {
				if len(config.ActiveProviders([]string{name})) == 0 {
					fmt.Fprintf(out, "  SKIP %s (no API key)\n", name)
					continue
				}
				res, err := app.Requester.Complete(ctx, "", "ping", config.TaskFast, []string{name}, 0)
				if err != nil {
					fmt.Fprintf(out, "  FAIL %s: %v\n", name, err)
					continue
				}
				fmt.Fprintf(out, "  OK   %s (model: %s)\n", name, res.Model)
			}
			return nil
		},
	}
}

func newHCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "h",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aih"},
		Short:      "Bantuan ringkas semua subcommand (legacy: aih)",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprint(cmd.OutOrStdout(), ui.RegistryRenderCategorized())
			return nil
		},
	}
}

func newMenuCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "menu",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Short:      "Buka LUNA Workspace (sama seperti luna tanpa argumen)",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Root().Help()
		},
	}
}

var errNotImplementedLog = fmt.Errorf(
	"luna: aihist/aistats' logic lives in 60-ui/10-help_stats.zsh, " +
		"which is not assigned to any SESSION-40..54 -- no internal/ package " +
		"reads AI_HISTORY_LOG/AI_USAGE_LOG's jsonl format yet. See " +
		"docs/MIGRATION_TRACEABILITY.md")

func newLogCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "log",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aihist"},
		Short:      "History chat/perintah lewat fzf (NOT PORTED -- see --help)",
		Long:       errNotImplementedLog.Error(),
		RunE:       func(cmd *cobra.Command, args []string) error { return errNotImplementedLog },
	}
}

func newStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "stats",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aistats"},
		Short:      "Statistik pemakaian token (NOT PORTED -- see --help)",
		Long:       errNotImplementedLog.Error(),
		RunE:       func(cmd *cobra.Command, args []string) error { return errNotImplementedLog },
	}
}

func newDevCmd() *cobra.Command {
	return &cobra.Command{
		Use:        "dev",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aidev"},
		Short:      "Tools development toolkit ini sendiri, workspace tmux (NOT PORTED -- see --help)",
		Long: "aidev's logic lives in 60-ui/25-research_dev.zsh and launches a tmux\n" +
			"workspace -- an interactive-terminal-session concern outside any\n" +
			"internal/ package's scope (and outside a single CLI subcommand's\n" +
			"natural boundaries). Not wired in SESSION-55.",
		RunE: func(cmd *cobra.Command, args []string) error { return errNoGoPort },
	}
}
