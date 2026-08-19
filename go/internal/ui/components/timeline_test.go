package components

import "testing"

func TestTimelineUnicode(t *testing.T) {
	steps := []string{"Plan", "Search", "Edit", "Test"}
	got := Timeline(steps, 3, testMode(true, 40))
	want := "" +
		"  ✓ Plan\n" +
		"  ✓ Search\n" +
		"  ● Edit\n" +
		"  ○ Test\n"
	if got != want {
		t.Fatalf("Timeline = %q, want %q", got, want)
	}
}

func TestTimelineAsciiDoneIconSwaps(t *testing.T) {
	steps := []string{"Plan", "Search"}
	got := Timeline(steps, 2, testMode(false, 40))
	want := "" +
		"  + Plan\n" +
		"  ● Search\n"
	if got != want {
		t.Fatalf("Timeline = %q, want %q", got, want)
	}
}

func TestTimelineSkipsEmptyStepsButKeepsIndex(t *testing.T) {
	steps := []string{"Plan", "", "Edit"}
	// index 2 (the empty one) is skipped for output but still occupies
	// the position, so "Edit" is idx=3.
	got := Timeline(steps, 3, testMode(true, 40))
	want := "" +
		"  ✓ Plan\n" +
		"  ● Edit\n"
	if got != want {
		t.Fatalf("Timeline = %q, want %q", got, want)
	}
}
