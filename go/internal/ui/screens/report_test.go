package screens

import "testing"

func TestReportFull(t *testing.T) {
	got := Report("4", "58s", "rg,go build", "luna commit", []string{"Fixed bug A", "Added test B"}, testMode(true, 40))
	want := "✓ Done  ·  Files: 4  Tools: rg,go build  ·  58s\n" +
		"  ✓ Fixed bug A\n" +
		"  ✓ Added test B\n" +
		"\n" +
		"  Next: luna commit\n" +
		"\n"
	if got != want {
		t.Fatalf("Report = %q, want %q", got, want)
	}
}

func TestReportDefaults(t *testing.T) {
	got := Report("", "", "", "", nil, testMode(true, 40))
	want := "✓ Done  ·  Files: 0  ·  ?\n\n"
	if got != want {
		t.Fatalf("Report = %q, want %q", got, want)
	}
}

func TestReportNoNextAction(t *testing.T) {
	got := Report("1", "10s", "", "", []string{"Done thing"}, testMode(false, 40))
	// StateDone's own "Done" line respects ascii mode ("+"), but the
	// per-item bullet below it is a literal "✓" in the zsh source
	// (report.zsh's summary_items loop never checks unicode support) —
	// preserved as-is (bug-for-bug parity), not a Go-side inconsistency.
	want := "+ Done  ·  Files: 1  ·  10s\n" +
		"  ✓ Done thing\n" +
		"\n"
	if got != want {
		t.Fatalf("Report = %q, want %q", got, want)
	}
}
