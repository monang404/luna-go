// Traceability: 30-luna/60-ui/components/timeline.zsh -> timeline.go.
//
// Parity note (STEP 4 finding, documented per session rules — "catat,
// jangan dikerjakan" for anything outside SESSION-53): ui_timeline()'s
// "done" branch prefers `_ai_ui_line "✓" "$step"` when that function is
// defined, falling back to a hardcoded unicode "✓" otherwise. In the real
// zsh runtime _ai_ui_line (05-ui_box.zsh) is always loaded, so the
// happy path always runs — and _ai_ui_line itself DOES swap ✓/+ under
// ASCII mode. SESSION-52 did not port _ai_ui_line (out of its declared
// scope: tokens/text/box/diff only). To keep AC-01 byte-identical parity
// with the real (happy-path) zsh behavior without inventing a new
// standalone rendering primitive, the ASCII/unicode swap for the "done"
// icon is inlined here using the already-available Tokens/mode.Unicode —
// the same pattern every other component in this package already uses.
// The "active"/"pending" icons (● / ○) are or NOT gated by
// _ai_ui_line in the zsh source (printf'd directly, no unicode fallback)
// — that asymmetry is preserved exactly (bug-for-bug) below.
package components

import (
	"strings"

	"github.com/monang404/luna-go/internal/ui"
)

// Timeline is the port of ui_timeline(steps_str, current_idx?). steps is
// the already-split ("${(@f)steps_str}") array of step lines; an empty
// step still occupies an index slot but renders nothing, matching
// `[ -z "$step" ] && (( i++ )) && continue`.
func Timeline(steps []string, currentIdx int, mode ui.Mode) string {
	t := mode.Tokens
	var b strings.Builder
	for i, step := range steps {
		idx := i + 1
		if step == "" {
			continue
		}
		switch {
		case idx < currentIdx:
			doneIcon := "✓"
			if !mode.Unicode {
				doneIcon = "+"
			}
			b.WriteString("  " + t.OK + doneIcon + t.Reset + " " + step + "\n")
		case idx == currentIdx:
			b.WriteString("  " + t.Primary + "●" + t.Reset + " " + t.Bold + step + t.Reset + "\n")
		default:
			b.WriteString("  " + t.Muted + "○ " + step + t.Reset + "\n")
		}
	}
	return b.String()
}
