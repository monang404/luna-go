package components

import (
	"strings"
	"testing"
)

func TestDetailLogEmpty(t *testing.T) {
	var d DetailLog
	got := d.Show(testMode(true, 20))
	if got != "(Tidak ada detail log untuk ditampilkan)\n" {
		t.Fatalf("Show = %q", got)
	}
}

func TestDetailLogPushClearShow(t *testing.T) {
	var d DetailLog
	d.Push("[thinking] Searching...")
	d.Push("[done] 3 files changed")
	d.Push("[error] boom")
	d.Push("[acting] rg | Found 24 files")
	d.Push("[tool] rg pattern")
	d.Push("[approval] rm -rf")
	d.Push("[debug] raw")
	d.Push("plain unlabeled line")

	got := d.Show(testMode(true, 12))
	if !strings.Contains(got, "Detail Log") {
		t.Fatalf("expected header, got:\n%s", got)
	}
	if !strings.Contains(got, "◌ Searching...") {
		t.Fatalf("missing thinking line:\n%s", got)
	}
	if !strings.Contains(got, "✓ 3 files changed") {
		t.Fatalf("missing done line:\n%s", got)
	}
	if !strings.Contains(got, "✗ boom") {
		t.Fatalf("missing error line:\n%s", got)
	}
	if !strings.Contains(got, "→ rg | Found 24 files") {
		t.Fatalf("missing acting line:\n%s", got)
	}
	if !strings.Contains(got, "  Tool: rg pattern") {
		t.Fatalf("missing tool line:\n%s", got)
	}
	if !strings.Contains(got, "⚠ rm -rf") {
		t.Fatalf("missing approval line:\n%s", got)
	}
	if !strings.Contains(got, "[DBG] raw") {
		t.Fatalf("missing debug line:\n%s", got)
	}
	if !strings.Contains(got, "  plain unlabeled line") {
		t.Fatalf("missing plain line:\n%s", got)
	}

	d.Clear()
	got2 := d.Show(testMode(true, 12))
	if got2 != "(Tidak ada detail log untuk ditampilkan)\n" {
		t.Fatalf("after Clear, Show = %q", got2)
	}
}
