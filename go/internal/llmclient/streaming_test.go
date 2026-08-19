package llmclient

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/monang404/luna-go/internal/config"
)

// collect drains ch and returns every Event received.
func collect(ch <-chan Event) []Event {
	var got []Event
	for ev := range ch {
		got = append(got, ev)
	}
	return got
}

// --- AC-01: identical events for a whole-body read vs. one chunked byte
// by byte, proving line splitting survives arbitrary TCP-style chunking
// ---

// byteAtATimeReader wraps a Reader and returns at most n bytes per Read
// call, simulating a response that arrives in many small TCP packets
// rather than one big buffer -- the scenario AC-01 exists to guard
// against.
type byteAtATimeReader struct {
	r io.Reader
	n int
}

func (b *byteAtATimeReader) Read(p []byte) (int, error) {
	if len(p) > b.n {
		p = p[:b.n]
	}
	return b.r.Read(p)
}

func runStreamBody(t *testing.T, body string, chunkSize int) []Event {
	t.Helper()
	var r io.Reader = strings.NewReader(body)
	if chunkSize > 0 {
		r = &byteAtATimeReader{r: strings.NewReader(body), n: chunkSize}
	}
	ch := make(chan Event)
	go func() {
		streamBody(context.Background(), r, 200, ch)
		close(ch)
	}()
	return collect(ch)
}

const groqLikeStream = "data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n" +
	"data: {\"choices\":[{\"delta\":{\"content\":\", \"}}]}\n" +
	"data: {\"choices\":[{\"delta\":{\"content\":\"world!\"}}]}\n" +
	"data: [DONE]\n"

func TestStreamBody_ChunkBoundary_WholeVsByteAtATime(t *testing.T) {
	whole := runStreamBody(t, groqLikeStream, 0)
	chunked := runStreamBody(t, groqLikeStream, 1) // 1 byte per Read: worst case
	chunked3 := runStreamBody(t, groqLikeStream, 3)

	wantContent := "Hello, world!"
	for name, events := range map[string][]Event{"whole": whole, "1-byte": chunked, "3-byte": chunked3} {
		var got strings.Builder
		doneCount := 0
		for _, ev := range events {
			got.WriteString(ev.Content)
			if ev.Done {
				doneCount++
			}
			if ev.Err != nil {
				t.Errorf("[%s] unexpected Err: %v", name, ev.Err)
			}
		}
		if got.String() != wantContent {
			t.Errorf("[%s] concatenated Content = %q, want %q", name, got.String(), wantContent)
		}
		if doneCount != 1 {
			t.Errorf("[%s] Done event count = %d, want exactly 1", name, doneCount)
		}
		if !events[len(events)-1].Done {
			t.Errorf("[%s] last event should be Done", name)
		}
	}
}

func TestStreamBody_LineSplitAcrossReadBoundary(t *testing.T) {
	// A "data:" line's JSON deliberately split mid-token by forcing a 5
	// byte read size, so the split falls in the middle of the word
	// "content" on at least one line -- the exact failure mode a
	// fixed-size non-buffering reader would get wrong.
	events := runStreamBody(t, groqLikeStream, 5)
	var got strings.Builder
	for _, ev := range events {
		got.WriteString(ev.Content)
	}
	if got.String() != "Hello, world!" {
		t.Errorf("got %q, want %q", got.String(), "Hello, world!")
	}
}

// --- AC-02: "[DONE]" sentinel ends the channel cleanly, no error ---

func TestStreamBody_DoneSentinelNoError(t *testing.T) {
	events := runStreamBody(t, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\ndata: [DONE]\n", 0)
	last := events[len(events)-1]
	if !last.Done {
		t.Fatal("last event should be Done")
	}
	if last.Err != nil {
		t.Errorf("Err = %v, want nil for a clean [DONE]", last.Err)
	}
}

func TestStreamBody_EOFWithoutDoneStillClosesCleanly(t *testing.T) {
	// Some providers close the stream without ever sending "[DONE]".
	// That's a clean end, not an error.
	events := runStreamBody(t, "data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n", 0)
	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Errorf("got Done=%v Err=%v, want Done=true Err=nil", last.Done, last.Err)
	}
}

