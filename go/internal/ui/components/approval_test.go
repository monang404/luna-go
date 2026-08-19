package components

import (
	"strings"
	"testing"

	"github.com/monang404/luna-go/internal/ui"
)

func testMode(unicode bool, width int) ui.Mode {
	return ui.Mode{Tokens: ui.NoColorTokens, Unicode: unicode, Width: width}
}

func TestApprovalPrompt(t *testing.T) {
	got := ApprovalPrompt("rm -rf build/", testMode(true, 40))
	if !strings.Contains(got, "Command requires approval") {
		t.Fatalf("expected title in output, got:\n%s", got)
	}
	if !strings.Contains(got, "rm -rf build/") {
		t.Fatalf("expected command in output, got:\n%s", got)
	}
	// Must be identical to calling ui.Box directly (no new rendering).
	want := ui.Box("Command requires approval", []string{"rm -rf build/"}, testMode(true, 40))
	if got != want {
		t.Fatalf("ApprovalPrompt diverged from ui.Box:\ngot:  %q\nwant: %q", got, want)
	}
}
