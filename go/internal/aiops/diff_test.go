package aiops

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_NoChange(t *testing.T) {
	d := UnifiedDiff("f.txt", "a\nb\nc\n", "a\nb\nc\n")
	if strings.Contains(d, "@@") {
		t.Errorf("expected no hunks for identical content, got: %q", d)
	}
}

func TestUnifiedDiff_SimpleChange(t *testing.T) {
	old := "one\ntwo\nthree\n"
	new := "one\ntwo-changed\nthree\n"
	d := UnifiedDiff("f.txt", old, new)

	if !strings.Contains(d, "--- f.txt") || !strings.Contains(d, "+++ f.txt") {
		t.Errorf("missing diff headers: %q", d)
	}
	if !strings.Contains(d, "-two") {
		t.Errorf("missing removed line: %q", d)
	}
	if !strings.Contains(d, "+two-changed") {
		t.Errorf("missing added line: %q", d)
	}
	if !strings.Contains(d, " one") || !strings.Contains(d, " three") {
		t.Errorf("missing context lines: %q", d)
	}
}

func TestUnifiedDiff_Addition(t *testing.T) {
	old := "one\ntwo\n"
	new := "one\ntwo\nthree\n"
	d := UnifiedDiff("f.txt", old, new)
	if !strings.Contains(d, "+three") {
		t.Errorf("missing appended line: %q", d)
	}
}
