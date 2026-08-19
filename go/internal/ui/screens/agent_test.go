package screens

import (
	"strings"
	"testing"

	"github.com/monang404/luna-go/internal/ui"
)

func testMode(unicode bool, width int) ui.Mode {
	return ui.Mode{Tokens: ui.NoColorTokens, Unicode: unicode, Width: width}
}

func TestAgentStartDefaultGoal(t *testing.T) {
	got := AgentStart("", "?", 1, testMode(true, 40))
	// verbosity 1 -> StateStep prints "● Running...\n"; total=="?" -> no Steps line
	want := "● Running...\n\n"
	if got != want {
		t.Fatalf("AgentStart = %q, want %q", got, want)
	}
}

func TestAgentStartWithSteps(t *testing.T) {
	got := AgentStart("Refactor auth", "5", 1, testMode(true, 40))
	want := "● Refactor auth\n  Steps: 5\n\n"
	if got != want {
		t.Fatalf("AgentStart = %q, want %q", got, want)
	}
}

func TestAgentStartVerbosityZeroHidesStep(t *testing.T) {
	got := AgentStart("Refactor auth", "5", 0, testMode(true, 40))
	want := "  Steps: 5\n\n"
	if got != want {
		t.Fatalf("AgentStart = %q, want %q", got, want)
	}
}

func TestAgentDashboardComposesTimelineAndOutput(t *testing.T) {
	steps := "Plan\nSearch\nEdit\nTest"
	got := AgentDashboard("Editing files", steps, 3, "$ go build ./...", "", 1, testMode(true, 40))
	if !strings.Contains(got, "● Editing files\n") {
		t.Fatalf("missing action step:\n%s", got)
	}
	if !strings.Contains(got, "Progress 3/4\n") {
		t.Fatalf("missing progress line:\n%s", got)
	}
	if !strings.Contains(got, "  ✓ Plan\n") || !strings.Contains(got, "  ✓ Search\n") {
		t.Fatalf("missing done steps:\n%s", got)
	}
	if !strings.Contains(got, "  ● Edit\n") {
		t.Fatalf("missing active step:\n%s", got)
	}
	if !strings.Contains(got, "  ○ Test\n") {
		t.Fatalf("missing pending step:\n%s", got)
	}
	if !strings.Contains(got, "$ go build ./...") {
		t.Fatalf("missing output line:\n%s", got)
	}
}

func TestAgentDashboardEmptyStepsStillWorks(t *testing.T) {
	got := AgentDashboard("Thinking", "", 1, "", "", 1, testMode(true, 40))
	want := "● Thinking\n\n"
	if got != want {
		t.Fatalf("AgentDashboard = %q, want %q", got, want)
	}
}

func TestAgentDone(t *testing.T) {
	got := AgentDone("3", "42s", []string{"Fixed bug A", "Added test B"}, testMode(true, 40))
	want := "✓ Done  ·  Files: 3  ·  42s\n  ✓ Fixed bug A\n  ✓ Added test B\n\n"
	if got != want {
		t.Fatalf("AgentDone = %q, want %q", got, want)
	}
}

func TestAgentDoneDefaults(t *testing.T) {
	got := AgentDone("", "", nil, testMode(true, 40))
	want := "✓ Done  ·  Files: 0  ·  ?\n\n"
	if got != want {
		t.Fatalf("AgentDone = %q, want %q", got, want)
	}
}

func TestAgentErrorDefault(t *testing.T) {
	got := AgentError("", testMode(true, 40))
	want := "✗ Unknown error\n\n"
	if got != want {
		t.Fatalf("AgentError = %q, want %q", got, want)
	}
}
