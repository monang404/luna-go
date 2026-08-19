package components

import "testing"

func TestSplitContentLines(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", []string{""}},
		{"a", []string{"a"}},
		{"a\nb", []string{"a", "b"}},
		{"a\nb\n", []string{"a", "b"}},
	}
	for _, c := range cases {
		got := splitContentLines(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("splitContentLines(%q) = %#v, want %#v", c.in, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("splitContentLines(%q)[%d] = %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

func TestCardSummaryDefaultTitle(t *testing.T) {
	out := CardSummary("", "hello", testMode(true, 40))
	if len(out) == 0 {
		t.Fatal("expected non-empty output")
	}
}

func TestCardStats(t *testing.T) {
	got := CardStats("3", "42s", "rg,cat", testMode(true, 40))
	// NoColorTokens has empty escape codes, so color placeholders vanish.
	want := "✓ Files changed: 3  ·  42s  ·  Tools: rg,cat\n"
	if got != want {
		t.Fatalf("CardStats unicode = %q, want %q", got, want)
	}

	gotAscii := CardStats("0", "", "", testMode(false, 40))
	wantAscii := "+ Files changed: 0\n"
	if gotAscii != wantAscii {
		t.Fatalf("CardStats ascii = %q, want %q", gotAscii, wantAscii)
	}
}
