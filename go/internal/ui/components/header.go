// Traceability: 30-luna/60-ui/components/header.zsh -> header.go.
//
// Scope note: ui_header() in zsh reads several impure sources directly
// (PWD/HOME env vars, `git branch --show-current`, `git status
// --porcelain`). Per this session's "data -> rendered string" component
// contract, HeaderData below is the pre-gathered snapshot of that state;
// Header() itself does no env/process access, matching how
// CardSummary/Box etc. already work in this package.
package components

import (
	"strconv"
	"strings"

	"github.com/monang404/luna-go/internal/ui"
)

// HeaderData is the Go equivalent of the runtime state ui_header() reads
// (AI_CURRENT_PROVIDER, AI_CURRENT_MODEL, AI_CURRENT_SESSION, PWD (already
// tilde-collapsed), AI_TOKEN_USAGE, plus derived git info).
type HeaderData struct {
	Provider string
	Model    string
	Session  string // defaults to "main" if empty, like ${AI_CURRENT_SESSION:-main}
	PwdStr   string
	TokenStr string

	// InGitRepo mirrors the `git rev-parse --is-inside-work-tree` guard;
	// when false, GitBranch/GitDirty are ignored (branch_info stays "").
	InGitRepo bool
	GitBranch string
	GitDirty  int // count of `git status --porcelain` lines
}

// Header is the port of ui_header().
func Header(d HeaderData, t ui.Tokens) string {
	session := d.Session
	if session == "" {
		session = "main"
	}

	modelLabel := ""
	switch {
	case d.Provider != "" && d.Model != "":
		modelLabel = d.Provider + "/" + d.Model
	case d.Model != "":
		modelLabel = d.Model
	case d.Provider != "":
		modelLabel = d.Provider
	}
	if modelLabel == "" {
		modelLabel = t.Muted + "no model yet" + t.Reset
	}

	branchInfo := ""
	if d.InGitRepo {
		if d.GitDirty > 0 {
			b := d.GitBranch
			if b != "" {
				b += " "
			}
			branchInfo = b + t.Warn + "dirty " + strconv.Itoa(d.GitDirty) + "f" + t.Reset
		} else {
			branchInfo = d.GitBranch
		}
	}

	var b strings.Builder
	b.WriteString(t.Bold + "Bagas LUNA" + t.Reset)
	b.WriteString(" " + t.Muted + "•" + t.Reset + " " + t.Primary + session + t.Reset)
	b.WriteString(" " + t.Muted + "•" + t.Reset + " " + modelLabel)
	b.WriteString(" " + t.Muted + "•" + t.Reset + " " + t.Muted + d.PwdStr + t.Reset)
	if branchInfo != "" {
		b.WriteString(" " + t.Muted + "•" + t.Reset + " " + branchInfo)
	}
	if d.TokenStr != "" {
		b.WriteString(" " + t.Muted + "•" + t.Reset + " " + t.Muted + d.TokenStr + "tok" + t.Reset)
	}
	b.WriteString("\n") // echo "$parts"
	return b.String()
}
