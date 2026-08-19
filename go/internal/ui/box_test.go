package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func readGoldenFile(t *testing.T, parts ...string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(append([]string{"testdata"}, parts...)...))
	if err != nil {
		t.Fatalf("reading golden: %v", err)
	}
	return string(data)
}

// AC-03/golden-file test: Box must byte-for-byte match _ai_ui_box output
// captured from 05-ui_box.zsh, across the approval/non-approval paths,
// color/no-color, unicode/ASCII fallback, narrow widths, the 4-line body
// cap, and literal-newline body lines. See harness/gen_box.zsh for exactly
// how each fixture was generated.
func TestBox_GoldenParity(t *testing.T) {
	cases := []struct {
		golden string
		title  string
		lines  []string
		mode   Mode
	}{
		{
			golden: "approval_basic.out",
			title:  "Approval requires approval",
			lines:  []string{"Run this command?", "$ rm -rf /tmp/foo"},
			mode:   Mode{Tokens: ColorTokens, Unicode: true, Width: 40},
		},
		{
			golden: "approval_ascii.out",
			title:  "Approval requires approval",
			lines:  []string{"Run this command?", "$ rm -rf /tmp/foo"},
			mode:   Mode{Tokens: ColorTokens, Unicode: false, Width: 40},
		},
		{
			golden: "approval_nocolor.out",
			title:  "Approval requires approval",
			lines:  []string{"Run this command?", "$ rm -rf /tmp/foo"},
			mode:   Mode{Tokens: NoColorTokens, Unicode: true, Width: 40},
		},
		{
			golden: "approval_narrow.out",
			title:  "Approval requires approval",
			lines:  []string{"Path: /very/long/path/to/some/file/that/is/quite/long/indeed.txt"},
			mode:   Mode{Tokens: ColorTokens, Unicode: true, Width: 30},
		},
		{
			golden: "approval_manylines.out",
			title:  "Approval requires approval",
			lines:  []string{"line1", "line2", "line3", "line4", "line5", "line6"},
			mode:   Mode{Tokens: ColorTokens, Unicode: true, Width: 40},
		},
		{
			golden: "approval_empty.out",
			title:  "Approval requires approval",
			lines:  nil,
			mode:   Mode{Tokens: ColorTokens, Unicode: true, Width: 40},
		},
		{
			golden: "nonapproval_basic.out",
			title:  "Result: Completed",
			lines:  []string{"line one", "line two", "---", "line three"},
			mode:   Mode{Tokens: ColorTokens, Unicode: true, Width: 40},
		},
		{
			golden: "nonapproval_notitle.out",
			title:  "",
			lines:  []string{"just a line", "another line"},
			mode:   Mode{Tokens: ColorTokens, Unicode: true, Width: 40},
		},
		{
			golden: "nonapproval_nocolor.out",
			title:  "✗ Blocked",
			lines:  []string{"something went wrong"},
			mode:   Mode{Tokens: NoColorTokens, Unicode: true, Width: 40},
		},
		{
			golden: "nonapproval_newline.out",
			title:  "Result",
			lines:  []string{"multi\nline\ncontent", "second item"},
			mode:   Mode{Tokens: ColorTokens, Unicode: true, Width: 40},
		},
	}

	for _, c := range cases {
		t.Run(c.golden, func(t *testing.T) {
			want := readGoldenFile(t, "box", c.golden)
			got := Box(c.title, c.lines, c.mode)
			if got != want {
				t.Errorf("Box(%q, %v, %+v) =\n%q\nwant\n%q", c.title, c.lines, c.mode, got, want)
			}
		})
	}
}

func TestBoxAccent(t *testing.T) {
	tok := ColorTokens
	cases := []struct {
		title string
		want  string
	}{
		{"LUNA Agent starting", tok.Accent},
		{"Something requires approval", tok.Warn},
		{"✓ Done", tok.OK},
		{"+ Added", tok.OK},
		{"Task Completed", tok.OK},
		{"✗ Failed", tok.Err},
		{"x failure", tok.Err},
		{"Something blocked", tok.Err},
		{"Task Blocked", tok.Err},
		{"Neutral header", tok.Muted},
	}
	for _, c := range cases {
		if got := BoxAccent(c.title, tok); got != c.want {
			t.Errorf("BoxAccent(%q) = %q, want %q", c.title, got, c.want)
		}
	}
}
