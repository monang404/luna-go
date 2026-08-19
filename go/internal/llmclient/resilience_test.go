package llmclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/monang404/luna-go/internal/config"
)

func testLimits() config.Limits {
	l := config.LoadLimits()
	l.MaxRetries = 3
	l.RetryDelaySec = 0 // keep tests fast
	l.CurlTimeoutSec = 5
	return l
}

func testCandidate(t *testing.T, srv *httptest.Server) Candidate {
	t.Helper()
	t.Setenv("TEST_RESILIENCE_KEY", "k")
	return Candidate{
		Name: "test",
		Provider: config.Provider{
			Endpoint: srv.URL,
			Model:    "test-model",
			KeyVar:   "TEST_RESILIENCE_KEY",
		},
	}
}

func TestCallWithRetry_SucceedsFirstTry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"choices":[{"message":{"content":"hi"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	calls := 0
	buildPayload := func(maxTokens int) ([]byte, error) {
		calls++
		return []byte(`{}`), nil
	}

	resp, err := CallWithRetry(context.Background(), testCandidate(t, srv), "test/test-model", NewBreakerStore(1, time.Second), testLimits(), 4000, buildPayload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "hi" {
		t.Fatalf("Content = %q, want hi", resp.Content)
	}
	if calls != 1 {
		t.Fatalf("buildPayload called %d times, want 1", calls)
	}
}

func TestCallWithRetry_GivesUpOn429WithoutExhaustingAttempts(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(429)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	buildPayload := func(maxTokens int) ([]byte, error) { return []byte(`{}`), nil }
	limits := testLimits()
	limits.MaxRetries = 5

	resp, err := CallWithRetry(context.Background(), testCandidate(t, srv), "test/test-model", NewBreakerStore(1, time.Second), limits, 4000, buildPayload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.HTTPStatus != 429 {
		t.Fatalf("HTTPStatus = %d, want 429", resp.HTTPStatus)
	}
	if calls != 1 {
		t.Fatalf("server called %d times, want 1 (429 gives up immediately, not a real retry loop)", calls)
	}
}

func TestCallWithRetry_RetriesTransientFailureThenSucceeds(t *testing.T) {
	attempt := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempt++
		if attempt < 3 {
			w.WriteHeader(500)
			w.Write([]byte(`{}`))
			return
		}
		w.Write([]byte(`{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	buildPayload := func(maxTokens int) ([]byte, error) { return []byte(`{}`), nil }
	limits := testLimits()
	limits.MaxRetries = 5

	resp, err := CallWithRetry(context.Background(), testCandidate(t, srv), "test/test-model", NewBreakerStore(1, time.Second), limits, 4000, buildPayload)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("Content = %q, want ok (after transient retries)", resp.Content)
	}
	if attempt != 3 {
		t.Fatalf("server called %d times, want 3", attempt)
	}
}

func TestCallWithRetry_BreakerOpensAfterFailureAndBlocksNextCall(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(404)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	buildPayload := func(maxTokens int) ([]byte, error) { return []byte(`{}`), nil }
	limits := testLimits()
	breaker := NewBreakerStore(1, time.Hour)
	cand := testCandidate(t, srv)

	if _, err := CallWithRetry(context.Background(), cand, "test/test-model", breaker, limits, 4000, buildPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("first call count = %d, want 1", calls)
	}

	// Second call should be short-circuited by the now-open breaker.
	if _, err := CallWithRetry(context.Background(), cand, "test/test-model", breaker, limits, 4000, buildPayload); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 1 {
		t.Fatalf("call count after breaker opened = %d, want still 1 (breaker should have blocked it)", calls)
	}
}
