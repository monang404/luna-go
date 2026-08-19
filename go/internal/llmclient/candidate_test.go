package llmclient

import (
	"os"
	"testing"

	"github.com/monang404/luna-go/internal/config"
)

// clearProviderKeys unsets every provider key var so each test starts
// clean regardless of what the host env happens to have set -- same
// helper shape as internal/config's own config_test.go.
func clearProviderKeys(t *testing.T) {
	t.Helper()
	for _, p := range config.Providers() {
		t.Setenv(p.KeyVar, "")
		os.Unsetenv(p.KeyVar)
	}
}

// --- AC-03: provider candidate selection respects AI_PROVIDER_ORDER and
// skips providers without a key, same as the config layer ---

func TestHasFallback_TrueWhenAnotherProviderHasKey(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("GROQ_API_KEY", "k1")
	t.Setenv("GEMINI_API_KEY", "k2")

	if !HasFallback("groq", config.ProviderOrder) {
		t.Error("HasFallback(groq) = false, want true (gemini has a key)")
	}
}

func TestHasFallback_FalseWhenSoleCandidate(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("GROQ_API_KEY", "k1")

	if HasFallback("groq", config.ProviderOrder) {
		t.Error("HasFallback(groq) = true, want false (groq is the only configured provider)")
	}
}

func TestHasFallback_IgnoresProvidersWithoutKeys(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("GROQ_API_KEY", "k1")
	// gemini/cerebras left unset -- neither should count as a fallback.

	if HasFallback("groq", config.ProviderOrder) {
		t.Error("HasFallback(groq) = true, want false (gemini/cerebras have no key)")
	}
}

func TestSelectProviderCandidate_RespectsOrderAndSkipsMissingKey(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("CEREBRAS_API_KEY", "k1")
	t.Setenv("GEMINI_API_KEY", "k2")
	// groq (first in ProviderOrder) has no key -- must be skipped.

	got, err := SelectProviderCandidate(config.ProviderOrder, nil) // [groq gemini cerebras]
	if err != nil {
		t.Fatalf("SelectProviderCandidate error: %v", err)
	}
	if got.Name != "gemini" {
		t.Errorf("SelectProviderCandidate = %q, want %q (groq has no key, gemini is next in order)", got.Name, "gemini")
	}
}

func TestSelectProviderCandidate_SkipsPreviousFailures(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("GROQ_API_KEY", "k1")
	t.Setenv("GEMINI_API_KEY", "k2")

	got, err := SelectProviderCandidate(config.ProviderOrder, map[string]bool{"groq": true})
	if err != nil {
		t.Fatalf("SelectProviderCandidate error: %v", err)
	}
	if got.Name != "gemini" {
		t.Errorf("SelectProviderCandidate = %q, want %q (groq already failed)", got.Name, "gemini")
	}
}

func TestSelectProviderCandidate_NoneAvailable(t *testing.T) {
	clearProviderKeys(t)
	// No keys set at all.

	_, err := SelectProviderCandidate(config.ProviderOrder, nil)
	if err != ErrNoProviderAvailable {
		t.Errorf("SelectProviderCandidate error = %v, want ErrNoProviderAvailable", err)
	}
}

func TestSelectProviderCandidate_AllFailedEvenWithKeys(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("GROQ_API_KEY", "k1")
	t.Setenv("GEMINI_API_KEY", "k2")

	failed := map[string]bool{"groq": true, "gemini": true}
	_, err := SelectProviderCandidate(config.ProviderOrder, failed)
	if err != ErrNoProviderAvailable {
		t.Errorf("SelectProviderCandidate error = %v, want ErrNoProviderAvailable", err)
	}
}
