package ui

import (
	"strings"
	"testing"
)

// AC-01: every AI_C_* token must produce the exact ANSI escape sequence
// 02-ui_colors.zsh's _ai_ui_colors_init sets, for every single token (not
// just a sample).
func TestColorTokens_MatchZshEscapeSequences(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"Reset", ColorTokens.Reset, "\x1b[0m"},
		{"Bold", ColorTokens.Bold, "\x1b[1m"},
		{"Dim", ColorTokens.Dim, "\x1b[2m"},
		{"BG", ColorTokens.BG, "\x1b[48;2;13;17;23m"},
		{"Surface", ColorTokens.Surface, "\x1b[48;2;22;27;34m"},
		{"Border", ColorTokens.Border, "\x1b[38;2;48;54;61m"},
		{"Primary", ColorTokens.Primary, "\x1b[38;2;47;129;247m"},
		{"Accent", ColorTokens.Accent, "\x1b[38;2;47;129;247m"},
		{"OK", ColorTokens.OK, "\x1b[38;2;63;185;80m"},
		{"Err", ColorTokens.Err, "\x1b[38;2;248;81;73m"},
		{"Warn", ColorTokens.Warn, "\x1b[38;2;210;153;34m"},
		{"Info", ColorTokens.Info, "\x1b[38;2;47;129;247m"},
		{"Text", ColorTokens.Text, "\x1b[38;2;230;237;243m"},
		{"Muted", ColorTokens.Muted, "\x1b[38;2;139;148;158m"},
	}
	if len(cases) != 14 {
		t.Fatalf("expected 14 AI_C_* tokens per 02-ui_colors.zsh, got %d cases", len(cases))
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("Tokens.%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// Same 14 tokens, but every field must be empty in NoColorTokens (the
// _ai_ui_colors_init `else` branch).
func TestNoColorTokens_AllEmpty(t *testing.T) {
	fields := map[string]string{
		"Reset": NoColorTokens.Reset, "Bold": NoColorTokens.Bold, "Dim": NoColorTokens.Dim,
		"BG": NoColorTokens.BG, "Surface": NoColorTokens.Surface, "Border": NoColorTokens.Border,
		"Primary": NoColorTokens.Primary, "Accent": NoColorTokens.Accent,
		"OK": NoColorTokens.OK, "Err": NoColorTokens.Err, "Warn": NoColorTokens.Warn,
		"Info": NoColorTokens.Info, "Text": NoColorTokens.Text, "Muted": NoColorTokens.Muted,
	}
	if len(fields) != 14 {
		t.Fatalf("expected 14 fields, got %d", len(fields))
	}
	for name, v := range fields {
		if v != "" {
			t.Errorf("NoColorTokens.%s = %q, want empty", name, v)
		}
	}
}

// AC-02: NO_COLOR=1 (and any non-empty NO_COLOR value, per the shell's
// `[ -n "${NO_COLOR:-}" ]` check) must disable color outright.
func TestSupportsColor_NoColorEnv(t *testing.T) {
	cases := []struct {
		name      string
		noColor   string
		aiNoColor string
		term      string
		isTTY     bool
		wantColor bool
	}{
		{"NO_COLOR unset, tty, TERM set -> color on", "", "", "xterm-256color", true, true},
		{"NO_COLOR=1 -> color off", "1", "", "xterm-256color", true, false},
		{`NO_COLOR="" (unset semantics) -> color on`, "", "", "xterm-256color", true, true},
		{"NO_COLOR=anything (non-empty) -> color off", "anything", "", "xterm-256color", true, false},
		{"AI_UI_NO_COLOR=1 -> color off", "", "1", "xterm-256color", true, false},
		{"not a tty -> color off", "", "", "xterm-256color", false, false},
		{"TERM=dumb -> color off", "", "", "dumb", true, false},
		{"TERM unset -> color off", "", "", "", true, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", c.noColor)
			if c.noColor == "" {
				// t.Setenv("", "") still sets NO_COLOR="" which the shell
				// treats as unset (`-n` is false for empty string); Go's
				// os.Getenv can't tell "unset" from "set empty" so
				// SupportsColor must treat empty the same as unset, which
				// is what os.Getenv("NO_COLOR") != "" already does.
			}
			t.Setenv("AI_UI_NO_COLOR", c.aiNoColor)
			t.Setenv("TERM", c.term)

			got := SupportsColor(c.isTTY)
			if got != c.wantColor {
				t.Errorf("SupportsColor() = %v, want %v", got, c.wantColor)
			}
		})
	}
}

// The most important NO_COLOR assertion: when color is off, output must
// contain zero ANSI escape bytes at all -- not just that individual tokens
// are empty, but that C()/HighlightBody() built on top of them stay clean
// too.
func TestNoColor_NoEscapeBytesAnywhere(t *testing.T) {
	tok := NoColorTokens
	outputs := []string{
		// Callers always pass the active token's own color field (which is
		// itself empty when NoColorTokens is active) -- never a hardcoded
		// ColorTokens.* value while color is off, same as the shell where
		// $AI_C_OK etc. are blanked by _ai_ui_colors_init itself.
		tok.C(tok.OK, "some text"),
		tok.C(tok.Err, "other text"),
		tok.HighlightBody("$ ls -la"),
		tok.HighlightBody("Label: value"),
		tok.HighlightBody("plain passthrough"),
	}
	for _, out := range outputs {
		if strings.Contains(out, "\x1b[") {
			t.Errorf("output contains ANSI escape with NoColorTokens: %q", out)
		}
	}
}

func TestC_WrapsAndResets(t *testing.T) {
	got := ColorTokens.C(ColorTokens.OK, "hi")
	want := ColorTokens.OK + "hi" + ColorTokens.Reset
	if got != want {
		t.Errorf("C() = %q, want %q", got, want)
	}
}

func TestHighlightBody(t *testing.T) {
	tok := ColorTokens
	cases := []struct {
		in, want string
	}{
		{"$ rm -rf /tmp/foo", tok.Accent + "$" + tok.Reset + " rm -rf /tmp/foo"},
		{"Path: /a/b/c", tok.Muted + "Path:" + tok.Reset + " /a/b/c"},
		{"no pattern here", "no pattern here"},
	}
	for _, c := range cases {
		got := tok.HighlightBody(c.in)
		if got != c.want {
			t.Errorf("HighlightBody(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
