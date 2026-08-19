package screens

import (
	"strings"
	"testing"

	"github.com/monang404/luna-go/internal/ui/components"
)

func TestHomeFirstTimeUser(t *testing.T) {
	d := HomeData{
		Header: components.HeaderData{PwdStr: "~/proj"},
	}
	got := Home(d, testMode(true, 40))
	if !strings.Contains(got, "Bagas LUNA") {
		t.Fatalf("missing header:\n%s", got)
	}
	if !strings.Contains(got, "Contoh: \"bikin script python buat convert csv ke json\"") {
		t.Fatalf("missing first-time example:\n%s", got)
	}
	if strings.Contains(got, "Ketik prompt, ") {
		t.Fatalf("first-time user should not see returning-user copy:\n%s", got)
	}
	if !strings.HasSuffix(got, "> ") {
		t.Fatalf("expected output to end with the prompt indicator, got:\n%q", got)
	}
}

func TestHomeReturningUserWithRecentSession(t *testing.T) {
	d := HomeData{
		Header:         components.HeaderData{PwdStr: "~/proj"},
		IsReturning:    true,
		RecentSession:  "refactor-auth",
		CurrentSession: "main",
	}
	got := Home(d, testMode(true, 40))
	if !strings.Contains(got, "Sesi terakhir: ") {
		t.Fatalf("missing recent session line:\n%s", got)
	}
	if !strings.Contains(got, "luna session resume refactor-auth") {
		t.Fatalf("missing resume hint:\n%s", got)
	}
	if !strings.Contains(got, "Ketik prompt, ") {
		t.Fatalf("missing returning-user copy:\n%s", got)
	}
}

func TestHomeContextLines(t *testing.T) {
	d := HomeData{
		Header:         components.HeaderData{PwdStr: "~/proj"},
		ModifiedCount:  3,
		CurrentSession: "feature-x",
	}
	got := Home(d, testMode(true, 40))
	if !strings.Contains(got, "3 file belum di-commit") {
		t.Fatalf("missing dirty-file context:\n%s", got)
	}
	if !strings.Contains(got, "Session aktif: feature-x") {
		t.Fatalf("missing active session context:\n%s", got)
	}
}

func TestHomeMainSessionNotShownAsActive(t *testing.T) {
	d := HomeData{
		Header:         components.HeaderData{PwdStr: "~/proj"},
		CurrentSession: "main",
	}
	got := Home(d, testMode(true, 40))
	if strings.Contains(got, "Session aktif:") {
		t.Fatalf("main session should not be shown as an active context line:\n%s", got)
	}
}
