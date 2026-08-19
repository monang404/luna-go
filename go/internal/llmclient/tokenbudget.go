// Ported from 30-luna/10-core/42-token_budget.zsh (_ai_resolve_max_toks,
// _ai_chat_temp_for_mode, _ai_is_reasoning_model, _ai_reasoning_effort_for).
//
// EstimateTokens and TrimToFit below have **no zsh source** -- 42-token_budget.zsh
// is entirely about *outgoing request* shaping (max_tokens/temperature/
// reasoning_effort for the next call), not about measuring or trimming
// history by token count. The zsh codebase's only history-trimming logic
// is 60-session_trim.zsh (sessiontrim.go), which trims by raw message
// *count* (AI_SESSION_MAX_MSGS), never by an estimated token size. This
// session's own scope.include explicitly asks for
// "EstimateTokens(text) int + TrimToFit(messages, budget) []Message"
// regardless, so -- same precedent as SESSION-44's SelectProviderCandidate
// (a function synthesized to fill a real gap with no 1:1 zsh source) --
// they're implemented here as new functionality living next to the
// message-count-based TrimSession, not as a port of anything.
package llmclient

import (
	"os"
	"unicode/utf8"

	"github.com/monang404/luna-go/internal/config"
)

// defaultMaxTokens/fallbackMaxTokensCap mirror 42-token_budget.zsh's two
// literal constants (both named "4000" in the source, kept as separate
// constants here since they mean different things: the plain default,
// and the cap applied to a caller-supplied override for any model past
// the first in a fallback list).
const (
	defaultMaxTokens     = 4000
	fallbackMaxTokensCap = 4000
)

// ResolveMaxTokens ports _ai_resolve_max_toks: modelIdx is 1-based (the
// zsh source's own convention, `model_idx=$((model_idx + 1))` starting
// from 0). The first model in a provider's fallback list gets
// maxTokensOverride in full (or defaultMaxTokens if no override was
// given); every model after that gets the override capped at
// fallbackMaxTokensCap, so a large override meant for the primary "big"
// model doesn't get handed unchanged to a smaller fallback model that's
// more likely to hit a 413 with it.
func ResolveMaxTokens(modelIdx int, maxTokensOverride int) int {
	if modelIdx == 1 || maxTokensOverride == 0 {
		if maxTokensOverride != 0 {
			return maxTokensOverride
		}
		return defaultMaxTokens
	}
	if maxTokensOverride < fallbackMaxTokensCap {
		return maxTokensOverride
	}
	return fallbackMaxTokensCap
}

// TemperatureForMode ports _ai_chat_temp_for_mode: mode "json" (agent
// tool-call mode) uses a lower, more deterministic temperature; any other
// mode (plain chat/text) uses the higher default.
func TemperatureForMode(mode string) float64 {
	if mode == "json" {
		return 0.4
	}
	return 0.6
}

