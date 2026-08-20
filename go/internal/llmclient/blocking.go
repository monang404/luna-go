// Package llmclient ports 30-luna/10-core/ (HTTP blocking + SSE streaming
// call, circuit breaker, retry, token budget, payload builder, session
// trim) into Go. The blocking (non-streaming) request path -- payload
// building, provider candidate selection, and the HTTP call itself -- was
// ported in SESSION-44. SSE streaming (SESSION-45) and the circuit
// breaker/retry/token budget/session trim resilience layer (SESSION-46)
// are not part of this package yet.
//
// This file (blocking.go) ports 30-luna/10-core/48-http_call_blocking.zsh
// (_ai_http_call_blocking). Its response parsing (stripLeakedTrace/
// ParseResponse below) additionally ports the response-reading half of
// 30-luna/scripts/ai_extract.py (AI_EXTRACT_SCRIPT). That script has no
// migration session of its own anywhere in docs/execution_sessions/ --
// it's not listed in SESSION-44's source_zsh_files, but "parse response
// benar" is that session's own AC-02, and a curl call whose body is never
// actually decoded into a usable reply isn't a meaningful port of the
// blocking call. Rather than leave that gap for a session that doesn't
// claim it either, its logic (merge reasoning_content+content, strip a
// leaked shell-trace prefix) is absorbed here, next to the call that
// produces the body it parses. See CHANGELOG SESSION-44 entry.
//
// See payload.go (43-payload_builder.zsh) and candidate.go
// (41-provider_candidate.zsh) for this session's other two files.
package llmclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/monang404/luna-go/internal/config"
)

// ErrCancelled is returned by CallBlocking when ctx is cancelled while the
// request is in flight -- equivalent to _ai_http_call_blocking's `return
// 130` path (caller's TRAPINT/TRAPTERM having set _ai_cancelled=1).
var ErrCancelled = errors.New("llmclient: request cancelled")

// Usage mirrors the OpenAI-compatible "usage" object, passed through
// as-is (30-luna/10-core/35-logging.zsh's _ai_log_usage logs `.usage // {}`
// verbatim with no field-level interpretation, so neither does this).
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Response is the result of one blocking chat completion call: the raw
// transport outcome (HTTPStatus, RawBody) plus the fields
// 50-request_blocking.zsh and 44-retry_decision.zsh actually read out of
// that body (Content, FinishReason, Usage, ErrorMessage). Kept as one
// struct (rather than separate "raw" and "parsed" types) because every
// caller of CallBlocking needs both: retry/circuit-breaker decisions
// (SESSION-46) inspect HTTPStatus/ErrorMessage, while the agent loop
// (SESSION-49/50) wants Content.
type Response struct {
	HTTPStatus int
	RawBody    []byte

	// Content is reasoning_content+content merged and leaked-trace-
	// stripped, exactly like ai_extract.py's extract(); "" if the
	// response has no usable choice (matches extract.py returning "" on
	// any parse failure or empty choices, rather than erroring).
	Content      string
	FinishReason string
	Usage        Usage

	// ErrorMessage is `.error.message // .error // .message // empty`
	// (44-retry_decision.zsh:45 and 50-request_blocking.zsh:190's jq
	// fallback chain), "" if none of those three are present/non-null.
	ErrorMessage string
}

// wireResponse mirrors the JSON body's shape for decoding. Error is kept
// as json.RawMessage because providers send it as either a string
// ("error": "bad request") or an object ("error": {"message": "..."}) --
// the `.error.message // .error` jq fallback in the zsh source handles
// both shapes, so this decodes both too (see resolveErrorMessage).
type wireResponse struct {
	Choices []wireChoice    `json:"choices"`
	Usage   Usage           `json:"usage"`
	Error   json.RawMessage `json:"error"`
	Message string          `json:"message"`
}

type wireChoice struct {
	Message      wireMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type wireMessage struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
}

// leakedTraceVars mirrors ai_extract.py's _LEAKED_TRACE_VARS whitelist
// exactly -- see that file's own comment for why this is a name
// whitelist and not a generic "key=value" regex.
var leakedTraceVars = map[string]bool{
	"temp": true, "respfile": true, "http_status": true, "curl_exit": true,
	"resp": true, "reply": true, "payload": true, "provider": true,
	"endpoint": true, "model": true, "keyvar": true, "apikey": true,
	"modelkey": true, "models_str": true, "tries": true, "max_toks": true,
	"max_toks_override": true, "finish_reason": true, "msgfile": true,
	"order_str": true, "task_class": true, "is_reasoning_model": true,
}

