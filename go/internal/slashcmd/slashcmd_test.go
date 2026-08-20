package slashcmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/monang404/luna-go/internal/settings"
)

type mockState struct {
	out           bytes.Buffer
	err           bytes.Buffer
	cleared       bool
	model         string
	exitRequested bool
	exited        bool
}

func (m *mockState) Out() io.Writer                  { return &m.out }
func (m *mockState) Err() io.Writer                  { return &m.err }
func (m *mockState) ClearMessages()                  { m.cleared = true }
func (m *mockState) GetModel() string                { return m.model }
func (m *mockState) SetModel(mod string)             { m.model = mod }
func (m *mockState) GetSettings() *settings.Settings { return nil }
func (m *mockState) GetStats() (int, int, int)       { return 100, 50, 2 }
func (m *mockState) CompactHistory(ctx context.Context, instruction string) {
	m.out.WriteString("compacted with " + instruction + "\n")
}
func (m *mockState) PrintPermissions()   { m.out.WriteString("permissions printed\n") }
func (m *mockState) PrintStatus()        { m.out.WriteString("status printed\n") }
func (m *mockState) RequestExit()        { m.exitRequested = true; m.exited = true }
func (m *mockState) RewindHistory(n int) { m.out.WriteString(fmt.Sprintf("rewound %d\n", n)) }
func (m *mockState) LoadSession(id string) error {
	m.out.WriteString("loaded " + id + "\n")
	return nil
}
func (m *mockState) InjectPrompt(ctx context.Context, p string) {
	m.out.WriteString("injected: " + p + "\n")
}

func TestRegistry_Dispatch(t *testing.T) {
	r := NewRegistry()
	RegisterBuiltins(r, nil)

	t.Run("not a slash command", func(t *testing.T) {
		m := &mockState{}
		executed, err := r.Dispatch(context.Background(), "hello world", m)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if executed {
			t.Errorf("expected executed=false")
		}
	})

	t.Run("unknown slash command", func(t *testing.T) {
		m := &mockState{}
		executed, err := r.Dispatch(context.Background(), "/unknown", m)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !executed {
			t.Errorf("expected executed=true")
		}
		if !strings.Contains(m.out.String(), "Perintah tidak dikenal") {
			t.Errorf("expected unknown command message, got: %s", m.out.String())
		}
	})

	t.Run("exit command", func(t *testing.T) {
		m := &mockState{}
		executed, err := r.Dispatch(context.Background(), "/exit", m)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !executed {
			t.Errorf("expected executed=true")
		}
		if !m.exitRequested {
			t.Errorf("expected exitRequested=true")
		}
	})

	t.Run("clear command", func(t *testing.T) {
		m := &mockState{}
		executed, err := r.Dispatch(context.Background(), "/clear", m)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !executed {
			t.Errorf("expected executed=true")
		}
		if !m.cleared {
			t.Errorf("expected cleared=true")
		}
	})

	t.Run("model command with args", func(t *testing.T) {
		m := &mockState{}
		executed, err := r.Dispatch(context.Background(), "/model gpt-4", m)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if !executed {
			t.Errorf("expected executed=true")
		}
		if m.model != "gpt-4" {
			t.Errorf("expected model=gpt-4, got %s", m.model)
		}
	})
}
