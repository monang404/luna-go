package llmclient

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/monang404/luna-go/internal/config"
)

// --- AC-02: parse response correctly (ports ai_extract.py's extract() +
// the jq error-message fallback chain) ---

func TestParseResponse_PlainContent(t *testing.T) {
	body := `{"choices":[{"message":{"content":"hello world"},"finish_reason":"stop"}],"usage":{"prompt_tokens":10,"completion_tokens":2,"total_tokens":12}}`
	got := ParseResponse(200, []byte(body))

	if got.Content != "hello world" {
		t.Errorf("Content = %q, want %q", got.Content, "hello world")
	}
	if got.FinishReason != "stop" {
		t.Errorf("FinishReason = %q, want %q", got.FinishReason, "stop")
	}
	if got.Usage != (Usage{PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12}) {
		t.Errorf("Usage = %+v, want {10 2 12}", got.Usage)
	}
	if got.ErrorMessage != "" {
		t.Errorf("ErrorMessage = %q, want empty", got.ErrorMessage)
	}
}

func TestParseResponse_ReasoningContentMergedBeforeContent(t *testing.T) {
	body := `{"choices":[{"message":{"content":"the answer","reasoning_content":"thinking step by step"},"finish_reason":"stop"}]}`
	got := ParseResponse(200, []byte(body))

	want := "thinking step by step\nthe answer"
	if got.Content != want {
		t.Errorf("Content = %q, want %q", got.Content, want)
	}
}

func TestParseResponse_ReasoningOnlyNoContent(t *testing.T) {
	// deepseek-v4* empty-completion case documented in 42-token_budget.zsh:
	// content empty, reasoning_content carries the whole reply.
	body := `{"choices":[{"message":{"content":"","reasoning_content":"all the thinking"},"finish_reason":"length"}]}`
	got := ParseResponse(200, []byte(body))

	if got.Content != "all the thinking" {
		t.Errorf("Content = %q, want %q", got.Content, "all the thinking")
	}
}

func TestParseResponse_EmptyChoicesArray(t *testing.T) {
	// The old bug this guards against: `choices: []` is valid JSON but a
	// naive `choices[0]` access panics/errors -- must degrade to "" like
	// ai_extract.py's `if not choices: return ""`.
	body := `{"choices":[]}`
	got := ParseResponse(200, []byte(body))

	if got.Content != "" {
		t.Errorf("Content = %q, want empty for empty choices array", got.Content)
	}
}

func TestParseResponse_MalformedJSONDegradesGracefully(t *testing.T) {
	got := ParseResponse(200, []byte("not json at all"))
	if got.Content != "" || got.ErrorMessage != "" {
		t.Errorf("malformed body should degrade to zero-value fields, got Content=%q ErrorMessage=%q", got.Content, got.ErrorMessage)
	}
	if got.HTTPStatus != 200 {
		t.Errorf("HTTPStatus should still be recorded even on parse failure, got %d", got.HTTPStatus)
	}
}

func TestParseResponse_StripsLeakedTracePrefix(t *testing.T) {
	body := `{"choices":[{"message":{"content":"temp=0.6\nrespfile=/tmp/tmp.abc123\nhttp_status=200\ncurl_exit=28\n### FILE: utils.py\nreal content here"}}]}`
	got := ParseResponse(200, []byte(body))

	want := "### FILE: utils.py\nreal content here"
	if got.Content != want {
		t.Errorf("Content = %q, want %q (leaked trace prefix not stripped)", got.Content, want)
	}
}

func TestParseResponse_DoesNotStripLegitimateAssignmentLikeLines(t *testing.T) {
	// A generic "key=value" regex would wrongly eat this; the whitelist
	// must not, since "count" and "DEBUG" aren't in leakedTraceVars.
	body := `{"choices":[{"message":{"content":"count=0\nDEBUG=True\nprint(count)"}}]}`
	got := ParseResponse(200, []byte(body))

	want := "count=0\nDEBUG=True\nprint(count)"
	if got.Content != want {
		t.Errorf("Content = %q, want %q (legitimate code lines wrongly stripped)", got.Content, want)
	}
}

