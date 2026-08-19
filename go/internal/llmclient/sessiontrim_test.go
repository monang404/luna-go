package llmclient

import "testing"

func chatShapeHistory(pairs int) []Message {
	// _ai_session_ask shape: [system] + repeated [user, assistant].
	msgs := []Message{{Role: "system", Content: "sys"}}
	for i := 0; i < pairs; i++ {
		msgs = append(msgs, Message{Role: "user", Content: "u"}, Message{Role: "assistant", Content: "a"})
	}
	return msgs
}

func agentLoopHistory(steps int) []Message {
	// agent-loop shape: [system, user] + repeated [assistant, user].
	msgs := []Message{{Role: "system", Content: "sys"}, {Role: "user", Content: "u0"}}
	for i := 0; i < steps; i++ {
		msgs = append(msgs, Message{Role: "assistant", Content: "a"}, Message{Role: "user", Content: "u"})
	}
	return msgs
}

func TestTrimSession_NoOpUnderLimit(t *testing.T) {
	msgs := chatShapeHistory(5) // 1 + 10 = 11 messages
	got := TrimSession(msgs, 30)
	if len(got) != len(msgs) {
		t.Fatalf("len = %d, want %d (no trim expected)", len(got), len(msgs))
	}
}

func TestTrimSession_NoOpAtExactLimit(t *testing.T) {
	msgs := chatShapeHistory(5)
	got := TrimSession(msgs, len(msgs)) // len == max, zsh only trims when len > max
	if len(got) != len(msgs) {
		t.Fatalf("len = %d, want %d (== max should not trim)", len(got), len(msgs))
	}
}

func TestTrimSession_SystemMessageAlwaysKept(t *testing.T) {
	msgs := chatShapeHistory(50)
	got := TrimSession(msgs, 30)
	if len(got) == 0 || got[0].Role != "system" || got[0].Content != "sys" {
		t.Fatalf("system message not preserved: %+v", got[:min3(3, len(got))])
	}
}

// --- RC-010/BUG-005: role-aware fixup, both consumer shapes ---

func TestTrimSession_ChatShape_EvenTailFixedToStartOnUser(t *testing.T) {
	// _ai_session_ask shape: [1:] is always even-length. A max-1 (odd)
	// slice from the end lands on an "assistant" element first without
	// the fixup.
	msgs := chatShapeHistory(50) // 1 + 100 = 101 messages
	got := TrimSession(msgs, 30)

	if len(got) == 0 {
		t.Fatal("TrimSession returned empty result")
	}
	if got[0].Role != "system" {
		t.Fatalf("got[0].Role = %q, want system", got[0].Role)
	}
	if len(got) > 1 && got[1].Role != "user" {
		t.Fatalf("got[1].Role = %q, want user (role-aware fixup should have dropped a leading assistant)", got[1].Role)
	}
	// Role alternation must hold throughout the trimmed tail.
	assertAlternatesFromUser(t, got[1:])
}

func TestTrimSession_AgentLoopShape_OddTailNeverTriggersFixup(t *testing.T) {
	// agent-loop shape: [1:] is always odd-length, so the trimmed
	// tail's first element is already "user" and the fixup branch never
	// fires -- behavior must be identical to the un-fixed slice.
	msgs := agentLoopHistory(50) // 2 + 100 = 102 messages
	got := TrimSession(msgs, 30)

	if len(got) == 0 {
		t.Fatal("TrimSession returned empty result")
	}
	if got[0].Role != "system" {
		t.Fatalf("got[0].Role = %q, want system", got[0].Role)
	}
	if len(got) > 1 && got[1].Role != "user" {
		t.Fatalf("got[1].Role = %q, want user (agent-loop shape should never need the fixup)", got[1].Role)
	}
}

func TestTrimSession_ResultLengthMatchesJqFormula(t *testing.T) {
	// [.[0]] + (.[1:] | .[-(max-1):]) before the role-aware fixup would
	// be exactly max messages (1 + (max-1)); the fixup can make it
	// max-1 when it fires. Assert it's one of those two exact values,
	// not some other length.
	msgs := chatShapeHistory(50)
	maxMsgs := 30
	got := TrimSession(msgs, maxMsgs)
	if len(got) != maxMsgs && len(got) != maxMsgs-1 {
		t.Fatalf("len(got) = %d, want %d or %d", len(got), maxMsgs, maxMsgs-1)
	}
}

func TestTrimSession_SmallMaxMsgs(t *testing.T) {
	msgs := chatShapeHistory(10)
	got := TrimSession(msgs, 2)
	if len(got) == 0 || got[0].Role != "system" {
		t.Fatalf("system dropped or empty result: %+v", got)
	}
	assertAlternatesFromUser(t, got[1:])
}

func TestTrimSession_ZeroOrNegativeMaxIsNoOp(t *testing.T) {
	msgs := chatShapeHistory(5)
	if got := TrimSession(msgs, 0); len(got) != len(msgs) {
		t.Fatalf("maxMsgs=0: len = %d, want %d (no-op)", len(got), len(msgs))
	}
	if got := TrimSession(msgs, -1); len(got) != len(msgs) {
		t.Fatalf("maxMsgs=-1: len = %d, want %d (no-op)", len(got), len(msgs))
	}
}

func TestTrimSession_EmptyInput(t *testing.T) {
	if got := TrimSession(nil, 30); len(got) != 0 {
		t.Fatalf("TrimSession(nil, 30) = %+v, want empty", got)
	}
}

func assertAlternatesFromUser(t *testing.T, tail []Message) {
	t.Helper()
	want := "user"
	for i, m := range tail {
		if m.Role != want {
			t.Fatalf("tail[%d].Role = %q, want %q (alternation broken)", i, m.Role, want)
		}
		if want == "user" {
			want = "assistant"
		} else {
			want = "user"
		}
	}
}

func min3(a, b int) int {
	if a < b {
		return a
	}
	return b
}
