// resilience.go is the "opsional (helper)" line of this session's own
// scope.include: a thin wrapper that drives CallBlocking (SESSION-44)
// through DecideHTTPRetry/CircuitBreaker for a single already-selected
// candidate + model. It deliberately does NOT reproduce
// 50-request_blocking.zsh's full `for provider { for model { while tries
// } }` orchestration (provider/model fallback, spinner, AI_CURRENT_*
// state) -- per this session's own scope.exclude and the handoff notes
// SESSION-44/45 already left behind, that whole loop is agent-loop
// wiring reserved for SESSION-49/50, once every internal/llmclient
// building block (44+45+46) exists. CallWithRetry below is the single-
// model retry primitive that loop will call in an outer per-model
// iteration, the same relationship CallBlocking already has to that
// future loop.
package llmclient

import (
	"context"
	"time"

	"github.com/monang404/luna-go/internal/config"
)

// CallWithRetry repeatedly calls CallBlocking for one candidate+payload,
// applying breaker.Allow() before every attempt and DecideHTTPRetry after
// every non-empty-content failure, until either a usable Response.Content
// is returned, the breaker refuses the next attempt, or maxAttempts is
// reached (mirroring 50-request_blocking.zsh's `while [ $tries -lt
// $AI_MAX_RETRIES ]` inner loop plus its circuit-breaker gate and
// trailing `_ai_breaker_record_fail` on total failure).
//
// breakerKey should be "$provider/$model" (matching
// _ai_breaker_record_fail's own per-model key in the zsh source's model
// loop); breaker may be nil to skip circuit-breaker gating entirely
// (useful for callers/tests that only want the retry behavior).
//
// payload is rebuilt by the caller between attempts when NewMaxTokens
// changes (buildPayload is called with the possibly-adjusted max_tokens
// on every attempt, mirroring the zsh source rebuilding $payload fresh
// inside its `while` loop every time). On the final failed attempt (no
// more retries left), the last Response is returned alongside a nil
// error -- exactly like CallBlocking itself, a non-2xx/non-content
// response is not a Go error; only a transport failure is.
func CallWithRetry(
	ctx context.Context,
	candidate Candidate,
	breakerKey string,
	breaker *BreakerStore,
	limits config.Limits,
	maxTokens int,
	buildPayload func(maxTokens int) ([]byte, error),
) (Response, error) {
	retryDelay := time.Duration(limits.RetryDelaySec) * time.Second
	maxAttempts := limits.MaxRetries
	if maxAttempts <= 0 {
		maxAttempts = 1
	}

	var last Response
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if breaker != nil && breakerKey != "" && !breaker.Allow(breakerKey) {
			return last, nil
		}

		payload, err := buildPayload(maxTokens)
		if err != nil {
			return Response{}, err
		}

		resp, err := CallBlocking(ctx, candidate, payload, limits)
		if err != nil {
			if breaker != nil && breakerKey != "" {
				breaker.RecordFailure(breakerKey)
			}
			return Response{}, err
		}
		last = resp

		if resp.Content != "" {
			if breaker != nil && breakerKey != "" {
				breaker.RecordSuccess(breakerKey)
			}
			return resp, nil
		}

		outcome := DecideHTTPRetry(resp.HTTPStatus, resp.RawBody, maxTokens, retryDelay)
		if outcome.Action == GiveUpModel {
			if breaker != nil && breakerKey != "" {
				breaker.RecordFailure(breakerKey)
			}
			return resp, nil
		}

		if outcome.NewMaxTokens != 0 {
			maxTokens = outcome.NewMaxTokens
		}
		if outcome.Backoff > 0 {
			select {
			case <-ctx.Done():
				return resp, nil
			case <-time.After(outcome.Backoff):
			}
		}
	}

	if breaker != nil && breakerKey != "" {
		breaker.RecordFailure(breakerKey)
	}
	return last, nil
}
