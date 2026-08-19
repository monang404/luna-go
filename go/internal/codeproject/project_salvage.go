package codeproject

import (
	"context"
	"fmt"
	"os"

	"github.com/monang404/luna-go/internal/aiops"
)

// SalvageResult reports what SalvageIfEmpty did.
type SalvageResult struct {
	// DirWasEmpty is false when projectDir already had content --
	// SalvageIfEmpty is then a pure no-op success.
	DirWasEmpty bool
	// HardFailure is true when the directory was empty AND markers had
	// been detected during generation (meaning split should have
	// written files but didn't -- a real failure, not a
	// format-non-compliance case single-file salvage can paper over).
	HardFailure bool
	// SalvagedTo is set when a single-file fallback (main.py copied
	// from the raw log) was written.
	SalvagedTo string
}

// SalvageIfEmpty mirrors _ai_project_salvage_if_empty: if projectDir
// already has content, no-op success. If it's empty AND markers were
// found (split should have written something but the dir is still
// empty -- a genuine failure), report HardFailure=true (the caller
// mirrors `return 1`, i.e. treat this as fatal). If it's empty and NO
// markers were ever found, salvage the entire raw log into a single
// projectDir/main.py, sanitize it, and continue (matches the zsh
// source's own explicit "this may be incomplete" warning contract --
// callers surfacing this result to a user should show SalvagedTo and
// note the same caveat).
func (s *Service) SalvageIfEmpty(ctx context.Context, projectDir, rawLog, logfile string, hasMarkers bool) (SalvageResult, error) {
	entries, err := os.ReadDir(projectDir)
	if err != nil && !os.IsNotExist(err) {
		return SalvageResult{}, err
	}
	if len(entries) > 0 {
		return SalvageResult{DirWasEmpty: false}, nil
	}

	if hasMarkers {
		return SalvageResult{DirWasEmpty: true, HardFailure: true}, fmt.Errorf("codeproject: no files were generated in %s despite markers being present -- check %s", projectDir, logfile)
	}

	mainPy := projectDir + string(os.PathSeparator) + "main.py"
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return SalvageResult{DirWasEmpty: true}, err
	}
	if err := os.WriteFile(mainPy, []byte(rawLog), 0o644); err != nil {
		return SalvageResult{DirWasEmpty: true}, err
	}
	aiops.SanitizePyCode(ctx, s.Runner, s.SanitizeScript, mainPy)

	return SalvageResult{DirWasEmpty: true, SalvagedTo: mainPy}, nil
}
