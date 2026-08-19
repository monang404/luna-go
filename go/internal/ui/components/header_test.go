package components

import (
	"testing"

	"github.com/monang404/luna-go/internal/ui"
)

func TestHeaderNoModelYet(t *testing.T) {
	got := Header(HeaderData{PwdStr: "~/proj"}, ui.NoColorTokens)
	want := "Bagas LUNA • main • no model yet • ~/proj\n"
	if got != want {
		t.Fatalf("Header = %q, want %q", got, want)
	}
}

func TestHeaderFull(t *testing.T) {
	d := HeaderData{
		Provider:  "groq",
		Model:     "llama-3.1-8b-instant",
		Session:   "refactor",
		PwdStr:    "~/proj",
		TokenStr:  "1234",
		InGitRepo: true,
		GitBranch: "main",
		GitDirty:  2,
	}
	got := Header(d, ui.NoColorTokens)
	want := "Bagas LUNA • refactor • groq/llama-3.1-8b-instant • ~/proj • main dirty 2f • 1234tok\n"
	if got != want {
		t.Fatalf("Header = %q, want %q", got, want)
	}
}

func TestHeaderCleanGit(t *testing.T) {
	d := HeaderData{Model: "gpt", PwdStr: "~", InGitRepo: true, GitBranch: "main", GitDirty: 0}
	got := Header(d, ui.NoColorTokens)
	want := "Bagas LUNA • main • gpt • ~ • main\n"
	if got != want {
		t.Fatalf("Header = %q, want %q", got, want)
	}
}
