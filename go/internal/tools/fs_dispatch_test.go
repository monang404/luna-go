package tools

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/monang404/luna-go/internal/permission"
)

// newFsTestDispatcher registers all ten SESSION-47 fs tools, matching how
// a real agent loop (SESSION-49/50) is expected to wire them up via
// RegisterFromRegistry.
func newFsTestDispatcher(t *testing.T) *Dispatcher {
	t.Helper()
	d := NewDispatcher()
	fsTools := []Tool{
		ReadFileTool{}, ListDirTool{}, GrepSearchTool{}, GlobSearchTool{}, CountLinesTool{},
		WriteFileTool{}, EditFileTool{}, PatchFileTool{}, MoveFileTool{}, DeleteFileTool{},
	}
	for _, tool := range fsTools {
		if err := d.RegisterFromRegistry(tool); err != nil {
			t.Fatalf("RegisterFromRegistry(%s): %v", tool.Name(), err)
		}
	}
	return d
}

// alwaysApprove is an AskFunc that approves every prompt -- YOLO mode's
// own capability gate (permission/check.go's "Agent meminta capability
// ... Izinkan sekali?") still routes through one ask the first time a
// not-yet-granted capability (e.g. filesystem.write, starting false per
// context.go's defaultCapabilities) is needed, even with YoloMode=true;
// a nil AskFunc fails that ask closed by design (never fail-open), so
// end-to-end tests that exercise every tool need a non-nil AskFunc.
func alwaysApprove(string) (bool, error) { return true, nil }

func yoloPermDeps(cwd string) PermDeps {
	return PermDeps{
		AgentCtx: permission.NewAgentContext("test-session", cwd, true, permission.RolePrimary),
		Config:   permission.PermConfig{WriteMode: "yolo", ShellMode: "yolo", ProcessMode: "yolo"},
		Tracker:  permission.NewApprovalTracker(),
		Ask:      alwaysApprove,
		Cwd:      cwd,
	}
}

// TestDispatcher_AllTenFsToolsEndToEnd is this session's AC-01: every
// one of the ten tools this session ports is reachable through
// Dispatcher.Dispatch with valid args, not just by calling Execute
// directly.
func TestDispatcher_AllTenFsToolsEndToEnd(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping POSIX-dependent tests on Windows (ls, patch, etc)")
	}
	dir := t.TempDir()
	d := newFsTestDispatcher(t)
	deps := yoloPermDeps(dir)
	ctx := context.Background()

	readPath := writeTestFile(t, dir, "read_me.txt", "hello\nworld\n")

	steps := []struct {
		tool string
		args map[string]interface{}
	}{
		{"read_file", map[string]interface{}{"path": readPath}},
		{"list_dir", map[string]interface{}{"path": dir}},
		{"count_lines", map[string]interface{}{"path": readPath}},
		{"grep_search", map[string]interface{}{"pattern": "hello", "path": dir}},
		{"glob_search", map[string]interface{}{"pattern": "read_me"}},
		{"write_file", map[string]interface{}{"path": filepath.Join(dir, "new.txt"), "content": "new content"}},
		{"edit_file", map[string]interface{}{"path": filepath.Join(dir, "new.txt"), "old_str": "new content", "new_str": "edited"}},
		{"move_file", map[string]interface{}{"path": filepath.Join(dir, "new.txt"), "dest": filepath.Join(dir, "moved.txt")}},
		{"patch_file", map[string]interface{}{"path": readPath, "diff_content": "--- read_me.txt\n+++ read_me.txt\n@@ -1,2 +1,2 @@\n-hello\n+HELLO\n world\n"}},
		{"delete_file", map[string]interface{}{"path": filepath.Join(dir, "moved.txt")}},
	}

	for _, step := range steps {
		t.Run(step.tool, func(t *testing.T) {
			_, err := d.Dispatch(ctx, deps, step.tool, argsJSON(t, step.args))
			if err != nil {
				t.Fatalf("Dispatch(%s) failed: %v", step.tool, err)
			}
		})
	}
}

