package codeproject

import (
	"context"
	"strings"
	"testing"
)

func TestSniffStructure_ExtractsLongAnchorText(t *testing.T) {
	html := `
	<a href="/a" class="card-title">Short</a>
	<a href="/b" class="headline">This is a much longer headline text that exceeds thirty chars</a>
	<a class="no-href">No href here, some long text over thirty characters</a>
	`
	got := sniffStructure(html)
	if !strings.Contains(got, "class: headline") {
		t.Errorf("expected the long-text anchor with an href to be captured, got: %q", got)
	}
	if strings.Contains(got, "Short") {
		t.Error("short anchor text (<=30 chars) should be filtered out")
	}
	if strings.Contains(got, "no-href") {
		t.Error("anchors without an href should be filtered out")
	}
}

func TestSniffStructure_DedupesByClass(t *testing.T) {
	html := `
	<a href="/a" class="card">First long enough anchor text here for sure</a>
	<a href="/b" class="card">Second long enough anchor text here for sure</a>
	`
	got := sniffStructure(html)
	count := strings.Count(got, "class: card")
	if count != 1 {
		t.Errorf("expected exactly 1 line for class=card (dedup), got %d in %q", count, got)
	}
}

func TestSniffStructure_CapsAtTenLines(t *testing.T) {
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString(`<a href="/x" class="c` + string(rune('a'+i)) + `">This text is definitely longer than thirty characters ok</a>` + "\n")
	}
	got := sniffStructure(b.String())
	lines := strings.Split(strings.TrimSpace(got), "\n")
	if len(lines) != 10 {
		t.Errorf("expected at most 10 lines, got %d", len(lines))
	}
}

func TestScrap_RejectsBadScheme(t *testing.T) {
	withFakeKey(t)
	svc := &Service{Requester: &fakeCompleter{contents: []string{"code"}}, Runner: &fakeRunner{}}
	_, err := svc.Scrap(context.Background(), "ftp://example.com", "scrape it")
	if err == nil {
		t.Fatal("expected an error for a non-http(s) scheme")
	}
}

func TestScrap_NoProvider(t *testing.T) {
	svc := &Service{Requester: &fakeCompleter{contents: []string{"code"}}, Runner: &fakeRunner{}}
	_, err := svc.Scrap(context.Background(), "https://example.com", "scrape it")
	if err != ErrCodeNoProvider {
		t.Errorf("expected ErrCodeNoProvider, got %v", err)
	}
}
