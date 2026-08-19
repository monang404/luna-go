// Ported from 30-luna/10-core/41-provider_candidate.zsh (_ai_provider_has_fallback).
package llmclient

import (
	"errors"
	"fmt"

	"github.com/monang404/luna-go/internal/config"
)

// HasFallback reports whether at least one OTHER provider in order
// (besides provider itself) currently has its API key configured -- a
// literal port of _ai_provider_has_fallback's echo 1/0 lookahead. Callers
// (SESSION-46's circuit breaker wiring) use this to decide whether an open
// breaker should still be respected: if provider is the only candidate
// left, trying anyway beats a guaranteed failure with no attempt at all
// (see the zsh source's own comment, carried into this doc comment).
//
// Reuses config.ActiveProviders for the "key var currently set" check
// rather than re-reading os.Getenv directly, so this stays consistent
// with SESSION-41's single source of truth for "is provider p active".
func HasFallback(provider string, order []string) bool {
	for _, name := range config.ActiveProviders(order) {
		if name != provider {
			return true
		}
	}
	return false
}

// ErrNoProviderAvailable is returned by SelectProviderCandidate when every
// provider in order is either unconfigured (no API key) or already listed
// in previousFailures.
var ErrNoProviderAvailable = errors.New("llmclient: no provider candidate available (all excluded or missing api key)")

// Candidate bundles a provider name with its config.Provider entry --
// config.Provider alone doesn't carry the map key it came from, and
// callers (retry/logging in SESSION-46, and this package's own tests)
// need the name for diagnostics exactly like the zsh source's $provider
// local does throughout 50-request_blocking.zsh.
type Candidate struct {
	Name     string
	Provider config.Provider
}

// SelectProviderCandidate picks the next candidate from order: the first
// provider that (a) has its API key configured (config.ActiveProviders)
// and (b) is not already marked failed in previousFailures, preserving
// order.
//
// This is NOT a 1:1 port of a single zsh function -- 41-provider_candidate.zsh
// only contains the HasFallback lookahead above. The actual "pick the next
// provider to try" logic lives inline in 50-request_blocking.zsh's
// `for provider in "${provider_order[@]}"` loop (continuing past ones with
// no key, and -- via the circuit breaker, SESSION-46 -- past ones that
// just failed). That whole orchestrator file is not assigned to any single
// migration session's source_zsh_files (it becomes agent-loop wiring in
// SESSION-49/50 once every 4x/5x building block exists), but this
// session's own scope explicitly asks for a SelectProviderCandidate
// function, so the *provider-selection* fragment of that loop (steps a+b
// above) is synthesized here now, next to HasFallback, rather than left
// for a session that doesn't own it either. The circuit-breaker skip
// (part c of the real loop) is deliberately NOT included here -- that
// state doesn't exist until SESSION-46 -- so previousFailures is a plain
// caller-supplied set, not a breaker lookup. See CHANGELOG SESSION-44
// entry.
func SelectProviderCandidate(order []string, previousFailures map[string]bool) (Candidate, error) {
	providers := config.Providers()
	if DebugEnabled() {
		for _, name := range order {
			p, ok := providers[name]
			if !ok {
				continue
			}
			excluded := previousFailures[name]
			Debugf("provider=%s key=%s excluded=%v", name, DescribeKey(p.KeyVar), excluded)
		}
	}
	for _, name := range config.ActiveProviders(order) {
		if previousFailures[name] {
			continue
		}
		p, ok := providers[name]
		if !ok {
			continue
		}
		Debugf("selected_provider=%s selected_model=%s", name, p.Model)
		return Candidate{Name: name, Provider: p}, nil
	}
	Debugf("no provider candidate available: order=%v previously_failed=%d", order, len(previousFailures))
	return Candidate{}, ErrNoProviderAvailable
}

// ExhaustionError builds the final error defaultComplete/CompleteMessages
// return once every provider in order has been tried and failed
// (previousFailures is non-empty) or none was ever configured
// (previousFailures is empty). Distinguishing those two cases in the
// returned message is the fix for the audit brief's headline complaint:
// "no provider candidate available (all excluded or missing api key)"
// reads identically whether zero keys were ever configured or every
// configured provider was actually tried over the network and failed --
// those are very different problems and the previous message collapsed
// them into one confusing sentence. last carries whatever the most
// recent attempt's Response was (HTTPStatus/ErrorMessage), "" fields
// when no attempt was ever made.
func ExhaustionError(order []string, previousFailures map[string]bool, last Response) error {
	if len(previousFailures) == 0 {
		return ErrNoProviderAvailable
	}
	tried := make([]string, 0, len(previousFailures))
	for _, name := range order {
		if previousFailures[name] {
			tried = append(tried, name)
		}
	}
	detail := last.ErrorMessage
	if detail == "" && last.HTTPStatus != 0 {
		detail = fmt.Sprintf("http %d, no usable content in response", last.HTTPStatus)
	}
	if detail == "" {
		detail = "no usable content in any response"
	}
	return fmt.Errorf("llmclient: all %d configured provider(s) tried and failed (%v); last error: %s", len(tried), tried, detail)
}
