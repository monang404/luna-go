package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// --- exec_process ---

func TestExecProcessTool_MissingProgram(t *testing.T) {
	_, err := ExecProcessTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected error when args.program is missing")
	}
}

func TestExecProcessTool_RejectsProgramNotOnAllowlist(t *testing.T) {
	if _, err := exec.LookPath("ls"); err != nil {
		t.Skip("ls not available in PATH")
	}
	_, err := ExecProcessTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{"program": "ls"}))
	if err == nil {
		t.Fatal("expected 'ls' to be rejected -- it is not in execProcessAllowlist")
	}
	if !strings.Contains(err.Error(), "process allowlist") {
		t.Errorf("expected an allowlist rejection message, got: %v", err)
	}
}

func TestExecProcessTool_UnknownExecutable(t *testing.T) {
	_, err := ExecProcessTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{"program": "definitely_not_a_real_binary_xyz"}))
	if err == nil {
		t.Fatal("expected error for an executable that doesn't exist on PATH")
	}
}

func TestExecProcessTool_RunsAllowlistedProgram(t *testing.T) {
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not available in PATH")
	}
	res, err := ExecProcessTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"program": "grep",
		"args":    []string{"--version"},
	}))
	if err != nil {
		t.Fatalf("ExecProcessTool.Execute(grep --version): %v", err)
	}
	if !strings.Contains(strings.ToLower(res.Output), "grep") {
		t.Errorf("expected grep --version output to mention 'grep', got: %q", res.Output)
	}
}

func TestExecProcessTool_PathHijackProtection(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("shell-script fixture executable assumes a POSIX shell")
	}
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available in PATH (needed for project-root resolution)")
	}
	projectDir := t.TempDir()
	runGit(t, projectDir, "init", "-q")

	fakeMake := filepath.Join(projectDir, "make")
	script := "#!/bin/sh\necho fake-make-ran\n"
	if err := os.WriteFile(fakeMake, []byte(script), 0o755); err != nil {
		t.Fatalf("WriteFile fake make: %v", err)
	}

	chdirTemp(t, projectDir)
	t.Setenv("PATH", projectDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err := ExecProcessTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{"program": "make"}))
	if err == nil {
		t.Fatal("expected PATH-hijack protection to reject a project-local 'make' shadowing PATH")
	}
	if !strings.Contains(err.Error(), "PATH hijacking protection") {
		t.Errorf("expected a PATH-hijack error, got: %v", err)
	}
}

func TestExecProcessTool_NoShellInterpreterInvolved(t *testing.T) {
	// Regression check from the session brief: exec_process must never
	// pass a shell metacharacter-bearing string to a shell for
	// interpretation. A program argument containing shell metacharacters
	// is passed as a literal argv element (schema.go's
	// optionalNoNewlineStringArray only forbids newlines, not other
	// metacharacters -- exec.CommandContext never interprets them either
	// way, unlike RunCommandTool's zsh -c/sh -c path).
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not available in PATH")
	}
	res, err := ExecProcessTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"program": "grep",
		"args":    []string{"--version", ";", "echo", "should-not-run-separately"},
	}))
	if err != nil {
		t.Fatalf("ExecProcessTool.Execute: %v", err)
	}
	// If a shell were involved, "; echo should-not-run-separately" would
	// have executed as a second command; grep instead receives ";" as a
	// literal (harmless, ignored/erroring) extra argument.
	if strings.Contains(res.Output, "should-not-run-separately") {
		t.Errorf("exec_process must never interpret shell metacharacters, got: %q", res.Output)
	}
}

// --- run_test ---

func TestRunTestTool_RejectsNpmWithoutTestSubcommand(t *testing.T) {
	_, err := RunTestTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"runner": "npm", "args": []string{"install"},
	}))
	if err == nil {
		t.Fatal("expected rejection: npm runner must only run the 'test' subcommand")
	}
}

