package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/permission"
)

// This file ports 30-luna/05-tools/40-tool_git.zsh (_ai_tool_git_status,
// _ai_tool_git_diff) -- SESSION-48's first of four target_go_files. Both
// tools are thin, readonly `git` wrappers run against the process's own
// working directory (neither the zsh source nor this port ever `cd`s
// anywhere for these two -- unlike exec_process/run_test, there is no
// args.cwd/args.path-as-execution-root concept here; git_diff's optional
// path is a `git diff -- <path>` pathspec argument, not a directory to
// run inside).

// gitAvailable / insideGitWorkTree mirror the two guard checks both
// _ai_tool_git_status and _ai_tool_git_diff run before doing anything
// else: `command -v git` and `git rev-parse --is-inside-work-tree`.
func gitAvailable() bool {
	_, err := exec.LookPath("git")
	return err == nil
}

func insideGitWorkTree() bool {
	out, err := exec.Command("git", "rev-parse", "--is-inside-work-tree").Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
}

// GitStatusTool implements _ai_tool_git_status: current branch name plus
// `git status --short -b`, capped at the first 100 lines (_ai_head_n 100
// in the zsh source).
type GitStatusTool struct{}

func (GitStatusTool) Name() string                      { return "git_status" }
func (GitStatusTool) Capability() permission.Capability { return Registry["git_status"].Capability }

func (GitStatusTool) Execute(_ context.Context, _ json.RawMessage) (Result, error) {
	if !gitAvailable() {
		return Result{}, fmt.Errorf("ERROR: git gak ketemu di PATH")
	}
	if !insideGitWorkTree() {
		return Result{}, fmt.Errorf("ERROR: direktori saat ini bukan git repo")
	}

	branchOut, _ := exec.Command("git", "branch", "--show-current").Output()
	branch := strings.TrimSpace(string(branchOut))

	statusOut, _ := exec.Command("git", "status", "--short", "-b").CombinedOutput()
	body := firstNLines(string(statusOut), 100)

	return Result{Output: fmt.Sprintf("Branch: %s\n%s", branch, body)}, nil
}

// GitDiffTool implements _ai_tool_git_diff: `git diff` (optionally
// scoped to args.path as a pathspec), capped at
// config.Limits.GitDiffMaxChars *characters* (_ai_head_c, not
// _ai_head_n -- a diff is capped by size, not line count, since a single
// changed line in a minified file can already blow well past any
// reasonable line budget).
type GitDiffTool struct{}

func (GitDiffTool) Name() string                      { return "git_diff" }
func (GitDiffTool) Capability() permission.Capability { return Registry["git_diff"].Capability }

func (GitDiffTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	fsPath := ExtractPath(args)

	if !gitAvailable() {
		return Result{}, fmt.Errorf("ERROR: git gak ketemu di PATH")
	}
	if !insideGitWorkTree() {
		return Result{}, fmt.Errorf("ERROR: direktori saat ini bukan git repo")
	}

	var cmd *exec.Cmd
	if fsPath != "" {
		cmd = exec.Command("git", "diff", "--", fsPath)
	} else {
		cmd = exec.Command("git", "diff")
	}
	out, _ := cmd.CombinedOutput()

	if len(strings.TrimSpace(string(out))) == 0 {
		return Result{Output: "OK: gak ada perubahan (git diff kosong)"}, nil
	}

	maxChars := config.LoadLimits().GitDiffMaxChars
	return Result{Output: firstNChars(string(out), maxChars)}, nil
}
