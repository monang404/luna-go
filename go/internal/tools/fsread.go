package tools

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/permission"
)

// This file ports 30-luna/05-tools/10-tool_fs_read.zsh
// (_ai_tool_read_file, _ai_tool_list_dir, _ai_tool_count_lines) plus
// 30-luna/05-tools/15-tool_search.zsh (_ai_tool_grep_search,
// _ai_tool_glob_search). The search tools' source file is not listed in
// this session's own source_zsh_files (only 10/20/25 are), but the
// session's objective and scope.include both explicitly name
// grep_search/glob_search as two of "the ten" tools this session ports,
// and its own why_not_less rationale groups every path-bearing read
// tool together for the same reason grep_search/glob_search share
// ExtractPath/pathFieldTools wiring with the rest of this file's tools
// (args.go, SESSION-43) -- so they're ported here rather than left for
// SESSION-48, which has no read-family tools of its own. See CHANGELOG
// SESSION-47 for the full note on this deviation from source_zsh_files.
//
// Deliberately NOT ported: the aiindex (46-index.zsh) fast-path
// lookaside both _ai_tool_grep_search and _ai_tool_glob_search try
// before falling back to rg/fd/find -- 46-index.zsh is not assigned to
// any session's source_zsh_files anywhere in docs/execution_sessions/,
// so the JSON index format it reads from has no Go port to call into
// yet. Both tools here go straight to the always-correct fallback path
// (rg/grep, fd/find) the zsh source itself falls back to whenever the
// index is stale, missing, or jq is unavailable -- functionally
// complete, just without that optional optimization.

// ReadFileTool implements _ai_tool_read_file: read a file's content
// with 1-based line numbers, optionally windowed by offset/limit.
type ReadFileTool struct{}

func (ReadFileTool) Name() string                      { return "read_file" }
func (ReadFileTool) Capability() permission.Capability { return Registry["read_file"].Capability }

func (ReadFileTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	obj := mustObject(args)
	fsPath := ExtractPath(args)
	offsetStr := stringField(obj, "offset")
	limitStr := stringField(obj, "limit")
	// Schema validation (schema.go) already guarantees offset/limit, if
	// present, are JSON numbers -- but the field may also arrive as a
	// number rather than a string here, so fall back to numeric fields
	// directly when the string coercion above found nothing.
	if offsetStr == "" {
		offsetStr = numberFieldAsString(obj, "offset")
	}
	if limitStr == "" {
		limitStr = numberFieldAsString(obj, "limit")
	}

	if fsPath == "" {
		return Result{}, fmt.Errorf("ERROR: read_file membutuhkan args.path (string non-empty)")
	}
	info, err := os.Stat(fsPath)
	if err != nil || !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("ERROR: file gak ketemu: %s", fsPath)
	}
	if IsSecretFile(fsPath) {
		return Result{}, fmt.Errorf("ERROR: [%s] kelihatan kayak file secrets. Ditolak.", fsPath)
	}
	if IsBinaryFile(fsPath) {
		return Result{}, fmt.Errorf("ERROR: [%s] kelihatan file biner. Ditolak.", fsPath)
	}

	f, err := os.Open(fsPath)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membaca %s: %w", fsPath, err)
	}
	defer f.Close()

	offset, offsetOK := parsePositiveInt(offsetStr)
	limit, limitOK := parsePositiveInt(limitStr)

	var b strings.Builder
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	lineNo := 0
	// FIX BUG-5 equivalent: no shelling out to sed/awk with
	// interpolated offset/limit -- the windowing is plain Go arithmetic
	// over a line scanner instead.
	maxLines := config.LoadLimits().FileMaxChars
	// NOTE (preserved zsh quirk, ported "apa adanya" per this session's
	// own port-values-as-is convention): the zsh source names this
	// limit AI_FILE_MAX_CHARS but actually uses it as an awk `NR==N`
	// cutoff -- i.e. a *line* count, not a character count. That exact
	// behavior (not the name) is what's reproduced here.
	printed := 0
	for sc.Scan() {
		lineNo++
		if offsetOK && lineNo < offset {
			continue
		}
		if offsetOK && limitOK && lineNo > offset+limit-1 {
			break
		}
		fmt.Fprintf(&b, "%4d  %s\n", lineNo, sc.Text())
		printed++
		if printed >= maxLines {
			break
		}
	}
	if err := sc.Err(); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membaca %s: %w", fsPath, err)
	}
	return Result{Output: strings.TrimSuffix(b.String(), "\n")}, nil
}

// ListDirTool implements _ai_tool_list_dir: shell out to `eza`/`ls -lah`
// (matching the zsh source's own whence-based lookup order), capped at
// the first 50 lines of output.
type ListDirTool struct{}

func (ListDirTool) Name() string                      { return "list_dir" }
func (ListDirTool) Capability() permission.Capability { return Registry["list_dir"].Capability }

func (ListDirTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	fsPath := ExtractPath(args)
	if fsPath == "" {
		fsPath = "."
	}
	info, err := os.Stat(fsPath)
	if err != nil || !info.IsDir() {
		return Result{}, fmt.Errorf("ERROR: direktori gak ketemu (%s)", fsPath)
	}

	lsCmd := ""
	if p, err := exec.LookPath("eza"); err == nil {
		lsCmd = p
	} else if p, err := exec.LookPath("ls"); err == nil {
		lsCmd = p
	} else if _, err := os.Stat("/bin/ls"); err == nil {
		lsCmd = "/bin/ls"
	} else if _, err := os.Stat("/usr/bin/ls"); err == nil {
		lsCmd = "/usr/bin/ls"
	} else {
		return Result{}, fmt.Errorf("ERROR: executable 'ls' atau 'eza' gak ketemu di PATH")
	}

	out, _ := exec.Command(lsCmd, "-lah", fsPath).CombinedOutput()
	return Result{Output: firstNLines(string(out), 50)}, nil
}

