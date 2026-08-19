// Ported from 30-luna/55-subagent/00-design_contract.zsh (the contract
// itself) and 30-luna/55-subagent/20-run.zsh (_ai_subagent_run). SESSION-51
// scope only -- see docs/execution_sessions/51_port_subagent_orchestration.yaml.
//
// Design note (FASE 5/9 of the SESSION-51 brief): SpawnSubagent does not
// run a second execution loop. It reuses agent.RunLoop (SESSION-50)
// verbatim, the same way _ai_subagent_run reuses _ai_chat_request/
// _ai_agent_parse/_ai_tool_dispatch rather than re-implementing them.
// What this file adds on top of RunLoop is exactly what the zsh source
// adds on top of its own shared primitives: a role-scoped tool set
// (allowlist.go), a narrow sysprompt (sysprompt.go), a bounded step
// count that can never exceed the parent's, and a structured Result in
// place of RunLoop's own FinalResult.
package subagent

import (
	"context"
	"errors"
	"fmt"

	"github.com/monang404/luna-go/internal/agent"
	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
	"github.com/monang404/luna-go/internal/permission"
	"github.com/monang404/luna-go/internal/tools"
)

// Status is a subagent run's terminal outcome. The zsh source only ever
// distinguishes "success"/"failed" (_ai_subagent_run always sets one of
// exactly those two strings) -- there is no third "blocked" status in
// the behavioral reference, so this port does not invent one either
// (FASE 2's "jangan mengarang behavior yang tidak ada di source").
type Status string

const (
	StatusSuccess Status = "success"
	StatusFailed  Status = "failed"
)

// Result is the structured summary SpawnSubagent returns -- the Go
// equivalent of _ai_subagent_run's key=value stdout lines
// (status/role/summary/findings/changes/files_affected/error). It
// deliberately does NOT include the subagent's message history/
// transcript, matching design_contract.zsh §5: "Subagent TIDAK PERNAH
// ngembaliin transcript penuh ... Ringkas & gampang disuntikkan balik
// ke history/context main agent."
type Result struct {
	Status Status
	Role   Role
	// Summary is the one-line outcome description: the subagent's last
	// thought on success, or the block reason on failure (falls back
	// to a default string when neither is available, mirroring
	// _ai_subagent_run's "${summary:-...}" defaults).
	Summary string
	// Findings holds the subagent's final thought when Role ==
	// RoleResearcher (empty for RoleCoder), matching the zsh source's
	// role-conditional findings=/changes= split. Populated regardless
	// of Status, exactly like final_thought's use in 20-run.zsh (a
	// subagent that fails after producing a useful last thought still
	// reports it).
	Findings string
	// Changes holds the subagent's final thought when Role ==
	// RoleCoder (empty for RoleResearcher). See Findings.
	Changes string
	// FilesAffected lists paths the subagent successfully touched
	// (write_file/edit_file/patch_file/move_file/delete_file dest-or-
	// path, or any tool's path argument). Sourced from
	// agent.FinalResult.TouchedFiles.
	//
	// Deviation from source (documented per FASE 2): 15-run_step.zsh
	// records a tool's path/dest argument into files_affected even
	// when the tool call FAILED (exit_status != 0); agent.RunLoop's
	// touched-files tracking (loop.go's runTool) only records a path
	// on success. This port keeps RunLoop's stricter, more accurate
	// behavior rather than reintroducing the looser one -- "files
	// affected" reporting a path the subagent merely attempted but
	// never actually touched would be misleading to the parent agent
	// consuming this Result.
	FilesAffected []string
	// Error is a human-readable failure detail, "" on success. Mirrors
	// _ai_subagent_run's error= line.
	Error string
}

