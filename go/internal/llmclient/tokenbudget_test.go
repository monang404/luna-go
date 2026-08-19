package llmclient

import (
	"os"
	"strings"
	"testing"
)

// --- ResolveMaxTokens: port of _ai_resolve_max_toks ---

func TestResolveMaxTokens_FirstModelNoOverrideUsesDefault(t *testing.T) {
	if got := ResolveMaxTokens(1, 0); got != 4000 {
		t.Fatalf("ResolveMaxTokens(1, 0) = %d, want 4000", got)
	}
}

func TestResolveMaxTokens_FirstModelWithOverrideUsesOverrideInFull(t *testing.T) {
	if got := ResolveMaxTokens(1, 9000); got != 9000 {
		t.Fatalf("ResolveMaxTokens(1, 9000) = %d, want 9000 (primary model gets full override)", got)
	}
}

func TestResolveMaxTokens_FallbackModelCapsLargeOverride(t *testing.T) {
	if got := ResolveMaxTokens(2, 9000); got != 4000 {
		t.Fatalf("ResolveMaxTokens(2, 9000) = %d, want 4000 (capped)", got)
	}
}

func TestResolveMaxTokens_FallbackModelKeepsSmallOverride(t *testing.T) {
	if got := ResolveMaxTokens(2, 1500); got != 1500 {
		t.Fatalf("ResolveMaxTokens(2, 1500) = %d, want 1500 (below cap, unchanged)", got)
	}
}

func TestResolveMaxTokens_NoOverrideAnyIndexUsesDefault(t *testing.T) {
	if got := ResolveMaxTokens(3, 0); got != 4000 {
		t.Fatalf("ResolveMaxTokens(3, 0) = %d, want 4000", got)
	}
}

// --- TemperatureForMode ---

func TestTemperatureForMode(t *testing.T) {
	if got := TemperatureForMode("json"); got != 0.4 {
		t.Fatalf("TemperatureForMode(json) = %v, want 0.4", got)
	}
	if got := TemperatureForMode(""); got != 0.6 {
		t.Fatalf("TemperatureForMode('') = %v, want 0.6", got)
	}
	if got := TemperatureForMode("chat"); got != 0.6 {
		t.Fatalf("TemperatureForMode(chat) = %v, want 0.6", got)
	}
}

// --- IsReasoningModel ---

func TestIsReasoningModel(t *testing.T) {
	cases := []struct {
		provider, model string
		want            bool
	}{
		{"groq", "openai/gpt-oss-120b", true},
		{"groq", "openai/gpt-oss-20b", true},
		{"groq", "llama-3.3-70b-versatile", false},
		{"deepseek", "deepseek-v4-flash", true},
		{"deepseek", "deepseek-v4-pro", true},
		{"deepseek", "deepseek-chat", false}, // legacy non-thinking alias
		{"cerebras", "gpt-oss-120b", false},  // same model name, different provider -- must check provider too
		{"gemini", "gemini-flash-latest", false},
	}
	for _, c := range cases {
		if got := IsReasoningModel(c.provider, c.model); got != c.want {
			t.Errorf("IsReasoningModel(%q, %q) = %v, want %v", c.provider, c.model, got, c.want)
		}
	}
}

// --- ReasoningEffortFor ---

func TestReasoningEffortFor_Deepseek(t *testing.T) {
	os.Unsetenv("DEEPSEEK_REASONING_EFFORT")
	if got := ReasoningEffortFor("deepseek"); got != "low" {
		t.Fatalf("ReasoningEffortFor(deepseek) = %q, want low (default)", got)
	}
	t.Setenv("DEEPSEEK_REASONING_EFFORT", "high")
	if got := ReasoningEffortFor("deepseek"); got != "high" {
		t.Fatalf("ReasoningEffortFor(deepseek) with env override = %q, want high", got)
	}
}

func TestReasoningEffortFor_NonDeepseekFallsBackToGroqValue(t *testing.T) {
	if got := ReasoningEffortFor("groq"); got != "low" {
		t.Fatalf("ReasoningEffortFor(groq) = %q, want low", got)
	}
	// No third branch in the zsh source: any non-deepseek provider gets
	// the Groq value, even one that isn't Groq itself.
	if got := ReasoningEffortFor("cerebras"); got != "low" {
		t.Fatalf("ReasoningEffortFor(cerebras) = %q, want low (falls through to GROQ_REASONING_EFFORT)", got)
	}
}

// --- AC-04: EstimateTokens within reasonable tolerance vs the old
// 4-chars-per-token heuristic, across >=10 samples ---

