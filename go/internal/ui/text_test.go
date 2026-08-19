package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// wrapCase mirrors one run_case(width, text) call in
// harness/gen_text.zsh -- keep these two lists in sync if fixtures change.
type wrapCase struct {
	width int
	text  string
}

var wrapCases = []wrapCase{
	{10, "hello world"},
	{5, "supercalifragilistic"},
	{20, ""},
	{10, "a b c d e f g h"},
	{8, "unicode: héllo wörld ünïcödé test strîng"},
	{15, "   leading and trailing space text here   "},
	{3, "ab"},
	{40, "The quick brown fox jumps over the lazy dog and keeps running"},
	{1, "x yy zzz"},
	{6, "日本語テキストです"},
}

// readGoldenLines reads a golden/text/<mode>/case_N.out fixture (one word
// per line, each terminated with "\n" the way _ai_ui_wrap's repeated
// `echo` produced it) and returns the lines with trailing newlines
// stripped, matching what Wrap returns.
func readGoldenLines(t *testing.T, mode string, n int) []string {
	t.Helper()
	path := filepath.Join("testdata", "text", mode, "case_"+itoa(n)+".out")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading golden %s: %v", path, err)
	}
	s := string(data)
	s = strings.TrimSuffix(s, "\n")
	if s == "" {
		return []string{""}
	}
	return strings.Split(s, "\n")
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}

// AC (golden-file test): Wrap must byte-for-byte match _ai_ui_wrap output,
// captured from the real 00-ui_text.zsh, in both the UTF-8-locale
// (unicode=true, rune counting) and C-locale (unicode=false, byte
// counting) regimes -- see harness/gen_text.zsh for how these were
// generated and text.go's Wrap doc comment for why the flag exists.
func TestWrap_GoldenParity(t *testing.T) {
	for _, mode := range []string{"uni", "ascii"} {
		unicode := mode == "uni"
		for i, c := range wrapCases {
			n := i + 1
			t.Run(mode+"/case_"+itoa(n), func(t *testing.T) {
				want := readGoldenLines(t, mode, n)
				got := Wrap(c.text, c.width, unicode)
				if !equalSlices(got, want) {
					t.Errorf("Wrap(%q, %d, unicode=%v) = %q, want %q", c.text, c.width, unicode, got, want)
				}
			})
		}
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// Edge cases the golden fixtures don't otherwise isolate.
func TestWrap_EdgeCases(t *testing.T) {
	if got := Wrap("", 10, true); !equalSlices(got, []string{""}) {
		t.Errorf(`Wrap("", 10, true) = %q, want [""]`, got)
	}
	// width < 1 clamps to 1, same as the shell's `[ "$width" -lt 1 ] && width=1`.
	got := Wrap("ab", 0, true)
	want := []string{"a", "b"}
	if !equalSlices(got, want) {
		t.Errorf("Wrap(\"ab\", 0, true) = %q, want %q", got, want)
	}
}

func TestSupportsUnicode(t *testing.T) {
	cases := []struct {
		name                 string
		asciiOverride        string
		lcAll, lcCtype, lang string
		want                 bool
	}{
		{"AI_UI_ASCII_FALLBACK=1 forces ascii even with utf8 lang", "1", "en_US.UTF-8", "", "", false},
		{"LC_ALL utf-8", "", "en_US.UTF-8", "", "", true},
		{"LC_CTYPE utf8 (no dash)", "", "", "C.utf8", "", true},
		{"LANG utf-8 fallback", "", "", "", "en_US.UTF-8", true},
		{"POSIX locale -> ascii", "", "POSIX", "", "", false},
		{"C locale -> ascii", "", "C", "", "", false},
		{"all unset -> ascii", "", "", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("AI_UI_ASCII_FALLBACK", c.asciiOverride)
			t.Setenv("LC_ALL", c.lcAll)
			t.Setenv("LC_CTYPE", c.lcCtype)
			t.Setenv("LANG", c.lang)
			if got := SupportsUnicode(); got != c.want {
				t.Errorf("SupportsUnicode() = %v, want %v", got, c.want)
			}
		})
	}
}

func TestWidth(t *testing.T) {
	cases := []struct {
		columns string
		want    int
	}{
		{"80", 80},
		{"5", 20},   // clamped to minimum 20
		{"", -1},    // handled separately below (depends on tput availability)
		{"abc", -1}, // non-digit -> falls back; exact value depends on tput/40, checked below
	}
	for _, c := range cases {
		if c.want == -1 {
			continue
		}
		t.Run(c.columns, func(t *testing.T) {
			t.Setenv("COLUMNS", c.columns)
			if got := Width(); got != c.want {
				t.Errorf("Width() with COLUMNS=%q = %d, want %d", c.columns, got, c.want)
			}
		})
	}

	t.Run("non-digit COLUMNS falls back and is clamped >=20", func(t *testing.T) {
		t.Setenv("COLUMNS", "abc")
		if got := Width(); got < 20 {
			t.Errorf("Width() = %d, want >= 20", got)
		}
	})
}
