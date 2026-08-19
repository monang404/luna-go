// Traceability: 30-luna/60-ui/components/verbosity.zsh -> verbosity.go.
//
// Divergence note (documented per SESSION-53 STEP 4 parity audit): the
// zsh version reads/writes a single global AI_VERBOSITY env var implicitly
// in every function (_ai_verbose, _ai_verbose_c, _ai_verbose_level,
// _ai_verbose_ge). Components here are required to be pure `data ->
// rendered string` functions (see 53_port_ui_components_and_screens.yaml
// scope), so the verbosity level is an explicit parameter instead of a
// package-level mutable global. Callers (screens/CLI wiring in
// SESSION-55) own the current level and pass it in. This is an
// INTENTIONAL behavioral-shape change, not a missing port: every zsh
// call site's *decision logic* (level >= min) and *output bytes* are
// preserved exactly.
package components

import (
	"fmt"
	"strconv"

	"github.com/monang404/luna-go/internal/ui"
)

// Verbose is the port of _ai_verbose(min_level, message...): returns the
// message (with trailing newline, matching `printf '%s\n'`) if
// verbosity >= minLevel, else "".
func Verbose(verbosity, minLevel int, message string) string {
	if verbosity >= minLevel {
		return message + "\n"
	}
	return ""
}

// VerboseC is the port of _ai_verbose_c(min_level, color, message...):
// like Verbose but wraps message in color+reset.
func VerboseC(verbosity, minLevel int, color, message string, t ui.Tokens) string {
	if verbosity >= minLevel {
		return color + message + t.Reset + "\n"
	}
	return ""
}

// VerboseLevel is the port of _ai_verbose_level(): returns the level
// itself (the zsh version just echoes ${AI_VERBOSITY:-0}; here the level
// is already an explicit int, so this is a documentation-preserving
// pass-through for call sites that mirror the shell API 1:1).
func VerboseLevel(verbosity int) int {
	return verbosity
}

// VerboseGE is the port of _ai_verbose_ge(min_level).
func VerboseGE(verbosity, minLevel int) bool {
	return verbosity >= minLevel
}

// verbosityLabel is the port of the case/label table inside
// ai_verbosity_set.
func verbosityLabel(n int) string {
	switch n {
	case 0:
		return "Minimal (hanya hasil)"
	case 1:
		return "Normal"
	case 2:
		return "Detailed (tool+file)"
	case 3:
		return "Debug (semua log)"
	default:
		return ""
	}
}

// VerbositySet is the port of ai_verbosity_set(N). n is taken as a string
// (matching the shell's raw arg, so an invalid value like "9" or "abc"
// round-trips into the error message unchanged, byte for byte). Returns
// the rendered message and whether n was accepted (ok==false means the
// caller should NOT update its stored verbosity level, matching the
// shell's `return 1`).
func VerbositySet(n string, t ui.Tokens) (msg string, ok bool) {
	switch n {
	case "0", "1", "2", "3":
		lvl, _ := strconv.Atoi(n)
		label := verbosityLabel(lvl)
		return fmt.Sprintf("%s✓%s Verbosity → %s  %s(%s)%s\n", t.OK, t.Reset, n, t.Muted, label, t.Reset), true
	default:
		return fmt.Sprintf("%sVerbosity harus 0-3 (dapat: %s)%s\n", t.Warn, n, t.Reset), false
	}
}
