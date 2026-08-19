package workflow

import (
	"context"
	"fmt"
	"strings"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
)

// reviewSysPrompt mirrors _ai_review_diff_core's inline sysprompt.
const reviewSysPrompt = `Kamu senior code reviewer. Review diff git berikut dan kasih feedback terstruktur dengan bagian: 1) Bug/error potensial. 2) Masalah security. 3) Saran perbaikan style/readability. 4) Hal yang udah bagus (singkat aja). Bahasa Indonesia, to the point, pakai penomoran, tanpa markdown backtick.`

// ReviewDiffCore mirrors _ai_review_diff_core(diff, diffstat): the pure
// guard-diff -> build-prompt -> ask-the-model step, reusable by any
// caller with a diff in hand (aireview's CLI-invoked git detection
// lives in Review below; a future aiagent-driven caller could call this
// directly with its own diff source, exactly as the zsh source's own
// comment anticipates).
func (s *Service) ReviewDiffCore(ctx context.Context, diff, diffstat string) (string, error) {
	guarded := aiops.GuardDiff(diff, diffstat)
	res, err := s.Requester.Complete(ctx, reviewSysPrompt, guarded, config.TaskSmart, config.TaskProviderOrder, 0)
	if err != nil {
		return "", err
	}
	return res.Content, nil
}

// ReviewResult is Review's return shape.
type ReviewResult struct {
	Review    string
	WasStaged bool
}

// Review mirrors aireview(): review the staged diff, falling back to
// the unstaged diff if nothing is staged.
func (s *Service) Review(ctx context.Context) (ReviewResult, error) {
	if err := needAnyKey(config.TaskProviderOrder); err != nil {
		return ReviewResult{}, err
	}
	if s.Runner == nil || !s.isGitRepo(ctx) {
		return ReviewResult{}, ErrNotGitRepo
	}

	diff, _, _, _ := s.Runner.Run(ctx, "git", "diff", "--cached")
	wasStaged := true
	if strings.TrimSpace(diff) == "" {
		diff, _, _, _ = s.Runner.Run(ctx, "git", "diff")
		wasStaged = false
	}
	if strings.TrimSpace(diff) == "" {
		return ReviewResult{}, fmt.Errorf("workflow: nothing to review (no staged or unstaged changes)")
	}

	var diffstat string
	if wasStaged {
		diffstat, _, _, _ = s.Runner.Run(ctx, "git", "diff", "--cached", "--stat")
	} else {
		diffstat, _, _, _ = s.Runner.Run(ctx, "git", "diff", "--stat")
	}

	review, err := s.ReviewDiffCore(ctx, diff, diffstat)
	if err != nil {
		return ReviewResult{WasStaged: wasStaged}, err
	}
	return ReviewResult{Review: review, WasStaged: wasStaged}, nil
}
