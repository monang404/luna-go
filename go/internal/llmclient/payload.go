// Ported from 30-luna/10-core/43-payload_builder.zsh (_ai_build_chat_payload).
package llmclient

import "encoding/json"

// Message is one OpenAI-compatible chat message. This is the Go shape of
// what _ai_build_chat_payload reads from $msgfile via `jq --slurpfile`
// (an on-disk JSON array of {"role":..., "content":...} objects) --
// callers here pass the already-decoded slice directly instead of a file
// path, since Go has no reason to round-trip through disk for this.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// PayloadOptions bundles _ai_build_chat_payload's positional parameters
// (model, max_toks, temp, is_reasoning, stream, provider) minus the
// provider name itself. In the zsh source, `provider` exists only to
// resolve reasoning_effort's value via _ai_reasoning_effort_for
// (30-luna/10-core/42-token_budget.zsh) -- that resolution (which value,
// for which provider/model) is SESSION-46's job (port_llm_resilience_layer,
// token budget). So BuildPayload takes the already-resolved effort string
// directly: this keeps internal/llmclient's blocking-call path (this
// session) free of any dependency on SESSION-46 code that doesn't exist
// yet. See CHANGELOG SESSION-44 entry for the full rationale.
type PayloadOptions struct {
	Model       string
	MaxTokens   int
	Temperature float64

	// ReasoningEffort mirrors is_reasoning=1's branch: "" means
	// is_reasoning=0 (no reasoning_effort field emitted at all); any
	// non-empty value is emitted verbatim as "reasoning_effort".
	ReasoningEffort string

	// Stream mirrors stream=1/0: true adds "stream":true; false omits
	// the "stream" key entirely (the zsh template never emits
	// "stream":false -- the key is just absent on the blocking path).
	// This session's blocking path always passes false; SESSION-45
	// (SSE streaming) reuses BuildPayload with true, per this session's
	// handoff notes.
	Stream bool
}

// BuildPayload builds the OpenAI-compatible chat completion request body,
// a literal port of _ai_build_chat_payload's four jq template branches
// (the reasoning x stream boolean combinations) using encoding/json
// instead of shelling out to jq.
//
// messages is always marshaled as a JSON array (never `null`, even if
// empty/nil) to match jq's `--slurpfile msgs` always producing an array
// value for the "messages" key.
func BuildPayload(messages []Message, opts PayloadOptions) ([]byte, error) {
	if messages == nil {
		messages = []Message{}
	}

	body := map[string]any{
		"model":       opts.Model,
		"messages":    messages,
		"max_tokens":  opts.MaxTokens,
		"temperature": opts.Temperature,
	}
	if opts.ReasoningEffort != "" {
		body["reasoning_effort"] = opts.ReasoningEffort
	}
	if opts.Stream {
		body["stream"] = true
	}

	return json.Marshal(body)
}
