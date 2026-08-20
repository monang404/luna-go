package subagent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/monang404/luna-go/internal/agent"
	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
	"github.com/monang404/luna-go/internal/permission"
	"github.com/monang404/luna-go/internal/tools"
)

// ---------------------------------------------------------------------
// Fakes (same shape as internal/agent's own loop_test.go fakes)
// ---------------------------------------------------------------------

func scriptedComplete(replies ...string) func(context.Context, agent.Deps, []llmclient.Message) (llmclient.Response, error) {
	i := 0
	return func(ctx context.Context, deps agent.Deps, messages []llmclient.Message) (llmclient.Response, error) {
		if len(replies) == 0 {
			return llmclient.Response{}, nil
		}
		idx := i
		if idx >= len(replies) {
			idx = len(replies) - 1
		}
		i++
		return llmclient.Response{Content: replies[idx]}, nil
	}
}

// countingTool always succeeds and counts invocations.
type countingTool struct {
	name  string
	cap   permission.Capability
	calls *int
}

func (t countingTool) Name() string                      { return t.name }
func (t countingTool) Capability() permission.Capability { return t.cap }
func (t countingTool) Execute(_ context.Context, args json.RawMessage) (tools.Result, error) {
	*t.calls++
	return tools.Result{Output: "ok"}, nil
}

// failingTool always fails.
type failingTool struct {
	name  string
	cap   permission.Capability
	calls *int
}

func (t failingTool) Name() string                      { return t.name }
func (t failingTool) Capability() permission.Capability { return t.cap }
func (t failingTool) Execute(_ context.Context, args json.RawMessage) (tools.Result, error) {
	*t.calls++
	return tools.Result{}, errors.New("boom")
}

// baseDeps builds a Deps with a Dispatcher containing every tool in
// extra (registered readonly so CheckPermission never needs an Ask
// callback), a fresh primary-role AgentContext, and complete wired in.
func baseDeps(t *testing.T, complete func(context.Context, agent.Deps, []llmclient.Message) (llmclient.Response, error), extra ...tools.Tool) Deps {
	t.Helper()
	disp := tools.NewDispatcher()
	for _, tool := range extra {
		if err := disp.Register(tool.Name(), tools.Entry{Level: permission.LevelReadonly, Capability: tool.Capability()}, tool); err != nil {
			t.Fatalf("Register(%s): %v", tool.Name(), err)
		}
	}
	parentCtx := permission.NewAgentContext("parent-session", t.TempDir(), false, permission.RolePrimary)
	return Deps{
		Limits:          config.Limits{AgentMaxSteps: 5, AgentMaxSameFail: 3},
		Dispatcher:      disp,
		ParentAgentCtx:  parentCtx,
		Config:          permission.PermConfig{WriteMode: "ask_once_per_file", ShellMode: "ask_always", ProcessMode: "ask_always"},
		Tracker:         permission.NewApprovalTracker(),
		Cwd:             parentCtx.ProjectRoot,
		ParentSessionID: "parent-session",
		Complete:        complete,
	}
}

// ---------------------------------------------------------------------
// Test A -- role + subgoal -> subagent success
// ---------------------------------------------------------------------

func TestSpawnSubagent_Success(t *testing.T) {
	calls := 0
	deps := baseDeps(t, nil, countingTool{name: "read_file", cap: permission.CapFilesystemRead, calls: &calls})
	target := filepath.Join(deps.Cwd, "a.txt")
	deps.Complete = scriptedComplete(
		fmt.Sprintf(`{"thought":"reading","tool":"read_file","args":{"path":%q},"done":false}`, target),
		`{"thought":"found it, all good","tool":"","args":{},"done":true}`,
	)

	res, err := SpawnSubagent(context.Background(), deps, RoleResearcher, "investigate something")
	if err != nil {
		t.Fatalf("SpawnSubagent returned error: %v", err)
	}
	if res.Status != StatusSuccess {
		t.Fatalf("Status = %q, want success (Error=%q)", res.Status, res.Error)
	}
	if res.Findings != "found it, all good" {
		t.Fatalf("Findings = %q", res.Findings)
	}
	if res.Changes != "" {
		t.Fatalf("Changes should be empty for researcher, got %q", res.Changes)
	}
	if calls != 1 {
		t.Fatalf("read_file called %d times, want 1", calls)
	}
	if len(res.FilesAffected) != 1 || res.FilesAffected[0] != target {
		t.Fatalf("FilesAffected = %v, want [%s]", res.FilesAffected, target)
	}
}

// ---------------------------------------------------------------------
// Test B -- subagent failure -> parent does not panic, gets a Result
// ---------------------------------------------------------------------

func TestSpawnSubagent_Failure(t *testing.T) {
	calls := 0
	deps := baseDeps(t, nil, failingTool{name: "read_file", cap: permission.CapFilesystemRead, calls: &calls})
	target := filepath.Join(deps.Cwd, "a.txt")
	deps.Complete = scriptedComplete(
		fmt.Sprintf(`{"thought":"trying","tool":"read_file","args":{"path":%q},"done":false}`, target),
	)
	deps.Limits.AgentMaxSameFail = 2

	res, err := SpawnSubagent(context.Background(), deps, RoleResearcher, "investigate something")
	if err != nil {
		t.Fatalf("SpawnSubagent returned error: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed", res.Status)
	}
	if res.Error == "" {
		t.Fatal("Error should be non-empty on failure")
	}
	if calls != 2 {
		t.Fatalf("read_file called %d times, want 2 (AgentMaxSameFail)", calls)
	}
}

