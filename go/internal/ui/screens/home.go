// Traceability: 30-luna/60-ui/screens/home.zsh -> home.go.
// LUNA-FIRST UX: Header -> Context bar -> Prompt, no menu list.
//
// Scope note: several parts of ui_home() are excluded per this session's
// scope (SESSION-53 exclude list: stdin/readline, filesystem):
//   - `clear` (terminal control, not "rendering" in the data->string
//     sense).
//   - git detection (`git rev-parse`, `git status -s`) and history/session
//     file probing (AI_HISTORY_LOG, AI_SESSION_DIR globbing) — these are
//     impure reads; HomeData below is the pre-gathered snapshot, same
//     pattern as components.HeaderData.
//   - the `gum input` / `read -r user_input` prompt read, and the final
//     `echo "$user_input"` that echoes it back — genuinely interactive,
//     SESSION-55 territory.
//
// What IS rendering (and stays here): the header, the context lines, the
// returning-vs-first-time copy, and the static "> " prompt indicator
// printed just before the (excluded) read.
package screens

import (
	"strconv"
	"strings"

	"github.com/monang404/luna-go/internal/ui"
	"github.com/monang404/luna-go/internal/ui/components"
)

// HomeData is the pre-gathered snapshot of everything ui_home() would
// otherwise read live (git status, session/history probing).
type HomeData struct {
	Header components.HeaderData

	// ModifiedCount mirrors `git status -s | wc -l` (0 if clean or not a
	// git repo — the zsh source only ever branches on `-gt 0`).
	ModifiedCount int

	CurrentSession string // AI_CURRENT_SESSION

	// IsReturning mirrors the two-signal "returning vs first-time user"
	// detection (AI_HISTORY_LOG non-empty OR a saved session file exists).
	IsReturning   bool
	RecentSession string // most recently modified session name, "" if none
}

// Home is the port of ui_home() (see file doc comment for the excluded
// interactive/impure parts).
func Home(d HomeData, mode ui.Mode) string {
	t := mode.Tokens
	var b strings.Builder

	b.WriteString(components.Header(d.Header, t)) // already \n-terminated (echo "$parts")
	b.WriteString("\n")                           // echo "" right after the header

	hasContext := false
	if d.ModifiedCount > 0 {
		b.WriteString("  " + t.Warn + "●" + t.Reset + " " + strconv.Itoa(d.ModifiedCount) + " file belum di-commit\n")
		hasContext = true
	}
	if d.CurrentSession != "" && d.CurrentSession != "main" {
		b.WriteString("  " + t.Info + "●" + t.Reset + " Session aktif: " + t.Bold + d.CurrentSession + t.Reset + "\n")
		hasContext = true
	}
	if hasContext {
		b.WriteString("\n")
	}

	if d.IsReturning {
		if d.RecentSession != "" && d.RecentSession != d.CurrentSession {
			b.WriteString("  " + t.Muted + "Sesi terakhir: " + t.Reset + t.Bold + d.RecentSession + t.Reset +
				"  " + t.Muted + "(lanjut: luna session resume " + d.RecentSession + ")" + t.Reset + "\n")
			b.WriteString("\n")
		}
		b.WriteString("  " + t.Muted + "Ketik prompt, " + t.Reset + "/" + t.Muted +
			" untuk Command Palette, atau " + t.Reset + "luna h" + t.Muted + " buat lihat semua command" + t.Reset + "\n")
	} else {
		b.WriteString("  " + t.Muted + "Contoh: " + t.Reset + "\"bikin script python buat convert csv ke json\"\n")
		b.WriteString("  " + t.Muted + "Ketik prompt atau " + t.Reset + "/" + t.Muted + " untuk Command Palette" + t.Reset + "\n")
	}
	b.WriteString("\n")

	// Static prompt indicator only (no trailing newline — matches
	// `printf '%s> %s' ...` waiting on the same line for input).
	b.WriteString(t.Primary + "> " + t.Reset)

	return b.String()
}
