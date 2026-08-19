package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// chdirTemp changes the process's working directory to dir for the
// duration of the test, restoring the original on cleanup. GitStatusTool
// / GitDiffTool operate on the process cwd (like their zsh source, which
// never `cd`s anywhere for these two -- see git.go's own doc comment),
// so exercising them against a fixture repo means actually chdir-ing
// into it.
func chdirTemp(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(orig); err != nil {
			t.Fatalf("os.Chdir(restore %s): %v", orig, err)
		}
	})
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// newFixtureRepo builds a small git repo with one committed file, then
// modifies it (leaving an unstaged change) and adds an untracked file --
// enough surface for both git_status and git_diff to have something real
// to report on.
func newFixtureRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "checkout", "-q", "-b", "main")

	committed := filepath.Join(dir, "committed.txt")
	if err := os.WriteFile(committed, []byte("line one\nline two\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGit(t, dir, "add", "committed.txt")
	runGit(t, dir, "commit", "-q", "-m", "initial commit")

	// Uncommitted modification.
	if err := os.WriteFile(committed, []byte("line ONE\nline two\n"), 0o644); err != nil {
		t.Fatalf("WriteFile (modify): %v", err)
	}
	// Untracked file.
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("new\n"), 0o644); err != nil {
		t.Fatalf("WriteFile (untracked): %v", err)
	}
	return dir
}

func TestGitStatusTool_MatchesGitStatusShortB(t *testing.T) {
	dir := newFixtureRepo(t)
	chdirTemp(t, dir)

	res, err := GitStatusTool{}.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("GitStatusTool.Execute: %v", err)
	}

	wantStatus, err := exec.Command("git", "status", "--short", "-b").Output()
	if err != nil {
		t.Fatalf("reference git status: %v", err)
	}
	wantBranch, err := exec.Command("git", "branch", "--show-current").Output()
	if err != nil {
		t.Fatalf("reference git branch: %v", err)
	}

	if !strings.Contains(res.Output, strings.TrimSpace(string(wantBranch))) {
		t.Errorf("GitStatusTool output missing branch name %q:\n%s", strings.TrimSpace(string(wantBranch)), res.Output)
	}
	// Every non-empty line `git status --short -b` prints should show up
	// somewhere in our output too (format parity, AC-01).
	for _, line := range strings.Split(strings.TrimRight(string(wantStatus), "\n"), "\n") {
		if line == "" {
			continue
		}
		if !strings.Contains(res.Output, line) {
			t.Errorf("GitStatusTool output missing reference line %q:\n%s", line, res.Output)
		}
	}
}

func TestGitStatusTool_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	chdirTemp(t, dir)

	_, err := GitStatusTool{}.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error outside a git repo")
	}
}

func TestGitDiffTool_MatchesGitDiff(t *testing.T) {
	dir := newFixtureRepo(t)
	chdirTemp(t, dir)

	res, err := GitDiffTool{}.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("GitDiffTool.Execute: %v", err)
	}

	want, err := exec.Command("git", "diff").Output()
	if err != nil {
		t.Fatalf("reference git diff: %v", err)
	}
	if res.Output != string(want) {
		t.Errorf("GitDiffTool.Execute() output mismatch.\ngot:\n%s\nwant:\n%s", res.Output, want)
	}
}

func TestGitDiffTool_ScopedToPath(t *testing.T) {
	dir := newFixtureRepo(t)
	chdirTemp(t, dir)

	// A second committed+modified file the args.path scoping should
	// exclude from the result.
	other := filepath.Join(dir, "other.txt")
	if err := os.WriteFile(other, []byte("a\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGit(t, dir, "add", "other.txt")
	runGit(t, dir, "commit", "-q", "-m", "add other")
	if err := os.WriteFile(other, []byte("b\n"), 0o644); err != nil {
		t.Fatalf("WriteFile (modify other): %v", err)
	}

	res, err := GitDiffTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": "committed.txt"}))
	if err != nil {
		t.Fatalf("GitDiffTool.Execute: %v", err)
	}
	if strings.Contains(res.Output, "other.txt") {
		t.Errorf("GitDiffTool scoped to committed.txt should not mention other.txt, got:\n%s", res.Output)
	}
	if !strings.Contains(res.Output, "committed.txt") {
		t.Errorf("GitDiffTool scoped to committed.txt should mention it, got:\n%s", res.Output)
	}
}

func TestGitDiffTool_EmptyDiff(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH")
	}
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	runGit(t, dir, "checkout", "-q", "-b", "main")
	f := filepath.Join(dir, "a.txt")
	if err := os.WriteFile(f, []byte("x\n"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	runGit(t, dir, "add", "a.txt")
	runGit(t, dir, "commit", "-q", "-m", "init")
	chdirTemp(t, dir)

	res, err := GitDiffTool{}.Execute(context.Background(), nil)
	if err != nil {
		t.Fatalf("GitDiffTool.Execute: %v", err)
	}
	if !strings.Contains(res.Output, "gak ada perubahan") {
		t.Errorf("expected empty-diff message, got: %q", res.Output)
	}
}

func TestGitDiffTool_NotAGitRepo(t *testing.T) {
	dir := t.TempDir()
	chdirTemp(t, dir)

	_, err := GitDiffTool{}.Execute(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error outside a git repo")
	}
}
