package agent

import "testing"

func TestFinalize_CompleteHasNoBlockReason(t *testing.T) {
	s := NewState("goal")
	s.Phase = PhaseComplete
	s.Done = true
	s.Step = 4
	r := Finalize(s)
	if r.Phase != PhaseComplete {
		t.Errorf("Phase = %q, want COMPLETE", r.Phase)
	}
	if r.BlockReason != "" {
		t.Errorf("BlockReason = %q, want empty on COMPLETE", r.BlockReason)
	}
	if !r.Done {
		t.Errorf("Done = false, want true")
	}
}

func TestFinalize_BlockedDefaultsReasonWhenEmpty(t *testing.T) {
	s := NewState("goal")
	s.Phase = PhaseBlocked
	s.Step = 2
	r := Finalize(s)
	if r.BlockReason == "" {
		t.Errorf("BlockReason empty, want a default fallback reason")
	}
}

func TestFinalize_BlockedKeepsExplicitReason(t *testing.T) {
	s := NewState("goal")
	s.Phase = PhaseBlocked
	s.BlockReason = "Tool 'x' gagal 3 kali berturut-turut (step 3)"
	r := Finalize(s)
	if r.BlockReason != s.BlockReason {
		t.Errorf("BlockReason = %q, want unchanged %q", r.BlockReason, s.BlockReason)
	}
}

func TestFinalize_NilStateIsZeroValue(t *testing.T) {
	r := Finalize(nil)
	if r.Phase != "" || r.Goal != "" || r.Steps != 0 {
		t.Errorf("Finalize(nil) = %+v, want zero value", r)
	}
}

func TestFinalize_CopiesFileSlices(t *testing.T) {
	s := NewState("goal")
	s.Phase = PhaseComplete
	s.TouchedFiles = []string{"a.py", "b.py"}
	s.ChangedFiles = []string{"a.py"}
	r := Finalize(s)
	r.TouchedFiles[0] = "mutated"
	if s.TouchedFiles[0] != "a.py" {
		t.Errorf("Finalize result shares backing array with AgentState -- mutation leaked back")
	}
}
