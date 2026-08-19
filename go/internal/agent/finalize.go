// Ported from the data-derivation half of 30-luna/50-agent/44-finalize.zsh
// (_ai_agent_finalize). Everything that file does to *print* a
// COMPLETE/BLOCKED report (the box renderer, muted file-name listing,
// diff/review push to a detail log, "/details" hint) is UI/presentation
// and stays out of scope for this session (SESSION-53 owns rendering,
// per the session's own why_not_more note: "loop ini dites lewat
// log/print sederhana dulu ... supaya integrasi logic bisa diverifikasi
// terpisah dari kompleksitas visual"). What this file does port is the
// data-shaping logic that has nothing to do with drawing a box:
//   - defaulting an empty BlockReason on a BLOCKED run
//     ("Agent berhenti (step $step), alasan spesifik gak tercatat.")
//   - the terminal-state -> summary-fields mapping a renderer would need
//
// Checkpoint cleanup on a verified COMPLETE run (`rm -f checkpoint_file`)
// is NOT done here -- that is an I/O side effect tied to the loop's own
// Store/SessionID, not a pure function of AgentState, so it stays in
// loop.go's RunLoop right where the COMPLETE transition happens (see
// that file's doc comment for the exact call site).
package agent

import "fmt"

// FinalResult is the machine-readable outcome of one RunLoop call: the
// same fields 44-finalize.zsh reads out of $state_dir before it starts
// drawing anything, minus renderer-only concerns. A later
// UI/rendering session builds its box/summary output from this struct,
// not from AgentState directly.
type FinalResult struct {
	// Phase is the terminal lifecycle state: PhaseComplete or
	// PhaseBlocked. Any other value means RunLoop returned without
	// reaching a terminal state, which is itself a bug in the caller.
	Phase Phase
	// Goal is the run's original goal text.
	Goal string
	// Steps is the final ReAct iteration counter.
	Steps int
	// Done mirrors the loop's final done_flag.
	Done bool
	// BlockReason is always non-empty when Phase == PhaseBlocked (see
	// package doc comment above for the default-fill rule), and always
	// "" when Phase == PhaseComplete.
	BlockReason string
	// Thought is the last thought text the provider returned.
	Thought string
	// CommandsRun is how many tool calls actually executed.
	CommandsRun int
	// TouchedFiles are files opened/edited during the run.
	TouchedFiles []string
	// ChangedFiles is the subset of TouchedFiles whose content actually
	// changed (write_file/edit_file successes).
	ChangedFiles []string
}

// Finalize converts a terminal AgentState into a FinalResult. state must
// already be in a terminal phase (IsTerminal()) -- Finalize does not
// itself decide COMPLETE vs BLOCKED (that decision is RunLoop's, made
// from the same evidence 00-loop_main.zsh's own trailing if/elif block
// uses: done_flag plus the lifecycle state at the moment the loop
// exited). Finalize performs no I/O and never mutates state.
func Finalize(state *AgentState) FinalResult {
	if state == nil {
		return FinalResult{}
	}

	blockReason := state.BlockReason
	if state.Phase == PhaseBlocked && blockReason == "" {
		// Mirrors 44-finalize.zsh:
		//   [ -z "$block_reason" ] && block_reason="Agent berhenti (step $step), alasan spesifik gak tercatat."
		blockReason = fmt.Sprintf("Agent berhenti (step %d), alasan spesifik gak tercatat.", state.Step)
	} else if state.Phase == PhaseComplete {
		blockReason = ""
	}

	touched := append([]string(nil), state.TouchedFiles...)
	changed := append([]string(nil), state.ChangedFiles...)

	return FinalResult{
		Phase:        state.Phase,
		Goal:         state.Goal,
		Steps:        state.Step,
		Done:         state.Done,
		BlockReason:  blockReason,
		Thought:      state.Thought,
		CommandsRun:  state.CommandsRun,
		TouchedFiles: touched,
		ChangedFiles: changed,
	}
}
