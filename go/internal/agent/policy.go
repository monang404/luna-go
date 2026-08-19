// Ported from 30-luna/50-agent/39-agent-state-machine.zsh
// (AI_AGENT_STATE_TRANSITIONS, _ai_agent_state_transition). This is a
// literal port of the transition matrix -- no state has been added,
// removed, or made mutable that the zsh source does not already have.
package agent

import "fmt"

// transitions is the canonical lifecycle transition matrix, a literal
// port of AI_AGENT_STATE_TRANSITIONS:
//
//	PLAN     "EXECUTE VERIFY BLOCKED"
//	EXECUTE  "PLAN VERIFY BLOCKED"
//	VERIFY   "PLAN EXECUTE COMPLETE BLOCKED"
//	COMPLETE ""
//	BLOCKED  ""
//
// It is unexported and never returned by reference, so callers cannot
// mutate the canonical matrix (rule: "Jangan menyimpan transition matrix
// sebagai mutable package global yang bisa diubah caller").
var transitions = map[Phase]map[Phase]bool{
	PhasePlan: {
		PhaseExecute: true,
		PhaseVerify:  true,
		PhaseBlocked: true,
	},
	PhaseExecute: {
		PhasePlan:    true,
		PhaseVerify:  true,
		PhaseBlocked: true,
	},
	PhaseVerify: {
		PhasePlan:     true,
		PhaseExecute:  true,
		PhaseComplete: true,
		PhaseBlocked:  true,
	},
	PhaseComplete: {},
	PhaseBlocked:  {},
}

// CanTransition reports whether next is a valid transition target from
// from, per the canonical matrix above. Self-transitions (e.g.
// PLAN->PLAN) are valid only if explicitly listed -- the zsh source does
// not list any, so none are valid here either (rule 9: "Jangan
// mengasumsikan self-transition valid").
//
// An unrecognized `from` (not one of the five known phases) always
// returns false, matching the zsh behavior: looking up an unknown key
// in AI_AGENT_STATE_TRANSITIONS yields an empty allowed-list, so
// `[[ " " != *" $next "* ]]` is true and the transition is rejected.
func CanTransition(from, to Phase) bool {
	allowed, ok := transitions[from]
	if !ok {
		return false
	}
	return allowed[to]
}

// InvalidTransitionError is returned by Transition when the requested
// transition is not permitted by the canonical matrix. It mirrors the
// zsh error text ("Invalid agent lifecycle transition: $current -> $next")
// so downstream logging/tests can match on the same message shape.
type InvalidTransitionError struct {
	From Phase
	To   Phase
}

func (e *InvalidTransitionError) Error() string {
	return fmt.Sprintf("invalid agent lifecycle transition: %s -> %s", e.From, e.To)
}

// Transition attempts to move state from its current Phase to next. On
// success, state.Phase is updated to next and nil is returned. On
// failure (invalid transition, or state fails Validate()), state is left
// completely unchanged and a descriptive error is returned -- mirroring
// _ai_agent_state_transition, which only ever overwrites
// $state_dir/lifecycle_state on the success path (see also rule 22:
// "Jangan sampai invalid transition mengubah state sebelum validation
// selesai").
func Transition(state *AgentState, next Phase) error {
	if state == nil {
		return fmt.Errorf("agent: nil AgentState")
	}
	if err := state.Validate(); err != nil {
		return fmt.Errorf("agent: cannot transition invalid state: %w", err)
	}
	if !next.IsValid() {
		return fmt.Errorf("agent: invalid transition target %q", next)
	}
	if !CanTransition(state.Phase, next) {
		return &InvalidTransitionError{From: state.Phase, To: next}
	}
	state.Phase = next
	return nil
}