// --- reasoning-only stream: delivered as Reasoning, never as Content ---

func TestStreamBody_ReasoningOnlyNeverBecomesContent(t *testing.T) {
	body := "data: {\"choices\":[{\"delta\":{\"reasoning\":\"step one\"}}]}\n" +
		"data: {\"choices\":[{\"delta\":{\"reasoning\":\" step two\"}}]}\n" +
		"data: [DONE]\n"
	events := runStreamBody(t, body, 0)
	var content, reasoning strings.Builder
	for _, ev := range events {
		content.WriteString(ev.Content)
		reasoning.WriteString(ev.Reasoning)
	}
	if content.String() != "" {
		t.Errorf("Content = %q, want empty -- reasoning must never surface as Content", content.String())
	}
	if reasoning.String() != "step one step two" {
		t.Errorf("Reasoning = %q, want %q", reasoning.String(), "step one step two")
	}
}

// --- non-SSE fallback: provider ignored stream:true, sent one JSON body ---

func TestStreamBody_NonSSEFallbackToPlainJSON(t *testing.T) {
	body := `{"choices":[{"message":{"content":"plain answer"},"finish_reason":"stop"}],"usage":{"total_tokens":5}}`
	events := runStreamBody(t, body, 0)

	var gotContent []string
	for _, ev := range events {
		if ev.Content != "" {
			gotContent = append(gotContent, ev.Content)
		}
	}
	if len(gotContent) != 1 || gotContent[0] != "plain answer" {
		t.Errorf("Content events = %v, want exactly one %q", gotContent, "plain answer")
	}
	last := events[len(events)-1]
	if !last.Done || last.Err != nil {
		t.Errorf("got Done=%v Err=%v, want Done=true Err=nil", last.Done, last.Err)
	}
}

func TestStreamBody_NonSSEFallbackEmptyContentEmitsNoEvent(t *testing.T) {
	// A non-SSE body with no usable content at all (e.g. an error
	// object) should not emit a bogus empty Content event -- only the
	// final Done.
	body := `{"error":"rate limited"}`
	events := runStreamBody(t, body, 0)
	if len(events) != 1 || !events[0].Done {
		t.Errorf("events = %+v, want exactly one Done event", events)
	}
}

// --- AC-03: context cancellation mid-stream closes cleanly, no goroutine leak ---

// slowReader blocks each Read until ctx is done or a fixed delay passes,
// standing in for a real network connection that's still receiving more
// of the stream when the caller cancels.
type slowReader struct {
	ctx   context.Context
	chunk []byte
	sent  bool
}

func (s *slowReader) Read(p []byte) (int, error) {
	if !s.sent {
		s.sent = true
		n := copy(p, s.chunk)
		return n, nil
	}
	<-s.ctx.Done()
	return 0, s.ctx.Err()
}

func TestStreamBody_ContextCancelMidStreamClosesCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &slowReader{ctx: ctx, chunk: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"partial\"}}]}\n")}

	ch := make(chan Event)
	done := make(chan struct{})
	go func() {
		streamBody(ctx, r, 200, ch)
		close(ch)
		close(done)
	}()

	first := <-ch // the "partial" content delta
	if first.Content != "partial" {
		t.Fatalf("first event Content = %q, want %q", first.Content, "partial")
	}

	cancel()

	select {
	case ev, ok := <-ch:
		if ok {
			if !ev.Done {
				t.Errorf("event after cancel should be Done, got %+v", ev)
			}
			if !errors.Is(ev.Err, ErrCancelled) {
				t.Errorf("Err = %v, want ErrCancelled", ev.Err)
			}
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for channel to close after ctx cancel")
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("streamBody goroutine did not exit after ctx cancel")
	}
}

