package agent

import "testing"

func TestNewState(t *testing.T) {
	s := NewState("fix the bug")
	if s.Phase != PhasePlan {
		t.Fatalf("NewState phase = %q, want PLAN", s.Phase)
	}
	if s.Goal != "fix the bug" {
		t.Fatalf("NewState goal = %q, want %q", s.Goal, "fix the bug")
	}
	if s.Step != 0 || s.Done || s.CommandsRun != 0 {
		t.Fatalf("NewState non-zero defaults: %+v", s)
	}
	if err := s.Validate(); err != nil {
		t.Fatalf("NewState result failed Validate: %v", err)
	}
}

func TestPhaseIsTerminal(t *testing.T) {
	tests := []struct {
		phase Phase
		want  bool
	}{
		{PhasePlan, false},
		{PhaseExecute, false},
		{PhaseVerify, false},
		{PhaseComplete, true},
		{PhaseBlocked, true},
	}
	for _, tt := range tests {
		if got := tt.phase.IsTerminal(); got != tt.want {
			t.Errorf("Phase(%q).IsTerminal() = %v, want %v", tt.phase, got, tt.want)
		}
	}
}

func TestPhaseIsValid(t *testing.T) {
	valid := []Phase{PhasePlan, PhaseExecute, PhaseVerify, PhaseComplete, PhaseBlocked}
	for _, p := range valid {
		if !p.IsValid() {
			t.Errorf("Phase(%q).IsValid() = false, want true", p)
		}
	}
	invalid := []Phase{"", "RUNNING", "plan", "Done", "unknown"}
	for _, p := range invalid {
		if p.IsValid() {
			t.Errorf("Phase(%q).IsValid() = true, want false", p)
		}
	}
}

func TestAgentStateValidate(t *testing.T) {
	tests := []struct {
		name    string
		state   *AgentState
		wantErr bool
	}{
		{"nil state", nil, true},
		{"valid", &AgentState{Phase: PhasePlan, Goal: "goal"}, false},
		{"invalid phase", &AgentState{Phase: "RUNNING", Goal: "goal"}, true},
		{"empty phase", &AgentState{Phase: "", Goal: "goal"}, true},
		{"empty goal", &AgentState{Phase: PhasePlan, Goal: ""}, true},
		{"negative step", &AgentState{Phase: PhasePlan, Goal: "goal", Step: -1}, true},
		{"negative commands_run", &AgentState{Phase: PhasePlan, Goal: "goal", CommandsRun: -1}, true},
		{"terminal complete valid", &AgentState{Phase: PhaseComplete, Goal: "goal", Done: true}, false},
		{"terminal blocked valid", &AgentState{Phase: PhaseBlocked, Goal: "goal", BlockReason: "x"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.state.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAgentStateIsTerminal(t *testing.T) {
	s := NewState("g")
	if s.IsTerminal() {
		t.Fatalf("fresh PLAN state should not be terminal")
	}
	s.Phase = PhaseComplete
	if !s.IsTerminal() {
		t.Fatalf("COMPLETE state should be terminal")
	}
	s.Phase = PhaseBlocked
	if !s.IsTerminal() {
		t.Fatalf("BLOCKED state should be terminal")
	}
}
