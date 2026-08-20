package commands

import (
	"fmt"
	"strings"

	"github.com/monang404/luna-go/internal/filepatch"
	"github.com/spf13/cobra"
)

func newCodeCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "code <output-name> <prompt...>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aicode"},
		Short:      "Generate file kode baru dari nol (legacy: aicode)",
		Args:       cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Code.Code(cmd.Context(), args[0], strings.Join(args[1:], " "))
			if err != nil {
				return err
			}
			printCodeResult(cmd, res.NoChange, res.Overwrote, res.BackupPath, res.Diff)
			return nil
		},
	}
	return cmd
}

func newEditCmd(app *App) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:        "edit <file> <instruction...>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aipatch"},
		Short:      "Edit file yang sudah ada lewat diff/confirm terpandu (legacy: aipatch)",
		Args:       cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Patch.Patch(cmd.Context(), args[0], strings.Join(args[1:], " "), force)
			if err != nil {
				return err
			}
			printCodeResult(cmd, res.NoChange, res.Applied, res.BackupPath, res.Diff)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "lewati guard ukuran/secret-file")
	return cmd
}

func newViewCmd() *cobra.Command {
	var start, end int
	cmd := &cobra.Command{
		Use:        "view <file>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aicat"},
		Short:      "Lihat isi file per-baris (legacy: aicat)",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := filepatch.Cat(args[0], start, end)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), out)
			return nil
		},
	}
	cmd.Flags().IntVar(&start, "start", 0, "baris awal (1-based, 0 = dari awal)")
	cmd.Flags().IntVar(&end, "end", 0, "baris akhir (0 = sampai akhir)")
	return cmd
}

func newFixCmd(app *App) *cobra.Command {
	var inspect bool
	cmd := &cobra.Command{
		Use:        "fix <file> <error-message...>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aifix"},
		Short:      "Perbaiki file dari pesan error (legacy: aifix)",
		Args:       cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Code.Fix(cmd.Context(), args[0], strings.Join(args[1:], " "), inspect)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "fixed file: %s\n", res.FixedPath)
			if !inspect {
				printCodeResult(cmd, false, res.Apply.Applied, res.Apply.BackupPath, res.Apply.Diff)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&inspect, "inspect", false, "hanya tulis <file>.fixed, jangan langsung apply")
	return cmd
}

func newRunCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:        "run <file.py>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"airun"},
		Short:      "Jalankan Python, auto-fix kalau error sampai 2x (legacy: airun)",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Code.Run(cmd.Context(), args[0])
			fmt.Fprintln(cmd.OutOrStdout(), res.Output)
			fmt.Fprintf(cmd.ErrOrStderr(), "exit=%d fix_attempts=%d success=%v\n", res.ExitCode, res.FixAttempts, res.Success)
			return err
		},
	}
}

func newScrapCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "scrap <url> <task...>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aiscrap"},
		Short:      "Scraping/riset cepat lalu rangkum (legacy: aiscrap)",
		Args:       cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			reply, err := app.Code.Scrap(cmd.Context(), args[0], strings.Join(args[1:], " "))
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), reply)
			return nil
		},
	}
	return cmd
}

func newCommitCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:        "commit",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aicommit"},
		Short:      "Generate pesan commit dari git diff staged (legacy: aicommit)",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Workflow.Commit(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Message)
			if res.Committed {
				fmt.Fprintln(cmd.ErrOrStderr(), "committed.")
			}
			return nil
		},
	}
}

func newReviewCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:        "review",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aireview"},
		Short:      "Review diff/perubahan terakhir, read-only (legacy: aireview)",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Workflow.Review(cmd.Context())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Review)
			return nil
		},
	}
}

func printCodeResult(cmd *cobra.Command, noChange, applied bool, backup, diff string) {
	out := cmd.OutOrStdout()
	switch {
	case noChange:
		fmt.Fprintln(out, "LUNA gak mengusulkan perubahan apa pun.")
	case applied:
		fmt.Fprintf(out, "Diterapkan. Backup: %s\n\n%s\n", backup, diff)
	default:
		fmt.Fprintf(cmd.ErrOrStderr(), "Perubahan tidak diterapkan.\n\n%s\n", diff)
	}
}
