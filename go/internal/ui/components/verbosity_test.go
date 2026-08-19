package components

import (
	"testing"

	"github.com/monang404/luna-go/internal/ui"
)

func TestVerboseGate(t *testing.T) {
	if got := Verbose(0, 1, "hidden"); got != "" {
		t.Fatalf("Verbose = %q, want empty", got)
	}
	if got := Verbose(1, 1, "shown"); got != "shown\n" {
		t.Fatalf("Verbose = %q", got)
	}
}

func TestVerboseGE(t *testing.T) {
	if !VerboseGE(2, 1) {
		t.Fatal("expected true")
	}
	if VerboseGE(0, 1) {
		t.Fatal("expected false")
	}
}

func TestVerbositySetValid(t *testing.T) {
	msg, ok := VerbositySet("2", ui.NoColorTokens)
	if !ok {
		t.Fatal("expected ok=true")
	}
	want := "✓ Verbosity → 2  (Detailed (tool+file))\n"
	if msg != want {
		t.Fatalf("msg = %q, want %q", msg, want)
	}
}

func TestVerbositySetInvalid(t *testing.T) {
	msg, ok := VerbositySet("9", ui.NoColorTokens)
	if ok {
		t.Fatal("expected ok=false")
	}
	want := "Verbosity harus 0-3 (dapat: 9)\n"
	if msg != want {
		t.Fatalf("msg = %q, want %q", msg, want)
	}
}
