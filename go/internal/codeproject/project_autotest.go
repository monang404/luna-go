package codeproject

import (
	"context"
	"io/fs"
	"path/filepath"
)

// findPyFiles recursively finds every *.py file under root, matching
// the zsh source's `"$project_dir"/**/*.py(N)` glob (recursive, silent
// on a nonexistent/unreadable dir).
func findPyFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(d.Name()) == ".py" {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, nil // matches glob's silent empty-result on missing dir
	}
	return out, nil
}

// AutotestResult is Autotest's return shape.
type AutotestResult struct {
	SyntaxOK     bool
	FailedFiles  map[string]string // path -> compiler error output
	Completeness CompletenessResult
}

// Autotest mirrors _ai_project_autotest(projectDir, taskDesc):
// syntax-check every .py file under projectDir via `python3 -m
// py_compile` (through s.Runner, so tests never execute untrusted code)
// and, only if syntax is clean, run CheckCompleteness against
// projectDir/main.py. Runtime execution of generated code is
// deliberately NOT performed, matching the zsh source's own comment
// ("Generated projects are syntax-checked only... generated source is
// untrusted code").
func (s *Service) Autotest(ctx context.Context, projectDir, taskDesc string) (AutotestResult, error) {
	result := AutotestResult{SyntaxOK: true, FailedFiles: map[string]string{}}

	pyFiles, err := findPyFiles(projectDir)
	if err != nil {
		return result, err
	}
	for _, f := range pyFiles {
		if s.Runner == nil {
			continue
		}
		_, stderr, code, runErr := s.Runner.Run(ctx, "python3", "-m", "py_compile", f)
		if runErr != nil || code != 0 || stderr != "" {
			result.SyntaxOK = false
			result.FailedFiles[f] = stderr
		}
	}
	if !result.SyntaxOK {
		return result, nil
	}

	entry := filepath.Join(projectDir, "main.py")
	result.Completeness = s.CheckCompleteness(projectDir, taskDesc, entry)
	return result, nil
}
