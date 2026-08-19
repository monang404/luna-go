// Traceability: 30-luna/60-ui/screens/palette.zsh -> palette.go.
// LUNA-FIRST UX: full Command Palette, launcher-style.
//
// Scope note: the interactive picking itself (gum filter / fzf / plain
// numbered `read` fallback) is stdin/readline territory, excluded from
// this session (SESSION-55 CLI wiring). What's ported here is
// everything pure around it:
//   - Options(): the full flat option list (registry-derived + the 5
//     hardcoded non-`luna`-subcommand items), exactly what would be piped
//     into gum/fzf/the numbered listing.
//   - Command(): extracting the command part out of a selected line
//     (`sed 's/  .*//' | xargs`).
//   - Route(): the registry-vs-router dispatch decision documented in
//     the zsh source's own v-fix comment (RC-008/UX-007, SESSION-22):
//     registry subcommands go straight to the `luna` dispatcher, non-registry
//     items (details, config verbosity N) go through ui_router.
package screens

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/monang404/luna-go/internal/ui"
)

// Options is the port of the `options` array build at the top of
// ui_palette(): registry entries (via _ai_registry_flat_list) followed
// by the hardcoded non-registry items.
func Options() []string {
	opts := make([]string, 0, len(ui.CommandRegistry)+5)
	for _, e := range ui.CommandRegistry {
		opts = append(opts, fmt.Sprintf("%-14s%s", e.Name, e.Description))
	}
	opts = append(opts,
		"details       Tampilkan detail log terakhir",
		"config verbosity 0   Output minimal",
		"config verbosity 1   Output normal (default)",
		"config verbosity 2   Output detail (tool+file)",
		"config verbosity 3   Debug semua log",
	)
	return opts
}

var twoOrMoreSpacesAndRest = regexp.MustCompile(`  .*`)

// Command is the port of:
//
//	cmd_part=$(printf '%s' "$selected" | sed 's/  .*//' | xargs)
//
// i.e. everything before the first run of 2+ spaces, trimmed.
func Command(selected string) string {
	cmd := twoOrMoreSpacesAndRest.ReplaceAllString(selected, "")
	return strings.TrimSpace(cmd)
}

// RouteTarget mirrors the palette's dispatch decision after a selection.
type RouteTarget int

const (
	// RouteToDispatcher: cmd_part is a registry subcommand -> call `luna
	// <cmd_part>` (the full 40-dispatcher.zsh case-statement) directly,
	// bypassing ui_router entirely.
	RouteToDispatcher RouteTarget = iota
	// RouteToRouter: everything else (details, config verbosity N, ...)
	// -> ui_router "<cmd_part>".
	RouteToRouter
)

// Route is the port of:
//
//	if [[ " ${_AI_SUBCOMMANDS[*]} " == *" $cmd_part "* ]]; then
//	    luna "$cmd_part"
//	elif type ui_router >/dev/null 2>&1; then
//	    ui_router "$cmd_part"
//	...
//
// Note this checks cmd_part as a WHOLE against the subcommand list (not
// just its first word) — matching the shell's literal " $cmd_part "
// substring test. In practice registry items never carry args in the
// palette, so this only ever matches bare command names.
func Route(cmdPart string) RouteTarget {
	for _, name := range ui.Subcommands() {
		if name == cmdPart {
			return RouteToDispatcher
		}
	}
	return RouteToRouter
}
