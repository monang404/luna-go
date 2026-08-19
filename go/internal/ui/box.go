// Traceability: 05-ui_box.zsh -> box.go.
package ui

import (
	"strings"
)

// Mode bundles the three pieces of render state box/diff primitives need:
// which token set is active, whether unicode box-drawing/icons are
// supported, and the terminal width to render into. Building a Mode is the
// impure, environment-reading step (see DetectMode); rendering from a Mode
// is pure.
type Mode struct {
	Tokens  Tokens
	Unicode bool
	Width   int
}

// DetectMode reads the live environment/terminal the same way the shell
// primitives do: ActiveTokens() for color, SupportsUnicode() for box-drawing
// glyphs, Width() for terminal columns.
func DetectMode() Mode {
	return Mode{
		Tokens:  ActiveTokens(),
		Unicode: SupportsUnicode(),
		Width:   Width(),
	}
}

// BoxAccent is the port of _ai_ui_box_accent(title): pick a semantic color
// from the title text alone (approval=yellow, success=green,
// blocked=red, header/hero=accent, everything else=muted) so the reader
// doesn't have to read the body to tell them apart. Checked in this exact
// order, matching the shell case statement.
func BoxAccent(title string, t Tokens) string {
	switch {
	case strings.HasPrefix(title, "LUNA Agent"):
		return t.Accent
	case strings.Contains(title, "requires approval"):
		return t.Warn
	case strings.HasPrefix(title, "✓"), strings.HasPrefix(title, "+"), strings.Contains(title, "Completed"):
		return t.OK
	case strings.HasPrefix(title, "✗"), strings.HasPrefix(title, "x "), strings.Contains(title, "blocked"), strings.Contains(title, "Blocked"):
		return t.Err
	default:
		return t.Muted
	}
}

// Box is the port of _ai_ui_box(title, lines...). Pure function: input in,
// rendered string out (each logical output line, including the last,
// terminated with "\n" — the concatenation of what the shell version's
// repeated `echo` calls would have printed) — never writes to stdout
// itself, so SESSION-53 can compose it into larger screens.
//
// Two render paths, chosen by whether title contains "Approval"/"approval":
//   - non-approval: title (bold+accent) on its own line, then each line
//     printed left-aligned as-is (a "---" line becomes a muted separator);
//     no box border, no wrapping.
//   - approval: a bordered box (unicode ┌─┐│└┘ or ASCII +-|, chosen by
//     mode.Unicode), width from mode.Width, body lines word-wrapped to fit,
//     capped at 4 body lines, each line highlighted via
//     Tokens.HighlightBody.
func Box(title string, lines []string, mode Mode) string {
	t := mode.Tokens
	isApproval := strings.Contains(title, "Approval") || strings.Contains(title, "approval")

	var b strings.Builder

	if !isApproval {
		if title != "" {
			accent := BoxAccent(title, t)
			b.WriteString(accent)
			b.WriteString(t.Bold)
			b.WriteString(title)
			b.WriteString(t.Reset)
			b.WriteString("\n")
		}
		for _, line := range lines {
			if line == "---" {
				b.WriteString(t.Muted)
				b.WriteString("---")
				b.WriteString(t.Reset)
				b.WriteString("\n")
				continue
			}
			for _, subline := range strings.Split(line, "\n") {
				b.WriteString(subline)
				b.WriteString("\n")
			}
		}
		return b.String()
	}

	width := mode.Width
	inner := width - 2
	if inner < 10 {
		inner = 10
	}

	var tl, tr, bl, br, hz, vt string
	if mode.Unicode {
		tl, tr, bl, br, hz, vt = "┌", "┐", "└", "┘", "─", "│"
	} else {
		tl, tr, bl, br, hz, vt = "+", "+", "+", "+", "-", "|"
	}

	accent := BoxAccent(title, t)

	// Top border.
	b.WriteString(accent)
	b.WriteString(tl)
	b.WriteString(strings.Repeat(hz, inner))
	b.WriteString(tr)
	b.WriteString(t.Reset)
	b.WriteString("\n")

	avail := inner - 2
	if avail < 4 {
		avail = 4
	}

	writeRow := func(content string, visibleLen int) {
		pad := avail - visibleLen
		if pad < 0 {
			pad = 0
		}
		b.WriteString(accent)
		b.WriteString(vt)
		b.WriteString(t.Reset)
		b.WriteString(" ")
		b.WriteString(content)
		b.WriteString(strings.Repeat(" ", pad))
		b.WriteString(" ")
		b.WriteString(accent)
		b.WriteString(vt)
		b.WriteString(t.Reset)
		b.WriteString("\n")
	}

	if title != "" {
		writeRow(t.Bold+title+t.Reset, wordLen(title, mode.Unicode))
		writeRow(strings.Repeat(" ", avail), avail)
	}

	lineCount := 0
outer:
	for _, line := range lines {
		if line == "---" {
			continue
		}
		for _, subline := range strings.Split(line, "\n") {
			for _, wrapped := range Wrap(subline, avail, mode.Unicode) {
				if lineCount >= 4 {
					break outer
				}
				writeRow(t.HighlightBody(wrapped), wordLen(wrapped, mode.Unicode))
				lineCount++
			}
		}
	}

	// Bottom border.
	b.WriteString(accent)
	b.WriteString(bl)
	b.WriteString(strings.Repeat(hz, inner))
	b.WriteString(br)
	b.WriteString(t.Reset)
	b.WriteString("\n")

	return b.String()
}
