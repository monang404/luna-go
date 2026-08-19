package workflow

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/monang404/luna-go/internal/config"
)

func TestCommit_Success(t *testing.T) {
	withFakeKey(t)
	runner := &fakeRunner{responses: map[string]runnerResponse{
		"git rev-parse --is-inside-work-tree": {exitCode: 0},
		"git diff --cached":                   {stdout: "diff --git a/f.go b/f.go\n+added line\n"},
		"git diff --cached --stat":            {stdout: " f.go | 1 +\n"},
		"git commit -m feat: add feature":     {exitCode: 0},
	}}
	svc := &Service{Requester: &fakeCompleter{contents: []string{"feat: add feature"}}, Confirm: approveConfirm, Runner: runner, Paths: config.LoadPaths()}
	res, err := svc.Commit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Committed {
		t.Error("expected Committed=true")
	}
	if res.Message != "feat: add feature" {
		t.Errorf("Message = %q", res.Message)
	}
}

func TestCommit_NothingStaged(t *testing.T) {
	withFakeKey(t)
	runner := &fakeRunner{responses: map[string]runnerResponse{
		"git rev-parse --is-inside-work-tree": {exitCode: 0},
		"git diff --cached":                   {stdout: ""},
	}}
	svc := &Service{Requester: &fakeCompleter{contents: []string{"x"}}, Confirm: approveConfirm, Runner: runner}
	_, err := svc.Commit(context.Background())
	if err != ErrNothingStaged {
		t.Errorf("expected ErrNothingStaged, got %v", err)
	}
}

func TestCommit_NotGitRepo(t *testing.T) {
	withFakeKey(t)
	runner := &fakeRunner{responses: map[string]runnerResponse{
		"git rev-parse --is-inside-work-tree": {exitCode: 128, err: nil},
	}}
	svc := &Service{Requester: &fakeCompleter{contents: []string{"x"}}, Confirm: approveConfirm, Runner: runner}
	_, err := svc.Commit(context.Background())
	if err != ErrNotGitRepo {
		t.Errorf("expected ErrNotGitRepo, got %v", err)
	}
}

func TestCommit_Declined(t *testing.T) {
	withFakeKey(t)
	runner := &fakeRunner{responses: map[string]runnerResponse{
		"git rev-parse --is-inside-work-tree": {exitCode: 0},
		"git diff --cached":                   {stdout: "some diff\n"},
		"git diff --cached --stat":            {stdout: "stat\n"},
	}}
	svc := &Service{Requester: &fakeCompleter{contents: []string{"feat: x"}}, Confirm: declineConfirm, Runner: runner}
	_, err := svc.Commit(context.Background())
	if err != ErrCommitDeclined {
		t.Errorf("expected ErrCommitDeclined, got %v", err)
	}
	for _, c := range runner.calls {
		if filepath.Base(c) == "commit" {
			t.Error("git commit must not run when declined")
		}
	}
}

func TestCommit_LargeDiffIsGuarded(t *testing.T) {
	withFakeKey(t)
	big := make([]byte, 20000)
	for i := range big {
		big[i] = 'x'
	}
	runner := &fakeRunner{responses: map[string]runnerResponse{
		"git rev-parse --is-inside-work-tree": {exitCode: 0},
		"git diff --cached":                   {stdout: string(big)},
		"git diff --cached --stat":            {stdout: "stat summary\n"},
		"git commit -m feat: big change":      {exitCode: 0},
	}}
	fc := &fakeCompleter{contents: []string{"feat: big change"}}
	svc := &Service{Requester: fc, Confirm: approveConfirm, Runner: runner}
	_, err := svc.Commit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.lastUser) >= 20000 {
		t.Error("expected the diff sent to the model to be guarded/truncated")
	}
}
