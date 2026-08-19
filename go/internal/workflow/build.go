package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/codeproject"
	"github.com/monang404/luna-go/internal/config"
)

// ErrBuildUsage mirrors aibuild's "Usage: luna build [-o nama_folder]
// <deskripsi aplikasi>".
var ErrBuildUsage = errors.New("workflow: usage: app description is required")

// BuildResult is Build's return shape.
type BuildResult struct {
	ProjectName string
	SpecFile    string
	Spec        string
	Project     codeproject.ProjectResult
}

// Build mirrors aibuild([-o name], task): generate a spec (same
// AI_SPEC_SYSPROMPT as Spec, but saved separately, matching the zsh
// source's own duplicated-but-not-shared specfile write -- Spec's own
// output file is NOT reused here), then feed that spec straight into
// internal/codeproject.Project as the project description -- exactly
// how the zsh source's `aiproject "$project_name" "$specfile"` call
// works (aiproject reads $specfile's *content* via _ai_resolve_prompt,
// it doesn't just take the path as a literal string).
//
// projectSvc is the internal/codeproject.Service to delegate the
// generate-project step to; construct it with the same
// Requester/Confirm/Runner as this Service for consistent behavior.
func (s *Service) Build(ctx context.Context, outputName, task string, projectSvc *codeproject.Service) (BuildResult, error) {
	if err := needAnyKey(config.TaskProviderOrderBig); err != nil {
		return BuildResult{}, err
	}
	if task == "" {
		return BuildResult{}, ErrBuildUsage
	}
	projectName := outputName
	if projectName == "" {
		projectName = aiops.Slugify(task, 40)
	}

	if err := os.MkdirAll(s.Paths.PromptDir, 0o755); err != nil {
		return BuildResult{}, err
	}
	res, err := s.Requester.Complete(ctx, SpecSysPrompt, "Deskripsi aplikasi: "+task, config.TaskSmart, config.TaskProviderOrderBig, 0)
	if err != nil || res.Content == "" {
		return BuildResult{}, fmt.Errorf("workflow: [1/2] spec generation failed, [2/2] cancelled: %w", err)
	}
	specfile := filepath.Join(s.Paths.PromptDir, fmt.Sprintf("%s_spec_%s.txt", aiops.Slugify(task, 40), aiops.Timestamp()))
	if err := os.WriteFile(specfile, []byte(res.Content+"\n"), 0o644); err != nil {
		return BuildResult{}, err
	}

	result := BuildResult{ProjectName: projectName, SpecFile: specfile, Spec: res.Content}
	projectRes, err := projectSvc.Project(ctx, projectName, res.Content)
	result.Project = projectRes
	return result, err
}
