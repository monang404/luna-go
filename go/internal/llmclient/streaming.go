// This file (streaming.go) ports the single-request half of
// 30-luna/10-core/55-request_streaming.zsh (_ai_chat_request_stream): the
// `curl -N` call and the loop that feeds each line into
// _ai_sse_process_line (see sse.go for that half). It deliberately does
// NOT port:
//
//   - The `for provider in ... { for model in ... { while tries < max ...`
//     retry/fallback loop, circuit breaker checks (_ai_breaker_is_open/
//     _ai_breaker_record_fail), or _ai_chat_retry_decision -- all of that
//     is SESSION-46 (port_llm_resilience_layer)'s scope, per this
//     session's own scope.exclude. CallStreaming below is the thing
//     SESSION-46 will wrap in that loop, the same way CallBlocking
//     (SESSION-44) already is.
//   - Printing the model label / streamed deltas to a terminal
//     (`printf "%s > "`, `printf '%s' "$content"`) -- that's the chat UI,
//     SESSION-52+. This package only ever produces Events; a caller
//     decides what to do with them.
//
// See streaming.go's CallStreaming doc comment and sse.go's parseSSELine
// for the two halves of the actual port.
package llmclient

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/monang404/luna-go/internal/config"
)

// Event is one item on the channel CallStreaming returns: either an
// incremental piece of a genuine SSE stream, or -- for a provider that
// silently ignored `stream:true` and replied with one ordinary JSON body
// instead (55-request_streaming.zsh:101-130's own fallback branch) -- the
// whole final answer delivered as a single Event.
type Event struct {
	// Content is a delta.content chunk (the streamed case) or the
	// complete final answer (the non-SSE fallback case, extracted via
	// ParseResponse exactly like CallBlocking would).
	Content string

	// Reasoning is a delta.reasoning/delta.reasoning_content chunk --
	// internal-only, mirrors 56-sse_line_parser.zsh's $reasoningfile.
	// It is never meant to be rendered to the user (see that file's own
	// comment on why reasoning must never become user-facing output). A
	// caller may still want it, to reproduce
	// 55-request_streaming.zsh:101-112's "reasoning arrived but no
	// final content ever did -> suppress the whole response rather than
	// leak reasoning to stdout" decision -- but making that call is
	// this package's *caller*'s job (SESSION-46/49), not this file's;
	// CallStreaming just reports what arrived.
	Reasoning string

	// Done marks the last Event of the stream; Content/Reasoning are
	// always "" on this Event. The channel is closed immediately after
	// it is sent, exactly once per call.
	Done bool

	// Err is set on the final Event only, if the stream ended because
	// of a transport failure (dropped connection, timeout, or ctx
	// cancellation) rather than a clean "[DONE]" sentinel or plain EOF.
	// Done is always also true whenever Err is set. Equivalent in spirit
	// to CallBlocking's returned error, translated to this channel's
	// async shape -- see CallStreaming's own doc comment for why this
	// can't just be a second return value.
	Err error

	// HTTPStatus is the response's status code; the same value on every
	// Event of one call. A non-2xx status is not itself an Err here, for
	// the same reason it isn't in CallBlocking's Response -- deciding
	// what a given status means is the retry layer's job (SESSION-46).
	HTTPStatus int
}

// maxSSELineBytes caps one buffered SSE line. Providers stream small,
// token-sized delta.content chunks, so a real line is tiny; the ceiling
// exists only to bound memory against a pathological/runaway response
// (or the non-SSE fallback body, which -- unlike a real stream -- can
// legitimately be one very long line) rather than to reflect any expected
// size.
const maxSSELineBytes = 8 << 20 // 8 MiB

