package codeproject

import (
	"context"
	"testing"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
)

// fakeCompleter is a test double implementing aiops.Completer.
type fakeCompleter struct {
	contents []string // successive replies, one per call; last one repeats
	calls    int
	err      error
}

func (f *fakeCompleter) next() string {
	if len(f.contents) == 0 {
		return ""
	}
	idx := f.calls - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(f.contents) {
		idx = len(f.contents) - 1
	}
	return f.contents[idx]
}

func (f *fakeCompleter) Complete(ctx context.Context, systemPrompt, userPrompt string, class config.TaskClass, order []string, maxTokens int) (aiops.Result, error) {
	f.calls++
	if f.err != nil {
		return aiops.Result{}, f.err
	}
	return aiops.Result{Content: f.next(), Provider: "fake", Model: "fake-model"}, nil
}

func (f *fakeCompleter) CompleteMessages(ctx context.Context, messages []llmclient.Message, class config.TaskClass, order []string, maxTokens int) (aiops.Result, error) {
	return f.Complete(ctx, "", "", class, order, maxTokens)
}

// fakeRunner is a test double implementing aiops.CommandRunner --
// package tests never execute real python3/subprocesses.
type fakeRunner struct {
	// responses maps a joined "name args..." key to a canned result;
	// missing keys default to a clean success.
	responses map[string]runnerResponse
	calls     []string
}

type runnerResponse struct {
	stdout, stderr string
	exitCode       int
	err            error
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (string, string, int, error) {
	key := name
	for _, a := range args {
		key += " " + a
	}
	f.calls = append(f.calls, key)
	if f.responses != nil {
		if r, ok := f.responses[key]; ok {
			return r.stdout, r.stderr, r.exitCode, r.err
		}
	}
	return "", "", 0, nil
}

func withFakeKey(t *testing.T) {
	t.Helper()
	t.Setenv("GROQ_API_KEY", "fake-key-for-tests")
}

func approveConfirm(ctx context.Context, prompt string) (aiops.Decision, error) {
	return aiops.Approved, nil
}
func declineConfirm(ctx context.Context, prompt string) (aiops.Decision, error) {
	return aiops.Declined, nil
}
