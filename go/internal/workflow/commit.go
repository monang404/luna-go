package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
)

// Errors returned by Commit.
var (
	ErrNoProvider     = errors.New("workflow: no LUNA provider configured")
	ErrNotGitRepo     = errors.New("workflow: not a git repository")
	ErrNothingStaged  = errors.New("workflow: nothing staged (git add first)")
	ErrCommitDeclined = errors.New("workflow: commit declined by user")
	ErrCommitTimedOut = errors.New("workflow: commit confirmation timed out")
)

// Service bundles the shared dependencies every function in this
// package needs.
type Service struct {
	Requester aiops.Completer
	Confirm   aiops.ConfirmFunc
	Runner    aiops.CommandRunner
	Paths     config.Paths
}

// NewService builds a Service from the current environment's config.
func NewService(requester aiops.Completer, confirm aiops.ConfirmFunc, runner aiops.CommandRunner) *Service {
	return &Service{Requester: requester, Confirm: confirm, Runner: runner, Paths: config.LoadPaths()}
}

func needAnyKey(order []string) error {
	if len(config.ActiveProviders(order)) == 0 {
		return ErrNoProvider
	}
	return nil
}

func (s *Service) isGitRepo(ctx context.Context) bool {
	if s.Runner == nil {
		return false
	}
	_, _, code, err := s.Runner.Run(ctx, "git", "rev-parse", "--is-inside-work-tree")
	return err == nil && code == 0
}

// CommitResult is Commit's return shape.
type CommitResult struct {
	Message   string
	Committed bool
	GitOutput string
}

// Commit mirrors aicommit(): generate a one-line conventional-commit
// message from the staged diff, ask for confirmation, then `git commit
// -m <msg>` via s.Runner.
func (s *Service) Commit(ctx context.Context) (CommitResult, error) {
	if err := needAnyKey(config.TaskProviderOrderFast); err != nil {
		return CommitResult{}, err
	}
	if s.Runner == nil || !s.isGitRepo(ctx) {
		return CommitResult{}, ErrNotGitRepo
	}

	diff, _, _, err := s.Runner.Run(ctx, "git", "diff", "--cached")
	if err != nil {
		return CommitResult{}, err
	}
	if strings.TrimSpace(diff) == "" {
		return CommitResult{}, ErrNothingStaged
	}
	diffstat, _, _, _ := s.Runner.Run(ctx, "git", "diff", "--cached", "--stat")
	guarded := aiops.GuardDiff(diff, diffstat)

	const sysprompt = `Buat SATU baris pesan commit git conventional style (feat:/fix:/chore:/refactor:/docs:), bahasa Inggris, tanpa tanda kutip, tanpa penjelasan tambahan.`
	res, err := s.Requester.Complete(ctx, sysprompt, guarded, config.TaskFast, config.TaskProviderOrderFast, 0)
	if err != nil || res.Content == "" {
		return CommitResult{}, fmt.Errorf("workflow: commit message generation failed: %w", err)
	}
	msg := firstLine(res.Content)

	decision, err := s.Confirm(ctx, "Commit dengan pesan ini?")
	if err != nil {
		return CommitResult{Message: msg}, err
	}
	switch decision {
	case aiops.Approved:
	case aiops.TimedOut:
		return CommitResult{Message: msg}, ErrCommitTimedOut
	default:
		return CommitResult{Message: msg}, ErrCommitDeclined
	}

	stdout, stderr, code, err := s.Runner.Run(ctx, "git", "commit", "-m", msg)
	if err != nil || code != 0 {
		return CommitResult{Message: msg, GitOutput: stdout + stderr}, fmt.Errorf("workflow: git commit failed: %s", stdout+stderr)
	}
	return CommitResult{Message: msg, Committed: true, GitOutput: stdout + stderr}, nil
}

// firstLine mirrors `_ai_head_n 1`: the first line of s.
func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}
