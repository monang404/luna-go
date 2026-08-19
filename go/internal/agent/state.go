// Ported from 30-luna/50-agent/39-agent-state-machine.zsh (lifecycle state
// definitions) and the persistent-state subset of 30-luna/50-agent/10-state.zsh
// and 30-luna/50-agent/42-execution/00-loop_main.zsh (the fields
// _ai_agent_execute_loop actually persists to $state_dir at the end of a
// run: step, done, block_reason, thought, commands_run, touched_files,
// changed_files).
//
// SESSION-49 scope only: this file defines the in-memory state shape and
// pure validation/construction helpers. It does not run the ReAct loop,
// does not call any provider/tool, and does not do IO. See policy.go for
// transition rules and checkpoint.go for persistence.
package agent

import "fmt"

// Phase is the agent lifecycle state, a literal port of the five states
// enumerated in AI_AGENT_STATE_TRANSITIONS (39-agent-state-machine.zsh).
// It is a typed string (not `int`) so persisted/serialized values
// (lifecycle_state file, JSON, error messages) stay human-readable and
// identical to the zsh state names, without needing custom
// marshal/unmarshal code.
type Phase string

const (
	// PhasePlan: agent is asking the provider for the next thought/tool
	// (or a done:true claim). Initial state set by _ai_agent_state_init.
	PhasePlan Phase = "PLAN"
	// PhaseExecute: a tool call has been accepted and is about to run
	// (or is running). Entered right before _ai_agent_exec_run_tool.
	PhaseExecute Phase = "EXECUTE"
	// PhaseVerify: the provider claimed done:true and that claim is
	// being checked (commands_run > 0, touched files pass syntax
	// checks) before it can be accepted. Entered by
	// _ai_agent_exec_check_done_rejections.
	PhaseVerify Phase = "VERIFY"
	// PhaseComplete: terminal. Reached only from VERIFY once a done:true
	// claim survives rejection checks.
	PhaseComplete Phase = "COMPLETE"
	// PhaseBlocked: terminal. Reached on any exit path that is not a
	// verified completion (cancellation, fatal state-transition failure,
	// max-steps exhaustion, provider failure, no-tool-and-not-done).
	PhaseBlocked Phase = "BLOCKED"
)

// IsTerminal reports whether p is a terminal lifecycle state. Per
// AI_AGENT_STATE_TRANSITIONS, COMPLETE and BLOCKED both map to the empty
// transition set ("") -- nothing may follow them.
func (p Phase) IsTerminal() bool {
	return p == PhaseComplete || p == PhaseBlocked
}

// IsValid reports whether p is one of the five known lifecycle states.
// Any other value (including "") is not a phase the zsh source ever
// produces.
func (p Phase) IsValid() bool {
	switch p {
	case PhasePlan, PhaseExecute, PhaseVerify, PhaseComplete, PhaseBlocked:
		return true
	default:
		return false
	}
}

// AgentState is the persistent, per-run agent state. Field selection is
// deliberately narrow and traces directly to what
// _ai_agent_execute_loop (00-loop_main.zsh) writes to $state_dir at the
// end of a run, plus the Goal that seeds the whole run:
//
//	lifecycle_state -> Phase
//	(seed input)    -> Goal
//	step            -> Step
//	done            -> Done
//	block_reason    -> BlockReason
//	thought         -> Thought
//	commands_run    -> CommandsRun
//	touched_files   -> TouchedFiles
//	changed_files   -> ChangedFiles
//
// Deliberately EXCLUDED (SESSION-50 / ephemeral execution-loop concern,
// per session boundary rule 20; see docs/MIGRATION_TRACEABILITY.md for the
// full rationale of each):
//
//   - reply/thought/tool/args/done_flag/pdir/chat_status as raw per-iteration
//     locals -- only the final Thought/Done survive into AgentState.
//   - last_failed_tool, last_failed_args, same_fail_count -- same-command
//     retry bookkeeping local to one running loop, never written to
//     $state_dir and never read back by anything outside the loop.
//   - PendingTool / proposed-but-not-yet-run tool+args -- EXECUTE is
//     entered only immediately before running the tool in the zsh source;
//     there is no persisted "pending" tool state to port.
type AgentState struct {
	// Phase is the current lifecycle state.
	Phase Phase

	// Goal is the user-supplied task description that seeded this run.
	Goal string

	// Step is the current ReAct iteration counter (0-based before the
	// first plan request, matching zsh's `local step=$step_offset`).
	Step int

	// Done mirrors the loop's final done_flag: whether the provider's
	// last accepted claim was done:true.
	Done bool

	// BlockReason mirrors $state_dir/block_reason: a human-readable
	// explanation, set only when the run ends in PhaseBlocked (or, per
	// RC-012 tier 1, alongside a forced transition attempt to BLOCKED
	// on a fatal state-transition failure). Empty otherwise.
	BlockReason string

	// Thought is the last thought text returned by the provider,
	// mirrored from $state_dir/thought.
	Thought string

	// CommandsRun mirrors $state_dir/commands_run: how many tool calls
	// actually executed this run. Used by the VERIFY rejection check
	// (a done:true claim with CommandsRun == 0 is rejected as
	// unverified).
	CommandsRun int

	// TouchedFiles mirrors $state_dir/touched_files: files the run
	// opened/edited, used for the syntax-check gate before accepting a
	// done:true claim.
	TouchedFiles []string

	// ChangedFiles mirrors $state_dir/changed_files: the subset of
	// TouchedFiles whose content actually changed (used downstream for
	// finalize/summary; SESSION-53 UI, out of scope here beyond storing
	// the field faithfully).
	ChangedFiles []string
}

// NewState constructs the initial AgentState for a fresh run: PhasePlan,
// the given goal, and every other field at its zero value. This mirrors
// _ai_agent_state_init, which writes lifecycle_state=PLAN and nothing
// else (step/done/etc. are only written once, at the end of the loop).
func NewState(goal string) *AgentState {
	return &AgentState{
		Phase: PhasePlan,
		Goal:  goal,
	}
}

// Validate reports whether s is internally consistent. It never mutates
// s. Validate is intentionally narrow -- it checks structural invariants
// (valid phase, non-negative counters, terminal-state consistency), not
// business rules that belong to SESSION-50 (e.g. "Done can only be true
// if CommandsRun > 0" is a rejection-check concern enforced by the loop,
// not an invariant of the state shape itself).
func (s *AgentState) Validate() error {
	if s == nil {
		return fmt.Errorf("agent: nil AgentState")
	}
	if !s.Phase.IsValid() {
		return fmt.Errorf("agent: invalid phase %q", s.Phase)
	}
	if s.Goal == "" {
		return fmt.Errorf("agent: empty goal")
	}
	if s.Step < 0 {
		return fmt.Errorf("agent: negative step %d", s.Step)
	}
	if s.CommandsRun < 0 {
		return fmt.Errorf("agent: negative commands_run %d", s.CommandsRun)
	}
	return nil
}

// IsTerminal reports whether s.Phase is a terminal lifecycle state.
func (s *AgentState) IsTerminal() bool {
	return s.Phase.IsTerminal()
}
