package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// readStdinOrArgs joins args with a space; if args is empty and stdin
// is not a terminal, reads all of stdin instead -- the common
// "$*"-or-pipe pattern every chat-family zsh function used.
func readStdinOrArgs(args []string) string {
	if len(args) > 0 {
		return strings.Join(args, " ")
	}
	stat, err := os.Stdin.Stat()
	if err == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		data, _ := io.ReadAll(os.Stdin)
		return strings.TrimSpace(string(data))
	}
	return ""
}

func newChatCmd(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:        "chat [prompt...]",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aic"},
		Short:      "Chat cepat, model kelas fast (legacy: aic)",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := readStdinOrArgs(args)
			res, err := app.Chat.QuickChat(cmd.Context(), prompt)
			if err != nil {
				return err
			}
			printChatResult(cmd, res.Thought, res.Answer)
			return nil
		},
	}
	return cmd
}

func newLongCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:        "long [prompt...]",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aicl"},
		Short:      "Chat model kelas smart, 5 tahap (legacy: aicl)",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := readStdinOrArgs(args)
			res, err := app.Chat.LongChat(cmd.Context(), prompt)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Final)
			return nil
		},
	}
}

func newShellCmd(app *App) *cobra.Command {
	return &cobra.Command{
		Use:        "shell [prompt...]",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aish"},
		Short:      "Minta perintah shell/Termux yang aman (legacy: aish)",
		RunE: func(cmd *cobra.Command, args []string) error {
			prompt := readStdinOrArgs(args)
			res, err := app.Chat.Aish(cmd.Context(), prompt)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Raw)
			return nil
		},
	}
}

func newAskCmd(app *App) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:        "ask [question...]",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aiask"},
		Short:      "Tanya-jawab tunggal atas konten file/stdin (legacy: aiask)",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			var content string
			if file != "" {
				data, err := os.ReadFile(file)
				if err != nil {
					return err
				}
				content = string(data)
			} else {
				data, _ := io.ReadAll(os.Stdin)
				content = string(data)
			}
			res, err := app.Chat.Ask(cmd.Context(), content, query)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Answer)
			return nil
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "file to read context from (default: stdin)")
	return cmd
}

func newClipCmd(app *App) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:        "clip [query...]",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Aliases:    []string{"aiclip"},
		Short:      "Kirim isi clipboard ke LUNA (legacy: aiclip)",
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			res, err := app.Chat.Clip(cmd.Context(), app.Clipboard, force, query)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), res.Answer)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "kirim walau konten terlihat sensitif")
	return cmd
}

func newSessionCmd(app *App) *cobra.Command {
	root := &cobra.Command{
		Use:        "session",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Short:      "Sesi chat multi-turn tersimpan (start/end/list/resume/prune)",
	}
	root.AddCommand(&cobra.Command{
		Use:        "start <name>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Short:      "Mulai sesi baru",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.Sessions.Start(args[0])
		},
	})
	root.AddCommand(&cobra.Command{
		Use:        "end <name>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Short:      "Akhiri sesi",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.Sessions.End(args[0])
		},
	})
	root.AddCommand(&cobra.Command{
		Use:        "list",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Short:      "Daftar sesi tersimpan",
		RunE: func(cmd *cobra.Command, args []string) error {
			names, err := app.Sessions.List()
			if err != nil {
				return err
			}
			for _, n := range names {
				fmt.Fprintln(cmd.OutOrStdout(), n)
			}
			return nil
		},
	})
	root.AddCommand(&cobra.Command{
		Use:        "prune [days]",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Short:      "Hapus sesi lebih tua dari N hari (default 14)",
		RunE: func(cmd *cobra.Command, args []string) error {
			days := 14
			if len(args) > 0 {
				fmt.Sscanf(args[0], "%d", &days)
			}
			res, err := app.Sessions.SessionPrune(days)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Removed %d session(s)\n", res.Removed)
			return nil
		},
	})
	root.AddCommand(&cobra.Command{
		Use:        "resume <name> [message...]",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Short:      "Kirim satu giliran ke sesi (bikin sesi kalau belum ada)",
		Args:       cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			msg := readStdinOrArgs(args[1:])
			res, err := app.Chat.SessionAsk(cmd.Context(), app.Sessions, name, msg)
			if err != nil {
				return err
			}
			printChatResult(cmd, res.Thought, res.Answer)
			return nil
		},
	})
	root.AddCommand(&cobra.Command{
		Use:        "repl <name>",
		Deprecated: "gunakan REPL utama ('luna' tanpa argumen) sebagai gantinya.",
		Short:      "REPL baris-per-baris untuk sesi (Ctrl-D untuk keluar)",
		Args:       cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return app.Chat.SessionRepl(cmd.Context(), app.Sessions, args[0], bufio.NewReader(os.Stdin), cmd.OutOrStdout())
		},
	})
	return root
}

func printChatResult(cmd *cobra.Command, thought, answer string) {
	out := cmd.OutOrStdout()
	if thought != "" {
		fmt.Fprintf(cmd.ErrOrStderr(), "(%s)\n\n", thought)
	}
	fmt.Fprintln(out, answer)
}
