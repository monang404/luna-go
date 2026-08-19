// Traceability: 00-ui_text.zsh -> text.go.
package ui

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode/utf8"
)

// SupportsUnicode is the Go port of _ai_ui_supports_unicode():
//  1. AI_UI_ASCII_FALLBACK=1 (manual override) -> force ASCII (false).
//  2. Otherwise inspect locale (LC_ALL > LC_CTYPE > LANG); a value
//     containing "utf-8"/"utf8" (case-insensitive) means unicode box
//     drawing/icons are supported, anything else (e.g. "C", "POSIX",
//     unset) falls back to ASCII.
func SupportsUnicode() bool {
	if os.Getenv("AI_UI_ASCII_FALLBACK") == "1" {
		return false
	}
	loc := os.Getenv("LC_ALL")
	if loc == "" {
		loc = os.Getenv("LC_CTYPE")
	}
	if loc == "" {
		loc = os.Getenv("LANG")
	}
	lower := strings.ToLower(loc)
	return strings.Contains(lower, "utf-8") || strings.Contains(lower, "utf8")
}

// Width is the Go port of _ai_ui_width(): $COLUMNS if set to a plain
// non-negative integer, else `tput cols`, else 40 — clamped to a minimum
// of 20 so a misdetected/garbled width never shrinks a box to a few
// characters.
func Width() int {
	w := os.Getenv("COLUMNS")
	if w == "" {
		if out, err := exec.Command("tput", "cols").Output(); err == nil {
			w = strings.TrimSpace(string(out))
		}
	}
	n, err := parseNonNegativeInt(w)
	if err != nil {
		n = 40
	}
	if n < 20 {
		n = 20
	}
	return n
}

// parseNonNegativeInt mirrors the shell case pattern
// `(” |*[!0-9]*) w=40` — any empty string or any string containing a
// non-digit character is rejected (not just non-parseable strings).
func parseNonNegativeInt(s string) (int, error) {
	if s == "" {
		return 0, strconv.ErrSyntax
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, strconv.ErrSyntax
		}
	}
	return strconv.Atoi(s)
}

// Wrap is the Go port of _ai_ui_wrap(text, width): greedy word-wrap of a
// single logical line into pieces no wider than width. A single word
// longer than width is hard-cut across multiple lines rather than left to
// overflow.
//
// unicode selects the same character semantics the shell picks up
// implicitly from the active locale: when running with a UTF-8 locale
// (SupportsUnicode() true) zsh's `${#w}`/slicing operate on codepoints;
// under a non-UTF-8 locale (the ASCII-fallback case) they operate on raw
// bytes, which is what this flag reproduces. Pass the same value used for
// box/diff rendering in the same call so wrapping and box-drawing agree.
//
// An empty text returns a single empty line, matching the shell's
// `echo ""`.
func Wrap(text string, width int, unicode bool) []string {
	if width < 1 {
		width = 1
	}
	if text == "" {
		return []string{""}
	}

	var lines []string
	var cur string
	for _, w := range strings.Fields(text) {
		for wordLen(w, unicode) > width {
			if cur != "" {
				lines = append(lines, cur)
				cur = ""
			}
			head, tail := splitWord(w, width, unicode)
			lines = append(lines, head)
			w = tail
		}
		switch {
		case cur == "":
			cur = w
		case wordLen(cur, unicode)+1+wordLen(w, unicode) <= width:
			cur = cur + " " + w
		default:
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// wordLen is the length used for width comparisons: rune count in unicode
// mode, byte count otherwise (see Wrap's doc comment).
func wordLen(w string, unicode bool) int {
	if unicode {
		return utf8.RuneCountInString(w)
	}
	return len(w)
}

// splitWord cuts the first `width` units (runes or bytes, per unicode) off
// w, returning (head, remainder).
func splitWord(w string, width int, unicode bool) (string, string) {
	if unicode {
		r := []rune(w)
		return string(r[:width]), string(r[width:])
	}
	return w[:width], w[width:]
}
