package codeproject

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func newProjectService(t *testing.T, completer *fakeCompleter) *Service {
	t.Helper()
	dir := t.TempDir()
	return &Service{
		Requester: completer,
		Confirm:   approveConfirm,
		Runner:    &fakeRunner{},
		CodeDir:   filepath.Join(dir, "aicode"),
	}
}

func TestProject_HappyPath_MultiFile(t *testing.T) {
	withFakeKey(t)
	svc := newProjectService(t, &fakeCompleter{contents: []string{
		"### FILE: main.py\nif __name__ == '__main__':\n    print('hi')\n",
	}})

	res, err := svc.Project(context.Background(), "myapp", "buat aplikasi hello world")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Generate.HasMarkers {
		t.Error("expected HasMarkers=true")
	}
	if len(res.Split.Written) != 1 {
		t.Fatalf("expected 1 file written, got %d", len(res.Split.Written))
	}
	if _, err := os.Stat(filepath.Join(res.ProjectDir, "main.py")); err != nil {
		t.Errorf("expected main.py to exist: %v", err)
	}
}

func TestProject_UsageError(t *testing.T) {
	withFakeKey(t)
	svc := newProjectService(t, &fakeCompleter{})
	if _, err := svc.Project(context.Background(), "", "desc"); err != ErrProjectUsage {
		t.Errorf("expected ErrProjectUsage for empty name, got %v", err)
	}
	if _, err := svc.Project(context.Background(), "name", ""); err != ErrProjectUsage {
		t.Errorf("expected ErrProjectUsage for empty description, got %v", err)
	}
}

func TestProject_UnsafeName(t *testing.T) {
	withFakeKey(t)
	svc := newProjectService(t, &fakeCompleter{contents: []string{"anything"}})
	cases := []string{"../evil", "foo/bar", "foo..bar", "with space"}
	for _, name := range cases {
		if _, err := svc.Project(context.Background(), name, "desc"); err == nil {
			t.Errorf("expected unsafe-name rejection for %q", name)
		}
	}
}

func TestProject_AllGenerationAttemptsFail(t *testing.T) {
	withFakeKey(t)
	svc := newProjectService(t, &fakeCompleter{err: context.DeadlineExceeded})
	_, err := svc.Project(context.Background(), "myapp", "desc")
	if err != ErrProjectGenFailed {
		t.Errorf("expected ErrProjectGenFailed, got %v", err)
	}
}

func TestProject_NoMarkers_SalvagesSingleFile(t *testing.T) {
	withFakeKey(t)
	svc := newProjectService(t, &fakeCompleter{contents: []string{
		"print('no markers here, just raw code')\n",
	}})
	res, err := svc.Project(context.Background(), "myapp", "desc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Salvage.SalvagedTo == "" {
		t.Fatal("expected salvage to have kicked in")
	}
	if _, err := os.Stat(filepath.Join(res.ProjectDir, "main.py")); err != nil {
		t.Errorf("expected salvaged main.py: %v", err)
	}
}