func TestRunTestTool_RejectsDisallowedRunner(t *testing.T) {
	_, err := RunTestTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"runner": "rm", "args": []string{"-rf", "/"},
	}))
	if err == nil {
		t.Fatal("expected rejection: 'rm' is not an allowed test runner")
	}
}

func TestRunTestTool_RejectsPythonWithoutDashMPytest(t *testing.T) {
	_, err := RunTestTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"runner": "python3", "args": []string{"-c", "import os; os.system('rm -rf /')"},
	}))
	if err == nil {
		t.Fatal("expected rejection: python runner must only run '-m pytest'")
	}
}

func TestRunTestTool_RejectsCmdWithShellMetacharacters(t *testing.T) {
	_, err := RunTestTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"cmd": "pytest; rm -rf /",
	}))
	if err == nil {
		t.Fatal("expected rejection of a cmd string containing shell metacharacters")
	}
}

func TestRunTestTool_NoAutoDetectableRunner(t *testing.T) {
	dir := t.TempDir() // empty: no package.json/Cargo.toml/go.mod/pytest files
	res, err := RunTestTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"path": dir}))
	if err == nil {
		t.Fatalf("expected auto-detect failure for an empty directory, got result: %+v", res)
	}
	if !strings.Contains(err.Error(), "auto-detect") {
		t.Errorf("expected an auto-detect failure message, got: %v", err)
	}
}

func TestRunTestTool_CmdTokenizesIntoRunnerAndArgs(t *testing.T) {
	if _, err := exec.LookPath("pytest"); err != nil {
		t.Skip("pytest not available in PATH")
	}
	dir := t.TempDir()
	res, err := RunTestTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"cmd": "pytest --collect-only", "path": dir,
	}))
	// Whether or not pytest finds tests in an empty dir, this must reach
	// actual execution (not a validation error) -- i.e. either succeed
	// or fail with pytest's own exit status wrapped as an error, not a
	// tokenization/validation error.
	if err != nil && strings.Contains(err.Error(), "shell metacharacter") {
		t.Fatalf("unexpected validation error: %v", err)
	}
	_ = res
}

// --- run_command ---

func TestRunCommandTool_RejectsDangerousCommand(t *testing.T) {
	_, err := RunCommandTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"command": "rm -rf /some/path",
	}))
	if err == nil {
		t.Fatal("expected a dangerous command to be rejected before execution")
	}
	if !strings.Contains(err.Error(), "diblokir") {
		t.Errorf("expected the dangerous-command rejection message, got: %v", err)
	}
}

func TestRunCommandTool_MissingCommand(t *testing.T) {
	_, err := RunCommandTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{}))
	if err == nil {
		t.Fatal("expected error when args.command is missing")
	}
}

func TestRunCommandTool_RunsSafeCommand(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available in PATH")
	}
	res, err := RunCommandTool{}.Execute(context.Background(), argsJSON(t, map[string]interface{}{
		"command": "echo hello-from-run-command",
	}))
	if err != nil {
		t.Fatalf("RunCommandTool.Execute: %v", err)
	}
	if !strings.Contains(res.Output, "hello-from-run-command") {
		t.Errorf("expected echoed output, got: %q", res.Output)
	}
}

// --- shared helper tests ---

func TestClampTimeout_DefaultsAndClamps(t *testing.T) {
	if got := clampTimeout(map[string]interface{}{}, 30); got.Seconds() != 30 {
		t.Errorf("clampTimeout(missing) = %v, want 30s", got)
	}
	if got := clampTimeout(map[string]interface{}{"timeout": float64(9999)}, 30); got.Seconds() != 300 {
		t.Errorf("clampTimeout(9999) = %v, want clamped to 300s", got)
	}
	if got := clampTimeout(map[string]interface{}{"timeout": float64(0)}, 30); got.Seconds() != 30 {
		t.Errorf("clampTimeout(0) = %v, want default 30s (below minimum)", got)
	}
	if got := clampTimeout(map[string]interface{}{"timeout": float64(45)}, 30); got.Seconds() != 45 {
		t.Errorf("clampTimeout(45) = %v, want 45s", got)
	}
}
