package config

// Ported from 30-luna/00-config/00-models.zsh.

const (
	// GroqModel is the single Groq model used before/alongside the v4
	// multi-model fallback list below; it is NOT env-overridable in the
	// zsh source (plain "GROQ_MODEL=...", not "${GROQ_MODEL:-...}"), so
	// it's a Go const rather than reading os.Getenv, to match exactly.
	GroqModel = "openai/gpt-oss-120b"

	// GroqReasoningEffort is kept low deliberately: with reasoning
	// models (openai/gpt-oss-*), "medium"/"high" effort can consume the
	// entire max_tokens budget on chain-of-thought before any answer
	// text is written, and also risks hitting Groq's free-tier TPM
	// limit (HTTP 413) before max_tokens is even reached.
	GroqReasoningEffort = "low"
)

// TaskClass is a task's workload class: FAST (short, no deep reasoning
// needed -- chat, shell helper, commit message) or SMART (needs
// quality/reasoning -- code gen, plan, review, summarize, agent).
type TaskClass string

const (
	TaskFast  TaskClass = "fast"
	TaskSmart TaskClass = "smart"
)

// Models maps "<provider>_<class>" (matching AI_MODELS's flat key naming in
// zsh) to the ordered fallback list of models to try for that
// provider+class: leftmost first, advancing to the next model on failure
// before falling back to the next provider. Models known (as of the zsh
// source's 2026-08 audit) to consistently 429/404 or produce
// parser-incompatible output for this codebase are deliberately excluded --
// see 00-models.zsh's comment for the excluded list.
var Models = map[string][]string{
	"groq_fast":      {"llama-3.1-8b-instant", "llama-3.3-70b-versatile"},
	"groq_smart":     {"openai/gpt-oss-120b", "openai/gpt-oss-20b", "llama-3.3-70b-versatile", "llama-3.1-8b-instant"},
	"gemini_fast":    {"gemini-flash-latest", "gemini-3.5-flash-lite", "gemini-flash-lite-latest", "gemini-3-flash-preview"},
	"gemini_smart":   {"gemini-3.5-flash", "gemini-flash-latest", "gemini-3-flash-preview", "gemini-3.1-flash-lite-preview"},
	"cerebras_fast":  {"gpt-oss-120b", "gemma-4-31b", "zai-glm-4.7"},
	"cerebras_smart": {"gpt-oss-120b", "zai-glm-4.7", "gemma-4-31b"},
	"deepseek_fast":  {"deepseek-v4-flash"},
	"deepseek_smart": {"deepseek-v4-flash", "deepseek-v4-pro"},
}

// ModelsFor returns the ordered fallback model list for a given provider
// and task class -- equivalent to AI_MODELS[<provider>_<class>] in zsh. The
// returned slice is nil (not an error) if no entry exists for that
// provider+class combination.
func ModelsFor(provider string, class TaskClass) []string {
	return Models[provider+"_"+string(class)]
}
