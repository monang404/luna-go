package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/filepatch"
	"github.com/spf13/cobra"
)

func newUndoCmd(app *App) *cobra.Command {
	var selectMode bool
	cmd := &cobra.Command{
		Use:     "undo <file>",
		Aliases: []string{"aiundo"},
		Short:   "Restore dari backup .bak.* terbaru (legacy: aiundo)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := app.Patch.Undo(cmd.Context(), args[0], selectMode, terminalChoose)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Restored from %s (safety backup: %s)\n", res.RestoredFrom, res.SafetyBackup)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&selectMode, "select", "s", false, "pilih dari daftar backup, bukan otomatis pakai yang terbaru")
	return cmd
}

// terminalChoose is a plain numbered-menu picker for filepatch.ChooseFunc,
// replacing aiundo -s's `gum choose`/numbered `read` fallback.
func terminalChoose(_ context.Context, prompt string, options []string) (string, error) {
	if len(options) == 1 {
		return options[0], nil
	}
	fmt.Fprintln(os.Stderr, prompt)
	for i, o := range options {
		fmt.Fprintf(os.Stderr, "  %d) %s\n", i+1, o)
	}
	fmt.Fprint(os.Stderr, "Pilih nomor (kosong untuk batal): ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return "", nil
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > len(options) {
		return "", fmt.Errorf("commands: invalid choice %q", line)
	}
	return options[n-1], nil
}

func newBakCleanCmd(app *App) *cobra.Command {
	var days int
	cmd := &cobra.Command{
		Use:     "bakclean [root]",
		Aliases: []string{"aibakclean"},
		Short:   "Bersihin backup lebih tua dari N hari, default 14 (legacy: aibakclean)",
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) > 0 {
				root = args[0]
			}
			res, err := app.Patch.BakClean(cmd.Context(), root, app.Paths.CacheDir, days, os.Remove)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed=%v old_backups=%d old_cache=%d\n", res.Removed, len(res.OldBackups), len(res.OldCache))
			return nil
		},
	}
	cmd.Flags().IntVar(&days, "days", 14, "usia minimum (hari) sebelum dihapus")
	return cmd
}

func newShareCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "share <file>",
		Aliases: []string{"aishare"},
		Short:   "Share file lewat share-sheet Android (legacy: aishare)",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return shareFile(cmd.Context(), args[0])
		},
	}
}

// shareFile wires filepatch.Share to termux-share when available,
// falling back to a "not available" error off-Termux (there is no
// meaningful non-Termux share-sheet equivalent to fall back to).
func shareFile(ctx context.Context, file string) error {
	share := aiops.ShareFunc(func(ctx context.Context, path string) error {
		if _, err := exec.LookPath("termux-share"); err != nil {
			return fmt.Errorf("commands: termux-share not found (not running on Termux)")
		}
		return exec.CommandContext(ctx, "termux-share", path).Run()
	})
	return filepatch.Share(ctx, file, share)
}
