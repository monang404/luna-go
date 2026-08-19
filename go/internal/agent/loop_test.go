package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
	"github.com/monang404/luna-go/internal/permission"
	"github.com/monang404/luna-go/internal/tools"
)

// ---------------------------------------------------------------------
// Fakes
// ---------------------------------------------------------------------

// scriptedComplete returns replies in order, one per call; the last
// reply repeats for any call past the end of the script (handy for
// "keep declaring done" tests without over-specifying script length).
func scriptedComplete(replies ...string) func(ctx context.Context, deps Deps, messages []llmclient.Message) (llmclient.Response, error) {
	i := 0
	return func(ctx context.Context, deps Deps, messages []llmclient.Message) (llmclient.Response, error) {
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

// countingTool is a fake tools.Tool that always succeeds and counts
// invocations, for asserting no-double-execution / call-count behavior.
type countingTool struct {
	calls *int
}

func (countingTool) Name() string                      { return "run_command" }
func (countingTool) Capability() permission.Capability { return permission.CapProcessTest }
func (t countingTool) Execute(_ context.Context, args json.RawMessage) (tools.Result, error) {
	*t.calls++
	return tools.Result{Output: "ok"}, nil
}

// failingTool always fails, for the repeated-same-failure test.
type failingTool struct {
	calls *int
}

func (failingTool) Name() string                      { return "run_command" }
func (failingTool) Capability() permission.Capability { return permission.CapProcessTest }
func (t failingTool) Execute(_ context.Context, args json.RawMessage) (tools.Result, error) {
	*t.calls++
	return tools.Result{}, fmt.Errorf("boom")
}

// testDeps builds a Deps wired to a fresh Dispatcher containing tool,
// registered as a readonly-level entry (so CheckPermission always
// allows it without needing an Ask callback -- see permission/check.go:
// LevelReadonly short-circuits to Allow after the path-containment
// check, which is skipped entirely for args with no "path"/"dest"
// field).
func testDeps(t *testing.T, complete func(context.Context, Deps, []llmclient.Message) (llmclient.Response, error), tool tools.Tool) Deps {
	t.Helper()
	disp := tools.NewDispatcher()
	if tool != nil {
		if err := disp.Register(tool.Name(), tools.Entry{Level: permission.LevelReadonly, Capability: tool.Capability()}, tool); err != nil {
			t.Fatalf("Register: %v", err)
		}
	}
	agentCtx := permission.NewAgentContext("test-session", t.TempDir(), false, permission.RolePrimary)
	return Deps{
		Limits: config.Limits{
			AgentMaxSteps:    5,
			AgentMaxSameFail: 3,
			SessionMaxMsgs:   30,
			AgentMaxToks:     8000,
		},
		Complete:   complete,
		Dispatcher: disp,
		PermDeps: tools.PermDeps{
			AgentCtx: agentCtx,
			Config:   permission.PermConfig{},
			Tracker:  permission.NewApprovalTracker(),
			Cwd:      agentCtx.ProjectRoot,
		},
	}
}

// ---------------------------------------------------------------------
// Test 1: done claimed with zero tool calls must never fabricate COMPLETE
// ---------------------------------------------------------------------

func TestRunLoop_UnverifiedDoneNeverCompletesFalsely(t *testing.T) {
	deps := testDeps(t, scriptedComplete(`{"thought":"kayaknya udah kelar","done":true}`), nil)
	deps.Limits.AgentMaxSteps = 3

	result, err := RunLoop(context.Background(), deps, "goal apapun", nil)
	if err != nil {
		t.Fatalf("RunLoop error: %v", err)
	}
	if result.Phase == PhaseComplete {
		t.Fatalf("RunLoop declared COMPLETE without any verified tool call: %+v", result)
	}
	if result.Phase != PhaseBlocked {
		t.Errorf("Phase = %q, want BLOCKED", result.Phase)
	}
}

// ---------------------------------------------------------------------
// Test 2: tool succeeds, then a verified done:true reaches COMPLETE
// ---------------------------------------------------------------------

func TestRunLoop_ToolThenVerifiedDoneCompletes(t *testing.T) {
	var calls int
	deps := testDeps(t,
		scriptedComplete(
			`{"thought":"jalankan command","tool":"run_command","args":{"cmd":"echo hi"}}`,
			`{"thought":"udah, verified","done":true}`,
		),
		countingTool{calls: &calls},
	)

	result, err := RunLoop(context.Background(), deps, "goal apapun", nil)
	if err != nil {
		t.Fatalf("RunLoop error: %v", err)
	}
	if result.Phase != PhaseComplete {
		t.Fatalf("Phase = %q, want COMPLETE (result=%+v)", result.Phase, result)
	}
	if !result.Done {
		t.Errorf("Done = false, want true")
	}
	if calls != 1 {
		t.Errorf("tool called %d times, want exactly 1", calls)
	}
	if result.CommandsRun != 1 {
		t.Errorf("CommandsRun = %d, want 1", result.CommandsRun)
	}
}

// ---------------------------------------------------------------------
// Test 3: same (tool, args) failing repeatedly gives up -> BLOCKED
// ---------------------------------------------------------------------

func TestRunLoop_RepeatedSameFailureBlocks(t *testing.T) {
	var calls int
	deps := testDeps(t,
		scriptedComplete(`{"thought":"coba lagi","tool":"run_command","args":{"cmd":"x"}}`),
		failingTool{calls: &calls},
	)
	deps.Limits.AgentMaxSteps = 10
	deps.Limits.AgentMaxSameFail = 3

	result, err := RunLoop(context.Background(), deps, "goal apapun", nil)
	if err != nil {
		t.Fatalf("RunLoop error: %v", err)
	}
	if result.Phase != PhaseBlocked {
		t.Fatalf("Phase = %q, want BLOCKED (result=%+v)", result.Phase, result)
	}
	if calls != 3 {
		t.Errorf("tool called %d times, want exactly 3 (AgentMaxSameFail)", calls)
	}
	if result.BlockReason == "" {
		t.Errorf("BlockReason empty, want a reason mentioning repeated failure")
	}
}

// ---------------------------------------------------------------------
// Test 4: max steps exhausted without a done claim -> BLOCKED
// ---------------------------------------------------------------------

func TestRunLoop_MaxStepsBlocks(t *testing.T) {
	var calls int
	deps := testDeps(t,
		scriptedComplete(`{"thought":"terus jalan","tool":"run_command","args":{"cmd":"x"}}`),
		countingTool{calls: &calls},
	)
	deps.Limits.AgentMaxSteps = 3
	deps.Limits.AgentMaxSameFail = 100 // never trip the same-fail path

	result, err := RunLoop(context.Background(), deps, "goal apapun", nil)
	if err != nil {
		t.Fatalf("RunLoop error: %v", err)
	}
	if result.Phase != PhaseBlocked {
		t.Fatalf("Phase = %q, want BLOCKED (result=%+v)", result.Phase, result)
	}
	if result.Steps != 3 {
		t.Errorf("Steps = %d, want 3", result.Steps)
	}
	if calls != 3 {
		t.Errorf("tool called %d times, want 3", calls)
	}
}

// ---------------------------------------------------------------------
// Test 5: an invalid/unparseable plan must never reach the tool layer
// ---------------------------------------------------------------------

func TestRunLoop_InvalidPlanNeverDispatchesTool(t *testing.T) {
	var calls int
	deps := testDeps(t,
		scriptedComplete("ini bukan JSON sama sekali, model ngaco"),
		countingTool{calls: &calls},
	)

	result, err := RunLoop(context.Background(), deps, "goal apapun", nil)
	if err != nil {
		t.Fatalf("RunLoop error: %v", err)
	}
	if result.Phase != PhaseBlocked {
		t.Fatalf("Phase = %q, want BLOCKED (result=%+v)", result.Phase, result)
	}
	if calls != 0 {
		t.Errorf("tool called %d times, want 0 (invalid plan must never dispatch)", calls)
	}
}

// ---------------------------------------------------------------------
// Test 6: context cancellation stops the loop without starting new work
// ---------------------------------------------------------------------

func TestRunLoop_ContextCancellationStops(t *testing.T) {
	var calls int
	deps := testDeps(t,
		scriptedComplete(`{"thought":"jalan","tool":"run_command","args":{"cmd":"x"}}`),
		countingTool{calls: &calls},
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before RunLoop even starts

	result, err := RunLoop(ctx, deps, "goal apapun", nil)
	if err != nil {
		t.Fatalf("RunLoop error: %v", err)
	}
	if result.Phase != PhaseBlocked {
		t.Fatalf("Phase = %q, want BLOCKED (result=%+v)", result.Phase, result)
	}
	if calls != 0 {
		t.Errorf("tool called %d times, want 0 (cancelled before any step)", calls)
	}
}

// ---------------------------------------------------------------------
// FASE 12/13: checkpoint save + resume, and a same-args double-dispatch
// sanity check across the resume boundary.
// ---------------------------------------------------------------------

func TestRunLoop_CheckpointAndResume(t *testing.T) {
	var calls int
	tool := countingTool{calls: &calls}

	dir := t.TempDir()
	store := NewStore(dir)

	// First run: budget for exactly one step, which the fake spends on
	// a tool call. The run therefore ends BLOCKED (max steps), leaving
	// a checkpoint with step=1 and the tool's output already recorded
	// in message history.
	firstDeps := testDeps(t,
		scriptedComplete(`{"thought":"langkah pertama","tool":"run_command","args":{"cmd":"x"}}`),
		tool,
	)
	firstDeps.Store = store
	firstDeps.SessionID = "resume-test"
	firstDeps.Limits.AgentMaxSteps = 1

	first, err := RunLoop(context.Background(), firstDeps, "goal resume", nil)
	if err != nil {
		t.Fatalf("first RunLoop error: %v", err)
	}
	if first.Phase != PhaseBlocked {
		t.Fatalf("first run Phase = %q, want BLOCKED", first.Phase)
	}
	if calls != 1 {
		t.Fatalf("tool called %d times after first run, want 1", calls)
	}

	cp, err := store.Load("resume-test")
	if err != nil {
		t.Fatalf("Load checkpoint: %v", err)
	}
	if cp.Step != 1 {
		t.Fatalf("checkpoint Step = %d, want 1", cp.Step)
	}

	// Resume: commandsRun resets to 0 for the new run (checkpoints
	// never persist it -- see loop.go's package doc comment), so the
	// resumed run must itself call the tool once more before a
	// done:true claim can be accepted. This also doubles as the
	// double-execution audit: the checkpoint was only ever saved AFTER
	// the first run's tool call completed (never mid-execution, and
	// there is no "pending tool" field to replay), so resuming goes
	// straight to requesting a brand new plan -- it can never blindly
	// re-run the step-1 tool call.
	secondDeps := testDeps(t,
		scriptedComplete(
			`{"thought":"lanjut, verifikasi lagi","tool":"run_command","args":{"cmd":"y"}}`,
			`{"thought":"sekarang beneran selesai","done":true}`,
		),
		tool,
	)
	secondDeps.Store = store
	secondDeps.SessionID = "resume-test"
	secondDeps.Limits.AgentMaxSteps = 5

	second, err := RunLoop(context.Background(), secondDeps, "", cp)
	if err != nil {
		t.Fatalf("resumed RunLoop error: %v", err)
	}
	if second.Phase != PhaseComplete {
		t.Fatalf("resumed run Phase = %q, want COMPLETE (result=%+v)", second.Phase, second)
	}
	if second.Goal != "goal resume" {
		t.Errorf("resumed Goal = %q, want the checkpoint's original goal %q", second.Goal, "goal resume")
	}
	if calls != 2 {
		t.Errorf("tool called %d times total, want exactly 2 (1 before + 1 after resume, no double-execution)", calls)
	}
	if second.Steps <= 1 {
		t.Errorf("resumed Steps = %d, want > 1 (continued from checkpoint's step 1)", second.Steps)
	}

	// A verified COMPLETE deletes the checkpoint (mirrors
	// 44-finalize.zsh's `rm -f checkpoint_file`).
	if _, err := store.Load("resume-test"); err == nil {
		t.Errorf("checkpoint still present after COMPLETE, want it deleted")
	}
}