func TestStreamBody_NoGoroutineLeakAfterManyCancels(t *testing.T) {
	// go.uber.org/goleak would be the standard tool for this, but this
	// sandbox's network egress allowlist has no route to
	// go.uber.org/proxy.golang.org (see CHANGELOG SESSION-45 entry), so
	// this uses a plain before/after runtime.NumGoroutine() comparison
	// instead -- coarser, but it does catch a goroutine that's supposed
	// to exit on cancel but doesn't.
	runtime.GC()
	before := runtime.NumGoroutine()

	const n = 50
	for i := 0; i < n; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		r := &slowReader{ctx: ctx, chunk: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"x\"}}]}\n")}
		ch := make(chan Event)
		go func() {
			streamBody(ctx, r, 200, ch)
			close(ch)
		}()
		<-ch // first delta
		cancel()
		for range ch {
			// drain until closed
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	var after int
	for time.Now().Before(deadline) {
		runtime.GC()
		after = runtime.NumGoroutine()
		if after <= before+2 { // small slack for test/runtime scheduling noise
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if after > before+2 {
		t.Errorf("goroutine count grew from %d to %d after %d cancelled streams -- possible leak", before, after, n)
	}
}

// --- End-to-end CallStreaming against a real httptest.Server ---

func TestCallStreaming_RoundTripAgainstHTTPServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer secret-123" {
			t.Errorf("Authorization = %q, want %q", got, "Bearer secret-123")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)
		for _, chunk := range []string{
			"data: {\"choices\":[{\"delta\":{\"content\":\"Hel\"}}]}\n\n",
			"data: {\"choices\":[{\"delta\":{\"content\":\"lo\"}}]}\n\n",
			"data: [DONE]\n\n",
		} {
			w.Write([]byte(chunk))
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer srv.Close()

	t.Setenv("STREAMTEST_API_KEY", "secret-123")
	candidate := Candidate{Name: "streamtest", Provider: config.Provider{
		Endpoint: srv.URL,
		Model:    "test-model",
		KeyVar:   "STREAMTEST_API_KEY",
	}}
	payload, err := BuildPayload([]Message{{Role: "user", Content: "hi"}}, PayloadOptions{Model: "test-model", MaxTokens: 100, Temperature: 0.6, Stream: true})
	if err != nil {
		t.Fatalf("BuildPayload error: %v", err)
	}

	ch, err := CallStreaming(context.Background(), candidate, payload, config.Limits{CurlTimeoutSec: 45})
	if err != nil {
		t.Fatalf("CallStreaming error: %v", err)
	}

	var got strings.Builder
	var sawDone bool
	for ev := range ch {
		got.WriteString(ev.Content)
		if ev.Err != nil {
			t.Errorf("unexpected Err: %v", ev.Err)
		}
		if ev.HTTPStatus != 200 {
			t.Errorf("HTTPStatus = %d, want 200", ev.HTTPStatus)
		}
		if ev.Done {
			sawDone = true
		}
	}
	if got.String() != "Hello" {
		t.Errorf("Content = %q, want %q", got.String(), "Hello")
	}
	if !sawDone {
		t.Error("never received a Done event")
	}
}

func TestCallStreaming_NonSuccessHTTPStatusIsNotAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	t.Setenv("STREAMTEST_API_KEY", "k")
	candidate := Candidate{Name: "streamtest", Provider: config.Provider{Endpoint: srv.URL, KeyVar: "STREAMTEST_API_KEY"}}
	payload, _ := BuildPayload([]Message{{Role: "user", Content: "x"}}, PayloadOptions{Model: "m", MaxTokens: 10, Temperature: 0.6, Stream: true})

	ch, err := CallStreaming(context.Background(), candidate, payload, config.Limits{CurlTimeoutSec: 45})
	if err != nil {
		t.Fatalf("CallStreaming should not error synchronously on HTTP 429, got: %v", err)
	}
	events := collect(ch)
	if len(events) != 1 || !events[0].Done || events[0].HTTPStatus != 429 {
		t.Errorf("events = %+v, want exactly one Done event with HTTPStatus=429", events)
	}
}