// Deps bundles everything SpawnSubagent needs, deliberately narrower
// than agent.Deps: no Store/SessionID (see FASE 12 -- the zsh source has
// no persistent subagent checkpoint, so this port does not invent one;
// "documented that checkpoint tetap berada pada parent boundary"), and
// permission wiring split out so SpawnSubagent can build a fresh,
// subagent-scoped permission.AgentContext instead of reusing the
// parent's pointer (see ParentAgentCtx doc below -- this is what keeps
// AC-02's context isolation true).
type Deps struct {
	// Limits supplies AgentMaxSteps (the hard ceiling MaxSteps can
	// never exceed, per design_contract.zsh §7) and AgentMaxSameFail
	// (reused unchanged -- the zsh source does not override same-fail
	// budget for subagents).
	Limits config.Limits

	// ProviderOrder, Breaker: forwarded to agent.RunLoop unchanged.
	// Reusing the parent's circuit breaker (rather than giving the
	// subagent its own) is intentional (design_contract.zsh §7:
	// "circuit breaker per provider/model ... REUSE yang udah ada"),
	// not a context-isolation gap -- see package doc / AC-02 note on
	// ParentAgentCtx for what IS isolated.
	ProviderOrder []string
	Breaker       *llmclient.BreakerStore

	// Dispatcher is the PARENT's full dispatcher. SpawnSubagent never
	// passes this to RunLoop directly -- it derives a role-scoped
	// Dispatcher.Subset(AllowedTools(role)) first (AC-01). A tool
	// outside the role's allowlist is therefore not merely rejected by
	// a permission check the subagent could race or misconfigure --
	// it does not exist on the Dispatcher the subagent's loop holds at
	// all.
	Dispatcher *tools.Dispatcher

	// ParentAgentCtx, Config, Tracker, Ask, Cwd mirror tools.PermDeps'
	// fields, split apart because SpawnSubagent must NOT forward
	// ParentAgentCtx into the subagent's own tools.PermDeps unchanged.
	//
	// AC-02 ("Subagent tidak bisa membuka circuit breaker/provider
	// config milik parent agent (isolasi context)"): permission.
	// AgentContext is a mutable pointer -- Grant() calls made while
	// dispatching a subagent tool call would, if the parent's
	// AgentContext were reused directly, permanently escalate
	// capabilities on the PARENT's context too, once the subagent
	// returns. SpawnSubagent instead calls permission.NewAgentContext
	// with Role: permission.RoleSubagent (which SESSION-42 already
	// hard-clamps below RolePrimary -- see permission/context.go's
	// subagentDeniedCaps) and a session ID distinct from the parent's
	// (FASE 13), so escalations inside the subagent's run can never
	// leak back into the parent's context or vice versa. Config
	// (write/shell/process mode strings) and Ask (the interactive
	// confirmation hook) are plain values/funcs, not shared mutable
	// state, so those ARE reused as-is -- there is no new permission
	// model for subagents (design_contract.zsh §4/§6.2's own framing:
	// "BUKAN permission model global baru").
	ParentAgentCtx *permission.AgentContext
	Config         permission.PermConfig
	Tracker        *permission.ApprovalTracker
	Ask            permission.AskFunc
	Cwd            string

	// ParentSessionID names the parent run, used only to derive a
	// distinct subagent session ID (FASE 13: "parent session ID ≠
	// subagent session ID"). May be "" (a fresh id is still derived,
	// just without the parent prefix).
	ParentSessionID string

	// TermuxContext is injected into the coder sysprompt only (see
	// sysprompt.go's BuildSysprompt doc). "" is fine or absent Termux
	// deployments/tests.
	TermuxContext string

	// Log receives the same plain progress lines agent.Deps.Log does
	// (forwarded to RunLoop unchanged). May be nil.
	Log func(string)

	// Complete overrides the provider/model completion primitive,
	// forwarded to agent.Deps.Complete unchanged. nil means "use the
	// real provider loop" -- tests substitute a fake here exactly like
	// agent.Deps.Complete's own doc comment describes.
	Complete func(ctx context.Context, deps agent.Deps, messages []llmclient.Message) (llmclient.Response, error)

	// MaxSteps optionally overrides the subagent's step budget
	// (AI_SUBAGENT_MAX_STEPS in the zsh source). 0 means "use
	// Limits.AgentMaxSteps". Per design_contract.zsh §7 ("SAMA atau
	// LEBIH KECIL... tidak boleh lebih besar"), a value greater than
	// Limits.AgentMaxSteps is clamped down, never allowed through.
	MaxSteps int

	// Depth is the current subagent nesting depth as seen by THIS
	// call (0 for a subagent spawned directly by the primary agent).
	// MaxDepth is the ceiling Depth must stay under; 0 means "default
	// 1" (a subagent may not itself spawn a further subagent -- there
	// is currently no tool in tools.Registry that would let a
	// subagent's own loop reach SpawnSubagent recursively at all, but
	// FASE 14 asks for the guard to exist regardless, as a forward
	// guarantee rather than an incidental one).
	Depth    int
	MaxDepth int
}

// errNilDispatcher is the one case SpawnSubagent treats as a caller
// programming error (returned as a Go error) rather than a normal
// subagent-failure Result: there is no meaningful "role permission
// denied" story to tell when there was never a Dispatcher to scope in
// the first place.
var errNilDispatcher = errors.New("subagent: Deps.Dispatcher is nil")

