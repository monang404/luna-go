package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/permission"
)

// This file ports 30-luna/05-tools/50-tool_todo.zsh (_ai_tool_todo_write,
// _ai_tool_todo_read): a per-session checklist, stored as a JSON array
// under config.Paths.TodoDir, keyed by session slug -- not a project
// file, so both tools stay "readonly" level (auto-approve) exactly like
// the zsh source.
//
// Session-slug deviation: the zsh source keys the checklist file by
// $_AI_AGENT_SESSION_SLUG, a shell-local variable aiagent() sets once
// per run (30-agent/40-runtime/{10-load_checkpoint,15-prepare_new_goal}.zsh)
// with no Go equivalent yet -- the not-yet-ported agent loop
// (SESSION-49/50) owns that lifecycle, and Tool.Execute deliberately
// never receives an AgentContext (tool.go's own doc comment: a Tool
// "never itself" reaches into permission/session state; only Dispatcher
// does). todoSessionSlug below reads the env var
// AI_AGENT_SESSION_SLUG as an interim bridge -- same
// "${VAR:-default}" shape as the zsh source's own read, and the same
// env-var-bridge pattern config.envOr already uses elsewhere in this
// migration -- falling back to "default" until SESSION-49/50 wires a
// real per-run slug through.
func todoSessionSlug() string {
	if v := os.Getenv("AI_AGENT_SESSION_SLUG"); v != "" {
		return v
	}
	return "default"
}

func todoFilePath() string {
	return filepath.Join(config.LoadPaths().TodoDir, todoSessionSlug()+".json")
}

// todoItem mirrors one element of the `[{"text":"...","status":"..."}]`
// array format shared verbatim with the zsh source.
type todoItem struct {
	Text   string `json:"text"`
	Status string `json:"status"`
}

// TodoWriteTool implements _ai_tool_todo_write: schema.go's
// validateTodoItems has already guaranteed args.items is a non-empty
// array of {text: string, status: pending|doing|done} objects by the
// time Execute runs, so this just re-serializes and persists it, then
// echoes back the same rendering TodoReadTool produces (matching the
// zsh source's own `_ai_tool_todo_read "$args_json"` tail call).
type TodoWriteTool struct{}

func (TodoWriteTool) Name() string                      { return "todo_write" }
func (TodoWriteTool) Capability() permission.Capability { return Registry["todo_write"].Capability }

func (TodoWriteTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	obj := mustObject(args)
	rawItems, ok := obj["items"]
	if !ok {
		return Result{}, fmt.Errorf("ERROR: args.items (array todo) harus diisi, contoh: [{\"text\":\"baca config\",\"status\":\"pending\"}]")
	}
	encoded, err := json.Marshal(rawItems)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal encode items: %w", err)
	}

	paths := config.LoadPaths()
	if err := os.MkdirAll(paths.TodoDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membuat direktori todo: %w", err)
	}
	// printf '%s\n' equivalent -- write the JSON bytes verbatim (no
	// escape-sequence interpretation the zsh source's own comment warns
	// `echo` would otherwise do to a literal "\n" inside a text field),
	// plus one trailing newline.
	if err := os.WriteFile(todoFilePath(), append(encoded, '\n'), 0o644); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal menulis todo file: %w", err)
	}

	return TodoReadTool{}.Execute(ctx, args)
}

// TodoReadTool implements _ai_tool_todo_read: render the current
// session's checklist as one "[x]/[~]/[ ] text" line per item, falling
// back to the raw file content if it isn't valid JSON (matching the zsh
// source's `jq ... || cat "$todo_file"` fallback).
type TodoReadTool struct{}

func (TodoReadTool) Name() string                      { return "todo_read" }
func (TodoReadTool) Capability() permission.Capability { return Registry["todo_read"].Capability }

func (TodoReadTool) Execute(_ context.Context, _ json.RawMessage) (Result, error) {
	todoFile := todoFilePath()
	data, err := os.ReadFile(todoFile)
	if err != nil {
		return Result{Output: "OK: belum ada todo list buat sesi ini."}, nil
	}

	var items []todoItem
	if err := json.Unmarshal(data, &items); err != nil {
		// Fallback to raw content, exactly like the zsh source's `|| cat`.
		return Result{Output: strings.TrimRight(string(data), "\n")}, nil
	}

	var b strings.Builder
	for i, item := range items {
		var mark string
		switch item.Status {
		case "done":
			mark = "[x] "
		case "doing":
			mark = "[~] "
		default:
			mark = "[ ] "
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(mark)
		b.WriteString(item.Text)
	}
	return Result{Output: b.String()}, nil
}
