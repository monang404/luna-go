package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/monang404/luna-go/internal/agent"
	"github.com/monang404/luna-go/internal/subagent"
	"github.com/spf13/cobra"
)

func newAgentCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "agent <goal...>",
		Aliases: []string{"aiagent"},
		Short:   "Agent full akses: baca/tulis file, jalankan command, looping sendiri (legacy: aiagent)",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			goal := strings.Join(args, " ")
			logf := func(s string) { fmt.Fprintln(cmd.ErrOrStderr(), s) }
			deps := app.agentDeps(cwd, "", logf)
			result, err := agent.RunLoop(cmd.Context(), deps, goal, nil)
			if err != nil {
				return err
			}
			printAgentResult(cmd, result)
			return nil
		},
	}
}

func newDebugCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "debug <problem...>",
		Aliases: []string{"aidebug"},
		Short:   "Diagnosis + test/command, read-only, gak ada auto-fix (legacy: aidebug)",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			deps := app.subagentDeps(cwd)
			report, err := subagent.RunDebug(cmd.Context(), deps, strings.Join(args, " "))
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Diagnosis: %s\n", report.Diagnosis)
			if len(report.AffectedFiles) > 0 {
				fmt.Fprintf(out, "Affected files: %s\n", strings.Join(report.AffectedFiles, ", "))
			}
			if report.Error != "" {
				fmt.Fprintf(out, "Error: %s\n", report.Error)
			}
			fmt.Fprintf(out, "Success: %v\n", report.Success)
			return nil
		},
	}
}

func newResearchCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "research <goal...>",
		Aliases: []string{"airesearch"},
		Short:   "Riset/inspeksi codebase standalone, read-only (legacy: airesearch)",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStandaloneSubagent(app, cmd, subagent.RoleResearcher, strings.Join(args, " "))
		},
	}
}

func newDelegateCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:     "delegate <goal...>",
		Aliases: []string{"aidelegate"},
		Short:   "Standalone coder subagent, permission existing dapat menulis file (legacy: aidelegate)",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runStandaloneSubagent(app, cmd, subagent.RoleCoder, strings.Join(args, " "))
		},
	}
}

func runStandaloneSubagent(app *App, cmd *cobra.Command, role subagent.Role, goal string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	deps := app.subagentDeps(cwd)
	result, err := subagent.SpawnSubagent(cmd.Context(), deps, role, goal)
	if err != nil {
		return err
	}
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "status: %s\nsummary: %s\n", result.Status, result.Summary)
	if result.Findings != "" {
		fmt.Fprintf(out, "findings: %s\n", result.Findings)
	}
	if result.Changes != "" {
		fmt.Fprintf(out, "changes: %s\n", result.Changes)
	}
	if len(result.FilesAffected) > 0 {
		fmt.Fprintf(out, "files affected: %s\n", strings.Join(result.FilesAffected, ", "))
	}
	if result.Error != "" {
		fmt.Fprintf(out, "error: %s\n", result.Error)
	}
	return nil
}

func printAgentResult(cmd *cobra.Command, res agent.FinalResult) {
	out := cmd.OutOrStdout()
	fmt.Fprintf(out, "phase: %s\nsteps: %d\ndone: %v\n", res.Phase, res.Steps, res.Done)
	if res.BlockReason != "" {
		fmt.Fprintf(out, "block reason: %s\n", res.BlockReason)
	}
	if len(res.ChangedFiles) > 0 {
		fmt.Fprintf(out, "changed files: %s\n", strings.Join(res.ChangedFiles, ", "))
	}
}
