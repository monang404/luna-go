package codeproject

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/monang404/luna-go/internal/aiops"
)

// SplitResult is SplitFiles' return shape.
type SplitResult struct {
	Written  []string // paths actually written, in first-seen order
	Warnings []string // rejected-filename warnings (unsafe/escaping)
}

// SplitFiles mirrors _ai_project_split_files: parse "### FILE: <name>"
// markers out of rawLog and write each chunk under projectDir. This is
// a direct Go re-implementation of the embedded python3 heredoc in the
// zsh source (not a shell-out to it) -- same containment rules:
//   - an absolute path, or any path component that is "", ".", or
//     ".." is rejected outright (never even resolved).
//   - after resolving relative to projectDir, the result must be
//     projectDir itself or a path underneath it (symlink/`..`-escape
//     defense), matching the python source's `target != root and root
//     not in target.parents` check.
//
// Every accepted file is then run through aiops.SanitizePyCode (the
// zsh source's own post-split "auto-repair layer" loop over
// "$project_dir"/**/*.py). hasMarkers=false is a no-op success (matches
// `if [ "$has_markers" -eq 1 ]; then ... fi` guarding the whole
// parse+write step).
func (s *Service) SplitFiles(ctx context.Context, projectDir, rawLog string, hasMarkers bool) (SplitResult, error) {
	result := SplitResult{}
	if !hasMarkers {
		return result, nil
	}

	root, err := filepath.Abs(projectDir)
	if err != nil {
		return result, err
	}

	chunks := map[string][]string{}
	var order []string

	var current string
	haveCurrent := false
	scanner := bufio.NewScanner(strings.NewReader(rawLog))
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, fileMarkerPrefix) {
			raw := strings.TrimSpace(line[len(fileMarkerPrefix):])
			target, ok := safeJoin(root, raw)
			if !ok {
				result.Warnings = append(result.Warnings, fmt.Sprintf("rejected unsafe/escaping LUNA filename: %s", raw))
				haveCurrent = false
				continue
			}
			current = target
			haveCurrent = true
			if _, seen := chunks[current]; !seen {
				chunks[current] = nil
				order = append(order, current)
			}
			continue
		}
		if haveCurrent {
			chunks[current] = append(chunks[current], line)
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}

	for _, target := range order {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return result, err
		}
		content := strings.Join(chunks[target], "\n")
		if len(chunks[target]) > 0 {
			content += "\n"
		}
		if err := os.WriteFile(target, []byte(content), 0o644); err != nil {
			return result, err
		}
		result.Written = append(result.Written, target)
	}

	for _, f := range result.Written {
		if strings.HasSuffix(f, ".py") {
			aiops.SanitizePyCode(ctx, s.Runner, s.SanitizeScript, f)
		}
	}

	return result, nil
}

// safeJoin mirrors the embedded python splitter's containment check: raw
// must not be absolute, and no path component may be "", ".", or "..";
// the resolved join of root+raw must be root itself or strictly beneath
// it.
func safeJoin(root, raw string) (string, bool) {
	if raw == "" || filepath.IsAbs(raw) {
		return "", false
	}
	parts := strings.Split(filepath.ToSlash(raw), "/")
	for _, p := range parts {
		if p == "" || p == "." || p == ".." {
			return "", false
		}
	}
	target := filepath.Join(append([]string{root}, parts...)...)
	target, err := filepath.Abs(target)
	if err != nil {
		return "", false
	}
	if target != root && !strings.HasPrefix(target, root+string(filepath.Separator)) {
		return "", false
	}
	return target, true
}