func TestSpawnSubagent_InvalidRole(t *testing.T) {
	deps := baseDeps(t, scriptedComplete())
	res, err := SpawnSubagent(context.Background(), deps, Role("reviewer"), "goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed for invalid role", res.Status)
	}
}

func TestSpawnSubagent_EmptyGoal(t *testing.T) {
	deps := baseDeps(t, scriptedComplete())
	res, err := SpawnSubagent(context.Background(), deps, RoleResearcher, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed for empty goal", res.Status)
	}
}

// ---------------------------------------------------------------------
// Test C -- parent cancellation -> subagent stops
// ---------------------------------------------------------------------

func TestSpawnSubagent_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the run even starts

	deps := baseDeps(t, nil)
	target := filepath.Join(deps.Cwd, "a.txt")
	deps.Complete = scriptedComplete(
		fmt.Sprintf(`{"thought":"x","tool":"read_file","args":{"path":%q},"done":false}`, target),
	)

	res, err := SpawnSubagent(ctx, deps, RoleResearcher, "investigate")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed on cancellation", res.Status)
	}
}

// ---------------------------------------------------------------------
// Test D -- depth exceeded -> subagent rejected
// ---------------------------------------------------------------------

func TestSpawnSubagent_DepthExceeded(t *testing.T) {
	deps := baseDeps(t, scriptedComplete())
	deps.Depth = 1
	deps.MaxDepth = 1

	res, err := SpawnSubagent(context.Background(), deps, RoleResearcher, "goal")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed when depth >= MaxDepth", res.Status)
	}
}

// ---------------------------------------------------------------------
// AC-01 -- allowlist enforcement: tool outside allowlist is rejected
// ---------------------------------------------------------------------

func TestSpawnSubagent_ToolOutsideAllowlistRejected(t *testing.T) {
	calls := 0
	// researcher's loop tries to call write_file, which is NOT in
	// researcher's allowlist -- Dispatcher.Subset never registers it,
	// so the loop's own runTool sees "tool tidak dikenal" and records
	// it as a failed tool call (not a panic, not a bypass).
	deps := baseDeps(t, nil, countingTool{name: "write_file", cap: permission.CapFilesystemWrite, calls: &calls})
	target := filepath.Join(deps.Cwd, "a.txt")
	deps.Complete = scriptedComplete(
		fmt.Sprintf(`{"thought":"writing","tool":"write_file","args":{"path":%q},"done":false}`, target),
	)
	deps.Limits.AgentMaxSameFail = 1

	res, err := SpawnSubagent(context.Background(), deps, RoleResearcher, "try to write")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Status != StatusFailed {
		t.Fatalf("Status = %q, want failed (write_file outside researcher allowlist)", res.Status)
	}
	if calls != 0 {
		t.Fatalf("write_file.Execute was called %d times, want 0 -- allowlist bypassed!", calls)
	}
}

func TestToolAllowed(t *testing.T) {
	cases := []struct {
		role Role
		tool string
		want bool
	}{
		{RoleResearcher, "read_file", true},
		{RoleResearcher, "write_file", false},
		{RoleResearcher, "run_command", false},
		{RoleCoder, "write_file", true},
		{RoleCoder, "delete_file", true},
		{RoleCoder, "run_command", false},
		{RoleCoder, "exec_process", false},
		{RoleCoder, "web_fetch", false},
		{Role("bogus"), "read_file", false},
	}
	for _, c := range cases {
		if got := ToolAllowed(nil, c.role, c.tool); got != c.want {
			t.Errorf("ToolAllowed(%q, %q) = %v, want %v", c.role, c.tool, got, c.want)
		}
	}
}

// ---------------------------------------------------------------------
// AC-02 -- state/context isolation: subagent cannot mutate parent's
// AgentContext (capability escalation must not leak back to parent)
// ---------------------------------------------------------------------

func TestSpawnSubagent_DoesNotMutateParentAgentContext(t *testing.T) {
	calls := 0
	deps := baseDeps(t, nil, countingTool{name: "read_file", cap: permission.CapFilesystemRead, calls: &calls})
	target := filepath.Join(deps.Cwd, "a.txt")
	deps.Complete = scriptedComplete(
		fmt.Sprintf(`{"thought":"reading","tool":"read_file","args":{"path":%q},"done":false}`, target),
		`{"thought":"done","tool":"","args":{},"done":true}`,
	)

	before := deps.ParentAgentCtx.CapabilityAllowed(permission.CapFilesystemWrite)

	if _, err := SpawnSubagent(context.Background(), deps, RoleCoder, "investigate"); err != nil {
		t.Fatalf("SpawnSubagent error: %v", err)
	}

	after := deps.ParentAgentCtx.CapabilityAllowed(permission.CapFilesystemWrite)
	if before != after {
		t.Fatalf("parent AgentContext capability changed by subagent run: before=%v after=%v", before, after)
	}
	if deps.ParentAgentCtx.Role != permission.RolePrimary {
		t.Fatalf("parent AgentContext.Role mutated: %v", deps.ParentAgentCtx.Role)
	}
}

func TestSpawnSubagent_NilDispatcher(t *testing.T) {
	deps := baseDeps(t, scriptedComplete())
	deps.Dispatcher = nil
	if _, err := SpawnSubagent(context.Background(), deps, RoleResearcher, "goal"); err == nil {
		t.Fatal("expected error for nil Dispatcher")
	}
}