// varLineRE mirrors ai_extract.py's _VAR_LINE_RE: identifier=value, no
// space around "=".
var varLineRE = regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)=\S.*$`)

// stripLeakedTrace ports ai_extract.py's strip_leaked_trace: drops a
// leading contiguous run of "known_internal_var=value" lines (a shell
// trace fragment that occasionally leaks in front of a real answer -- see
// ai_extract.py's own comment for the observed root cause) and returns
// the rest unchanged.
func stripLeakedTrace(text string) string {
	lines := splitKeepEnds(text)
	i := 0
	for i < len(lines) {
		trimmed := strings.TrimSuffix(lines[i], "\n")
		m := varLineRE.FindStringSubmatch(trimmed)
		if m != nil && leakedTraceVars[m[1]] {
			i++
			continue
		}
		break
	}
	if i == 0 {
		return text
	}
	return strings.Join(lines[i:], "")
}

// splitKeepEnds mirrors Python's str.splitlines(keepends=True) closely
// enough for stripLeakedTrace's purposes (line-oriented prefix scan).
func splitKeepEnds(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

// resolveErrorMessage implements `.error.message // .error // .message //
// empty` over the decoded wireResponse.
func resolveErrorMessage(w wireResponse) string {
	if len(w.Error) > 0 && string(w.Error) != "null" {
		// Object shape: {"message": "..."}.
		var obj struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(w.Error, &obj); err == nil && obj.Message != "" {
			return obj.Message
		}
		// String shape: "error text".
		var s string
		if err := json.Unmarshal(w.Error, &s); err == nil && s != "" {
			return s
		}
	}
	return w.Message
}

// ParseResponse decodes a raw HTTP body into a Response, mirroring
// ai_extract.py's extract() (Content/FinishReason) plus the jq fallback
// chains 44-retry_decision.zsh and 50-request_blocking.zsh use for
// ErrorMessage. Like extract.py's `except Exception: return ""`, a body
// that isn't valid JSON never returns an error here -- it just yields a
// Response with empty Content/ErrorMessage, exactly like the zsh path
// where a malformed body degrades to "no usable reply" rather than a hard
// failure (the caller only ever inspects Response fields, same as the zsh
// caller only ever inspecting $reply/$resp).
func ParseResponse(httpStatus int, rawBody []byte) Response {
	resp := Response{HTTPStatus: httpStatus, RawBody: rawBody}

	var w wireResponse
	if err := json.Unmarshal(rawBody, &w); err != nil {
		return resp
	}

	resp.Usage = w.Usage
	resp.ErrorMessage = resolveErrorMessage(w)

	if len(w.Choices) == 0 {
		return resp
	}
	choice := w.Choices[0]
	resp.FinishReason = choice.FinishReason

	content := strings.TrimSpace(choice.Message.Content)
	reasoning := strings.TrimSpace(choice.Message.ReasoningContent)
	var full strings.Builder
	if reasoning != "" {
		full.WriteString(reasoning)
		full.WriteString("\n")
	}
	if content != "" {
		full.WriteString(content)
	}
	fullText := strings.TrimSpace(full.String())
	if fullText != "" {
		resp.Content = strings.TrimSpace(stripLeakedTrace(fullText))
	}
	return resp
}

// resolveTimeout mirrors 50-request_blocking.zsh:102-104's curl_timeout
// resolution: config.LoadLimits's CurlTimeoutSec already replicates the
// `${AI_CURL_TIMEOUT:-45}` / all-digits-else-45 half (envOrInt), so only
// the minimum-5-seconds floor -- specific to this call site, not a
// property of Limits itself -- is applied here.
func resolveTimeout(limits config.Limits) time.Duration {
	sec := limits.CurlTimeoutSec
	if sec < 5 {
		sec = 5
	}
	return time.Duration(sec) * time.Second
}

// CallBlocking sends one non-streaming chat completion request to
// candidate's endpoint and returns the parsed Response. A literal port of
// _ai_http_call_blocking, translated from "curl -o respfile -w
// %{http_code}" to net/http:
//
//   - The API key never appears in a command-line argument (there is no
//     subprocess at all) -- SESSION-32's SEC-003 fix that motivated the
//     zsh source's headerfile trick is structurally moot in Go, but the
//     same never-let-it-leak intent is preserved: apiKey only ever goes
//     into the Authorization header of this one request.
//   - ctx cancellation (caller's Ctrl-C handling, SESSION-49+) maps to
//     ErrCancelled, matching the `return 130` path.
//   - Any other transport failure (dial error, timeout) is wrapped and
//     returned as a plain error, matching curl_exit != 0 cases the zsh
//     caller inspects ($curl_exit -eq 28 for timeout specifically).
//   - A non-2xx HTTP response is NOT an error here -- exactly like the
//     zsh source, which always `return 0`s and lets the caller
//     (44-retry_decision.zsh, SESSION-46) decide what a given
//     http_status means. Only got-no-response-at-all is a Go error.
func CallBlocking(ctx context.Context, candidate Candidate, payload []byte, limits config.Limits) (Response, error) {
	apiKey := os.Getenv(candidate.Provider.KeyVar)
	if apiKey == "" {
		return Response{}, fmt.Errorf("llmclient: no api key set for provider %q (env var %s)", candidate.Name, candidate.Provider.KeyVar)
	}

	timeout := resolveTimeout(limits)
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, candidate.Provider.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return Response{}, fmt.Errorf("llmclient: building request for %s: %w", candidate.Name, err)
	}

	if candidate.Name == "anthropic" {
		return callAnthropicBlocking(req, apiKey)
	}

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := sharedHTTPClient.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			// Outer ctx (not just our own timeout) was cancelled --
			// this is the caller's Ctrl-C path, i.e. `return 130`.
			return Response{}, ErrCancelled
		}
		return Response{}, fmt.Errorf("llmclient: request to %s (%s) failed: %w", candidate.Name, candidate.Provider.Endpoint, err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("llmclient: reading response body from %s: %w", candidate.Name, err)
	}

	return ParseResponse(httpResp.StatusCode, body), nil
}
