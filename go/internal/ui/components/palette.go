// Traceability: 30-luna/60-ui/components/palette.zsh -> palette.go.
//
// Scope note: ui_palette_generic() in zsh is entirely interactive (feeds
// items into `gum filter` and reads the choice back) — no caller exists
// repo-wide today (per the zsh source's own comment) and the interactive
// half is out of scope for SESSION-53 (readline/stdin — SESSION-55).
// What's ported here is the one pure piece: extracting the label out of
// a "Label • Description" choice string, which screens/palette.go reuses
// for the real (registry-backed) command palette.
package components

import "strings"

// ExtractLabel is the port of `echo "${choice%% •*}" | xargs`: take
// everything before " •" and trim surrounding whitespace.
func ExtractLabel(choice string) string {
	if idx := strings.Index(choice, " •"); idx >= 0 {
		choice = choice[:idx]
	}
	return strings.TrimSpace(choice)
}
