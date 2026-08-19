package chat

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestSessionRepl_ExitCommand(t *testing.T) {
	st := newTestStore(t)
	svc := NewService(&fakeCompleter{content: "unused"})
	in := strings.NewReader("/exit\n")
	var out bytes.Buffer

	err := svc.SessionRepl(context.Background(), st, "main", in, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out.String(), `Keluar dari session "main"`) {
		t.Errorf("expected exit message, got: %s", out.String())
	}
}

func TestSessionRepl_EOFExitsCleanly(t *testing.T) {
	st := newTestStore(t)
	svc := NewService(&fakeCompleter{content: "unused"})
	in := strings.NewReader("") // immediate EOF
	var out bytes.Buffer

	if err := svc.SessionRepl(context.Background(), st, "main", in, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSessionRepl_HelpCommand(t *testing.T) {
	st := newTestStore(t)
	svc := NewService(&fakeCompleter{content: "unused"})
	in := strings.NewReader("/help\n/exit\n")
	var out bytes.Buffer

	svc.SessionRepl(context.Background(), st, "main", in, &out)
	if !strings.Contains(out.String(), "Perintah session:") {
		t.Errorf("expected help text, got: %s", out.String())
	}
}

func TestSessionRepl_UnknownCommand(t *testing.T) {
	st := newTestStore(t)
	svc := NewService(&fakeCompleter{content: "unused"})
	in := strings.NewReader("/bogus\n/exit\n")
	var out bytes.Buffer

	svc.SessionRepl(context.Background(), st, "main", in, &out)
	if !strings.Contains(out.String(), "Perintah tidak dikenal") {
		t.Errorf("expected unknown-command message, got: %s", out.String())
	}
}

func TestSessionRepl_OrdinaryMessageGoesToSessionAsk(t *testing.T) {
	withFakeKey(t)
	st := newTestStore(t)
	svc := NewService(&fakeCompleter{content: "balasan dari LUNA"})
	in := strings.NewReader("halo dunia\n/exit\n")
	var out bytes.Buffer

	svc.SessionRepl(context.Background(), st, "main", in, &out)
	if !strings.Contains(out.String(), "balasan dari LUNA") {
		t.Errorf("expected LUNA reply in output, got: %s", out.String())
	}
	msgs, _ := st.Load("main")
	if len(msgs) != 3 {
		t.Errorf("expected the turn to be committed to the session, got %d messages", len(msgs))
	}
}

func TestSessionRepl_ClearCommand(t *testing.T) {
	withFakeKey(t)
	st := newTestStore(t)
	svc := NewService(&fakeCompleter{content: "balasan"})
	in := strings.NewReader("halo\n/clear\n/exit\n")
	var out bytes.Buffer

	svc.SessionRepl(context.Background(), st, "main", in, &out)
	msgs, _ := st.Load("main")
	if len(msgs) != 1 {
		t.Errorf("expected /clear to reset the session, got %d messages", len(msgs))
	}
	if !strings.Contains(out.String(), "Context session dihapus.") {
		t.Errorf("expected clear confirmation, got: %s", out.String())
	}
}
