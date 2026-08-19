package components

import "testing"

func TestExtractLabel(t *testing.T) {
	cases := map[string]string{
		"agent • Agent full akses":      "agent",
		"  chat   • Chat cepat  ":       "chat",
		"noseparatorhere":               "noseparatorhere",
		"config verbosity 1   • Normal": "config verbosity 1",
	}
	for in, want := range cases {
		if got := ExtractLabel(in); got != want {
			t.Fatalf("ExtractLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
