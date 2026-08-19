package workflow

import (
	"context"
	"os"
	"testing"

	"github.com/monang404/luna-go/internal/config"
)

func TestPlan_Success(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	svc := &Service{Requester: &fakeCompleter{contents: []string{"# Rencana\n- [ ] task 1\n"}}, Paths: config.Paths{PlanDir: dir}}
	res, err := svc.Plan(context.Background(), "belajar golang")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outfile == "" {
		t.Fatal("expected an outfile path")
	}
	got, _ := os.ReadFile(res.Outfile)
	if string(got) != "# Rencana\n- [ ] task 1\n\n" {
		t.Errorf("unexpected file content: %q", got)
	}
}

func TestPlan_UsageError(t *testing.T) {
	withFakeKey(t)
	svc := &Service{Requester: &fakeCompleter{}, Paths: config.Paths{PlanDir: t.TempDir()}}
	_, err := svc.Plan(context.Background(), "")
	if err != ErrPlanUsage {
		t.Errorf("expected ErrPlanUsage, got %v", err)
	}
}

func TestPrompt_SuccessWithClipboard(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	svc := &Service{Requester: &fakeCompleter{contents: []string{"[ROLE] ...\n"}}, Paths: config.Paths{PromptDir: dir}}
	clip := &fakeClipboard{}
	res, err := svc.Prompt(context.Background(), "buat prompt buat X", clip)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.CopiedBack {
		t.Error("expected CopiedBack=true")
	}
	if clip.set != "[ROLE] ...\n" {
		t.Errorf("clipboard content = %q", clip.set)
	}
}

func TestPrompt_NilClipboardIsSafe(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	svc := &Service{Requester: &fakeCompleter{contents: []string{"content"}}, Paths: config.Paths{PromptDir: dir}}
	res, err := svc.Prompt(context.Background(), "task", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.CopiedBack {
		t.Error("expected CopiedBack=false with nil clipboard")
	}
}

func TestSpec_ProducesStructuredOutput(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	specContent := "[APLIKASI] Todo app\n[FILES]\n- main.py: entry point\n"
	svc := &Service{Requester: &fakeCompleter{contents: []string{specContent}}, Paths: config.Paths{PromptDir: dir}}
	res, err := svc.Spec(context.Background(), "buat todo app", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Content != specContent {
		t.Errorf("Content = %q", res.Content)
	}
	got, _ := os.ReadFile(res.Outfile)
	if string(got) != specContent+"\n" {
		t.Errorf("file content = %q", got)
	}
}

func TestSpec_UsageError(t *testing.T) {
	withFakeKey(t)
	svc := &Service{Requester: &fakeCompleter{}, Paths: config.Paths{PromptDir: t.TempDir()}}
	_, err := svc.Spec(context.Background(), "", nil)
	if err != ErrSpecUsage {
		t.Errorf("expected ErrSpecUsage, got %v", err)
	}
}
