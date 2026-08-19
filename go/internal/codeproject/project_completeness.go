package codeproject

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CompletenessResult is CheckCompleteness's return shape -- purely
// advisory warnings, never a hard failure (matching the zsh source,
// which only ever echoes WARNING lines and always returns 0).
type CompletenessResult struct {
	MissingFiles []string
	NoMainGuard  bool // entry exists but has no `__main__` block
}

// specFileRE mirrors the zsh source's `grep -oE '^-[[:space:]]*[A-Za-z0-9_./]+\.py'`
// applied to a `[FILES]` spec section: a "- name.py" bullet line.
var specFileRE = regexp.MustCompile(`(?m)^-\s*([A-Za-z0-9_./]+\.py)`)

// CheckCompleteness mirrors _ai_project_check_completeness(projectDir,
// taskDesc, entry): if taskDesc looks like a structured `luna spec`
// output (contains "[FILES]"), every ".py" bullet listed there must
// exist under projectDir (else it's flagged as likely-truncated
// generation). Separately, if entry exists and has no `__main__` guard,
// that's flagged too (a soft signal, not proof of an incomplete app --
// matches the zsh source's own caveat).
func (s *Service) CheckCompleteness(projectDir, taskDesc, entry string) CompletenessResult {
	result := CompletenessResult{}

	if strings.Contains(taskDesc, "[FILES]") {
		for _, m := range specFileRE.FindAllStringSubmatch(taskDesc, -1) {
			expected := m[1]
			if _, err := os.Stat(filepath.Join(projectDir, expected)); err != nil {
				result.MissingFiles = append(result.MissingFiles, expected)
			}
		}
	}

	if data, err := os.ReadFile(entry); err == nil {
		if !strings.Contains(string(data), "__main__") {
			result.NoMainGuard = true
		}
	}

	return result
}
