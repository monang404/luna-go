package config

import "os"

// Provider mirrors one entry of zsh's AI_PROVIDERS assoc map: an
// OpenAI-compatible endpoint + model + the name of the env var holding its
// API key. Ported from 30-luna/00-config/35-providers.zsh.
type Provider struct {
	Endpoint string
	Model    string
	KeyVar   string
}

// envOr returns os.Getenv(key) if non-empty, else def -- equivalent to
// zsh's "${VAR:-default}" expansion used throughout 35-providers.zsh. This
// is the single place a provider's default model is written down (AC-04:
// one source of truth per provider), so every default lives in exactly one
// Providers() entry below and nowhere else in this package.
func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Providers returns the provider map, matching AI_PROVIDERS in
// 35-providers.zsh. Built fresh on each call (not a package-level var) so
// that model-override env vars (GEMINI_MODEL, CEREBRAS_MODEL,
// DEEPSEEK_MODEL) set after process start are still picked up -- matching
// zsh re-evaluating "${VAR:-default}" every time 00-config is sourced.
func Providers() map[string]Provider {
	return map[string]Provider{
		"groq": {
			Endpoint: "https://api.groq.com/openai/v1/chat/completions",
			Model:    GroqModel,
			KeyVar:   "GROQ_API_KEY",
		},
		"gemini": {
			Endpoint: "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions",
			Model:    envOr("GEMINI_MODEL", "gemini-flash-latest"),
			KeyVar:   "GEMINI_API_KEY",
		},
		"cerebras": {
			Endpoint: "https://api.cerebras.ai/v1/chat/completions",
			Model:    envOr("CEREBRAS_MODEL", "gpt-oss-120b"),
			KeyVar:   "CEREBRAS_API_KEY",
		},
		"deepseek": {
			Endpoint: "https://api.deepseek.com/chat/completions",
			Model:    envOr("DEEPSEEK_MODEL", "deepseek-v4-flash"),
			KeyVar:   "DEEPSEEK_API_KEY",
		},
		"anthropic": {
			Endpoint: "https://api.anthropic.com/v1/messages",
			Model:    envOr("ANTHROPIC_MODEL", "claude-3-7-sonnet-20250219"),
			KeyVar:   "ANTHROPIC_API_KEY",
		},
		"openrouter": {
			Endpoint: "https://openrouter.ai/api/v1/chat/completions",
			Model:    envOr("OPENROUTER_MODEL", "anthropic/claude-3.7-sonnet"),
			KeyVar:   "OPENROUTER_API_KEY",
		},
	}
}

// ProviderOrder is the legacy fallback order (AI_PROVIDER_ORDER in
// 35-providers.zsh), used by callers not yet routed to a specific
// fast/smart/big/agent task class.
//
// NOTE: this intentionally excludes "deepseek" -- that's a direct port of
// 35-providers.zsh:33 (`AI_PROVIDER_ORDER=(groq gemini cerebras)`), which
// never added deepseek to this particular list even though it exists in
// AI_PROVIDERS and in every AI_TASK_PROVIDER_ORDER_* below. Preserved here
// for parity with the zsh source, not because it's necessarily intentional
// upstream -- flagged in the SESSION-41 changelog entry for visibility.
var ProviderOrder = []string{"groq", "gemini", "cerebras"}

// Task-class provider orders, ported from 05-provider_order.zsh.
var (
	// TaskProviderOrderFast: FAST tasks (chat, shell helper, commit msg,
	// summarize) -- Groq/Gemini fastest for short single-turn requests.
	TaskProviderOrderFast = []string{"groq", "gemini", "anthropic", "openrouter", "cerebras", "deepseek"}

	// TaskProviderOrderSmart: SMART tasks (aiplan, aireview, aiask,
	// aifix, luna session) -- DeepSeek primary for reasoning quality,
	// Cerebras next for its more generous limits.
	TaskProviderOrderSmart = []string{"anthropic", "openrouter", "deepseek", "cerebras", "gemini", "groq"}

	// TaskProviderOrderBig: BIG tasks (aiproject, aibuild, aiscrap) --
	// long completions; same order as SMART in the zsh source.
	TaskProviderOrderBig = []string{"anthropic", "openrouter", "deepseek", "cerebras", "gemini", "groq"}

	// TaskProviderOrderAgent: AGENT tasks (aiagent ReAct loop) --
	// DeepSeek/Cerebras for fast, accurate JSON-mode tool calls.
	TaskProviderOrderAgent = []string{"anthropic", "openrouter", "deepseek", "cerebras", "groq", "gemini"}
)

// TaskProviderOrder is the default alias (AI_TASK_PROVIDER_ORDER in zsh):
// callers that haven't picked a specific task class get the SMART order.
var TaskProviderOrder = TaskProviderOrderSmart

// ActiveProviders filters order down to providers whose API key env var is
// currently set (non-empty), preserving order. This is the auto-skip
// behavior described throughout 35-providers.zsh: no provider is ever
// hard-required, a provider with an unset/empty key var is silently
// skipped rather than erroring. See AC-01.
func ActiveProviders(order []string) []string {
	providers := Providers()
	active := make([]string, 0, len(order))
	for _, name := range order {
		p, ok := providers[name]
		if !ok {
			continue
		}
		if os.Getenv(p.KeyVar) != "" {
			active = append(active, name)
		}
	}
	return active
}

// HasAnyKey reports whether at least one provider in order currently has
// its API key set -- equivalent to _ai_need_any_key() in
// 30-luna/10-core/00-security.zsh.
func HasAnyKey(order []string) bool {
	return len(ActiveProviders(order)) > 0
}
