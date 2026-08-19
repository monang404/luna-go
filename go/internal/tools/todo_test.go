package tools

import (
	"context"
	"testing"
)

// withTodoDir points AI_TODO_DIR (config.LoadPaths().TodoDir) and
// AI_AGENT_SESSION_SLUG at a fresh temp location for the duration of the
// test, so todo round-trip tests never touch a real session's checklist
// file and never collide with each other.
func withTodoDir(t *testing.T) {
	t.Helper()
	t.Setenv("AI_TODO_DIR", t.TempDir())
	t.Setenv("AI_AGENT_SESSION_SLUG", "test-session-"+t.Name())
}

func TestTodoRoundTrip_WriteThenRead(t *testing.T) {
	withTodoDir(t)
	ctx := context.Background()

	writeArgs := argsJSON(t, map[string]interface{}{
		"items": []map[string]string{
			{"text": "baca config", "status": "pending"},
			{"text": "tulis test", "status": "doing"},
			{"text": "selesai", "status": "done"},
		},
	})

	writeRes, err := TodoWriteTool{}.Execute(ctx, writeArgs)
	if err != nil {
		t.Fatalf("TodoWriteTool.Execute: %v", err)
	}

	readRes, err := TodoReadTool{}.Execute(ctx, nil)
	if err != nil {
		t.Fatalf("TodoReadTool.Execute: %v", err)
	}

	// AC-03: write's own returned rendering and a subsequent independent
	// read must be identical (both are _ai_tool_todo_read's rendering of
	// the same just-persisted file).
	if writeRes.Output != readRes.Output {
		t.Errorf("todo round-trip mismatch:\nwrite returned: %q\nread returned:  %q", writeRes.Output, readRes.Output)
	}

	wantLines := []string{"[ ] baca config", "[~] tulis test", "[x] selesai"}
	for _, want := range wantLines {
		if !containsLine(readRes.Output, want) {
			t.Errorf("TodoReadTool output missing line %q, got:\n%s", want, readRes.Output)
		}
	}
}

func TestTodoRead_NoTodoYet(t *testing.T) {
	withTodoDir(t)
	res, err := TodoReadTool{}.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("TodoReadTool.Execute: %v", err)
	}
	if res.Output != "OK: belum ada todo list buat sesi ini." {
		t.Errorf("TodoReadTool.Execute() with no prior write = %q, want the empty-state message", res.Output)
	}
}

func TestTodoWrite_RejectsMissingItems(t *testing.T) {
	withTodoDir(t)
	_, err := TodoWriteTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected TodoWriteTool to reject args with no items field")
	}
}

func TestTodoWrite_OverwritesPreviousChecklist(t *testing.T) {
	withTodoDir(t)
	ctx := context.Background()

	first := argsJSON(t, map[string]interface{}{
		"items": []map[string]string{{"text": "first plan", "status": "pending"}},
	})
	if _, err := (TodoWriteTool{}).Execute(ctx, first); err != nil {
		t.Fatalf("first write: %v", err)
	}

	second := argsJSON(t, map[string]interface{}{
		"items": []map[string]string{{"text": "revised plan", "status": "doing"}},
	})
	if _, err := (TodoWriteTool{}).Execute(ctx, second); err != nil {
		t.Fatalf("second write: %v", err)
	}

	res, err := TodoReadTool{}.Execute(ctx, nil)
	if err != nil {
		t.Fatalf("TodoReadTool.Execute: %v", err)
	}
	if containsLine(res.Output, "first plan") {
		t.Errorf("second write should have replaced the checklist entirely, still saw 'first plan':\n%s", res.Output)
	}
	if !containsLine(res.Output, "[~] revised plan") {
		t.Errorf("expected the revised plan after overwrite, got:\n%s", res.Output)
	}
}

func TestTodoRoundTrip_DifferentSessionSlugsAreIsolated(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("AI_TODO_DIR", dir)
	ctx := context.Background()

	t.Setenv("AI_AGENT_SESSION_SLUG", "session-a")
	if _, err := (TodoWriteTool{}).Execute(ctx, argsJSON(t, map[string]interface{}{
		"items": []map[string]string{{"text": "session a task", "status": "pending"}},
	})); err != nil {
		t.Fatalf("write session-a: %v", err)
	}

	t.Setenv("AI_AGENT_SESSION_SLUG", "session-b")
	res, err := TodoReadTool{}.Execute(ctx, nil)
	if err != nil {
		t.Fatalf("read session-b: %v", err)
	}
	if res.Output != "OK: belum ada todo list buat sesi ini." {
		t.Errorf("session-b should see no todo list of its own, got: %q", res.Output)
	}
}

func containsLine(haystack, line string) bool {
	for _, l := range splitLines(haystack) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i, r := range s {
		if r == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	lines = append(lines, s[start:])
	return lines
}
