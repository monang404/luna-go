package repl

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/monang404/luna-go/internal/agent"
	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
	"github.com/monang404/luna-go/internal/settings"
	"github.com/monang404/luna-go/internal/tools"
)

// fakeComplete returns a fixed response, simulating an LLM.
func fakeComplete(thought string, done bool) func(context.Context, agent.Deps, []llmclient.Message) (llmclient.Response, error) {
	return func(_ context.Context, _ agent.Deps, _ []llmclient.Message) (llmclient.Response, error) {
		content := `{"thought":"` + thought + `","done":` + boolStr(done) + `}`
		return llmclient.Response{Content: content, HTTPStatus: 200}, nil
	}
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func newTestDispatcher() *tools.Dispatcher {
	return tools.NewDispatcher()
}

func TestREPL_SlashHelp(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("/help\n/exit\n")

	r := New(Options{
		In:          in,
		Out:         &out,
		Err:         &out,
		Dispatcher:  newTestDispatcher(),
		ProjectRoot: t.TempDir(),
		Limits:      config.LoadLimits(),
		Paths:       config.LoadPaths(),
	})

	err := r.interactiveLoop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "/help") {
		t.Error("expected help output to contain /help")
	}
	if !strings.Contains(output, "/clear") {
		t.Error("expected help output to list /clear")
	}
	if !strings.Contains(output, "/exit") {
		t.Error("expected help output to list /exit")
	}
}

func TestREPL_SlashClear(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("/clear\n/exit\n")

	r := New(Options{
		In:          in,
		Out:         &out,
		Err:         &out,
		Dispatcher:  newTestDispatcher(),
		ProjectRoot: t.TempDir(),
		Limits:      config.LoadLimits(),
		Paths:       config.LoadPaths(),
	})

	// Seed some messages
	r.messages = []llmclient.Message{
		{Role: "system", Content: "sys prompt"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}

	err := r.interactiveLoop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// After /clear, only system prompt should remain
	if len(r.messages) != 1 {
		t.Errorf("expected 1 message after clear, got %d", len(r.messages))
	}
	if r.messages[0].Role != "system" {
		t.Errorf("expected system message, got %q", r.messages[0].Role)
	}
}

func TestREPL_SlashModel(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("/model\n/model claude-sonnet-4-20250514\n/model\n/exit\n")

	r := New(Options{
		In:          in,
		Out:         &out,
		Err:         &out,
		Dispatcher:  newTestDispatcher(),
		ProjectRoot: t.TempDir(),
		Limits:      config.LoadLimits(),
		Paths:       config.LoadPaths(),
	})
	r.settings = &settings.Settings{}

	err := r.interactiveLoop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "claude-sonnet-4-20250514") {
		t.Error("expected model name in output")
	}
}

func TestREPL_SlashCost(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("/cost\n/exit\n")

	r := New(Options{
		In:          in,
		Out:         &out,
		Err:         &out,
		Dispatcher:  newTestDispatcher(),
		ProjectRoot: t.TempDir(),
		Limits:      config.LoadLimits(),
		Paths:       config.LoadPaths(),
	})
	// Populate messages instead of raw token counts because GetStats recalculates it
	r.messages = []llmclient.Message{
		{Role: "user", Content: strings.Repeat("a", 6000)}, // exactly 1500 tokens
	}
	r.turnCount = 3

	err := r.interactiveLoop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "1500") {
		t.Error("expected token count in cost output")
	}
}

func TestREPL_SlashExitVariants(t *testing.T) {
	for _, cmd := range []string{"/exit", "/quit", "/q"} {
		var out bytes.Buffer
		in := strings.NewReader(cmd + "\n")

		r := New(Options{
			In:          in,
			Out:         &out,
			Err:         &out,
			Dispatcher:  newTestDispatcher(),
			ProjectRoot: t.TempDir(),
			Limits:      config.LoadLimits(),
			Paths:       config.LoadPaths(),
		})

		err := r.interactiveLoop(context.Background())
		if err != nil {
			t.Errorf("%s: unexpected error: %v", cmd, err)
		}
	}
}

func TestREPL_UnknownSlashCommand(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("/invalidcommand\n/exit\n")

	r := New(Options{
		In:          in,
		Out:         &out,
		Err:         &out,
		Dispatcher:  newTestDispatcher(),
		ProjectRoot: t.TempDir(),
		Limits:      config.LoadLimits(),
		Paths:       config.LoadPaths(),
	})

	err := r.interactiveLoop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "tidak dikenal") {
		t.Error("expected unknown command message")
	}
}

func TestREPL_EOF(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("") // immediate EOF

	r := New(Options{
		In:          in,
		Out:         &out,
		Err:         &out,
		Dispatcher:  newTestDispatcher(),
		ProjectRoot: t.TempDir(),
		Limits:      config.LoadLimits(),
		Paths:       config.LoadPaths(),
	})

	err := r.interactiveLoop(context.Background())
	if err != nil {
		t.Fatalf("expected nil error on EOF, got: %v", err)
	}
}

func TestREPL_EmptyInput(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("\n\n   \n/exit\n")

	r := New(Options{
		In:          in,
		Out:         &out,
		Err:         &out,
		Dispatcher:  newTestDispatcher(),
		ProjectRoot: t.TempDir(),
		Limits:      config.LoadLimits(),
		Paths:       config.LoadPaths(),
	})

	err := r.interactiveLoop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestREPL_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	var out bytes.Buffer
	in := strings.NewReader("hello\n")

	r := New(Options{
		In:          in,
		Out:         &out,
		Err:         &out,
		Dispatcher:  newTestDispatcher(),
		ProjectRoot: t.TempDir(),
		Limits:      config.LoadLimits(),
		Paths:       config.LoadPaths(),
	})

	err := r.interactiveLoop(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestREPL_CompactShortHistory(t *testing.T) {
	var out bytes.Buffer
	in := strings.NewReader("/compact\n/exit\n")

	r := New(Options{
		In:          in,
		Out:         &out,
		Err:         &out,
		Dispatcher:  newTestDispatcher(),
		ProjectRoot: t.TempDir(),
		Limits:      config.LoadLimits(),
		Paths:       config.LoadPaths(),
	})
	r.messages = []llmclient.Message{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "hi"},
	}

	err := r.interactiveLoop(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	output := out.String()
	if !strings.Contains(output, "terlalu pendek") {
		t.Error("expected 'too short' message for compact with few messages")
	}
}

func TestREPL_PrintHeader(t *testing.T) {
	var out bytes.Buffer
	r := New(Options{
		Out:         &out,
		Err:         &out,
		ProjectRoot: "/test/project",
		Model:       "sonnet",
	})
	r.settings = &settings.Settings{}

	r.printHeader("default")
	output := out.String()
	if !strings.Contains(output, "LUNA") {
		t.Error("header should contain LUNA")
	}
	if !strings.Contains(output, "sonnet") {
		t.Error("header should show model")
	}
	if !strings.Contains(output, "/test/project") {
		t.Error("header should show project root")
	}
}