// TestDispatcher_UnknownTool mirrors _ai_tool_dispatch's own
// unknown-tool-name error path.
func TestDispatcher_UnknownTool(t *testing.T) {
	d := newFsTestDispatcher(t)
	_, err := d.Dispatch(context.Background(), yoloPermDeps(t.TempDir()), "not_a_real_tool", nil)
	if err == nil {
		t.Fatal("expected error for unknown tool name")
	}
}

// TestDispatcher_WriteFileDeniedWithoutPermission is this session's
// AC-05: a write-capable tool call that permission.CheckPermission
// denies must never reach Execute, and must never touch the
// filesystem.
func TestDispatcher_WriteFileDeniedWithoutPermission(t *testing.T) {
	dir := t.TempDir()
	d := newFsTestDispatcher(t)

	// Not YOLO, ask_once_per_file with a nil AskFunc: CheckPermission's
	// askOnce fails closed (permission/check.go) since there is no UI
	// layer to ask through, so this must deny rather than silently
	// allow.
	deps := PermDeps{
		AgentCtx: permission.NewAgentContext("test-session", dir, false, permission.RolePrimary),
		Config:   permission.PermConfig{WriteMode: "ask_once_per_file"},
		Tracker:  permission.NewApprovalTracker(),
		Ask:      nil,
		Cwd:      dir,
	}

	target := filepath.Join(dir, "should_not_exist.txt")
	_, err := d.Dispatch(context.Background(), deps, "write_file", argsJSON(t, map[string]string{"path": target, "content": "x"}))
	if err == nil {
		t.Fatal("expected write_file to be denied without an approved permission decision")
	}
	if _, statErr := os.Stat(target); statErr == nil {
		t.Fatal("write_file must not have touched the filesystem when permission was denied")
	}
}

// TestDispatcher_DeleteFileDeniedWithoutPermission exercises the same
// AC-05 guarantee for delete_file, whose Level is "shell" rather than
// "write" (see registry.go) -- a different branch of
// permission.CheckPermission's level dispatch.
func TestDispatcher_DeleteFileDeniedWithoutPermission(t *testing.T) {
	dir := t.TempDir()
	target := writeTestFile(t, dir, "precious.txt", "do not delete me")
	d := newFsTestDispatcher(t)

	deps := PermDeps{
		AgentCtx: permission.NewAgentContext("test-session", dir, false, permission.RolePrimary),
		Config:   permission.PermConfig{ShellMode: "ask_always"},
		Tracker:  permission.NewApprovalTracker(),
		Ask:      nil,
		Cwd:      dir,
	}

	_, err := d.Dispatch(context.Background(), deps, "delete_file", argsJSON(t, map[string]string{"path": target}))
	if err == nil {
		t.Fatal("expected delete_file to be denied without an approved permission decision")
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Fatal("delete_file must not have removed the file when permission was denied")
	}
}

// TestDispatcher_PathOutsideProjectRootDenied exercises the path
// containment guard (permission.ValidateProjectPath) that
// Dispatcher.Dispatch runs before any level-specific ask/allow logic,
// for a tool in this session's ten.
func TestDispatcher_PathOutsideProjectRootDenied(t *testing.T) {
	projectDir := t.TempDir()
	outsideDir := t.TempDir()
	outsidePath := writeTestFile(t, outsideDir, "outside.txt", "x")

	d := newFsTestDispatcher(t)
	deps := yoloPermDeps(projectDir)
	deps.AgentCtx = permission.NewAgentContext("test-session", projectDir, true, permission.RolePrimary)

	_, err := d.Dispatch(context.Background(), deps, "read_file", argsJSON(t, map[string]string{"path": outsidePath}))
	if err == nil {
		t.Fatal("expected read_file outside the project root to be denied")
	}
}