func TestParseResponse_ErrorMessageObjectShape(t *testing.T) {
	body := `{"error":{"message":"model does not exist"},"choices":[]}`
	got := ParseResponse(404, []byte(body))
	if got.ErrorMessage != "model does not exist" {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, "model does not exist")
	}
}

func TestParseResponse_ErrorStringShape(t *testing.T) {
	body := `{"error":"rate limited","choices":[]}`
	got := ParseResponse(429, []byte(body))
	if got.ErrorMessage != "rate limited" {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, "rate limited")
	}
}

func TestParseResponse_TopLevelMessageFallback(t *testing.T) {
	body := `{"message":"service unavailable","choices":[]}`
	got := ParseResponse(503, []byte(body))
	if got.ErrorMessage != "service unavailable" {
		t.Errorf("ErrorMessage = %q, want %q", got.ErrorMessage, "service unavailable")
	}
}

// --- AC-02 (round trip): CallBlocking against a real HTTP server. A live
// call to an actual provider (groq/gemini/cerebras/deepseek) needs a dev
// API key and network egress this sandboxed environment doesn't have --
// see CHANGELOG SESSION-44 entry. httptest.Server is the closest
// equivalent: a real TCP round trip through net/http, an
// OpenAI-compatible body, exercised end-to-end through CallBlocking. ---

