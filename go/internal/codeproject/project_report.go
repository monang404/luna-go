package codeproject

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
)

// FinishReport is FinishReport's return shape.
type FinishReport struct {
	Files          []string // basenames under projectDir (matches `ls -la`'s listing intent)
	ImportWarnings []string // "<file> imports '<mod>' but <mod>.py not found"
	Autotest       AutotestResult
}

// fromImportRE mirrors `grep -oP "^from \K[a-zA-Z_]+(?= import)"`.
var fromImportRE = regexp.MustCompile(`(?m)^from\s+([a-zA-Z_]+)\s+import`)

// FinishReport mirrors _ai_project_finish_report(projectDir, logfile,
// prompt, projectName): list the generated files, flag any `from X
// import` referencing a local module X.py that doesn't exist in
// projectDir, then run Autotest. Logging/notification side effects
// (_ai_log, _ai_notify) are the caller's responsibility (not part of
// this package -- see SESSION-54 §2's file list; those helpers live in
// 10-core/60-ui, out of scope here).
func (s *Service) FinishReport(ctx context.Context, projectDir, prompt string) (FinishReport, error) {
	report := FinishReport{}

	entries, err := os.ReadDir(projectDir)
	if err != nil {
		return report, err
	}
	for _, e := range entries {
		report.Files = append(report.Files, e.Name())
	}

	pyFiles, err := findPyFiles(projectDir)
	if err != nil {
		return report, err
	}
	for _, f := range pyFiles {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		for _, m := range fromImportRE.FindAllStringSubmatch(string(data), -1) {
			mod := m[1]
			if _, err := os.Stat(filepath.Join(projectDir, mod+".py")); err != nil {
				report.ImportWarnings = append(report.ImportWarnings,
					filepath.Base(f)+" import '"+mod+"' tapi "+mod+".py tidak ditemukan")
			}
		}
	}

	autotest, err := s.Autotest(ctx, projectDir, prompt)
	report.Autotest = autotest
	return report, err
}
