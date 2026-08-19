package llmclient

import (
	"os"
	"strings"
	"testing"

	"github.com/monang404/luna-go/internal/config"
)

// --- Regression: ExhaustionError must distinguish "never configured"
// from "tried and failed", per the audit brief's headline finding that
// "no provider candidate available (all excluded or missing api key)"
// gave no way to tell those two cases apart. ---

func TestExhaustionError_NoneTriedReturnsSentinel(t *testing.T) {
	err := ExhaustionError(config.TaskProviderOrderAgent, map[string]bool{}, Response{})
	if err != ErrNoProviderAvailable {
		t.Errorf("ExhaustionError with empty previousFailures = %v, want ErrNoProviderAvailable", err)
	}
}

func TestExhaustionError_TriedAndFailedIncludesDetail(t *testing.T) {
	order := config.TaskProviderOrderAgent
	failed := map[string]bool{"deepseek": true, "cerebras": true}
	last := Response{HTTPStatus: 429, ErrorMessage: "rate limit exceeded"}

	err := ExhaustionError(order, failed, last)
	if err == nil {
		t.Fatal("ExhaustionError with non-empty previousFailures = nil, want an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "deepseek") || !strings.Contains(msg, "cerebras") {
		t.Errorf("ExhaustionError message %q does not name the tried providers", msg)
	}
	if !strings.Contains(msg, "rate limit exceeded") {
		t.Errorf("ExhaustionError message %q does not carry the last failure detail", msg)
	}
	if err == ErrNoProviderAvailable {
		t.Error("ExhaustionError with tried providers must not equal the generic sentinel")
	}
}

func TestExhaustionError_TriedButNoDetailFallsBackToHTTPStatus(t *testing.T) {
	err := ExhaustionError([]string{"groq"}, map[string]bool{"groq": true}, Response{HTTPStatus: 500})
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("ExhaustionError message %q should mention the HTTP status when no ErrorMessage is set", err.Error())
	}
}

// --- Regression: AIOPS_DEBUG must never leak an API key's value, only
// whether it's configured/missing (audit brief section 9/22). ---

func TestDescribeKey_NeverLeaksValue(t *testing.T) {
	t.Setenv("GROQ_API_KEY", "sk-super-secret-value")
	defer os.Unsetenv("GROQ_API_KEY")

	got := DescribeKey("GROQ_API_KEY")
	if got != "configured" {
		t.Errorf("DescribeKey = %q, want %q", got, "configured")
	}
	if strings.Contains(got, "sk-super-secret-value") {
		t.Fatal("DescribeKey leaked the raw key value")
	}
}

func TestDescribeKey_MissingWhenUnset(t *testing.T) {
	os.Unsetenv("SOME_UNSET_KEY_VAR")
	if got := DescribeKey("SOME_UNSET_KEY_VAR"); got != "missing" {
		t.Errorf("DescribeKey = %q, want %q", got, "missing")
	}
}

func TestDebugf_SilentWhenDisabled(t *testing.T) {
	os.Unsetenv("AIOPS_DEBUG")
	if DebugEnabled() {
		t.Fatal("DebugEnabled() = true with AIOPS_DEBUG unset, want false")
	}
}

func TestDebugf_EnabledOnTruthyValue(t *testing.T) {
	t.Setenv("AIOPS_DEBUG", "1")
	if !DebugEnabled() {
		t.Error("DebugEnabled() = false with AIOPS_DEBUG=1, want true")
	}
	t.Setenv("AIOPS_DEBUG", "0")
	if DebugEnabled() {
		t.Error("DebugEnabled() = true with AIOPS_DEBUG=0, want false")
	}
}

// --- Regression: SelectProviderCandidate's real candidate/selection
// behavior must be unaffected by the added debug instrumentation. ---

func TestSelectProviderCandidate_DebugEnabledStillSelects(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("GROQ_API_KEY", "k1")
	t.Setenv("AIOPS_DEBUG", "1")

	cand, err := SelectProviderCandidate(config.TaskProviderOrderAgent, map[string]bool{})
	if err != nil {
		t.Fatalf("SelectProviderCandidate() error = %v, want nil", err)
	}
	if cand.Name != "groq" {
		t.Errorf("SelectProviderCandidate().Name = %q, want %q", cand.Name, "groq")
	}
}
