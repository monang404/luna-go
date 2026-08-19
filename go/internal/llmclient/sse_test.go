package llmclient

import "testing"

func TestParseSSELine_NonDataLineIsIgnored(t *testing.T) {
	for _, line := range []string{"", "event: message", "id: 42", ": comment", "retry: 3000"} {
		got := parseSSELine(line)
		if got.IsData {
			t.Errorf("parseSSELine(%q).IsData = true, want false", line)
		}
	}
}

func TestParseSSELine_ContentDelta(t *testing.T) {
	got := parseSSELine(`data: {"choices":[{"delta":{"content":"Hel"}}]}`)
	if !got.IsData || got.Done {
		t.Fatalf("got %+v, want IsData=true Done=false", got)
	}
	if got.Content != "Hel" {
		t.Errorf("Content = %q, want %q", got.Content, "Hel")
	}
	if got.Reasoning != "" {
		t.Errorf("Reasoning = %q, want empty", got.Reasoning)
	}
}

func TestParseSSELine_NoLeadingSpaceAfterColon(t *testing.T) {
	// "data:" with no space before the JSON is still valid SSE.
	got := parseSSELine(`data:{"choices":[{"delta":{"content":"x"}}]}`)
	if got.Content != "x" {
		t.Errorf("Content = %q, want %q", got.Content, "x")
	}
}

func TestParseSSELine_StripsExactlyOneLeadingSpace(t *testing.T) {
	// _ai_sse_process_line's `${data_line# }` strips one leading space,
	// not all of it -- a second space is significant/kept, though it'll
	// just make the JSON fail to parse (leading whitespace before a `{`
	// is legal JSON, so this actually still parses fine; the point is
	// only ONE space is stripped by construction).
	got := parseSSELine(`data:  {"choices":[{"delta":{"content":"y"}}]}`)
	if got.Content != "y" {
		t.Errorf("Content = %q, want %q (leading whitespace before '{' is valid JSON)", got.Content, "y")
	}
}

func TestParseSSELine_CarriageReturnStripped(t *testing.T) {
	got := parseSSELine("data: {\"choices\":[{\"delta\":{\"content\":\"z\"}}]}\r")
	if got.Content != "z" {
		t.Errorf("Content = %q, want %q", got.Content, "z")
	}
}

func TestParseSSELine_DoneSentinel(t *testing.T) {
	got := parseSSELine("data: [DONE]")
	if !got.IsData || !got.Done {
		t.Fatalf("got %+v, want IsData=true Done=true", got)
	}
	if got.Content != "" || got.Reasoning != "" {
		t.Errorf("[DONE] line should carry no Content/Reasoning, got %+v", got)
	}
}

func TestParseSSELine_ReasoningFieldPreferredOverReasoningContent(t *testing.T) {
	got := parseSSELine(`data: {"choices":[{"delta":{"reasoning":"thinking...","reasoning_content":"ignored"}}]}`)
	if got.Reasoning != "thinking..." {
		t.Errorf("Reasoning = %q, want %q", got.Reasoning, "thinking...")
	}
	if got.Content != "" {
		t.Errorf("Content = %q, want empty when only reasoning present", got.Content)
	}
}

func TestParseSSELine_ReasoningContentFallback(t *testing.T) {
	got := parseSSELine(`data: {"choices":[{"delta":{"reasoning_content":"step 1"}}]}`)
	if got.Reasoning != "step 1" {
		t.Errorf("Reasoning = %q, want %q", got.Reasoning, "step 1")
	}
}

func TestParseSSELine_ContentTakesPriorityOverReasoning(t *testing.T) {
	// Mirrors the zsh source's own `if [ -n "$content" ]; then ...
	// else ... reasoning ...` -- a delta line that somehow carries both
	// only ever surfaces as Content.
	got := parseSSELine(`data: {"choices":[{"delta":{"content":"final","reasoning":"scratch"}}]}`)
	if got.Content != "final" || got.Reasoning != "" {
		t.Errorf("got Content=%q Reasoning=%q, want Content=%q Reasoning=empty", got.Content, got.Reasoning, "final")
	}
}

func TestParseSSELine_EmptyDeltaIsStillIsData(t *testing.T) {
	// A keep-alive-style delta with an empty/absent content and no
	// reasoning either: still counts as a real SSE data line (sse=1 in
	// the zsh $statefile) even though it carries nothing usable.
	got := parseSSELine(`data: {"choices":[{"delta":{}}]}`)
	if !got.IsData {
		t.Error("IsData = false, want true")
	}
	if got.Content != "" || got.Reasoning != "" || got.Done {
		t.Errorf("got %+v, want all empty/false", got)
	}
}

func TestParseSSELine_MalformedJSONStillIsData(t *testing.T) {
	// Mirrors `jq -r ... 2>/dev/null` swallowing a parse error rather
	// than aborting the whole stream over one bad line.
	got := parseSSELine(`data: {not valid json`)
	if !got.IsData {
		t.Error("IsData = false, want true (a malformed data: line is still an SSE data line)")
	}
	if got.Content != "" || got.Reasoning != "" || got.Done {
		t.Errorf("got %+v, want all empty/false for malformed JSON", got)
	}
}

func TestParseSSELine_EmptyChoicesArray(t *testing.T) {
	got := parseSSELine(`data: {"choices":[]}`)
	if !got.IsData {
		t.Error("IsData = false, want true")
	}
	if got.Content != "" || got.Reasoning != "" {
		t.Errorf("got %+v, want empty Content/Reasoning for empty choices", got)
	}
}
