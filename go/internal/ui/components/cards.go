// Traceability: 30-luna/60-ui/components/cards.zsh -> cards.go.
//
// Status note (carried over from the zsh source's own header comment):
// UX-019/SESSION-04 flagged ui_card_summary/ui_card_stats as
// RESERVED-FOR-FUTURE-USE — confirmed no caller repo-wide as of
// 2026-08-15. Ported anyway per this session's scope (all 9
// components), kept for the same future-reuse rationale; not a
// parity gap.
package components

import (
	"strings"

	"github.com/monang404/luna-go/internal/ui"
)

// splitContentLines mirrors the shell's
//
//	while IFS= read -r line || [[ -n "$line" ]]; do lines+=("$line"); done <<< "$content"
//
// idiom: empty content still yields one empty line (matching `<<< ""`
// feeding a lone newline to read); a trailing newline in content does
// NOT produce a trailing empty element (read's final EOF iteration is
// gated off by `-n "$line"`).
func splitContentLines(content string) []string {
	if content == "" {
		return []string{""}
	}
	parts := strings.Split(content, "\n")
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	return parts
}

// CardSummary is the port of ui_card_summary(title?, content?).
func CardSummary(title, content string, mode ui.Mode) string {
	if title == "" {
		title = "Summary"
	}
	return ui.Box(title, splitContentLines(content), mode)
}

// CardStats is the port of ui_card_stats(files?, runtime?, tools?).
func CardStats(files, runtime, tools string, mode ui.Mode) string {
	t := mode.Tokens
	if files == "" {
		files = "0"
	}
	sep := "·"
	if !mode.Unicode {
		sep = "-"
	}
	line := "Files changed: " + files
	if runtime != "" {
		line += "  " + sep + "  " + runtime
	}
	if tools != "" {
		line += "  " + sep + "  Tools: " + tools
	}
	if mode.Unicode {
		return t.OK + "✓" + t.Reset + " " + line + "\n"
	}
	return "+ " + line + "\n"
}