// CallStreaming starts an SSE chat completion request against
// candidate's endpoint and returns a channel of Events, filled in
// asynchronously as the response streams in. A literal port of
// _ai_chat_request_stream's single HTTP attempt (`curl -N` piped line by
// line into _ai_sse_process_line), translated from that subshell pipeline
// to net/http + a goroutine the same way SESSION-44's CallBlocking
// translated _ai_http_call_blocking's plain curl call -- see that
// function's own doc comment for the API-key/cancellation parallels,
// which all apply here unchanged.
//
// Unlike CallBlocking, the error CallStreaming itself returns is only
// ever a *setup* failure: no API key configured, a request that can't be
// built, or the initial connection attempt failing outright (including
// ctx already being cancelled before the call started -- mapped to
// ErrCancelled, exactly like CallBlocking). Once a non-nil channel has
// been handed back, every later failure -- including ctx being cancelled
// while the stream is in flight, AC-03's Ctrl-C case -- is reported as
// that channel's final Event instead, since a function that has already
// returned has no second error to give a caller; see streamBody.
func CallStreaming(ctx context.Context, candidate Candidate, payload []byte, limits config.Limits) (<-chan Event, error) {
	apiKey := os.Getenv(candidate.Provider.KeyVar)
	if apiKey == "" {
		return nil, fmt.Errorf("llmclient: no api key set for provider %q (env var %s)", candidate.Name, candidate.Provider.KeyVar)
	}

	if candidate.Name == "anthropic" {
		return nil, fmt.Errorf("llmclient: streaming is not yet supported for Anthropic API")
	}

	timeout := resolveTimeout(limits)
	reqCtx, cancel := context.WithTimeout(ctx, timeout)

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, candidate.Provider.Endpoint, bytes.NewReader(payload))
	if err != nil {
		cancel()
		return nil, fmt.Errorf("llmclient: building request for %s: %w", candidate.Name, err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := sharedHTTPClient.Do(req)
	if err != nil {
		cancel()
		if ctx.Err() != nil {
			// Outer ctx (not just this call's own timeout) was
			// cancelled before/while connecting -- the caller's
			// Ctrl-C path, same as CallBlocking's `return 130`.
			return nil, ErrCancelled
		}
		return nil, fmt.Errorf("llmclient: request to %s (%s) failed: %w", candidate.Name, candidate.Provider.Endpoint, err)
	}

	events := make(chan Event)
	go func() {
		defer cancel()
		defer httpResp.Body.Close()
		streamBody(ctx, httpResp.Body, httpResp.StatusCode, events)
		close(events)
	}()
	return events, nil
}

// streamBody reads r line by line and turns each line into zero or more
// Events on ch, blocking on send (respecting ctx cancellation -- AC-03) so
// a slow consumer applies natural backpressure rather than this goroutine
// buffering an unbounded number of undelivered Events.
//
// It is split out from CallStreaming's goroutine specifically so tests
// can feed it an arbitrary io.Reader -- including one that deliberately
// hands back only a handful of bytes per Read call, to prove a line split
// across reads is still parsed correctly (AC-01) -- without needing a
// real HTTP round trip for every case.
//
// bufio.Scanner is what makes AC-01 hold: it buffers internally until it
// has a complete line, regardless of how many underlying Read calls (i.e.
// how the response happened to be TCP-chunked) it took to get there. A
// naive fixed-size read-and-split would instead corrupt any "data: {...}"
// JSON line unlucky enough to straddle a chunk boundary.
func streamBody(ctx context.Context, r io.Reader, status int, ch chan<- Event) {
	send := func(ev Event) bool {
		ev.HTTPStatus = status
		select {
		case ch <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 4096), maxSSELineBytes)

	// raw accumulates lines the same way _ai_sse_process_line's
	// unconditional `printf '%s\n' "$line" >> "$rawfile"` does, but only
	// until the first real "data:" line arrives (sawData flips true) --
	// past that point this is confirmed genuine SSE and the non-SSE
	// fallback below can never trigger, so there is no reason to keep
	// buffering a long stream's full text in memory (a >5000-token reply
	// would otherwise mean holding the whole raw response AND emitting
	// every delta, unlike the zsh source where $rawfile lives on disk
	// rather than in a shell variable). This is the one deliberate
	// divergence from a byte-for-byte port of that line -- it changes
	// memory behavior, not any observable Event, since raw is only ever
	// read back while !sawData.
	var raw bytes.Buffer
	sawData := false

	for scanner.Scan() {
		line := scanner.Text()
		if !sawData {
			raw.WriteString(line)
			raw.WriteByte('\n')
		}

		parsed := parseSSELine(line)
		if !parsed.IsData {
			continue
		}
		sawData = true

		if parsed.Done {
			send(Event{Done: true})
			return
		}
		if parsed.Content != "" || parsed.Reasoning != "" {
			if !send(Event{Content: parsed.Content, Reasoning: parsed.Reasoning}) {
				return
			}
		}
	}

	if err := scanner.Err(); err != nil {
		if ctx.Err() != nil {
			// Outer ctx cancelled mid-stream (Ctrl-C while tokens
			// were still arriving) -- AC-03.
			send(Event{Done: true, Err: ErrCancelled})
			return
		}
		send(Event{Done: true, Err: fmt.Errorf("llmclient: reading stream: %w", err)})
		return
	}

	// EOF with no "[DONE]" sentinel. 55-request_streaming.zsh:101-130
	// draws a line here: if not one single "data:" line ever arrived,
	// the provider ignored `stream:true` and this is actually one plain
	// JSON blocking response -- re-parse the whole accumulated body and
	// surface its content as a single final Event, so a caller sees the
	// same text CallBlocking would have returned for it. If at least one
	// "data:" line did arrive, this was genuine SSE that simply ended
	// without an explicit [DONE] (some providers omit it on a clean
	// close) -- nothing further to extract.
	if !sawData {
		resp := ParseResponse(status, raw.Bytes())
		if resp.Content != "" {
			if !send(Event{Content: resp.Content}) {
				return
			}
		}
	}
	send(Event{Done: true})
}
