package codeproject

import "strings"

// splitLines/joinLines/containsFence back stripFences: `grep -v
// '```'` drops any line CONTAINING a triple-backtick anywhere (not
// just as a prefix -- unlike filepatch.stripCodeFences, which mirrors
// aipatch's own `grep -vE '^```'` anchored variant. Both zsh source
// files really do use slightly different grep patterns; this
// difference is preserved intentionally, not a bug).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n")
}

func containsFence(line string) bool {
	return strings.Contains(line, "```")
}
