package llmclient

import (
	"errors"
	"testing"
	"time"
)

// --- AC-02: retry decision respects max attempts & backoff parity with
// retry_decision.zsh ---

func TestDecideHTTPRetry_413ShrinksAndRetriesImmediately(t *testing.T) {
	got := DecideHTTPRetry(413, nil, 4000, 2*time.Second)
	if got.Action != RetrySameModel {
		t.Fatalf("Action = %v, want RetrySameModel", got.Action)
	}
	if got.NewMaxTokens != 2000 {
		t.Fatalf("NewMaxTokens = %d, want 2000 (half of 4000)", got.NewMaxTokens)
	}
	if got.Backoff != 0 {
		t.Fatalf("Backoff = %v, want 0 (413 retries immediately, no sleep)", got.Backoff)
	}
}

func TestDecideHTTPRetry_413GivesUpBelowFloor(t *testing.T) {
	// 900/2 = 450 < 500 floor -> give up on this model.
	got := DecideHTTPRetry(413, nil, 900, 2*time.Second)
	if got.Action != GiveUpModel {
		t.Fatalf("Action = %v, want GiveUpModel (450 < 500 floor)", got.Action)
	}
}

func TestDecideHTTPRetry_413ExactlyAtFloorStillRetries(t *testing.T) {
	// 1000/2 = 500, which is >= 500 -> still retries.
	got := DecideHTTPRetry(413, nil, 1000, 2*time.Second)
	if got.Action != RetrySameModel || got.NewMaxTokens != 500 {
		t.Fatalf("got %+v, want RetrySameModel with NewMaxTokens=500", got)
	}
}

func TestDecideHTTPRetry_429NeverRetried(t *testing.T) {
	got := DecideHTTPRetry(429, nil, 4000, 2*time.Second)
	if got.Action != GiveUpModel {
		t.Fatalf("Action = %v, want GiveUpModel (429 is never transient)", got.Action)
	}
}

func TestDecideHTTPRetry_404NeverRetried(t *testing.T) {
	got := DecideHTTPRetry(404, nil, 4000, 2*time.Second)
	if got.Action != GiveUpModel {
		t.Fatalf("Action = %v, want GiveUpModel (404 is never transient)", got.Action)
	}
}

// --- regression_checks: never retry a non-transient error such as a 401
// auth error, even when it's reported as HTTP 200 with an error payload
// (the zsh source has no dedicated 401 branch -- this IS how 401 is
// caught, via the generic .error.message check) ---

func TestDecideHTTPRetry_401AuthErrorAsErrorPayloadNeverRetried(t *testing.T) {
	body := []byte(`{"error":{"message":"Incorrect API key provided","type":"invalid_request_error"}}`)
	got := DecideHTTPRetry(200, body, 4000, 2*time.Second)
	if got.Action != GiveUpModel {
		t.Fatalf("Action = %v, want GiveUpModel (401-style error payload is never retried)", got.Action)
	}
}

func TestDecideHTTPRetry_ErrorAsPlainString(t *testing.T) {
	// `.error` can be a bare string instead of an object -- the jq
	// fallback `.error.message // .error` handles both shapes.
	body := []byte(`{"error":"rate limited"}`)
	got := DecideHTTPRetry(500, body, 4000, 2*time.Second)
	if got.Action != GiveUpModel {
		t.Fatalf("Action = %v, want GiveUpModel (string .error still counts)", got.Action)
	}
}

func TestDecideHTTPRetry_TransientFailureRetriesWithBackoff(t *testing.T) {
	got := DecideHTTPRetry(500, []byte(`{"choices":[]}`), 4000, 2*time.Second)
	if got.Action != RetrySameModel {
		t.Fatalf("Action = %v, want RetrySameModel", got.Action)
	}
	if got.Backoff != 2*time.Second {
		t.Fatalf("Backoff = %v, want 2s (AI_RETRY_DELAY parity)", got.Backoff)
	}
	if got.NewMaxTokens != 0 {
		t.Fatalf("NewMaxTokens = %d, want 0 (unchanged) for a non-413 failure", got.NewMaxTokens)
	}
}

func TestDecideHTTPRetry_MalformedBodyStillRetries(t *testing.T) {
	// jq silently produces empty output for invalid JSON; so does
	// jsonErrorMessage -- a malformed body is not itself a reason to
	// give up.
	got := DecideHTTPRetry(500, []byte("not json"), 4000, 2*time.Second)
	if got.Action != RetrySameModel {
		t.Fatalf("Action = %v, want RetrySameModel (malformed body isn't an .error)", got.Action)
	}
}

func TestDecideHTTPRetry_EmptyBodyStillRetries(t *testing.T) {
	got := DecideHTTPRetry(0, nil, 4000, 2*time.Second)
	if got.Action != RetrySameModel {
		t.Fatalf("Action = %v, want RetrySameModel", got.Action)
	}
}

// --- ShouldRetry: transport-level errors + attempt-count ceiling ---

func TestShouldRetry_NilErrorNeverRetries(t *testing.T) {
	retry, _ := ShouldRetry(nil, 0, 3, time.Second)
	if retry {
		t.Fatal("ShouldRetry(nil, ...) = true, want false")
	}
}

func TestShouldRetry_CancelledNeverRetries(t *testing.T) {
	retry, _ := ShouldRetry(ErrCancelled, 0, 3, time.Second)
	if retry {
		t.Fatal("ShouldRetry(ErrCancelled, ...) = true, want false")
	}
}

func TestShouldRetry_WrappedCancelledNeverRetries(t *testing.T) {
	wrapped := errors.New("wrapped: " + ErrCancelled.Error())
	// Not errors.Is-wrapped, so this should still retry (only a real
	// ErrCancelled short-circuits); this guards against
	// over-matching on message text.
	retry, _ := ShouldRetry(wrapped, 0, 3, time.Second)
	if !retry {
		t.Fatal("ShouldRetry on an unrelated error containing similar text = false, want true")
	}
}

func TestShouldRetry_RespectsMaxAttempts(t *testing.T) {
	genericErr := errors.New("network blip")
	if retry, _ := ShouldRetry(genericErr, 0, 1, time.Second); retry {
		t.Fatal("ShouldRetry with maxAttempts=1 at attempt 0 = true, want false (no attempts left)")
	}
	if retry, backoff := ShouldRetry(genericErr, 0, 2, time.Second); !retry || backoff != time.Second {
		t.Fatalf("ShouldRetry with maxAttempts=2 at attempt 0 = (%v, %v), want (true, 1s)", retry, backoff)
	}
	if retry, _ := ShouldRetry(genericErr, 1, 2, time.Second); retry {
		t.Fatal("ShouldRetry with maxAttempts=2 at attempt 1 = true, want false (exhausted)")
	}
}
