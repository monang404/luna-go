// Ported from 30-luna/10-core/44-retry_decision.zsh (_ai_chat_retry_decision).
package llmclient

import (
	"encoding/json"
	"errors"
	"time"
)

// RetryAction is what a caller should do after DecideHTTPRetry inspects
// one failed attempt's response.
type RetryAction int

const (
	// RetrySameModel: try the same model again (either right away, for
	// the 413-shrink case, or after Backoff for an ordinary transient
	// failure).
	RetrySameModel RetryAction = iota
	// GiveUpModel: stop retrying this model/provider and move on
	// (equivalent to the zsh source's `return 1` -> caller `break`s out
	// of its retry loop).
	GiveUpModel
)

// HTTPRetryOutcome is DecideHTTPRetry's result.
type HTTPRetryOutcome struct {
	Action RetryAction

	// NewMaxTokens is only set (non-zero) when Action==RetrySameModel
	// and the 413 branch fired: the caller's next attempt should use
	// this max_tokens value instead of the one it just tried, mirroring
	// _ai_chat_retry_decision mutating the caller's `max_toks` local in
	// place. Zero means "leave max_tokens unchanged".
	NewMaxTokens int

	// Backoff is how long the caller should sleep before its next
	// attempt. Zero for the 413 branch (the zsh source retries that one
	// immediately, no sleep -- shrinking max_tokens is itself the fix,
	// not something a delay would help). AI_RETRY_DELAY-equivalent for
	// the generic transient-failure branch.
	Backoff time.Duration

	// Reason is a short human-readable label for diagnostics/logging,
	// mirroring the zsh source's _ai_chat_diag calls at each branch
	// (info/warn lines) without reproducing their exact wording.
	Reason string
}

// minMaxTokensFloor mirrors 44-retry_decision.zsh's literal `500` floor:
// below this, halving max_tokens again is judged not worth another
// attempt and the zsh source gives up on the model instead.
const minMaxTokensFloor = 500

// errorEnvelope decodes just enough of a response body to read
// `.error.message // .error` -- the same jq fallback chain
// 44-retry_decision.zsh:38 uses to detect an API error embedded in an
// HTTP-200 body.
type errorEnvelope struct {
	Error json.RawMessage `json:"error"`
}

// jsonErrorMessage extracts the effective error string from rawBody the
// same way 44-retry_decision.zsh's `jq -r '.error.message // .error //
// empty'` does: prefer an object's .message field, else the raw string
// value, else "". A body that isn't valid JSON (or has no .error key)
// yields "", exactly like jq silently producing empty output for either
// case.
func jsonErrorMessage(rawBody []byte) string {
	var env errorEnvelope
	if err := json.Unmarshal(rawBody, &env); err != nil || len(env.Error) == 0 || string(env.Error) == "null" {
		return ""
	}
	var obj struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(env.Error, &obj); err == nil && obj.Message != "" {
		return obj.Message
	}
	var s string
	if err := json.Unmarshal(env.Error, &s); err == nil && s != "" {
		return s
	}
	return ""
}

// DecideHTTPRetry ports _ai_chat_retry_decision's branching exactly:
//
//  1. HTTP 413 (request too large for the account's TPM limit): halve
//     currentMaxTokens; if the result is still >=500, retry the SAME
//     model immediately with that smaller value; otherwise give up on
//     this model.
//  2. HTTP 429 (quota exhausted) or 404 (model unavailable): never
//     transient -- give up on this model immediately, no retry.
//  3. A `.error.message`/`.error` field present in the body even on an
//     otherwise-successful status (e.g. a 401 auth error some providers
//     report as HTTP 200 with an error payload): give up on this model.
//     This is also what makes retry correctly refuse to retry a genuine
//     401 -- it arrives as exactly this shape, no separate HTTP-401
//     branch exists in the zsh source or here.
//  4. Anything else: retry the same model after backoff.
//
// httpStatus/rawBody are the failed attempt's response; retryDelay is
// AI_RETRY_DELAY (config.Limits.RetryDelaySec, as a time.Duration).
// DecideHTTPRetry does not itself compare an attempt counter against a
// max -- exactly like the zsh source, whose `while [ $tries -lt
// $AI_MAX_RETRIES ]` loop lives in the caller (50-request_blocking.zsh),
// not in _ai_chat_retry_decision. A caller that wants an attempt-count
// ceiling applies it around this function, e.g. by checking
// attempt<maxAttempts before honoring Action==RetrySameModel.
func DecideHTTPRetry(httpStatus int, rawBody []byte, currentMaxTokens int, retryDelay time.Duration) HTTPRetryOutcome {
	if httpStatus == 413 {
		newMax := currentMaxTokens / 2
		if newMax >= minMaxTokensFloor {
			return HTTPRetryOutcome{
				Action:       RetrySameModel,
				NewMaxTokens: newMax,
				Reason:       "413: request too large, shrinking max_tokens and retrying immediately",
			}
		}
		return HTTPRetryOutcome{Action: GiveUpModel, Reason: "413: still too large at minimum max_tokens"}
	}

	if httpStatus == 429 || httpStatus == 404 {
		return HTTPRetryOutcome{Action: GiveUpModel, Reason: "quota exhausted or model unavailable (non-transient)"}
	}

	if msg := jsonErrorMessage(rawBody); msg != "" {
		return HTTPRetryOutcome{Action: GiveUpModel, Reason: "API returned an error payload: " + msg}
	}

	return HTTPRetryOutcome{Action: RetrySameModel, Backoff: retryDelay, Reason: "transient failure, retrying after backoff"}
}

// ShouldRetry is the generic transport-level counterpart to
// DecideHTTPRetry, for a failed attempt that never produced an HTTP
// response at all (err from CallBlocking/CallStreaming -- a dial error,
// timeout, or context cancellation), plus the attempt-count ceiling the
// zsh source's own `while [ $tries -lt $AI_MAX_RETRIES ]` loop applies.
// Named to match this session's own scope.include signature
// ("func ShouldRetry(err, attempt, maxAttempts) (bool, backoff
// time.Duration)").
//
// A cancelled request (ErrCancelled, from blocking.go/streaming.go) is
// never retried -- the caller explicitly asked to stop, matching the zsh
// source's TRAPINT/TRAPTERM handlers returning 130/143 directly instead
// of falling into the retry loop at all. Any other error is retried
// (with retryDelay backoff, mirroring AI_RETRY_DELAY) as long as attempts
// remain.
func ShouldRetry(err error, attempt, maxAttempts int, retryDelay time.Duration) (bool, time.Duration) {
	if err == nil {
		return false, 0
	}
	if errors.Is(err, ErrCancelled) {
		return false, 0
	}
	if attempt+1 >= maxAttempts {
		return false, 0
	}
	return true, retryDelay
}
