package codeproject

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/monang404/luna-go/internal/aiops"
)

// Errors returned by Project.
var (
	ErrProjectUsage       = errors.New("codeproject: usage: project name and description are required")
	ErrProjectUnsafeName  = errors.New("codeproject: unsafe project name (no path traversal / simple directory name only)")
	ErrProjectGenFailed   = errors.New("codeproject: all provider attempts failed, no source code was generated")
	ErrProjectSplitFailed = errors.New("codeproject: parsing generated files failed")
)

// unsafeProjectNameRE mirrors the zsh source's `[[ "$project_name" ==
// *[!A-Za-z0-9._-]* || "$project_name" == *..* ]]` guard.
var unsafeProjectNameRE = regexp.MustCompile(`[^A-Za-z0-9._-]`)

func isUnsafeProjectName(name string) bool {
	if name == "" || unsafeProjectNameRE.MatchString(name) {
		return true
	}
	return containsDotDot(name)
}

func containsDotDot(s string) bool {
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '.' && s[i+1] == '.' {
			return true
		}
	}
	return false
}

// ProjectResult is Project's return shape.
type ProjectResult struct {
	ProjectDir string
	Logfile    string
	Generate   GenerateResult
	Split      SplitResult
	Salvage    SalvageResult
	Report     FinishReport
}

// Project mirrors aiproject(name, ...descriptionArgs): the full
// generate -> split -> salvage-if-empty -> report pipeline.
//
// Not ported here (out of SESSION-54's file-list scope, see doc.go):
// battery/budget/data-saver pre-flight checks (_ai_battery_check/
// _ai_budget_check/_ai_data_saver_check, 10-core) and the wake-lock
// acquire/release wrapper (_ai_wakelock_*, 10-core) -- callers that
// need those guards run them before/around calling Project.
func (s *Service) Project(ctx context.Context, name, prompt string) (ProjectResult, error) {
	if err := needAnyKeyBig(); err != nil {
		return ProjectResult{}, err
	}
	if name == "" || prompt == "" {
		return ProjectResult{}, ErrProjectUsage
	}
	if isUnsafeProjectName(name) {
		return ProjectResult{}, fmt.Errorf("%w: %q", ErrProjectUnsafeName, name)
	}

	projectDir := filepath.Join(s.CodeDir, name)
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return ProjectResult{}, err
	}
	logfile := filepath.Join(filepath.Dir(s.CodeDir), "logs", name+"_"+aiops.Timestamp()+".txt")
	if err := os.MkdirAll(filepath.Dir(logfile), 0o755); err != nil {
		return ProjectResult{}, err
	}

	result := ProjectResult{ProjectDir: projectDir, Logfile: logfile}

	result.Generate = s.GenerateProject(ctx, prompt, logfile, 2)

	split, err := s.SplitFiles(ctx, projectDir, result.Generate.RawLog, result.Generate.HasMarkers)
	result.Split = split
	if err != nil {
		return result, fmt.Errorf("%w: %v", ErrProjectSplitFailed, err)
	}

	if !result.Generate.GenerationOK {
		return result, ErrProjectGenFailed
	}

	salvage, err := s.SalvageIfEmpty(ctx, projectDir, result.Generate.RawLog, logfile, result.Generate.HasMarkers)
	result.Salvage = salvage
	if err != nil {
		return result, err
	}

	report, err := s.FinishReport(ctx, projectDir, prompt)
	result.Report = report
	return result, err
}
