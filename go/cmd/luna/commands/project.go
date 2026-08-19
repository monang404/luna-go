package commands

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func newProjectCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "project <name> <description...>",
		Aliases: []string{"aiproject"},
		Short:   "Generate project multi-file dari nol (legacy: aiproject)",
		Args:    cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Code.Project(cmd.Context(), args[0], strings.Join(args[1:], " "))
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "project dir: %s\nlogfile: %s\nfiles: %s\n", res.ProjectDir, res.Logfile, strings.Join(res.Report.Files, ", "))
			return nil
		},
	}
}

func newBuildCmd(app *App) *cobra.Command {
	var outputName string
	cmd := &cobra.Command{
		Use:     "build <app-description...>",
		Aliases: []string{"aibuild"},
		Short:   "Mirip project, alur lebih terpandu (legacy: aibuild)",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Workflow.Build(cmd.Context(), outputName, strings.Join(args, " "), app.Code)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "project: %s\nspec file: %s\nproject dir: %s\n", res.ProjectName, res.SpecFile, res.Project.ProjectDir)
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputName, "output", "o", "", "nama folder project (default: slug dari deskripsi)")
	return cmd
}

// errNoGoPort is returned by commands whose underlying zsh logic was
// never ported to any internal/ package by SESSION-40..54 -- flagged
// per this repository's own convention (docs/MIGRATION_TRACEABILITY.md
// "Flagged rather than fabricated") rather than reimplemented from
// scratch inside the CLI layer, which SESSION-55's own YAML scope
// (wiring existing packages, "Belum ada instalasi/distribusi... fokus
// murni ke wiring") does not license.
var errNoGoPort = errors.New("luna: this command's zsh source was never ported to internal/ by SESSION-40..54 -- see docs/MIGRATION_TRACEABILITY.md")

func newScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "scan",
		Aliases: []string{"aiscan"},
		Short:   "Scan ulang ringkasan project (NOT PORTED -- see --help)",
		Long: "aiscan's logic lives in 45-project.zsh, which is not assigned to any\n" +
			"SESSION-40..54 in docs/execution_sessions/ -- no internal/ package\n" +
			"implements it yet. This subcommand is registered (for AC-01/AC-02\n" +
			"parity and future wiring) but returns an error until a future session\n" +
			"ports 45-project.zsh.",
		RunE: func(cmd *cobra.Command, args []string) error { return errNoGoPort },
	}
}

func newIndexCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "index",
		Aliases: []string{"aiindex"},
		Short:   "Bikin/lihat index codebase (NOT PORTED -- see --help)",
		Long: "aiindex's logic lives in 46-index.zsh, which is not assigned to any\n" +
			"SESSION-40..54 in docs/execution_sessions/ -- no internal/ package\n" +
			"implements it yet (internal/tools' GrepSearchTool/GlobSearchTool\n" +
			"already fall back correctly when no index exists, per SESSION-47's\n" +
			"own file-level note). This subcommand is registered but returns an\n" +
			"error until a future session ports 46-index.zsh.",
		RunE: func(cmd *cobra.Command, args []string) error { return errNoGoPort },
	}
}
