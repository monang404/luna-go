package chat

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/monang404/luna-go/internal/config"
)

func newTestStore(t *testing.T) *SessionStore {
	t.Helper()
	return &SessionStore{Dir: t.TempDir()}
}

func TestSessionStore_LoadCreatesNewSession(t *testing.T) {
	st := newTestStore(t)
	msgs, err := st.Load("main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(msgs) != 1 || msgs[0].Role != "system" || msgs[0].Content != config.PersonaChatLong {
		t.Errorf("expected a fresh session with just the persona system message, got %+v", msgs)
	}
	if !st.Exists("main") {
		t.Error("expected session file to now exist")
	}
}

func TestSessionStore_SanitizesLabelPrefix(t *testing.T) {
	st := newTestStore(t)
	// Write a pre-fix-contaminated session file directly, bypassing Load.
	contaminated := `[{"role":"system","content":"persona"},{"role":"assistant","content":"llama > gemini > jawaban bersih"}]`
	os.MkdirAll(st.Dir, 0o755)
	os.WriteFile(filepath.Join(st.Dir, "main.json"), []byte(contaminated), 0o644)

	msgs, err := st.Load("main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msgs[1].Content != "jawaban bersih" {
		t.Errorf("expected label prefix stripped, got %q", msgs[1].Content)
	}
}

func TestSessionAsk_CommitsOnlyAfterSuccess(t *testing.T) {
	withFakeKey(t)
	st := newTestStore(t)
	svc := NewService(&fakeCompleter{content: "balasan LUNA"})

	res, err := svc.SessionAsk(context.Background(), st, "main", "halo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Answer != "balasan LUNA" {
		t.Errorf("Answer = %q", res.Answer)
	}

	msgs, _ := st.Load("main")
	if len(msgs) != 3 {
		t.Fatalf("expected [system,user,assistant], got %d messages: %+v", len(msgs), msgs)
	}
	if msgs[1].Role != "user" || msgs[1].Content != "halo" {
		t.Errorf("unexpected user message: %+v", msgs[1])
	}
	if msgs[2].Role != "assistant" || msgs[2].Content != "balasan LUNA" {
		t.Errorf("unexpected assistant message: %+v", msgs[2])
	}
}

func TestSessionAsk_FailureLeavesSessionUnchanged(t *testing.T) {
	withFakeKey(t)
	st := newTestStore(t)
	st.Load("main")
	svc := NewService(&fakeCompleter{err: context.DeadlineExceeded})

	_, err := svc.SessionAsk(context.Background(), st, "main", "halo")
	if err == nil {
		t.Fatal("expected an error")
	}
	msgs, _ := st.Load("main")
	if len(msgs) != 1 {
		t.Errorf("expected session untouched (just system message) after a failed turn, got %d messages", len(msgs))
	}
}

func TestSessionAsk_EmptyMessageIsNoop(t *testing.T) {
	withFakeKey(t)
	st := newTestStore(t)
	svc := NewService(&fakeCompleter{content: "should not be called"})
	res, err := svc.SessionAsk(context.Background(), st, "main", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Answer != "" {
		t.Errorf("expected empty result for empty message, got %+v", res)
	}
}

func TestSessionStore_List(t *testing.T) {
	st := newTestStore(t)
	st.Load("alpha")
	st.Load("beta")
	names, err := st.List()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(names) != 2 {
		t.Fatalf("expected 2 sessions, got %d: %v", len(names), names)
	}
}

func TestSessionStore_Clear(t *testing.T) {
	withFakeKey(t)
	st := newTestStore(t)
	svc := NewService(&fakeCompleter{content: "hi"})
	svc.SessionAsk(context.Background(), st, "main", "halo")

	if err := st.Clear("main"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	msgs, _ := st.Load("main")
	if len(msgs) != 1 {
		t.Errorf("expected session reset to just the system message, got %d", len(msgs))
	}
}

func TestSessionEnd_ArchivesFile(t *testing.T) {
	st := newTestStore(t)
	st.Load("main")
	if err := st.End("main"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if st.Exists("main") {
		t.Error("expected session file to be moved out of the active dir")
	}
	archiveEntries, _ := os.ReadDir(filepath.Join(st.Dir, "archive"))
	if len(archiveEntries) != 1 {
		t.Fatalf("expected 1 archived file, got %d", len(archiveEntries))
	}
	if !strings.HasPrefix(archiveEntries[0].Name(), "main_") {
		t.Errorf("unexpected archive filename: %s", archiveEntries[0].Name())
	}
}

func TestSessionEnd_NoActiveSession(t *testing.T) {
	st := newTestStore(t)
	if err := st.End(""); err == nil {
		t.Error("expected an error when there's no active session name")
	}
}

func TestSessionPrune_RemovesOldArchives(t *testing.T) {
	st := newTestStore(t)
	archiveDir := filepath.Join(st.Dir, "archive")
	os.MkdirAll(archiveDir, 0o755)
	oldFile := filepath.Join(archiveDir, "old_20200101_000000_0001.json")
	newFile := filepath.Join(archiveDir, "new_20990101_000000_0001.json")
	os.WriteFile(oldFile, []byte("[]"), 0o644)
	os.WriteFile(newFile, []byte("[]"), 0o644)
	oldTime := time.Now().AddDate(0, 0, -60)
	os.Chtimes(oldFile, oldTime, oldTime)

	res, err := st.SessionPrune(30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Removed != 1 {
		t.Errorf("expected 1 removed, got %d", res.Removed)
	}
	if _, err := os.Stat(oldFile); !os.IsNotExist(err) {
		t.Error("old archive should have been removed")
	}
	if _, err := os.Stat(newFile); err != nil {
		t.Error("new archive should still exist")
	}
}

func TestSessionPrune_NoArchiveDir(t *testing.T) {
	st := newTestStore(t)
	res, err := st.SessionPrune(30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Removed != 0 {
		t.Errorf("expected 0 removed, got %d", res.Removed)
	}
}