func TestCallStreaming_MissingAPIKeyErrors(t *testing.T) {
	t.Setenv("STREAMTEST_NOKEY", "")
	candidate := Candidate{Name: "streamtest", Provider: config.Provider{Endpoint: "http://example.invalid", KeyVar: "STREAMTEST_NOKEY"}}
	ch, err := CallStreaming(context.Background(), candidate, []byte(`{}`), config.Limits{CurlTimeoutSec: 45})
	if err == nil {
		t.Error("CallStreaming with no API key set should return an error")
	}
	if ch != nil {
		t.Error("channel should be nil on a setup error")
	}
}

func TestCallStreaming_OuterCancelBeforeConnectMapsToErrCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	t.Setenv("STREAMTEST_API_KEY", "k")
	candidate := Candidate{Name: "streamtest", Provider: config.Provider{Endpoint: srv.URL, KeyVar: "STREAMTEST_API_KEY"}}
	payload, _ := BuildPayload([]Message{{Role: "user", Content: "x"}}, PayloadOptions{Model: "m", MaxTokens: 10, Temperature: 0.6, Stream: true})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before the call starts

	_, err := CallStreaming(ctx, candidate, payload, config.Limits{CurlTimeoutSec: 45})
	if !errors.Is(err, ErrCancelled) {
		t.Errorf("err = %v, want ErrCancelled", err)
	}
}

// --- AC-04: fixture replay from recorded-shape provider responses ---
//
// This sandbox's network egress allowlist has no route to
// api.groq.com / generativelanguage.googleapis.com / api.cerebras.luna (see
// CHANGELOG SESSION-45 entry, same constraint SESSION-44 hit for its own
// AC-02), so these are NOT literal byte-for-byte network captures.
// Each fixture is built to match that provider's actual OpenAI-compatible
// SSE wire shape (chat.completion.chunk objects, "data: " lines,
// "[DONE]" sentinel) as documented for its API, replacing what a `curl -N`
// recording would have produced. testdata/streaming_fixtures/*.sse.

func TestStreamBody_FixtureReplay(t *testing.T) {
	fixtures := []struct {
		provider string
		file     string
		want     string
	}{
		{"groq", "testdata/streaming_fixtures/groq.sse", "The capital of France is Paris."},
		{"gemini", "testdata/streaming_fixtures/gemini.sse", "The capital of France is Paris."},
		{"cerebras", "testdata/streaming_fixtures/cerebras.sse", "The capital of France is Paris."},
	}

	for _, fx := range fixtures {
		t.Run(fx.provider, func(t *testing.T) {
			data, err := readFixture(fx.file)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}

			for _, chunkSize := range []int{0, 1, 7} {
				events := runStreamBody(t, data, chunkSize)
				var got strings.Builder
				for _, ev := range events {
					got.WriteString(ev.Content)
					if ev.Err != nil {
						t.Errorf("chunkSize=%d: unexpected Err: %v", chunkSize, ev.Err)
					}
				}
				if got.String() != fx.want {
					t.Errorf("chunkSize=%d: replayed Content = %q, want %q", chunkSize, got.String(), fx.want)
				}
				if !events[len(events)-1].Done {
					t.Errorf("chunkSize=%d: last event should be Done", chunkSize)
				}
			}
		})
	}
}

// --- regression_checks: long stream (>5000 tokens) doesn't grow memory
// unboundedly, since streamBody stops buffering into `raw` once genuine
// SSE is confirmed (see streaming.go's comment on that decision) ---

func TestStreamBody_LongStreamDoesNotBufferRawAfterFirstDataLine(t *testing.T) {
	const numChunks = 6000
	var body strings.Builder
	for i := 0; i < numChunks; i++ {
		body.WriteString(`data: {"choices":[{"delta":{"content":"tok "}}]}` + "\n")
	}
	body.WriteString("data: [DONE]\n")

	events := runStreamBody(t, body.String(), 4096)

	total := 0
	for _, ev := range events {
		total += len(ev.Content)
	}
	wantLen := numChunks * len("tok ")
	if total != wantLen {
		t.Errorf("total streamed Content length = %d, want %d", total, wantLen)
	}
	if !events[len(events)-1].Done {
		t.Error("last event should be Done")
	}
}

func readFixture(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(bytes.TrimRight(b, "\n")) + "\n", nil
}
