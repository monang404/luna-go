package llmclient

import (
	"testing"
	"time"
)

// --- AC-01: opens after threshold consecutive failures, and the full
// closed->open->half-open->closed cycle ---

func TestCircuitBreaker_ClosedAllowsByDefault(t *testing.T) {
	cb := NewCircuitBreaker(1, 30*time.Second)
	if !cb.Allow() {
		t.Fatal("Allow() = false on a fresh breaker, want true")
	}
	if got := cb.State(); got != StateClosed {
		t.Fatalf("State() = %v, want StateClosed", got)
	}
}

func TestCircuitBreaker_OpensAfterThresholdFailures(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)
	cb.RecordFailure()
	cb.RecordFailure()
	if got := cb.State(); got != StateClosed {
		t.Fatalf("State() after 2/3 failures = %v, want StateClosed", got)
	}
	cb.RecordFailure()
	if got := cb.State(); got != StateOpen {
		t.Fatalf("State() after 3/3 failures = %v, want StateOpen", got)
	}
	if cb.Allow() {
		t.Fatal("Allow() = true while open and within cooldown, want false")
	}
}

func TestCircuitBreaker_DefaultThresholdMatchesZshParity(t *testing.T) {
	// zsh source (_ai_breaker_record_fail/_ai_breaker_is_open): one
	// failure opens the breaker, no failure-count accumulation.
	cb := NewCircuitBreaker(DefaultBreakerThreshold, 30*time.Second)
	cb.RecordFailure()
	if got := cb.State(); got != StateOpen {
		t.Fatalf("State() after 1 failure (default threshold) = %v, want StateOpen", got)
	}
}

func TestCircuitBreaker_FullCycle_ClosedOpenHalfOpenClosed(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)

	if got := cb.State(); got != StateClosed {
		t.Fatalf("initial State() = %v, want StateClosed", got)
	}

	cb.RecordFailure()
	if got := cb.State(); got != StateOpen {
		t.Fatalf("State() after failure = %v, want StateOpen", got)
	}
	if cb.Allow() {
		t.Fatal("Allow() = true immediately after opening, want false")
	}

	time.Sleep(15 * time.Millisecond)
	if got := cb.State(); got != StateHalfOpen {
		t.Fatalf("State() after cooldown = %v, want StateHalfOpen", got)
	}
	if !cb.Allow() {
		t.Fatal("Allow() = false for the half-open trial, want true")
	}
	// A second Allow() before the trial resolves must not permit a
	// concurrent second trial through.
	if cb.Allow() {
		t.Fatal("Allow() = true for a second concurrent half-open trial, want false")
	}

	cb.RecordSuccess()
	if got := cb.State(); got != StateClosed {
		t.Fatalf("State() after successful trial = %v, want StateClosed", got)
	}
	if !cb.Allow() {
		t.Fatal("Allow() = false after breaker closed again, want true")
	}
}

func TestCircuitBreaker_HalfOpenFailureReopens(t *testing.T) {
	cb := NewCircuitBreaker(1, 10*time.Millisecond)
	cb.RecordFailure()
	time.Sleep(15 * time.Millisecond)
	if !cb.Allow() {
		t.Fatal("Allow() = false for the half-open trial, want true")
	}
	cb.RecordFailure()
	if got := cb.State(); got != StateOpen {
		t.Fatalf("State() after failed half-open trial = %v, want StateOpen", got)
	}
	if cb.Allow() {
		t.Fatal("Allow() = true immediately after a re-opened breaker, want false")
	}
}

func TestCircuitBreaker_SuccessResetsFailureCount(t *testing.T) {
	cb := NewCircuitBreaker(3, time.Hour)
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()
	cb.RecordFailure()
	cb.RecordFailure()
	if got := cb.State(); got != StateClosed {
		t.Fatalf("State() after success reset + 2 more failures = %v, want StateClosed (threshold 3)", got)
	}
}

// --- BreakerStore: keyed multi-provider/model behavior ---

func TestBreakerStore_UnknownKeyAlwaysAllowed(t *testing.T) {
	s := NewBreakerStore(1, 30*time.Second)
	if !s.Allow("groq/llama-3.1-8b-instant") {
		t.Fatal("Allow() for a never-seen key = false, want true")
	}
}

func TestBreakerStore_KeysAreIndependent(t *testing.T) {
	s := NewBreakerStore(1, time.Hour)
	s.RecordFailure("groq/model-a")
	if s.Allow("groq/model-a") {
		t.Error("Allow(groq/model-a) = true after its own failure, want false")
	}
	if !s.Allow("groq/model-b") {
		t.Error("Allow(groq/model-b) = false after a DIFFERENT key's failure, want true")
	}
	if !s.Allow("gemini/model-a") {
		t.Error("Allow(gemini/model-a) = false after groq/model-a's failure, want true")
	}
}

func TestBreakerStore_MatchesZshWindowSemantics(t *testing.T) {
	// Reproduces _ai_breaker_is_open's own arithmetic:
	// now - last < AI_CIRCUIT_BREAKER_WINDOW.
	s := NewBreakerStore(DefaultBreakerThreshold, 20*time.Millisecond)
	s.RecordFailure("cerebras")
	if s.Allow("cerebras") {
		t.Fatal("Allow() = true immediately after failure, want false (within window)")
	}
	time.Sleep(25 * time.Millisecond)
	if !s.Allow("cerebras") {
		t.Fatal("Allow() = false after window elapsed, want true (half-open trial)")
	}
}
