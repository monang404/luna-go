// Traceability:
//
//	06-ui_diff.zsh (_ai_ui_diff_header/_ai_ui_diff_footer) -> DiffHeader/DiffFooter.
//	30-code/05-code.zsh & 35-files/10-aipatch.zsh (SESSION-25 body
//	colorizer: `diff -u ... | sed -e 's/^-/${AI_C_ERR}-/' -e 's/^+/${AI_C_OK}+/'
//	-e 's/$/${AI_C_RESET}/'`, the AI_C_*-compliant pattern
//	docs/RENDERING_CONTRACT.md §3 documents for `aicode -o`/`aipatch`)
//	-> ColorizeDiffBody.
//
// NOTE: the raw-ANSI body colorizer in 30-code/45-fix.zsh (_ai_fix_apply,
// used by aifix/airun) is explicitly documented known debt
// (RENDERING_CONTRACT.md §4) — it is NOT AI_C_*-compliant and is
// intentionally not what ColorizeDiffBody ports.
package ui

import "strings"

// DiffHeader is the port of _ai_ui_diff_header(path):
// "\n── Diff yang diusulkan: PATH ──\n" (or "--" ASCII fallback), wrapped in
// Muted/Reset. The literal text "Diff yang diusulkan" must stay unchanged —
// some shell tests grep for it verbatim.
func DiffHeader(path string, mode Mode) string {
	t := mode.Tokens
	hz := "──"
	if !mode.Unicode {
		hz = "--"
	}
	return "\n" + t.Muted + hz + " Diff yang diusulkan: " + path + " " + hz + t.Reset + "\n"
}

// DiffFooter is the port of _ai_ui_diff_footer(): a Muted/Reset-wrapped
// rule, mode.Width characters wide, "─" (unicode) or "-" (ASCII fallback).
func DiffFooter(mode Mode) string {
	t := mode.Tokens
	hz := "─"
	if !mode.Unicode {
		hz = "-"
	}
	return t.Muted + strings.Repeat(hz, mode.Width) + t.Reset + "\n"
}

// ColorizeDiffBody colors a unified-diff body line-by-line, matching the
// AI_C_*-compliant `sed` pattern used by `aicode -o`/`aipatch`
// (RENDERING_CONTRACT.md §3): a line starting with "-" is prefixed with
// Err, a line starting with "+" is prefixed with OK, and every line
// (including unprefixed context/header/hunk lines) gets Reset appended at
// its end. Because sed's `s/^-/...-/ ` only ever substitutes the single
// leading "-"/"+" character, the net effect is simply "prefix the whole
// original line with the color code" -- so "---" file headers keep all
// three dashes and end up Err-colored, "+++" headers OK-colored.
//
// A trailing newline in diffText (if present) is preserved as-is.
func ColorizeDiffBody(diffText string, t Tokens) string {
	if diffText == "" {
		return ""
	}
	trailingNewline := strings.HasSuffix(diffText, "\n")
	body := diffText
	if trailingNewline {
		body = body[:len(body)-1]
	}
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		switch {
		case strings.HasPrefix(line, "-"):
			line = t.Err + line
		case strings.HasPrefix(line, "+"):
			line = t.OK + line
		}
		lines[i] = line + t.Reset
	}
	out := strings.Join(lines, "\n")
	if trailingNewline {
		out += "\n"
	}
	return out
}