// IsReasoningModel ports _ai_is_reasoning_model exactly, including its
// provider-prefix matching (config provider names are the bare "groq"/
// "deepseek" strings SelectProviderCandidate.Name carries, matching the
// zsh source's own $provider local).
func IsReasoningModel(provider, model string) bool {
	if hasPrefix(provider, "groq") && contains(model, "gpt-oss") {
		return true
	}
	if hasPrefix(provider, "deepseek") && hasPrefix(model, "deepseek-v4-") {
		return true
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// DeepseekReasoningEffortDefault mirrors 42-token_budget.zsh's
// `: ${DEEPSEEK_REASONING_EFFORT:="low"}` default. Unlike
// config.GroqReasoningEffort (a plain non-overridable zsh assignment,
// already a Go const in internal/config/models.go), this one IS
// env-overridable in the zsh source, so ReasoningEffortFor below reads
// the env var itself rather than a config constant.
const DeepseekReasoningEffortDefault = "low"

// ReasoningEffortFor ports _ai_reasoning_effort_for: DeepSeek providers
// use $DEEPSEEK_REASONING_EFFORT (env override, default "low"); every
// other provider uses $GROQ_REASONING_EFFORT (config.GroqReasoningEffort
// -- not env-overridable in the zsh source, see that const's own doc
// comment). Note this mirrors the zsh source's own fallback exactly,
// including that a non-Groq, non-DeepSeek provider (e.g. Cerebras/Gemini)
// still gets the Groq value -- 42-token_budget.zsh's `[[ "$provider" ==
// deepseek* ]] && ... || echo "$GROQ_REASONING_EFFORT"` has no third
// branch, and neither does this.
func ReasoningEffortFor(provider string) string {
	if hasPrefix(provider, "deepseek") {
		if v := os.Getenv("DEEPSEEK_REASONING_EFFORT"); v != "" {
			return v
		}
		return DeepseekReasoningEffortDefault
	}
	return config.GroqReasoningEffort
}

// EstimateTokens gives a rough token-count estimate for text using the
// widely-used ~4-characters-per-token approximation for English/code
// text (no zsh source -- see this file's package-level doc comment).
// Deliberately simple (no tokenizer dependency) since it only needs to
// be within the "reasonable tolerance" this session's own AC-04 asks
// for, not exact -- an exact count would require the specific
// tokenizer of whichever provider/model ends up handling the request,
// which varies per call and isn't worth the dependency weight for a
// context-budget heuristic.
func EstimateTokens(text string) int {
	if text == "" {
		return 0
	}
	n := utf8.RuneCountInString(text)
	// Round up so a short non-empty string never estimates to 0 tokens.
	return (n + 3) / 4
}

// estimateMessageTokens estimates one message's contribution to a
// budget: its content plus a small fixed overhead per message for the
// role field and per-message JSON/protocol framing (OpenAI's own
// cookbook uses a similar few-tokens-per-message constant; exact value
// doesn't matter here, only that it's consistent between messages).
const perMessageOverheadTokens = 4

func estimateMessageTokens(m Message) int {
	return EstimateTokens(m.Content) + perMessageOverheadTokens
}

// TrimToFit drops the oldest non-system messages from messages until the
// estimated total token count is at or below budget, always preserving
// messages[0] if it is a system message (never trimmed, regardless of
// budget) and always preserving the most recent messages first (oldest
// dropped first) -- the same two invariants 60-session_trim.zsh's
// count-based TrimSession protects, just applied against an estimated
// token budget instead of a raw message count.
//
// Like TrimSession, trimming stays role-aware: if dropping the oldest
// remaining non-system message would leave the tail starting on an
// "assistant" message (breaking the user/assistant alternation a
// provider expects), one additional message is dropped so the tail
// always starts on "user". If honoring the budget exactly would require
// breaking that invariant (or would require dropping the system message
// itself), TrimToFit stops short of budget rather than corrupt message
// order -- exactly like TrimSession's own role-aware branch never
// trims below what role-safety allows.
func TrimToFit(messages []Message, budget int) []Message {
	if len(messages) == 0 {
		return messages
	}

	hasSystem := messages[0].Role == "system"
	head := 0
	if hasSystem {
		head = 1
	}

	total := func(from int) int {
		sum := 0
		if hasSystem {
			sum += estimateMessageTokens(messages[0])
		}
		for _, m := range messages[from:] {
			sum += estimateMessageTokens(m)
		}
		return sum
	}

	start := head
	for start < len(messages) && total(start) > budget {
		start++
	}

	// Role-aware fixup: don't let the tail start on "assistant" if a
	// "user" message immediately precedes it and is still droppable --
	// mirrors 60-session_trim.zsh's `if $tail[0].role == "assistant"
	// then $tail[1:]` fixup, generalized to walk forward past as many
	// leading "assistant" messages as needed (TrimSession's fixed
	// even/odd slice only ever needs one step; token-budget trimming can
	// in principle land anywhere).
	for start < len(messages) && messages[start].Role == "assistant" {
		start++
	}

	if start <= head {
		return messages
	}
	if start >= len(messages) {
		// Budget (or the role-aware fixup pushing past the last
		// still-fitting message, e.g. when the only message that fits
		// is itself an "assistant" message) leaves no non-system
		// messages at all. Unlike TrimSession's fixed even/odd slice
		// (which can only ever be off by one and always has room to
		// spare), a tight token budget can genuinely leave nothing
		// role-safe to keep -- returning system-only here is the same
		// choice 60-session_trim.zsh's own tail[1:] fixup makes when
		// tail has length 1 and that element is "assistant": drop it,
		// even down to empty, rather than start on the wrong role.
		if hasSystem {
			return messages[:1]
		}
		return nil
	}

	if !hasSystem {
		return append([]Message{}, messages[start:]...)
	}
	out := make([]Message, 0, len(messages)-start+1)
	out = append(out, messages[0])
	out = append(out, messages[start:]...)
	return out
}
