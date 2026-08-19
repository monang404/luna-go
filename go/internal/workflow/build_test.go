package workflow

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/monang404/luna-go/internal/codeproject"
	"github.com/monang404/luna-go/internal/config"
)

func TestBuild_ComposesSpecAndProject(t *testing.T) {
	withFakeKey(t)
	promptDir := t.TempDir()
	codeDir := t.TempDir()

	specContent := "[APLIKASI] Todo app\n[FILES]\n- main.py: entry point\n"
	fc := &fakeCompleter{contents: []string{
		specContent, // Build's own spec-generation call
		"### FILE: main.py\nif __name__ == '__main__':\n    print('todo')\n", // codeproject.Project's generate call
	}}

	svc := &Service{Requester: fc, Confirm: approveConfirm, Runner: &fakeRunner{}, Paths: config.Paths{PromptDir: promptDir}}
	projectSvc := &codeproject.Service{Requester: fc, Confirm: approveConfirm, Runner: &fakeRunner{}, CodeDir: codeDir}

	res, err := svc.Build(context.Background(), "todoapp", "buat aplikasi todo list", projectSvc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Spec != specContent {
		t.Errorf("Spec = %q", res.Spec)
	}
	if _, err := os.Stat(res.SpecFile); err != nil {
		t.Errorf("expected spec file to exist: %v", err)
	}
	if len(res.Project.Split.Written) != 1 {
		t.Fatalf("expected the project step to write 1 file, got %d", len(res.Project.Split.Written))
	}
	if filepath.Base(res.Project.ProjectDir) != "todoapp" {
		t.Errorf("expected project dir named 'todoapp', got %s", res.Project.ProjectDir)
	}
}

func TestBuild_SpecFailureCancelsProjectStep(t *testing.T) {
	withFakeKey(t)
	promptDir := t.TempDir()
	fc := &fakeCompleter{err: context.DeadlineExceeded}
	svc := &Service{Requester: fc, Confirm: approveConfirm, Runner: &fakeRunner{}, Paths: config.Paths{PromptDir: promptDir}}
	projectSvc := &codeproject.Service{Requester: fc, Confirm: approveConfirm, Runner: &fakeRunner{}, CodeDir: t.TempDir()}

	_, err := svc.Build(context.Background(), "app", "desc", projectSvc)
	if err == nil {
		t.Fatal("expected an error when spec generation fails")
	}
}

func TestBuild_UsageError(t *testing.T) {
	withFakeKey(t)
	svc := &Service{Requester: &fakeCompleter{}, Paths: config.Paths{PromptDir: t.TempDir()}}
	_, err := svc.Build(context.Background(), "", "", nil)
	if err != ErrBuildUsage {
		t.Errorf("expected ErrBuildUsage, got %v", err)
	}
}

func TestBuild_AutoDerivesProjectNameFromTask(t *testing.T) {
	withFakeKey(t)
	promptDir := t.TempDir()
	codeDir := t.TempDir()
	fc := &fakeCompleter{contents: []string{"[APLIKASI] x\n", "### FILE: main.py\nprint(1)\n"}}
	svc := &Service{Requester: fc, Confirm: approveConfirm, Runner: &fakeRunner{}, Paths: config.Paths{PromptDir: promptDir}}
	projectSvc := &codeproject.Service{Requester: fc, Confirm: approveConfirm, Runner: &fakeRunner{}, CodeDir: codeDir}

	res, err := svc.Build(context.Background(), "", "Buat Kalkulator Sederhana", projectSvc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.ProjectName == "" {
		t.Fatal("expected an auto-derived project name")
	}
}