// SpawnSubagent runs role against subGoal to completion, blockage, or ctx
// cancellation, and returns a structured Result -- never panics, never
// returns a non-nil error for an ordinary subagent failure (invalid
// role, empty goal, depth exceeded, tool failure, step-limit exhaustion,
// cancellation: all of these come back as Result{Status: StatusFailed,
// ...}, err == nil, mirroring _ai_subagent_run's own contract of always
// `return 0`/`return 1` from itself rather than aborting its caller).
// The returned error is non-nil only for the Dispatcher-nil case above.
func SpawnSubagent(ctx context.Context, deps Deps, role Role, subGoal string) (Result, error) {
	if !IsValidRole(role) {
		return Result{
			Status:  StatusFailed,
			Role:    role,
			Summary: "Role subagent tidak dikenal.",
			Error:   fmt.Sprintf("role harus 'researcher' atau 'coder', dapat: '%s'", role),
		}, nil
	}
	if subGoal == "" {
		return Result{
			Status:  StatusFailed,
			Role:    role,
			Summary: "sub_goal kosong.",
			Error:   "sub_goal wajib diisi",
		}, nil
	}
	if deps.Dispatcher == nil {
		return Result{}, errNilDispatcher
	}

	maxDepth := deps.MaxDepth
	if maxDepth <= 0 {
		maxDepth = 1
	}
	if deps.Depth >= maxDepth {
		return Result{
			Status:  StatusFailed,
			Role:    role,
			Summary: "Subagent depth limit tercapai.",
			Error:   fmt.Sprintf("subagent nesting depth %d melebihi batas %d", deps.Depth, maxDepth),
		}, nil
	}

	// FASE 5/AC-01: role-scoped dispatcher, built BEFORE the loop runs
	// at all -- a tool outside AllowedTools(role) simply is not present
	// on scoped, so RunLoop's own runTool -> Dispatcher.Dispatch call
	// rejects it exactly like an unknown tool, before permission.
	// CheckPermission is ever reached for it.
	scoped := deps.Dispatcher.Subset(AllowedTools(role))

	// FASE 5/AC-02: a fresh, subagent-scoped AgentContext -- never the
	// parent's pointer. See Deps.ParentAgentCtx doc comment above.
	subCtx := subagentContext(deps, role)

	limits := deps.Limits
	limits.AgentMaxSteps = subagentMaxSteps(deps)

	agentDeps := agent.Deps{
		Limits:        limits,
		ProviderOrder: deps.ProviderOrder,
		Breaker:       deps.Breaker,
		SystemPrompt:  BuildSysprompt(role, subGoal, deps.TermuxContext),
		Dispatcher:    scoped,
		PermDeps: tools.PermDeps{
			AgentCtx: subCtx,
			Config:   deps.Config,
			Tracker:  deps.Tracker,
			Ask:      deps.Ask,
			Cwd:      deps.Cwd,
		},
		// No Store/SessionID: subagents have no persistent checkpoint
		// (FASE 12) -- see package doc comment.
		Log:      deps.Log,
		Complete: deps.Complete,
	}

	final, err := agent.RunLoop(ctx, agentDeps, subGoal, nil)
	if err != nil {
		// RC-012 tier-1 fatal transition failure from RunLoop itself
		// (a bug in the state machine, not an ordinary subagent
		// outcome) -- FASE 15 says the parent must not crash, so this
		// still comes back as a Result, not a propagated error.
		return Result{
			Status:  StatusFailed,
			Role:    role,
			Summary: "Subagent gagal karena kegagalan internal state machine.",
			Error:   err.Error(),
		}, nil
	}

	return toResult(role, final), nil
}

// subagentMaxSteps applies design_contract.zsh §7's "SAMA atau LEBIH
// KECIL, tidak boleh lebih besar" rule: deps.MaxSteps if set and not
// greater than the parent ceiling, else the parent ceiling itself.
func subagentMaxSteps(deps Deps) int {
	parentMax := deps.Limits.AgentMaxSteps
	if parentMax <= 0 {
		parentMax = 15 // agent.RunLoop's own default, mirrored here for the clamp comparison
	}
	if deps.MaxSteps > 0 && deps.MaxSteps < parentMax {
		return deps.MaxSteps
	}
	return parentMax
}

// subagentContext builds the isolated permission.AgentContext described
// in Deps.ParentAgentCtx's doc comment: same ProjectRoot as the parent
// (a subagent operates in the same project, it is not sandboxed to a
// different one), RoleSubagent (SESSION-42's capability ceiling), and a
// session ID distinct from the parent's (FASE 13).
func subagentContext(deps Deps, role Role) *permission.AgentContext {
	projectRoot := ""
	yolo := false
	parentSessionID := deps.ParentSessionID
	if deps.ParentAgentCtx != nil {
		projectRoot = deps.ParentAgentCtx.ProjectRoot
		yolo = deps.ParentAgentCtx.YoloMode
	}
	sessionID := parentSessionID + "-subagent-" + string(role)
	if parentSessionID == "" {
		sessionID = "subagent-" + string(role)
	}
	return permission.NewAgentContext(sessionID, projectRoot, yolo, permission.RoleSubagent)
}

// toResult maps a terminal agent.FinalResult onto Result, mirroring
// _ai_subagent_run's trailing echo block (status/summary/findings-or-
// changes/files_affected/error).
func toResult(role Role, final agent.FinalResult) Result {
	status := StatusFailed
	if final.Phase == agent.PhaseComplete && final.Done {
		status = StatusSuccess
	}

	summary := final.Thought
	if summary == "" {
		if status == StatusSuccess {
			summary = "Subagent selesai tanpa catatan tambahan."
		} else {
			summary = final.BlockReason
		}
	}

	res := Result{
		Status:        status,
		Role:          role,
		Summary:       summary,
		FilesAffected: append([]string(nil), final.TouchedFiles...),
		Error:         final.BlockReason,
	}
	if status == StatusSuccess {
		res.Error = ""
	}

	// final_thought fallback to summary, then role-conditional split --
	// literal port of 20-run.zsh's trailing if/else.
	lastThought := final.Thought
	if lastThought == "" {
		lastThought = summary
	}
	if role == RoleResearcher {
		res.Findings = lastThought
	} else {
		res.Changes = lastThought
	}

	return res
}
