package codeproject

import (
	"context"
	"errors"
	"fmt"
)

// ErrRunUsage mirrors airun's "Usage: airun <file.py>" message.
var ErrRunUsage = errors.New("codeproject: usage: file is required")

// RunResult is Run's return shape.
type RunResult struct {
	Success  bool
	Output   string
	ExitCode int
	// FixAttempts is how many auto-fix cycles were run (0, 1, or 2).
	FixAttempts int
}

// Run mirrors airun(file): execute a Python file (via s.Runner, so
// tests never actually run untrusted generated code); on a non-zero
// exit, ask Fix to generate "<file>.fixed" (inspectOnly=true, matching
// `aifix --inspect`) and apply it via FixApply's own confirm gate, then
// retry -- up to 2 auto-fix cycles, matching the zsh source's fixed
// `tries < 2` loop. The script is executed at most once per loop
// iteration (never twice to "check" then "show" output), matching the
// zsh source's own v-fix comment about eliminating double-execution.
func (s *Service) Run(ctx context.Context, file string) (RunResult, error) {
	if file == "" {
		return RunResult{}, ErrRunUsage
	}
	if s.Runner == nil {
		return RunResult{}, errors.New("codeproject: no CommandRunner configured")
	}

	var stdout, stderr string
	var exitCode int
	tries := 0
	for tries < 2 {
		out, errOut, code, runErr := s.Runner.Run(ctx, "python3", file)
		stdout, stderr, exitCode = out, errOut, code
		combined := stdout + stderr
		if runErr == nil && exitCode == 0 {
			return RunResult{Success: true, Output: combined, ExitCode: 0, FixAttempts: tries}, nil
		}

		fixRes, fixErr := s.Fix(ctx, file, combined, true)
		if fixErr != nil {
			return RunResult{Success: false, Output: combined, ExitCode: exitCode, FixAttempts: tries}, fixErr
		}
		applyRes, applyErr := s.FixApply(ctx, file, fmt.Sprintf("%s: airun auto-fix percobaan %d/2", file, tries+1))
		_ = fixRes
		if applyErr != nil {
			return RunResult{Success: false, Output: combined, ExitCode: exitCode, FixAttempts: tries + 1}, applyErr
		}
		_ = applyRes
		tries++
	}

	return RunResult{Success: false, Output: stdout + stderr, ExitCode: exitCode, FixAttempts: tries}, fmt.Errorf("codeproject: %s still failing after 2 auto-fix attempts (backups at %s.bak.*)", file, file)
}
