// Package ui is the Go port of the legacy zsh source's 30-luna/60-ui/ rendering primitives
// (design tokens, text wrap, box drawing, diff colorizer).
//
// SESSION-52 scope: tokens.go, text.go, box.go, diff.go — low-level
// primitives only. Screens, components, and the command registry are
// SESSION-53+ and deliberately not touched here (see
// docs/execution_sessions/52_port_ui_tokens_and_primitives.yaml).
//
// Traceability: 02-ui_colors.zsh -> tokens.go.
package ui

import (
	"os"
	"strings"
)

// Tokens is the Go equivalent of the AI_C_* shell variables set by
// _ai_ui_colors_init in 02-ui_colors.zsh. When color is disabled every
// field is the empty string, matching the shell's "no-op" convention
// (callers never need to branch on color support themselves).
type Tokens struct {
	Reset, Bold, Dim    string
	BG, Surface, Border string
	Primary, Accent     string
	OK, Err, Warn, Info string
	Text, Muted         string
}

// ColorTokens is the "color enabled" token set. Escape sequences are
// byte-identical to _ai_ui_colors_init's `if _ai_ui_supports_color` branch.
var ColorTokens = Tokens{
	Reset:   "\x1b[0m",
	Bold:    "\x1b[1m",
	Dim:     "\x1b[2m",
	BG:      "\x1b[48;2;13;17;23m",    // Background #0D1117
	Surface: "\x1b[48;2;22;27;34m",    // Surface #161B22
	Border:  "\x1b[38;2;48;54;61m",    // Border #30363D
	Primary: "\x1b[38;2;47;129;247m",  // Primary #2F81F7
	Accent:  "\x1b[38;2;47;129;247m",  // Accent (using Primary)
	OK:      "\x1b[38;2;63;185;80m",   // Success #3FB950
	Err:     "\x1b[38;2;248;81;73m",   // Error #F85149
	Warn:    "\x1b[38;2;210;153;34m",  // Warning #D29922
	Info:    "\x1b[38;2;47;129;247m",  // Info (using Primary)
	Text:    "\x1b[38;2;230;237;243m", // Text #E6EDF3
	Muted:   "\x1b[38;2;139;148;158m", // Muted #8B949E
}

// NoColorTokens is the "color disabled" set: every field empty, matching
// _ai_ui_colors_init's `else` branch.
var NoColorTokens = Tokens{}

// SupportsColor is the Go port of _ai_ui_supports_color(). Color is off
// when:
//  1. AI_UI_NO_COLOR=1 or NO_COLOR is set (non-empty) — standard convention.
//  2. stdout is not an interactive tty (isTTY == false) — never leak raw
//     ANSI into a pipe/redirect/file.
//  3. TERM is "dumb" or unset/empty.
//
// isTTY is passed in rather than detected here so the function stays a
// pure, easily testable boolean — see IsTerminal for the actual stdout
// check callers should use in real runs.
func SupportsColor(isTTY bool) bool {
	if os.Getenv("AI_UI_NO_COLOR") == "1" {
		return false
	}
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if !isTTY {
		return false
	}
	switch os.Getenv("TERM") {
	case "dumb", "":
		return false
	}
	return true
}

// IsTerminal reports whether f is an interactive terminal (character
// device), the Go equivalent of shell's `[ -t 1 ]`.
func IsTerminal(f *os.File) bool {
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

// ActiveTokens returns ColorTokens or NoColorTokens depending on
// SupportsColor(IsTerminal(os.Stdout)) — the single boundary where
// NO_COLOR/tty/TERM detection happens, so primitives elsewhere (Box, Diff,
// ...) just take a Tokens value and never re-implement this check.
func ActiveTokens() Tokens {
	if SupportsColor(IsTerminal(os.Stdout)) {
		return ColorTokens
	}
	return NoColorTokens
}

// C wraps text in color and an automatic reset, the port of _ai_ui_c
// (color_var_value, text...). Safe to call with an empty color (no-op),
// same as the shell version when color is disabled.
func (t Tokens) C(color, text string) string {
	return color + text + t.Reset
}

// HighlightBody is the port of _ai_ui_highlight_body(text): light
// highlighting for a single already-wrapped/padded box body line,
// recognizing two patterns:
//   - "$ command"       -> the "$" gets Accent, the rest is left plain.
//   - "Label: value..."  -> "Label:" is dimmed (Muted), the value stays
//     full brightness so the eye goes to the value (path, number, ...).
//
// Anything matching neither pattern is returned unchanged (no-op).
func (t Tokens) HighlightBody(text string) string {
	if strings.HasPrefix(text, "$ ") {
		return t.Accent + "$" + t.Reset + " " + strings.TrimPrefix(text, "$ ")
	}
	if idx := strings.Index(text, ": "); idx >= 0 {
		label := text[:idx]
		rest := text[idx+2:]
		return t.Muted + label + ":" + t.Reset + " " + rest
	}
	return text
}
