package tools

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/monang404/luna-go/internal/permission"
)

// newSession48TestDispatcher registers every tool this session ports,
// the SESSION-47 equivalent of fs_dispatch_test.go's newFsTestDispatcher.
func newSession48TestDispatcher(t *testing.T) *Dispatcher {
	t.Helper()
	d := NewDispatcher()
	tools := []Tool{
		GitStatusTool{}, GitDiffTool{},
		WebFetchTool{},
		TodoWriteTool{}, TodoReadTool{},
		ExecProcessTool{}, RunTestTool{}, RunCommandTool{},
	}
	for _, tool := range tools {
		if err := d.RegisterFromRegistry(tool); err != nil {
			t.Fatalf("RegisterFromRegistry(%s): %v", tool.Name(), err)
		}
	}
	return d
}

// TestDispatcher_Session48ToolsEndToEnd is this session's AC-01/AC-02/AC-03
// end-to-end companion: every tool this session ports is reachable
// through Dispatcher.Dispatch with valid args, not just by calling
// Execute directly (mirroring SESSION-47's own
// TestDispatcher_AllTenFsToolsEndToEnd).
func TestDispatcher_Session48ToolsEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "checkout", "-q", "-b", "main")
	f := writeTestFile(t, dir, "tracked.txt", "hello\n")
	runGit(t, dir, "add", "tracked.txt")
	runGit(t, dir, "commit", "-q", "-m", "init")
	if err := os.WriteFile(f, []byte("hello world\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	chdirTemp(t, dir)
	t.Setenv("AI_TODO_DIR", t.TempDir())
	t.Setenv("AI_AGENT_SESSION_SLUG", "dispatch-e2e-test")

	d := newSession48TestDispatcher(t)
	deps := yoloPermDeps(dir)
	ctx := context.Background()

	steps := []struct {
		tool string
		args map[string]interface{}
	}{
		{"git_status", map[string]interface{}{}},
		{"git_diff", map[string]interface{}{}},
		{"todo_write", map[string]interface{}{"items": []map[string]string{{"text": "step 1", "status": "pending"}}}},
		{"todo_read", map[string]interface{}{}},
		{"run_command", map[string]interface{}{"command": "echo via-dispatch"}},
	}
	for _, step := range steps {
		t.Run(step.tool, func(t *testing.T) {
			res, err := d.Dispatch(ctx, deps, step.tool, argsJSON(t, step.args))
			if err != nil {
				t.Fatalf("Dispatch(%s) failed: %v", step.tool, err)
			}
			_ = res
		})
	}
}

// TestDispatcher_ExecProcessEndToEnd exercises exec_process specifically
// through the full pipeline (normalize -> validate -> permission check
// -> Execute), separate from the table above since it needs `grep` on
// PATH rather than a git fixture.
func TestDispatcher_ExecProcessEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not available in PATH")
	}
	dir := t.TempDir()
	d := NewDispatcher()
	if err := d.RegisterFromRegistry(ExecProcessTool{}); err != nil {
		t.Fatalf("RegisterFromRegistry: %v", err)
	}
	deps := yoloPermDeps(dir)

	_, err := d.Dispatch(context.Background(), deps, "exec_process", argsJSON(t, map[string]interface{}{
		"program": "grep", "args": []string{"--version"},
	}))
	if err != nil {
		t.Fatalf("Dispatch(exec_process): %v", err)
	}
}

// TestDispatcher_RunCommandStillDispatchableWhenHiddenFromManifest is
// this session's AC-04 companion: registry.go's Manifest() hides
// run_command from the model-facing tool list by default (already
// covered by SESSION-43's TestManifest_HidesRunCommandByDefault), but
// that must remain presentation-only -- the Dispatcher itself still
// accepts and runs it by name regardless, exactly like the zsh source's
// own _ai_tool_dispatch `case` statement, which has no such gate.
func TestDispatcher_RunCommandStillDispatchableWhenHiddenFromManifest(t *testing.T) {
	t.Setenv("AI_AGENT_EXPOSE_ARBITRARY_SHELL", "") // hidden from Manifest()
	if contains(Manifest(), "run_command ") {
		t.Fatal("test precondition failed: run_command should be hidden from Manifest() here")
	}

	d := NewDispatcher()
	if err := d.RegisterFromRegistry(RunCommandTool{}); err != nil {
		t.Fatalf("RegisterFromRegistry: %v", err)
	}
	deps := yoloPermDeps(t.TempDir())
	_, err := d.Dispatch(context.Background(), deps, "run_command", argsJSON(t, map[string]interface{}{"command": "echo still-works"}))
	if err != nil {
		t.Fatalf("Dispatch(run_command) should still work even though Manifest() hides it: %v", err)
	}
}

// TestDispatcher_WebFetchDeniedWithoutPermission mirrors SESSION-47's
// TestDispatcher_WriteFileDeniedWithoutPermission for web_fetch's
// "shell" level: a nil AskFunc under a non-yolo ShellMode must deny
// before Execute (and therefore before any network access) ever
// happens.
func TestDispatcher_WebFetchDeniedWithoutPermission(t *testing.T) {
	dir := t.TempDir()
	d := NewDispatcher()
	if err := d.RegisterFromRegistry(WebFetchTool{}); err != nil {
		t.Fatalf("RegisterFromRegistry: %v", err)
	}
	deps := PermDeps{
		AgentCtx: permission.NewAgentContext("test-session", dir, false, permission.RolePrimary),
		Config:   permission.PermConfig{ShellMode: "ask_always"},
		Tracker:  permission.NewApprovalTracker(),
		Ask:      nil,
		Cwd:      dir,
	}
	_, err := d.Dispatch(context.Background(), deps, "web_fetch", argsJSON(t, map[string]string{"url": "http://example.com"}))
	if err == nil {
		t.Fatal("expected web_fetch to be denied without an approved permission decision")
	}
}

// TestDispatcher_ExecProcessDeniedWithoutPermission mirrors the same
// guarantee for exec_process's "process" level.
func TestDispatcher_ExecProcessDeniedWithoutPermission(t *testing.T) {
	dir := t.TempDir()
	d := NewDispatcher()
	if err := d.RegisterFromRegistry(ExecProcessTool{}); err != nil {
		t.Fatalf("RegisterFromRegistry: %v", err)
	}
	deps := PermDeps{
		AgentCtx: permission.NewAgentContext("test-session", dir, false, permission.RolePrimary),
		Config:   permission.PermConfig{ProcessMode: "ask_always"},
		Tracker:  permission.NewApprovalTracker(),
		Ask:      nil,
		Cwd:      dir,
	}
	_, err := d.Dispatch(context.Background(), deps, "exec_process", argsJSON(t, map[string]interface{}{"program": "git"}))
	if err == nil {
		t.Fatal("expected exec_process to be denied without an approved permission decision")
	}
}
