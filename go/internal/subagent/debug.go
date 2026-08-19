// Ported from 30-luna/55-subagent/25-debug_allowlist.zsh (allowlist.go's
// debugTools/DebugToolAllowed already ports this file's function),
// 30-luna/55-subagent/30-debug_step.zsh, 30-luna/55-subagent/35-debug_report.zsh,
// and 30-luna/55-subagent/40-debug.zsh (aidebug()). SESSION-51 scope only.
//
// Like SpawnSubagent, RunDebug reuses agent.RunLoop rather than
// re-implementing a second loop -- 30-debug_step.zsh is structurally the
// same chat+tool step as 15-run_step.zsh (and RunLoop's own runTool),
// just with the debug allowlist swapped in and a diagnosis-only
// sysprompt. What is genuinely different about `luna debug` and therefore
// DOES get its own code here: the forced-safe permission defaults
// 40-debug.zsh applies with `local AI_AGENT_YOLO_MODE=0; local
// AI_PERM_SHELL_MODE=ask_always`, and the structured Report shape from
// 35-debug_report.zsh.
package subagent

import (
	"context"

	"github.com/monang404/luna-go/internal/agent"
	"github.com/monang404/luna-go/internal/tools"
)

// debugSysprompt is 40-debug.zsh's literal inline sysprompt. Kept as a
// package-level template function for the same reason
// BuildSysprompt(role, ...) is: a pure function of the one caller
// argument that varies (the problem description).
func debugSysprompt(problem string) string {
	return "You are a debugging agent.\n\n" +
		"Goal:\n" + problem + "\n\n" +
		"Your job is to diagnose the problem.\n\n" +
		"You may inspect files and run tests or commands needed to reproduce\n" +
		"and understand the issue.\n\n" +
		"Do NOT modify files.\n" +
		"Do NOT propose executing file mutations through tools.\n\n" +
		"Return a concise diagnosis and recommended next steps.\n\n" +
		"Do not claim a fix was applied.\n\n" +
		`You must respond only as JSON:` + "\n" +
		`{"thought":"...","tool":"...","args":{...},"done":true|false}`
}

// Report is the structured diagnosis output, a field-for-field port of
// _ai_debug_print_report's sections (AC-04: "Debug report yang
// dihasilkan memuat field yang sama dengan debug_report.zsh: ringkasan
// langkah, hasil, error"). A renderer that wants the exact zsh text
// layout can format these fields itself; this session does not own
// presentation (see docs/execution_sessions/51_..._orchestration.yaml's
// scope.exclude: "UI rendering progress subagent secara live").
type Report struct {
	// Diagnosis is final_thought, or "Unable to determine root cause."
	// when empty (both the "error present" and "no error, no thought"
	// zsh branches print that same fallback string).
	Diagnosis string
	// AffectedFiles lists paths read/inspected during the run, sourced
	// from agent.FinalResult.TouchedFiles (see Result.FilesAffected's
	// doc comment in run.go for the same success-only-tracking
	// deviation note -- it applies here identically).
	AffectedFiles []string
	// Reproduction lists "tool: output" lines for every run_test/
	// run_command call, in the order they occurred.
	//
	// Deviation from source (documented per FASE 2): 30-debug_step.zsh
	// appends a reproduction entry for every run_test/run_command call
	// regardless of success. agent.RunLoop does not expose a per-step
	// tool/output trace to its caller (FinalResult is a terminal
	// summary, not a transcript -- SESSION-50's own scope), so this
	// port cannot reconstruct Reproduction from RunLoop's return value
	// alone. Left empty for now; a future session that wants this back
	// would need RunLoop (or a caller-supplied Deps.Log sink) to
	// surface tool-level events, which is out of SESSION-51's scope
	// (no changes to SESSION-50's loop are permitted here).
	Reproduction []string
	// Error is the failure detail, "" on success.
	Error string
	// Success mirrors diagnosis_status == "success" (final done:true
	// claim reached, not "a fix was applied" -- aidebug() never applies
	// fixes).
	Success bool
}

// RunDebug runs a bounded, read-mostly diagnosis session for problem and
// returns a Report -- the Go port of aidebug(). Like SpawnSubagent, it
// never returns a non-nil error for an ordinary outcome; the returned
// error is reserved for the same Dispatcher-nil / RunLoop-fatal cases
// SpawnSubagent uses.
func RunDebug(ctx context.Context, deps Deps, problem string) (Report, error) {
	if problem == "" {
		return Report{Error: "problem wajib diisi"}, nil
	}
	if deps.Dispatcher == nil {
		return Report{}, errNilDispatcher
	}

	scoped := deps.Dispatcher.Subset(debugTools)

	// 40-debug.zsh's forced-safe override: regardless of what the
	// caller's PermConfig/AgentContext say, `luna debug` never runs with
	// yolo shell mode. Ported as a value copy here (not a mutation of
	// deps.ParentAgentCtx), matching the zsh `local` shadow's own
	// scope: only THIS run is forced safe, the caller's Deps is
	// untouched for any other use.
	safeConfig := deps.Config
	safeConfig.ShellMode = "ask_always"

	debugCtx := subagentContext(deps, "debug")
	debugCtx.YoloMode = false

	limits := deps.Limits
	maxSteps := deps.MaxSteps
	if maxSteps <= 0 {
		maxSteps = limits.AgentMaxSteps
	}
	if maxSteps <= 0 {
		maxSteps = 8 // aidebug()'s own default (AI_DEBUG_MAX_STEPS ?: AI_AGENT_MAX_STEPS ?: 8)
	}
	limits.AgentMaxSteps = maxSteps

	agentDeps := agent.Deps{
		Limits:        limits,
		ProviderOrder: deps.ProviderOrder,
		Breaker:       deps.Breaker,
		SystemPrompt:  debugSysprompt(problem),
		Dispatcher:    scoped,
		PermDeps: tools.PermDeps{
			AgentCtx: debugCtx,
			Config:   safeConfig,
			Tracker:  deps.Tracker,
			Ask:      deps.Ask,
			Cwd:      deps.Cwd,
		},
		Log:      deps.Log,
		Complete: deps.Complete,
	}

	final, err := agent.RunLoop(ctx, agentDeps, problem, nil)
	if err != nil {
		return Report{Error: err.Error()}, nil
	}

	return toReport(final), nil
}

// toReport maps a terminal agent.FinalResult onto Report, mirroring
// _ai_debug_print_report's field derivation (minus presentation).
func toReport(final agent.FinalResult) Report {
	success := final.Phase == agent.PhaseComplete && final.Done

	diagnosis := final.Thought
	if diagnosis == "" {
		diagnosis = "Unable to determine root cause."
	}

	errStr := final.BlockReason
	if success {
		errStr = ""
	}

	return Report{
		Diagnosis:     diagnosis,
		AffectedFiles: append([]string(nil), final.TouchedFiles...),
		Reproduction:  nil,
		Error:         errStr,
		Success:       success,
	}
}
