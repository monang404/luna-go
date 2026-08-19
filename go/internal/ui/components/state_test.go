package components

import "testing"

func TestStateThinkingGatedByVerbosity(t *testing.T) {
	r0 := StateThinking("Searching files...", 0, testMode(true, 40))
	if r0.Output != "" {
		t.Fatalf("expected no output at verbosity 0, got %q", r0.Output)
	}
	if r0.LogLine != "[thinking] Searching files..." {
		t.Fatalf("LogLine = %q", r0.LogLine)
	}

	r1 := StateThinking("Searching files...", 1, testMode(true, 40))
	if r1.Output != "● Searching files...\n" {
		t.Fatalf("Output = %q", r1.Output)
	}

	rAscii := StateThinking("", 1, testMode(false, 40))
	if rAscii.Output != "* Thinking...\n" {
		t.Fatalf("Output = %q", rAscii.Output)
	}
}

func TestStateActing(t *testing.T) {
	r := StateActing("rg", "Found 24 files", 1, testMode(true, 40))
	if r.Output != "→ rg  Found 24 files\n" {
		t.Fatalf("Output = %q", r.Output)
	}
	if r.LogLine != "[acting] rg | Found 24 files" {
		t.Fatalf("LogLine = %q", r.LogLine)
	}

	rNoDetail := StateActing("rg", "", 1, testMode(true, 40))
	if rNoDetail.Output != "→ rg\n" {
		t.Fatalf("Output = %q", rNoDetail.Output)
	}
	if rNoDetail.LogLine != "[acting] rg" {
		t.Fatalf("LogLine = %q", rNoDetail.LogLine)
	}

	rAscii := StateActing("rg", "x", 1, testMode(false, 40))
	if rAscii.Output != "> rg  x\n" {
		t.Fatalf("Output = %q", rAscii.Output)
	}
}

func TestStateWaitingAlwaysShown(t *testing.T) {
	r := StateWaiting("rm -rf build/", testMode(true, 40))
	want := "⚠  Needs approval\n  rm -rf build/\n"
	if r.Output != want {
		t.Fatalf("Output = %q, want %q", r.Output, want)
	}
	if r.LogLine != "[approval] rm -rf build/" {
		t.Fatalf("LogLine = %q", r.LogLine)
	}

	rAscii := StateWaiting("rm -rf build/", testMode(false, 40))
	if rAscii.Output != "! Needs approval: rm -rf build/\n" {
		t.Fatalf("Output = %q", rAscii.Output)
	}
}

func TestStateDone(t *testing.T) {
	r := StateDone("3 files changed", "42s", testMode(true, 40))
	if r.Output != "✓ Done  ·  3 files changed  ·  42s\n" {
		t.Fatalf("Output = %q", r.Output)
	}
	rBare := StateDone("", "", testMode(false, 40))
	if rBare.Output != "+ Done\n" {
		t.Fatalf("Output = %q", rBare.Output)
	}
}

func TestStateErrorDefault(t *testing.T) {
	r := StateError("", testMode(true, 40))
	if r.Output != "✗ Error\n" {
		t.Fatalf("Output = %q", r.Output)
	}
	if r.LogLine != "[error] Error" {
		t.Fatalf("LogLine = %q", r.LogLine)
	}
}

func TestStateToolAndDebugGates(t *testing.T) {
	rTool1 := StateTool("rg", "pattern", 1, testMode(true, 40))
	if rTool1.Output != "" {
		t.Fatalf("expected gated tool output at level 1, got %q", rTool1.Output)
	}
	rTool2 := StateTool("rg", "pattern", 2, testMode(true, 40))
	if rTool2.Output != "Tool: rg  pattern\n" {
		t.Fatalf("Output = %q", rTool2.Output)
	}

	rDbg2 := StateDebug("raw line", 2, testMode(true, 40))
	if rDbg2.Output != "" {
		t.Fatalf("expected gated debug output at level 2, got %q", rDbg2.Output)
	}
	rDbg3 := StateDebug("raw line", 3, testMode(true, 40))
	if rDbg3.Output != "[DEBUG] raw line\n" {
		t.Fatalf("Output = %q", rDbg3.Output)
	}
}