// CountLinesTool implements _ai_tool_count_lines: total line count,
// plus an optional pattern-match occurrence count.
type CountLinesTool struct{}

func (CountLinesTool) Name() string                      { return "count_lines" }
func (CountLinesTool) Capability() permission.Capability { return Registry["count_lines"].Capability }

func (CountLinesTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	fsPath := ExtractPath(args)
	pattern := ExtractField(args, "pattern")

	if fsPath == "" {
		return Result{}, fmt.Errorf("ERROR: count_lines membutuhkan args.path (string non-empty)")
	}
	info, err := os.Stat(fsPath)
	if err != nil || !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("ERROR: file gak ketemu: %s", fsPath)
	}
	if IsSecretFile(fsPath) {
		return Result{}, fmt.Errorf("ERROR: [%s] kelihatan kayak file secrets. Ditolak.", fsPath)
	}

	data, err := os.ReadFile(fsPath)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membaca %s: %w", fsPath, err)
	}
	total := bytes.Count(data, []byte("\n"))

	if pattern != "" {
		matches := 0
		if grepBin, err := exec.LookPath("grep"); err == nil {
			out, _ := exec.Command(grepBin, "-c", "-e", pattern, fsPath).Output()
			matches, _ = strconv.Atoi(strings.TrimSpace(string(out)))
		}
		return Result{Output: fmt.Sprintf("File: %s | Total baris: %d | Kemunculan '%s': %d", fsPath, total, pattern, matches)}, nil
	}
	return Result{Output: fmt.Sprintf("File: %s | Total baris: %d", fsPath, total)}, nil
}

// GrepSearchTool implements _ai_tool_grep_search's fallback path
// (rg preferred, else find+grep / grep -rn), each capped at
// config.Limits.GrepMaxResults lines.
type GrepSearchTool struct{}

func (GrepSearchTool) Name() string                      { return "grep_search" }
func (GrepSearchTool) Capability() permission.Capability { return Registry["grep_search"].Capability }

func (GrepSearchTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	pattern := ExtractField(args, "pattern")
	path := ExtractPath(args)
	glob := ExtractField(args, "glob")

	if pattern == "" {
		return Result{}, fmt.Errorf("ERROR: grep_search membutuhkan args.pattern (string non-empty)")
	}
	if path == "" {
		path = "."
	}
	maxResults := config.LoadLimits().GrepMaxResults

	var cmd *exec.Cmd
	if rgBin, err := exec.LookPath("rg"); err == nil {
		if glob != "" {
			cmd = exec.Command(rgBin, "-n", "-g", glob, "-e", pattern, path)
		} else {
			cmd = exec.Command(rgBin, "-n", "-e", pattern, path)
		}
		out, _ := cmd.Output() // rg exits 1 on "no matches" -- not an error.
		return Result{Output: firstNLines(string(out), maxResults)}, nil
	}
	if glob != "" {
		if findBin, err := exec.LookPath("find"); err == nil {
			out, _ := exec.Command(findBin, path, "-name", glob, "-type", "f", "-exec", "grep", "-Hn", "-e", pattern, "{}", "+").Output()
			return Result{Output: firstNLines(string(out), maxResults)}, nil
		}
	}
	grepBin, err := exec.LookPath("grep")
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: 'rg'/'grep' gak ketemu di PATH")
	}
	out, _ := exec.Command(grepBin, "-rn", "-e", pattern, path).Output()
	return Result{Output: firstNLines(string(out), maxResults)}, nil
}

// GlobSearchTool implements _ai_tool_glob_search's fallback path (fd
// preferred, else `find . -name "*pattern*"`), capped at 100 results
// (the zsh source's own literal `_ai_head_n 100`, not the configurable
// grep-results limit).
type GlobSearchTool struct{}

func (GlobSearchTool) Name() string                      { return "glob_search" }
func (GlobSearchTool) Capability() permission.Capability { return Registry["glob_search"].Capability }

func (GlobSearchTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	pattern := ExtractField(args, "pattern")
	if pattern == "" {
		return Result{}, fmt.Errorf("ERROR: glob_search membutuhkan args.pattern (string non-empty)")
	}
	if fdBin, err := exec.LookPath("fd"); err == nil {
		out, _ := exec.Command(fdBin, pattern).Output()
		return Result{Output: firstNLines(string(out), 100)}, nil
	}
	out, _ := exec.Command("find", ".", "-name", "*"+pattern+"*").Output()
	return Result{Output: firstNLines(string(out), 100)}, nil
}

// --- shared helpers ---

func mustObject(args json.RawMessage) map[string]interface{} {
	var obj map[string]interface{}
	_ = json.Unmarshal(args, &obj)
	return obj
}

func stringField(obj map[string]interface{}, field string) string {
	if s, ok := obj[field].(string); ok {
		return s
	}
	return ""
}

func numberFieldAsString(obj map[string]interface{}, field string) string {
	if n, ok := obj[field].(float64); ok {
		return strconv.FormatFloat(n, 'f', -1, 64)
	}
	return ""
}

func boolField(obj map[string]interface{}, field string) bool {
	if b, ok := obj[field].(bool); ok {
		return b
	}
	return false
}

func parsePositiveInt(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// firstNLines mirrors _ai_head_n: the first n lines of s (s may or may
// not end in a trailing newline; the result never adds one that wasn't
// implied by the input).
func firstNLines(s string, n int) string {
	if s == "" {
		return ""
	}
	lines := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
