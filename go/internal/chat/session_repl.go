package chat

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// replHelp mirrors the REPL's `/help` heredoc verbatim.
const replHelp = `Perintah session:
  /help      tampilkan bantuan
  /history   tampilkan riwayat percakapan
  /clear     hapus context dan mulai dari system prompt
  /name      tampilkan nama session aktif
  /exit      keluar dari session
  /quit      keluar dari session
`

// ErrReplUnknownCommand is written to stderr-equivalent output (never
// returned as an error) when an unrecognized "/..." command is entered,
// matching the zsh source printing to stderr and continuing the loop.
var ErrReplUnknownCommand = errors.New("chat: unknown REPL command")

// SessionRepl mirrors _ai_session_repl(name): a line-oriented
// interactive multi-turn loop. in/out replace the zsh source's
// stdin/stdout so this is unit-testable without a real terminal --
// SESSION-55 wires os.Stdin/os.Stdout (or a richer TUI) on top of this.
// True Ctrl+C-cancels-in-flight-request behavior needs an interactive
// terminal signal handler and is therefore also a SESSION-55 concern;
// this function honors ctx cancellation between turns (not mid-request)
// as its closest equivalent.
//
// Returns nil on a clean exit (EOF on in, or /exit|/quit|/q).
func (s *Service) SessionRepl(ctx context.Context, store *SessionStore, name string, in io.Reader, out io.Writer) error {
	if name == "" {
		name = "main"
	}
	if _, err := store.Load(name); err != nil {
		return fmt.Errorf("chat: failed to open session %q: %w", name, err)
	}

	fmt.Fprintf(out, "\nAI session: %s\n", name)
	fmt.Fprintln(out, "Ketik pesan untuk melanjutkan percakapan.")
	fmt.Fprintln(out, "/help untuk perintah, /exit untuk keluar.")

	scanner := bufio.NewScanner(in)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		fmt.Fprint(out, "\nYou> ")
		if !scanner.Scan() {
			fmt.Fprintln(out)
			return nil // EOF, matches Ctrl+D exit
		}
		msg := strings.TrimSpace(scanner.Text())
		if msg == "" {
			continue
		}

		switch {
		case msg == "/exit" || msg == "/quit" || msg == "/q":
			fmt.Fprintf(out, "Keluar dari session \"%s\".\n", name)
			return nil
		case msg == "/help" || msg == "/h":
			fmt.Fprint(out, replHelp)
			continue
		case msg == "/name":
			fmt.Fprintln(out, name)
			continue
		case msg == "/history":
			history, err := store.Load(name)
			if err != nil {
				fmt.Fprintln(out, "ERROR: session JSON rusak.")
				continue
			}
			for _, m := range history {
				if m.Role == "system" {
					continue
				}
				fmt.Fprintf(out, "[%s] %s\n", m.Role, m.Content)
			}
			continue
		case msg == "/clear":
			if err := store.Clear(name); err != nil {
				fmt.Fprintln(out, "ERROR: gagal menghapus context.")
				continue
			}
			fmt.Fprintln(out, "Context session dihapus.")
			continue
		case strings.HasPrefix(msg, "/"):
			fmt.Fprintln(out, "Perintah tidak dikenal. Gunakan /help.")
			continue
		}

		res, err := s.SessionAsk(ctx, store, name, msg)
		if err != nil {
			fmt.Fprintln(out, "[request gagal, session tetap aktif]")
			continue
		}
		fmt.Fprintln(out, res.Answer)
	}
}
