// Ported from 30-luna/10-core/40-circuit_breaker.zsh (_ai_breaker_record_fail,
// _ai_breaker_is_open).
//
// The zsh source is not a textbook N-failure-threshold breaker: one call to
// _ai_breaker_record_fail("$key") stamps $key's last-failure time to "now",
// and _ai_breaker_is_open("$key") is simply `now - last < AI_CIRCUIT_
// BREAKER_WINDOW` -- i.e. threshold=1, no half-open probe state, no
// explicit "closed again" transition (the key just ages out of the window
// and reads as not-open again). This session's own scope.include asks for
// a classic state{closed,failureCount,threshold,cooldown} shape with
// Allow()/RecordSuccess()/RecordFailure(), which is a superset of that
// behavior -- CircuitBreaker below defaults its Threshold to 1 so that,
// used with the zsh source's own AI_CIRCUIT_BREAKER_WINDOW value
// (config.Limits.CircuitBreakerWindowSec, 30s), it reproduces the exact
// same open/closed timing per scope.exclude's "port nilai apa adanya"
// instruction, while also supporting Threshold>1 for callers that want
// real N-failure debouncing.
//
// One further deliberate difference: the zsh source persists failure
// timestamps to a file (AI_CIRCUIT_BREAKER_FILE) so the breaker survives
// across separate `luna ...` invocations (each one a fresh zsh process).
// CircuitBreaker here is purely in-memory, scoped to this session's own
// scope.include ("Struct CircuitBreaker{...}" -- no file persistence is
// listed). A long-running Go process (the eventual cmd/luna agent
// loop, SESSION-49+) keeps this state for its own lifetime, which is the
// scenario the breaker actually protects against (a hot loop hammering a
// provider that just failed); cross-invocation persistence would need a
// file-backed store layered on top, deliberately left out of this file.
package llmclient

import (
	"sync"
	"time"
)

// State is one of a CircuitBreaker's three states.
type State int

const (
	// StateClosed: requests are allowed; failures accumulate toward
	// Threshold.
	StateClosed State = iota
	// StateOpen: requests are blocked until Cooldown has elapsed since
	// the failure that tripped the breaker.
	StateOpen
	// StateHalfOpen: Cooldown has elapsed; exactly one trial request is
	// allowed through to test whether the provider has recovered.
	StateHalfOpen
)

func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// DefaultBreakerThreshold matches the zsh source's implicit behavior:
// _ai_breaker_record_fail marks a key failed on the very first failure,
// with no failure-count accumulation before the breaker opens.
const DefaultBreakerThreshold = 1

// CircuitBreaker guards a single key (one provider, or one
// "provider/model" pair -- callers choose the granularity, exactly like
// the zsh source's _ai_breaker_is_open/_ai_breaker_record_fail take a
// plain string key for both).
type CircuitBreaker struct {
	threshold int
	cooldown  time.Duration

	mu                sync.Mutex
	state             State
	failureCount      int
	openedAt          time.Time
	halfOpenAttempted bool
}

// NewCircuitBreaker builds a CircuitBreaker that opens after threshold
// consecutive failures (via RecordFailure with no intervening
// RecordSuccess) and stays open for cooldown before allowing one
// half-open trial. threshold<=0 is treated as DefaultBreakerThreshold
// (the zsh source's own behavior -- a breaker that never opens would not
// be a port of 40-circuit_breaker.zsh at all).
func NewCircuitBreaker(threshold int, cooldown time.Duration) *CircuitBreaker {
	if threshold <= 0 {
		threshold = DefaultBreakerThreshold
	}
	return &CircuitBreaker{threshold: threshold, cooldown: cooldown, state: StateClosed}
}

// State reports the breaker's current state, resolving an Open breaker
// whose cooldown has already elapsed to HalfOpen as a side effect (same
// lazy "just check the timestamp" evaluation _ai_breaker_is_open does --
// there is no background timer in either the zsh source or here).
func (cb *CircuitBreaker) State() State {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.resolveLocked()
	return cb.state
}

