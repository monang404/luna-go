package ui

import (
	"os"
	"path/filepath"
	"testing"
)

// AC-04/golden-file test: DiffHeader + ColorizeDiffBody + DiffFooter,
// concatenated, must byte-for-byte match the shell pipeline
// `_ai_ui_diff_header path; diff -u ... | sed -e 's/^-/AI_C_ERR-/'
// -e 's/^+/AI_C_OK+/' -e 's/$/AI_C_RESET/'; _ai_ui_diff_footer`
// captured from the real zsh sources, across >=5 representative diff
// fixtures (small, addition-heavy, deletion-heavy, mixed context/header,
// multi-file) plus color/no-color and unicode/ASCII variants. See
// harness/gen_diff.zsh for exactly how each fixture was generated.
func TestDiff_GoldenParity(t *testing.T) {
	cases := []struct {
		golden  string
		path    string
		fixture string
		mode    Mode
	}{
		{"small_color_uni.out", "foo.txt", "small.diff", Mode{Tokens: ColorTokens, Unicode: true, Width: 40}},
		{"small_color_ascii.out", "foo.txt", "small.diff", Mode{Tokens: ColorTokens, Unicode: false, Width: 40}},
		{"small_nocolor.out", "foo.txt", "small.diff", Mode{Tokens: NoColorTokens, Unicode: true, Width: 40}},
		{"addition_heavy.out", "new.txt", "addition_heavy.diff", Mode{Tokens: ColorTokens, Unicode: true, Width: 30}},
		{"deletion_heavy.out", "old.txt", "deletion_heavy.diff", Mode{Tokens: ColorTokens, Unicode: true, Width: 30}},
		{"mixed_context_header.out", "mixed.txt", "mixed_context_header.diff", Mode{Tokens: ColorTokens, Unicode: true, Width: 50}},
		{"multi_file.out", "multi.txt", "multi_file.diff", Mode{Tokens: ColorTokens, Unicode: true, Width: 60}},
		{"multi_file_nocolor_ascii.out", "multi.txt", "multi_file.diff", Mode{Tokens: NoColorTokens, Unicode: false, Width: 60}},
	}

	if len(cases) < 5 {
		t.Fatalf("need >=5 diff fixtures per AC-04, have %d", len(cases))
	}

	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			want := readGoldenFile(t, "diff", c.golden)
			diffBody, err := os.ReadFile(filepath.Join("testdata", "diff", "fixtures", c.fixture))
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			got := DiffHeader(c.path, c.mode) + ColorizeDiffBody(string(diffBody), c.mode.Tokens) + DiffFooter(c.mode)
			if got != want {
				t.Errorf("diff render mismatch for %s:\ngot:\n%q\nwant:\n%q", c.golden, got, want)
			}
		})
	}
}

// AC-02/AC-04 combined: NO_COLOR must leave the rendered diff (header +
// colorized body + footer) completely free of ANSI escape bytes, not just
// individual tokens.
func TestDiff_NoColor_NoEscapeBytes(t *testing.T) {
	mode := Mode{Tokens: NoColorTokens, Unicode: true, Width: 40}
	diffBody := "--- a/x\n+++ b/x\n@@ -1,1 +1,1 @@\n-old\n+new\n"
	out := DiffHeader("x", mode) + ColorizeDiffBody(diffBody, mode.Tokens) + DiffFooter(mode)
	for i := 0; i < len(out); i++ {
		if out[i] == 0x1b {
			t.Fatalf("found ESC byte in NO_COLOR diff output: %q", out)
		}
	}
}

func TestDiffHeader_LiteralTextPreserved(t *testing.T) {
	mode := Mode{Tokens: NoColorTokens, Unicode: true, Width: 40}
	got := DiffHeader("some/path.go", mode)
	want := "\nDiff yang diusulkan: some/path.go \n"
	// hz is "──" (unicode) around the literal text; with NoColorTokens the
	// color wrapper is empty but the hz markers remain.
	wantWithHz := "\n" + "──" + " Diff yang diusulkan: some/path.go " + "──" + "\n"
	if got != wantWithHz {
		t.Errorf("DiffHeader() = %q, want %q (sanity want without hz: %q)", got, wantWithHz, want)
	}
}
