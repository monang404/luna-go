package chat

import (
	"context"
	"testing"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
)

// fakeCompleter is a test double implementing aiops.Completer so
// package tests never depend on live LUNA providers.
type fakeCompleter struct {
	content string
	err     error
	// stageContents, if set, returns successive contents per call
	// (used to test multi-stage LongChat).
	stageContents []string
	calls         int
	lastMessages  []llmclient.Message
}

func (f *fakeCompleter) Complete(ctx context.Context, systemPrompt, userPrompt string, class config.TaskClass, order []string, maxTokens int) (aiops.Result, error) {
	f.calls++
	if f.err != nil {
		return aiops.Result{}, f.err
	}
	if len(f.stageContents) > 0 {
		idx := f.calls - 1
		if idx < len(f.stageContents) {
			return aiops.Result{Content: f.stageContents[idx], Provider: "fake", Model: "fake-model"}, nil
		}
	}
	return aiops.Result{Content: f.content, Provider: "fake", Model: "fake-model"}, nil
}

func (f *fakeCompleter) CompleteMessages(ctx context.Context, messages []llmclient.Message, class config.TaskClass, order []string, maxTokens int) (aiops.Result, error) {
	f.lastMessages = messages
	return f.Complete(ctx, "", "", class, order, maxTokens)
}

func withFakeKey(t *testing.T) {
	t.Helper()
	t.Setenv("GROQ_API_KEY", "fake-key-for-tests")
}

func TestQuickChat_SplitsReply(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "Analisis singkat.\n@@JAWABAN@@\nJawaban bersih."})
	res, err := svc.QuickChat(context.Background(), "halo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Answer != "Jawaban bersih." {
		t.Errorf("Answer = %q", res.Answer)
	}
	if res.Thought != "Analisis singkat." {
		t.Errorf("Thought = %q", res.Thought)
	}
}

func TestQuickChat_EmptyPrompt(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "x"})
	_, err := svc.QuickChat(context.Background(), "")
	if err != ErrNoPrompt {
		t.Errorf("expected ErrNoPrompt, got %v", err)
	}
}

func TestQuickChat_NoProvider(t *testing.T) {
	svc := NewService(&fakeCompleter{content: "x"})
	_, err := svc.QuickChat(context.Background(), "halo")
	if err != ErrNoProvider {
		t.Errorf("expected ErrNoProvider, got %v", err)
	}
}

func TestAish_NoSplit(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "ls -la"})
	res, err := svc.Aish(context.Background(), "list files")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Answer != "ls -la" {
		t.Errorf("Answer = %q", res.Answer)
	}
}

func TestLongChat_RunsAllFiveStages(t *testing.T) {
	withFakeKey(t)
	fc := &fakeCompleter{stageContents: []string{"outline", "draft", "refined", "reviewed", "final output"}}
	svc := NewService(fc)
	res, err := svc.LongChat(context.Background(), "buatkan artikel")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Stages) != 5 {
		t.Fatalf("expected 5 stages, got %d", len(res.Stages))
	}
	if res.Final != "final output" {
		t.Errorf("Final = %q", res.Final)
	}
	if res.Stages[0].Stage != StageOutline || res.Stages[4].Stage != StageFinal {
		t.Errorf("unexpected stage order: %+v", res.Stages)
	}
}

func TestLongChat_StopsOnStageFailure(t *testing.T) {
	withFakeKey(t)
	fc := &failAfterNCompleter{n: 2, content: "outline"}
	svc := NewService(fc)
	res, err := svc.LongChat(context.Background(), "buatkan artikel")
	if err == nil {
		t.Fatal("expected an error when a stage fails")
	}
	if len(res.Stages) != 2 {
		t.Errorf("expected exactly 2 completed stages before failure, got %d", len(res.Stages))
	}
	if res.Final != "" {
		t.Errorf("expected no Final on failure, got %q", res.Final)
	}
}

func TestLongChat_EmptyPrompt(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "x"})
	_, err := svc.LongChat(context.Background(), "")
	if err != ErrNoPrompt {
		t.Errorf("expected ErrNoPrompt, got %v", err)
	}
}

// failAfterNCompleter succeeds for the first n calls, then errors --
// used to test that LongChat stops immediately on a mid-pipeline
// failure rather than continuing with a synthesized/partial result.
type failAfterNCompleter struct {
	n       int
	calls   int
	content string
}

func (f *failAfterNCompleter) Complete(ctx context.Context, systemPrompt, userPrompt string, class config.TaskClass, order []string, maxTokens int) (aiops.Result, error) {
	f.calls++
	if f.calls > f.n {
		return aiops.Result{}, context.DeadlineExceeded
	}
	return aiops.Result{Content: f.content, Provider: "fake", Model: "fake-model"}, nil
}

func (f *failAfterNCompleter) CompleteMessages(ctx context.Context, messages []llmclient.Message, class config.TaskClass, order []string, maxTokens int) (aiops.Result, error) {
	return f.Complete(ctx, "", "", class, order, maxTokens)
}