// resolveLocked transitions Open->HalfOpen once cooldown has elapsed.
// Must be called with cb.mu held.
func (cb *CircuitBreaker) resolveLocked() {
	if cb.state == StateOpen && time.Since(cb.openedAt) >= cb.cooldown {
		cb.state = StateHalfOpen
		cb.halfOpenAttempted = false
	}
}

// Allow reports whether a request through this breaker's key should be
// attempted right now: always true when Closed, true exactly once per
// cooldown window when HalfOpen (the trial request), false when Open and
// still within cooldown. Equivalent to `! _ai_breaker_is_open "$key"`
// gating a call in 50-request_blocking.zsh/55-request_streaming.zsh.
func (cb *CircuitBreaker) Allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.resolveLocked()

	switch cb.state {
	case StateClosed:
		return true
	case StateHalfOpen:
		if cb.halfOpenAttempted {
			return false
		}
		cb.halfOpenAttempted = true
		return true
	default: // StateOpen
		return false
	}
}

// RecordSuccess resets the breaker to Closed with a zero failure count --
// a successful half-open trial (or any success while closed) clears
// whatever failure history preceded it, matching the zsh source's own
// implicit behavior (a provider that succeeds is never written to
// AI_CIRCUIT_BREAKER_FILE, so its next _ai_breaker_is_open check reads
// "not open" regardless of past failures).
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.state = StateClosed
	cb.failureCount = 0
	cb.halfOpenAttempted = false
}

// RecordFailure records one failure. A failure while HalfOpen re-opens
// the breaker immediately (the trial failed -- back to a full cooldown),
// matching _ai_breaker_record_fail being called again for a key that was
// only just about to age out of its window. A failure while Closed
// increments failureCount and opens the breaker once it reaches
// threshold (1, by default -- see DefaultBreakerThreshold's doc comment).
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.resolveLocked()

	switch cb.state {
	case StateHalfOpen:
		cb.state = StateOpen
		cb.openedAt = time.Now()
		cb.failureCount = cb.threshold
	default: // Closed or (already) Open
		cb.failureCount++
		if cb.failureCount >= cb.threshold {
			cb.state = StateOpen
			cb.openedAt = time.Now()
		}
	}
}

// BreakerStore is a keyed collection of CircuitBreakers sharing the same
// threshold/cooldown, created lazily per key on first use. This is the
// direct replacement for the zsh source's single AI_CIRCUIT_BREAKER_FILE
// keyed by an arbitrary "$provider" or "$provider/$model" string
// (_ai_breaker_is_open/_ai_breaker_record_fail both take that string as
// their only parameter) -- callers here use the same two key shapes
// (provider name, or "provider/model"), one BreakerStore per process.
type BreakerStore struct {
	threshold int
	cooldown  time.Duration

	mu       sync.Mutex
	breakers map[string]*CircuitBreaker
}

// NewBreakerStore builds an empty BreakerStore. Pass
// config.Limits.CircuitBreakerWindowSec (as a time.Duration) for cooldown
// to match the zsh source's AI_CIRCUIT_BREAKER_WINDOW default exactly.
func NewBreakerStore(threshold int, cooldown time.Duration) *BreakerStore {
	return &BreakerStore{threshold: threshold, cooldown: cooldown, breakers: make(map[string]*CircuitBreaker)}
}

// get returns (creating if needed) the CircuitBreaker for key.
func (s *BreakerStore) get(key string) *CircuitBreaker {
	s.mu.Lock()
	defer s.mu.Unlock()
	cb, ok := s.breakers[key]
	if !ok {
		cb = NewCircuitBreaker(s.threshold, s.cooldown)
		s.breakers[key] = cb
	}
	return cb
}

// Allow reports whether key's breaker currently permits a request. A key
// never seen before is always allowed (equivalent to
// _ai_breaker_is_open returning 1/false for a provider with no line yet
// in AI_CIRCUIT_BREAKER_FILE).
func (s *BreakerStore) Allow(key string) bool { return s.get(key).Allow() }

// RecordSuccess records a success for key.
func (s *BreakerStore) RecordSuccess(key string) { s.get(key).RecordSuccess() }

// RecordFailure records a failure for key -- equivalent to
// _ai_breaker_record_fail("$key").
func (s *BreakerStore) RecordFailure(key string) { s.get(key).RecordFailure() }