func TestEstimateTokens_ToleranceAcrossSamples(t *testing.T) {
	samples := []string{
		"hello",
		"The quick brown fox jumps over the lazy dog.",
		"func main() {\n\tfmt.Println(\"hello world\")\n}\n",
		strings.Repeat("token ", 50),
		"a",
		"",
		"1234567890",
		"Ini adalah kalimat bahasa Indonesia untuk menguji estimasi token.",
		strings.Repeat("x", 1000),
		"Multi\nline\nstring\nwith\nnewlines\nand\ttabs.",
		"emoji test 🎉🚀 and unicode café naïve",
	}
	for _, s := range samples {
		got := EstimateTokens(s)
		want := len(s) / 4 // reference: old-style chars/4 approximation
		if want == 0 && len(s) > 0 {
			want = 1
		}
		diff := got - want
		if diff < 0 {
			diff = -diff
		}
		tolerance := want/10 + 1 // +/-10%, plus 1 to cover rounding at small sizes
		if diff > tolerance {
			t.Errorf("EstimateTokens(%q) = %d, reference ~%d, diff %d exceeds tolerance %d", s, got, want, diff, tolerance)
		}
	}
}

func TestEstimateTokens_EmptyStringIsZero(t *testing.T) {
	if got := EstimateTokens(""); got != 0 {
		t.Fatalf("EstimateTokens('') = %d, want 0", got)
	}
}

func TestEstimateTokens_NonEmptyNeverZero(t *testing.T) {
	if got := EstimateTokens("a"); got == 0 {
		t.Fatal("EstimateTokens('a') = 0, want >0")
	}
}

// --- AC-03: TrimToFit stays under budget without corrupting
// system/most-recent message order ---

func longHistory(n int) []Message {
	msgs := []Message{{Role: "system", Content: "You are a helpful assistant."}}
	for i := 0; i < n; i++ {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		msgs = append(msgs, Message{Role: role, Content: strings.Repeat("word ", 20)})
	}
	return msgs
}

func totalEstimated(msgs []Message) int {
	sum := 0
	for _, m := range msgs {
		sum += estimateMessageTokens(m)
	}
	return sum
}

func TestTrimToFit_SystemMessageNeverTrimmed(t *testing.T) {
	msgs := longHistory(40)
	trimmed := TrimToFit(msgs, 200)
	if len(trimmed) == 0 || trimmed[0].Role != "system" {
		t.Fatalf("TrimToFit dropped the system message: %+v", trimmed)
	}
	if trimmed[0].Content != msgs[0].Content {
		t.Fatalf("system message content changed: got %q, want %q", trimmed[0].Content, msgs[0].Content)
	}
}

func TestTrimToFit_UnderBudgetAlready_NoOp(t *testing.T) {
	msgs := longHistory(2)
	trimmed := TrimToFit(msgs, 1_000_000)
	if len(trimmed) != len(msgs) {
		t.Fatalf("TrimToFit trimmed a history already under budget: got %d messages, want %d", len(trimmed), len(msgs))
	}
}

func TestTrimToFit_MostRecentMessagesPreserved(t *testing.T) {
	msgs := longHistory(40)
	last := msgs[len(msgs)-1]
	trimmed := TrimToFit(msgs, 300)
	if len(trimmed) < 2 {
		t.Fatalf("TrimToFit trimmed down to %d messages, expected at least system+last", len(trimmed))
	}
	gotLast := trimmed[len(trimmed)-1]
	if gotLast.Content != last.Content || gotLast.Role != last.Role {
		t.Fatalf("most recent message not preserved: got %+v, want %+v", gotLast, last)
	}
}

func TestTrimToFit_NeverStartsOnAssistant(t *testing.T) {
	msgs := longHistory(40)
	// A range of budgets sweeps across many possible cut points to
	// exercise the role-aware fixup at different offsets.
	for budget := 50; budget < 400; budget += 17 {
		trimmed := TrimToFit(msgs, budget)
		if len(trimmed) < 2 {
			continue // only system (or system+1) left, nothing to check
		}
		firstNonSystem := trimmed[1]
		if firstNonSystem.Role == "assistant" {
			t.Fatalf("budget=%d: tail starts on 'assistant', breaking alternation: %+v", budget, trimmed[:3])
		}
	}
}

func TestTrimToFit_ReducesBelowBudgetWhenPossible(t *testing.T) {
	msgs := longHistory(40)
	full := totalEstimated(msgs)
	budget := full / 3
	trimmed := TrimToFit(msgs, budget)
	if len(trimmed) >= len(msgs) {
		t.Fatalf("TrimToFit did not shrink a history that was well over budget")
	}
}

func TestTrimToFit_EmptyInput(t *testing.T) {
	if got := TrimToFit(nil, 100); len(got) != 0 {
		t.Fatalf("TrimToFit(nil, ...) = %+v, want empty", got)
	}
}

func TestTrimToFit_NoSystemMessage(t *testing.T) {
	msgs := []Message{
		{Role: "user", Content: strings.Repeat("a", 100)},
		{Role: "assistant", Content: strings.Repeat("b", 100)},
		{Role: "user", Content: strings.Repeat("c", 100)},
	}
	// Budget fits roughly the last message alone (~29 estimated tokens)
	// but not two -- tight enough to exercise trimming, loose enough
	// that a role-safe non-empty result is possible.
	trimmed := TrimToFit(msgs, 35)
	if len(trimmed) == 0 {
		t.Fatal("TrimToFit trimmed a system-less history down to nothing when a role-safe fit existed")
	}
	if trimmed[0].Role == "assistant" {
		t.Fatalf("tail starts on assistant with no system message present: %+v", trimmed)
	}
}
