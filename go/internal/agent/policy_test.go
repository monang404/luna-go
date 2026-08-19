package agent

import (
	"errors"
	"testing"
)

// allPhases enumerates every known phase, used to build the exhaustive
// 5x5 transition matrix test (rule 21).
var allPhases = []Phase{PhasePlan, PhaseExecute, PhaseVerify, PhaseComplete, PhaseBlocked}

// wantValid mirrors AI_AGENT_STATE_TRANSITIONS from
// 39-agent-state-machine.zsh exactly:
//
//	PLAN     "EXECUTE VERIFY BLOCKED"
//	EXECUTE  "PLAN VERIFY BLOCKED"
//	VERIFY   "PLAN EXECUTE COMPLETE BLOCKED"
//	COMPLETE ""
//	BLOCKED  ""
var wantValid = map[Phase]map[Phase]bool{
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

func TestCanTransition_ExhaustiveMatrix(t *testing.T) {
	for _, from := range allPhases {
		for _, to := range allPhases {
			want := wantValid[from][to]
			got := CanTransition(from, to)
			if got != want {
				t.Errorf("CanTransition(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestCanTransition_SelfTransitionsAreInvalid(t *testing.T) {
	// Rule 9: "Jangan mengasumsikan self-transition valid." None of the
	// five states self-transition in the zsh matrix.
	for _, p := range allPhases {
		if CanTransition(p, p) {
			t.Errorf("CanTransition(%s, %s) = true, want false (no self-transition in source)", p, p)
		}
	}
}

func TestCanTransition_UnknownFrom(t *testing.T) {
	if CanTransition(Phase("BOGUS"), PhasePlan) {
		t.Errorf("CanTransition from unknown phase should be false")
	}
}

func TestTransition_ValidTransitionsUpdateState(t *testing.T) {
	valid := []struct{ from, to Phase }{
		{PhasePlan, PhaseExecute},
		{PhasePlan, PhaseVerify},
		{PhasePlan, PhaseBlocked},
		{PhaseExecute, PhasePlan},
		{PhaseExecute, PhaseVerify},
		{PhaseExecute, PhaseBlocked},
		{PhaseVerify, PhasePlan},
		{PhaseVerify, PhaseExecute},
		{PhaseVerify, PhaseComplete},
		{PhaseVerify, PhaseBlocked},
	}
	for _, tt := range valid {
		s := &AgentState{Phase: tt.from, Goal: "g"}
		if err := Transition(s, tt.to); err != nil {
			t.Errorf("Transition(%s -> %s) unexpected error: %v", tt.from, tt.to, err)
		}
		if s.Phase != tt.to {
			t.Errorf("Transition(%s -> %s): state.Phase = %s, want %s", tt.from, tt.to, s.Phase, tt.to)
		}
	}
}

func TestTransition_InvalidTransitionsRejectedAndStateUnchanged(t *testing.T) {
	invalid := []struct{ from, to Phase }{
		// terminal states: nowhere
		{PhaseComplete, PhasePlan},
		{PhaseComplete, PhaseExecute},
		{PhaseComplete, PhaseVerify},
		{PhaseComplete, PhaseBlocked},
		{PhaseBlocked, PhasePlan},
		{PhaseBlocked, PhaseExecute},
		{PhaseBlocked, PhaseVerify},
		{PhaseBlocked, PhaseComplete},
		// self-transitions
		{PhasePlan, PhasePlan},
		{PhaseExecute, PhaseExecute},
		{PhaseVerify, PhaseVerify},
		// not-in-matrix
		{PhasePlan, PhaseComplete},
		{PhaseExecute, PhaseComplete},
	}
	for _, tt := range invalid {
		s := &AgentState{Phase: tt.from, Goal: "g"}
		err := Transition(s, tt.to)
		if err == nil {
			t.Errorf("Transition(%s -> %s) expected error, got nil", tt.from, tt.to)
			continue
		}
		var ite *InvalidTransitionError
		if !errors.As(err, &ite) {
			t.Errorf("Transition(%s -> %s) error is not InvalidTransitionError: %v", tt.from, tt.to, err)
		}
		if s.Phase != tt.from {
			t.Errorf("Transition(%s -> %s): state.Phase mutated to %s on failure", tt.from, tt.to, s.Phase)
		}
	}
}

func TestTransition_ErrorMessageIsDescriptive(t *testing.T) {
	s := &AgentState{Phase: PhaseComplete, Goal: "g"}
	err := Transition(s, PhasePlan)
	if err == nil {
		t.Fatal("expected error")
	}
	want := "invalid agent lifecycle transition: COMPLETE -> PLAN"
	if err.Error() != want {
		t.Errorf("error message = %q, want %q", err.Error(), want)
	}
}

func TestTransition_NilState(t *testing.T) {
	if err := Transition(nil, PhasePlan); err == nil {
		t.Fatal("expected error for nil state")
	}
}

func TestTransition_InvalidTargetPhase(t *testing.T) {
	s := &AgentState{Phase: PhasePlan, Goal: "g"}
	if err := Transition(s, Phase("RUNNING")); err == nil {
		t.Fatal("expected error for invalid target phase")
	}
	if s.Phase != PhasePlan {
		t.Errorf("state mutated on invalid target phase: %s", s.Phase)
	}
}

func TestTransition_RejectsAlreadyInvalidState(t *testing.T) {
	s := &AgentState{Phase: PhasePlan, Goal: ""} // fails Validate: empty goal
	if err := Transition(s, PhaseExecute); err == nil {
		t.Fatal("expected error transitioning from an already-invalid state")
	}
}

// TestDeadEndAnalysis documents the reachability/dead-end audit (rule
// 27): every non-terminal phase can reach every terminal phase, and
// PLAN/EXECUTE/VERIFY are all reachable from each other (forming one
// connected non-terminal component) with COMPLETE reachable only via
// VERIFY, matching the zsh source exactly.
func TestDeadEndAnalysis(t *testing.T) {
	reachable := func(from Phase) map[Phase]bool {
		seen := map[Phase]bool{from: true}
		queue := []Phase{from}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, to := range allPhases {
				if CanTransition(cur, to) && !seen[to] {
					seen[to] = true
					queue = append(queue, to)
				}
			}
		}
		return seen
	}

	fromPlan := reachable(PhasePlan)
	for _, p := range allPhases {
		if !fromPlan[p] {
			t.Errorf("phase %s is unreachable from PLAN", p)
		}
	}

	// COMPLETE and BLOCKED must be true dead ends: nothing reachable
	// from them except themselves.
	for _, term := range []Phase{PhaseComplete, PhaseBlocked} {
		r := reachable(term)
		if len(r) != 1 || !r[term] {
			t.Errorf("terminal phase %s is not a dead end: reaches %v", term, r)
		}
	}
}
