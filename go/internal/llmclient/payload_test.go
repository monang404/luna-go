package llmclient

import (
	"encoding/json"
	"reflect"
	"testing"
)

// --- AC-01: BuildPayload output is structurally identical to
// payload_builder.zsh's 4 jq template branches for the same input ---

func decodePayload(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("BuildPayload output is not valid JSON: %v\nraw: %s", err, raw)
	}
	return m
}

func TestBuildPayload_PlainNoReasoningNoStream(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hi"}}
	raw, err := BuildPayload(msgs, PayloadOptions{Model: "m1", MaxTokens: 4000, Temperature: 0.6})
	if err != nil {
		t.Fatalf("BuildPayload error: %v", err)
	}
	got := decodePayload(t, raw)

	want := map[string]any{
		"model":       "m1",
		"messages":    []any{map[string]any{"role": "user", "content": "hi"}},
		"max_tokens":  float64(4000),
		"temperature": 0.6,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("plain payload = %#v, want %#v", got, want)
	}
	if _, ok := got["reasoning_effort"]; ok {
		t.Errorf("is_reasoning=0 branch must not emit reasoning_effort, got %v", got["reasoning_effort"])
	}
	if _, ok := got["stream"]; ok {
		t.Errorf("stream=0 branch must not emit \"stream\" key at all, got %v", got["stream"])
	}
}

func TestBuildPayload_ReasoningNoStream(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hi"}}
	raw, err := BuildPayload(msgs, PayloadOptions{
		Model: "openai/gpt-oss-120b", MaxTokens: 4000, Temperature: 0.6,
		ReasoningEffort: "low",
	})
	if err != nil {
		t.Fatalf("BuildPayload error: %v", err)
	}
	got := decodePayload(t, raw)
	if got["reasoning_effort"] != "low" {
		t.Errorf("reasoning_effort = %v, want \"low\"", got["reasoning_effort"])
	}
	if _, ok := got["stream"]; ok {
		t.Errorf("stream key must be absent, got %v", got["stream"])
	}
}

func TestBuildPayload_PlainWithStream(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hi"}}
	raw, err := BuildPayload(msgs, PayloadOptions{Model: "m1", MaxTokens: 4000, Temperature: 0.6, Stream: true})
	if err != nil {
		t.Fatalf("BuildPayload error: %v", err)
	}
	got := decodePayload(t, raw)
	if got["stream"] != true {
		t.Errorf("stream = %v, want true", got["stream"])
	}
	if _, ok := got["reasoning_effort"]; ok {
		t.Errorf("reasoning_effort must be absent, got %v", got["reasoning_effort"])
	}
}

func TestBuildPayload_ReasoningWithStream(t *testing.T) {
	msgs := []Message{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "yo"}}
	raw, err := BuildPayload(msgs, PayloadOptions{
		Model: "deepseek-v4-flash", MaxTokens: 8000, Temperature: 0.4,
		ReasoningEffort: "high", Stream: true,
	})
	if err != nil {
		t.Fatalf("BuildPayload error: %v", err)
	}
	got := decodePayload(t, raw)
	want := map[string]any{
		"model": "deepseek-v4-flash",
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{"role": "assistant", "content": "yo"},
		},
		"max_tokens":       float64(8000),
		"temperature":      0.4,
		"reasoning_effort": "high",
		"stream":           true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reasoning+stream payload = %#v, want %#v", got, want)
	}
}

func TestBuildPayload_EmptyMessagesStillArray(t *testing.T) {
	raw, err := BuildPayload(nil, PayloadOptions{Model: "m1", MaxTokens: 100, Temperature: 0.6})
	if err != nil {
		t.Fatalf("BuildPayload error: %v", err)
	}
	got := decodePayload(t, raw)
	msgs, ok := got["messages"].([]any)
	if !ok {
		t.Fatalf("messages field is not an array (nil messages must marshal as [], not null): %#v", got["messages"])
	}
	if len(msgs) != 0 {
		t.Errorf("messages = %v, want empty array", msgs)
	}
}
