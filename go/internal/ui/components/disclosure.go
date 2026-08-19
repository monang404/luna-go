// Traceability: 30-luna/60-ui/components/disclosure.zsh -> disclosure.go.
//
// Divergence note: AI_LAST_DETAIL_LOG (global shell string, appended to
// by every _ai_state_* call in state.go's zsh source) becomes the
// DetailLog type here — an explicit value the caller owns and threads
// through, instead of a package-level global. Push() takes the exact
// LogLine a StateResult.LogLine carries.
package components

import (
	"strings"

	"github.com/monang404/luna-go/internal/ui"
)

// DetailLog is the Go equivalent of AI_LAST_DETAIL_LOG.
type DetailLog struct {
	lines []string
}

// Push is the port of _ai_detail_push(line).
func (d *DetailLog) Push(line string) {
	d.lines = append(d.lines, line)
}

// Clear is the port of _ai_detail_clear().
func (d *DetailLog) Clear() {
	d.lines = nil
}

// Lines exposes the accumulated log lines (read-only view for tests /
// callers that need to inspect state without rendering).
func (d *DetailLog) Lines() []string {
	return append([]string(nil), d.lines...)
}

// formatDetailLine is the port of the `case "$logline" in ...` block
// inside _ai_detail_show. Note these icons are literal unicode glyphs in
// the zsh source with NO ascii-fallback check (unlike most other
// components) — preserved exactly here for byte-identical parity.
func formatDetailLine(logline string, t ui.Tokens) string {
	switch {
	case strings.HasPrefix(logline, "[done]"):
		return t.OK + "✓" + t.Reset + " " + strings.TrimPrefix(logline, "[done] ")
	case strings.HasPrefix(logline, "[error]"):
		return t.Err + "✗" + t.Reset + " " + strings.TrimPrefix(logline, "[error] ")
	case strings.HasPrefix(logline, "[thinking]"):
		return t.Info + "◌" + t.Reset + " " + strings.TrimPrefix(logline, "[thinking] ")
	case strings.HasPrefix(logline, "[acting]"):
		return t.Primary + "→" + t.Reset + " " + strings.TrimPrefix(logline, "[acting] ")
	case strings.HasPrefix(logline, "[tool]"):
		return t.Muted + "  Tool: " + strings.TrimPrefix(logline, "[tool] ") + t.Reset
	case strings.HasPrefix(logline, "[approval]"):
		return t.Warn + "⚠" + t.Reset + " " + strings.TrimPrefix(logline, "[approval] ")
	case strings.HasPrefix(logline, "[debug]"):
		return t.Muted + "[DBG] " + strings.TrimPrefix(logline, "[debug] ") + t.Reset
	default:
		return "  " + logline
	}
}

// Show is the port of _ai_detail_show(). mode.Width/mode.Unicode drive
// the top/bottom divider (same rule as elsewhere: unicode "─" repeated,
// ascii "-" repeated); the bullet glyphs inside are fixed per
// formatDetailLine's doc comment.
func (d *DetailLog) Show(mode ui.Mode) string {
	t := mode.Tokens
	if len(d.lines) == 0 {
		return t.Muted + "(Tidak ada detail log untuk ditampilkan)" + t.Reset + "\n"
	}

	inner := mode.Width - 2
	if inner < 0 {
		inner = 0
	}
	hz := "-"
	if mode.Unicode {
		hz = "─"
	}
	fill := strings.Repeat(hz, inner)

	var b strings.Builder
	b.WriteString(t.Muted + fill + t.Reset + "\n")
	b.WriteString(" " + t.Bold + "Detail Log" + t.Reset + "\n")
	b.WriteString(t.Muted + fill + t.Reset + "\n")
	for _, logline := range d.lines {
		if logline == "" {
			continue
		}
		b.WriteString(formatDetailLine(logline, t) + "\n")
	}
	b.WriteString(t.Muted + fill + t.Reset + "\n")
	return b.String()
}