func TestCallBlocking_RoundTripAgainstHTTPServer(t *testing.T) {
	var gotAuth, gotContentType, gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		buf := make([]byte, r.ContentLength)
		r.Body.Read(buf)
		gotBody = string(buf)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"choices":[{"message":{"content":"pong"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	defer srv.Close()

	t.Setenv("TESTPROV_API_KEY", "secret-123")
	candidate := Candidate{Name: "testprov", Provider: config.Provider{
		Endpoint: srv.URL,
		Model:    "test-model",
		KeyVar:   "TESTPROV_API_KEY",
	}}
	payload, err := BuildPayload([]Message{{Role: "user", Content: "ping"}}, PayloadOptions{Model: "test-model", MaxTokens: 100, Temperature: 0.6})
	if err != nil {
		t.Fatalf("BuildPayload error: %v", err)
	}

	resp, err := CallBlocking(context.Background(), candidate, payload, config.Limits{CurlTimeoutSec: 45})
	if err != nil {
		t.Fatalf("CallBlocking error: %v", err)
	}

	if resp.HTTPStatus != 200 {
		t.Errorf("HTTPStatus = %d, want 200", resp.HTTPStatus)
	}
	if resp.Content != "pong" {
		t.Errorf("Content = %q, want %q", resp.Content, "pong")
	}
	if resp.Usage.TotalTokens != 2 {
		t.Errorf("Usage.TotalTokens = %d, want 2", resp.Usage.TotalTokens)
	}
	if gotAuth != "Bearer secret-123" {
		t.Errorf("Authorization header = %q, want %q", gotAuth, "Bearer secret-123")
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", gotContentType, "application/json")
	}
	if !strings.Contains(gotBody, `"ping"`) {
		t.Errorf("request body = %q, want it to contain the payload", gotBody)
	}
}

func TestCallBlocking_NonSuccessHTTPStatusIsNotAnError(t *testing.T) {
	// Mirrors _ai_http_call_blocking always `return 0`ing regardless of
	// http_status -- a 429/413/404 is data for the caller (retry
	// decision, SESSION-46), never a Go error by itself.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	t.Setenv("TESTPROV_API_KEY", "k")
	candidate := Candidate{Name: "testprov", Provider: config.Provider{Endpoint: srv.URL, KeyVar: "TESTPROV_API_KEY"}}
	payload, _ := BuildPayload([]Message{{Role: "user", Content: "x"}}, PayloadOptions{Model: "m", MaxTokens: 10, Temperature: 0.6})

	resp, err := CallBlocking(context.Background(), candidate, payload, config.Limits{CurlTimeoutSec: 45})
	if err != nil {
		t.Fatalf("CallBlocking should not error on HTTP 429, got: %v", err)
	}
	if resp.HTTPStatus != 429 {
		t.Errorf("HTTPStatus = %d, want 429", resp.HTTPStatus)
	}
	if resp.ErrorMessage != "rate limited" {
		t.Errorf("ErrorMessage = %q, want %q", resp.ErrorMessage, "rate limited")
	}
}

func TestCallBlocking_MissingAPIKeyErrors(t *testing.T) {
	t.Setenv("TESTPROV_NOKEY", "")
	candidate := Candidate{Name: "testprov", Provider: config.Provider{Endpoint: "http://example.invalid", KeyVar: "TESTPROV_NOKEY"}}
	_, err := CallBlocking(context.Background(), candidate, []byte(`{}`), config.Limits{CurlTimeoutSec: 45})
	if err == nil {
		t.Error("CallBlocking with no API key set should return an error")
	}
}

// --- AC-04: HTTP timeout is respected ---

func TestResolveTimeout_FloorAtFiveSeconds(t *testing.T) {
	cases := []struct {
		name string
		sec  int
		want time.Duration
	}{
		{"below floor clamped to 5s", 2, 5 * time.Second},
		{"zero clamped to 5s", 0, 5 * time.Second},
		{"exactly floor unchanged", 5, 5 * time.Second},
		{"above floor passed through", 45, 45 * time.Second},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := resolveTimeout(config.Limits{CurlTimeoutSec: c.sec})
			if got != c.want {
				t.Errorf("resolveTimeout(%d) = %v, want %v", c.sec, got, c.want)
			}
		})
	}
}

func TestCallBlocking_CancelsSlowRequest(t *testing.T) {
	handlerStarted := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerStarted <- struct{}{}
		time.Sleep(2 * time.Second) // longer than the test's deadline below
	}))
	// CloseClientConnections (not Close) so we don't block waiting for the
	// still-sleeping handler goroutine to return -- Close() would hang
	// for the remainder of that 2s sleep every run, which is exactly the
	// slow-teardown trap this helper avoids.
	defer srv.CloseClientConnections()

	t.Setenv("TESTPROV_API_KEY", "k")
	candidate := Candidate{Name: "testprov", Provider: config.Provider{Endpoint: srv.URL, KeyVar: "TESTPROV_API_KEY"}}
	payload, _ := BuildPayload([]Message{{Role: "user", Content: "x"}}, PayloadOptions{Model: "m", MaxTokens: 10, Temperature: 0.6})

	// A tight outer deadline stands in for a slow request hitting
	// config.Limits.CurlTimeoutSec -- resolveTimeout's floor (tested
	// above) is what derives that duration from Limits in the real call
	// path; this proves CallBlocking actually cancels once whichever
	// deadline is active elapses, without a real multi-second sleep in
	// the test itself.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := CallBlocking(ctx, candidate, payload, config.Limits{CurlTimeoutSec: 45})
	elapsed := time.Since(start)
	<-handlerStarted // make sure the slow handler really was reached

	if err == nil {
		t.Fatal("CallBlocking should return an error when the request times out")
	}
	if elapsed > 1*time.Second {
		t.Errorf("CallBlocking took %v to return after timeout, want well under 1s", elapsed)
	}
}

func TestCallBlocking_OuterCancelMapsToErrCancelled(t *testing.T) {
	handlerStarted := make(chan struct{}, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerStarted <- struct{}{}
		time.Sleep(2 * time.Second)
	}))
	defer srv.CloseClientConnections()

	t.Setenv("TESTPROV_API_KEY", "k")
	candidate := Candidate{Name: "testprov", Provider: config.Provider{Endpoint: srv.URL, KeyVar: "TESTPROV_API_KEY"}}
	payload, _ := BuildPayload([]Message{{Role: "user", Content: "x"}}, PayloadOptions{Model: "m", MaxTokens: 10, Temperature: 0.6})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel() // simulates the caller's TRAPINT/TRAPTERM path (Ctrl-C)
	}()

	_, err := CallBlocking(ctx, candidate, payload, config.Limits{CurlTimeoutSec: 45})
	<-handlerStarted
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("CallBlocking error = %v, want ErrCancelled", err)
	}
}
