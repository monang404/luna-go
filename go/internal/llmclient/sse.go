// Ported from 30-luna/10-core/56-sse_line_parser.zsh (_ai_sse_process_line).
package llmclient

import (
	"encoding/json"
	"strings"
)

// sseLine is the result of parsing one line of an SSE response body -- the
// pure, line-at-a-time half of _ai_sse_process_line, split out from
// streaming.go's I/O loop (streamBody) so it can be unit tested directly
// against plain strings, with no bufio.Scanner/HTTP involved at all.
type sseLine struct {
	// IsData is true for any line starting with "data:" -- including
	// glitch cases with no usable content, and the "[DONE]" sentinel
	// itself. streamBody uses this (not Content != "") to decide whether
	// the response was genuine SSE at all, mirroring the zsh source's
	// own $statefile "sse=1" bookkeeping, which is written for *every*
	// data: line, not just ones that carry content.
	IsData bool

	// Done is true for the "data: [DONE]" sentinel line specifically.
	Done bool

	// Content is delta.content, "" if this line carries none.
	Content string

	// Reasoning is delta.reasoning, falling back to
	// delta.reasoning_content, "" if this line carries neither --
	// mirrors the jq fallback chain
	// '.choices[0].delta.reasoning // .choices[0].delta.reasoning_content // empty'.
	Reasoning string
}

// sseDeltaWire mirrors the one shape _ai_sse_process_line's jq filter
// actually reads out of a "data: {...}" line.
type sseDeltaWire struct {
	Choices []struct {
		Delta struct {
			Content          string `json:"content"`
			Reasoning        string `json:"reasoning"`
			ReasoningContent string `json:"reasoning_content"`
		} `json:"delta"`
	} `json:"choices"`
}

// parseSSELine parses one line of an SSE body, a literal port of
// _ai_sse_process_line's `case "$line" in data:*)` branch. line must NOT
// include its trailing newline -- bufio.Scanner's ScanLines (used by
// streamBody) already strips both "\n" and a preceding "\r".
//
// Non-"data:" lines (blank lines, "event:"/"id:"/comment lines some SSE
// servers send) are not part of _ai_sse_process_line's case statement at
// all -- it silently does nothing for them beyond the unconditional
// append to $rawfile, which streamBody replicates itself before calling
// this function. So this returns the zero sseLine{} for anything that
// isn't a "data:" line.
func parseSSELine(line string) sseLine {
	rest, ok := strings.CutPrefix(line, "data:")
	if !ok {
		return sseLine{}
	}
	// _ai_sse_process_line strips at most one leading space
	// (`${data_line# }`), not all leading whitespace.
	dataLine := strings.TrimPrefix(rest, " ")
	// A defensive trailing-\r strip survives here even though
	// bufio.ScanLines already removes it for a normally-terminated
	// line -- matches the zsh source's own belt-and-suspenders
	// `${data_line%$'\r'}`.
	dataLine = strings.TrimSuffix(dataLine, "\r")

	if dataLine == "[DONE]" {
		return sseLine{IsData: true, Done: true}
	}

	var wire sseDeltaWire
	if err := json.Unmarshal([]byte(dataLine), &wire); err != nil || len(wire.Choices) == 0 {
		// Malformed/empty data line: still an SSE data line (sse=1 in
		// the zsh $statefile), just with nothing usable in it --
		// mirrors `jq -r ... 2>/dev/null` swallowing the parse error
		// and yielding "" rather than aborting the stream.
		return sseLine{IsData: true}
	}
	delta := wire.Choices[0].Delta
	if delta.Content != "" {
		return sseLine{IsData: true, Content: delta.Content}
	}
	reasoning := delta.Reasoning
	if reasoning == "" {
		reasoning = delta.ReasoningContent
	}
	return sseLine{IsData: true, Reasoning: reasoning}
}
