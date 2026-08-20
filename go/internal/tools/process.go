package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/monang404/luna-go/internal/permission"
)

// This file ports 30-luna/05-tools/30-tool_process.zsh's _ai_tool_exec_process
// and _ai_tool_run_command, plus 30-luna/05-tools/35-tool_run_test.zsh's
// _ai_tool_run_test. Not ported here: _ai_yolo_shell_safe (see
// policy.go's doc comment for why) and the exit-127 autodep retry both
// _ai_tool_exec_process's zsh sibling and _ai_tool_run_command have --
// autodep.go's own doc comment already draws that same boundary for the
// same reason (the install-triggering half of autodep was explicitly
// deferred past SESSION-47, and nothing in this session's scope changes
// that).

const processOutputCap = 3000 // matches every `_ai_head_c 3000` call site in these three zsh functions.

// execProcessAllowlist mirrors _ai_tool_exec_process's `case "$program"`
// allowlist exactly.
var execProcessAllowlist = map[string]bool{
	"git": true, "python": true, "python3": true, "node": true, "npm": true,
	"pnpm": true, "yarn": true, "bun": true, "cargo": true, "go": true,
	"pytest": true, "rg": true, "grep": true, "sed": true, "awk": true, "make": true,
}

// resolveNonProjectExecutable mirrors the shared resolve-then-hijack-check
// sequence both _ai_tool_exec_process and _ai_tool_run_test run before
// ever executing anything: look `name` up on PATH, then reject it if the
// resolved path lives inside the current project (a modified
// ./git/./node/etc. shadowing the real binary via a poisoned PATH entry
// -- see 00-core/env.zsh's own note on $HOME/.local/bin and
// $HOME/go/bin being user-writable and PATH-prepended). Uses the
// process's actual working directory for project-root resolution (not
// any args.cwd/args.path the caller passed in -- those name where the
// command *runs*, not which project's PATH-hijack boundary applies),
// exactly like the zsh source's own `_ai_project_root`/`_ai_path_within_project`
// calls, which never take the target cwd as an argument either.
func resolveNonProjectExecutable(name string) (string, error) {
	resolved, err := exec.LookPath(name)
	if err != nil {
		return "", fmt.Errorf("executable '%s' tidak ditemukan di PATH", name)
	}
	procCwd, err := os.Getwd()
	if err == nil {
		if within, werr := permission.PathWithinProject(nil, procCwd, resolved); werr == nil && within {
			return "", fmt.Errorf("executable '%s' resolves inside project; PATH hijacking protection", name)
		}
	}
	return resolved, nil
}

// resolveRunDir turns an args.cwd/args.path value (relative to the
// process's own working directory, "." if empty) into an absolute,
// canonicalized directory to `cmd.Dir` into -- the Go equivalent of the
// zsh source's `cd -- "$real_cwd" && ...` subshell, without actually
// changing this process's own working directory (exec.Cmd.Dir does that
// per-child instead, which is both simpler and safe for concurrent tool
// calls).
func resolveRunDir(arg string) (string, error) {
	if arg == "" {
		arg = "."
	}
	if !filepath.IsAbs(arg) {
		procCwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		arg = filepath.Join(procCwd, arg)
	}
	return permission.CanonicalPath(arg)
}

// clampTimeout mirrors the schema's own `optionalNumberInRange(..., 1,
// 300)` bound (schema.go) plus each function's own `[ -z "$timeout_s" ]
// && timeout_s=<default>` fallback -- Execute re-applies both here too,
// the same defense-in-depth posture fsread.go's tools already take
// (never assume Dispatcher's schema validation is the only caller).
func clampTimeout(obj map[string]interface{}, def int) time.Duration {
	raw := numberFieldAsString(obj, "timeout")
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		n = def
	}
	if n > 300 {
		n = 300
	}
	return time.Duration(n) * time.Second
}

// stringArrayField reads obj[field] as a []string, silently skipping any
// non-string element (schema.go's optionalNoNewlineStringArray has
// already guaranteed every element is a newline-free string for any
// call that went through Dispatcher; this tolerates a directly-called
// Execute with looser input the same way stringField/numberFieldAsString
// already do elsewhere in this package).
func stringArrayField(obj map[string]interface{}, field string) []string {
	raw, ok := obj[field].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// runCapped runs cmd (already fully constructed, no shell interpreter
// involved anywhere in this call chain), capping combined output at
// processOutputCap and formatting a Result exactly like the shared
// zsh-source tail every one of these three functions ends with:
// "OK (exit 0, no output)" / raw output on success, "ERROR (exit N):" +
// output on failure.
func runCapped(cmd *exec.Cmd, timeout time.Duration, deadlineErr error) (Result, error) {
	out, err := cmd.CombinedOutput()
	capped := firstNChars(string(out), processOutputCap)

	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Result{}, fmt.Errorf("ERROR (exit %d):\n%s", exitErr.ExitCode(), capped)
		}
		if deadlineErr != nil {
			return Result{}, fmt.Errorf("ERROR: process melebihi timeout %s", timeout)
		}
		return Result{}, fmt.Errorf("ERROR: gagal menjalankan process: %w", err)
	}
	if strings.TrimSpace(capped) == "" {
		capped = "OK (exit 0, no output)"
	}
	return Result{Output: capped}, nil
}

// ExecProcessTool implements _ai_tool_exec_process: a typed, no-shell
// executable launch (program + argv, never a shell command string),
// gated by the same allowlist + PATH-hijack protection as the zsh
// source.
type ExecProcessTool struct{}

func (ExecProcessTool) Name() string                      { return "exec_process" }
func (ExecProcessTool) Capability() permission.Capability { return Registry["exec_process"].Capability }

func (ExecProcessTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	obj := mustObject(args)
	program := stringField(obj, "program")
	if program == "" {
		return Result{}, fmt.Errorf("ERROR: exec_process membutuhkan args.program (string non-empty)")
	}

	runDir, err := resolveRunDir(stringField(obj, "cwd"))
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: cwd tidak valid: %w", err)
	}

	resolved, err := resolveNonProjectExecutable(program)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: %s", err.Error())
	}
	if !execProcessAllowlist[program] {
		return Result{}, fmt.Errorf("ERROR: executable '%s' belum masuk process allowlist", program)
	}

	background := boolField(obj, "background")

	if background {
		cmd := exec.Command(resolved, stringArrayField(obj, "args")...)
		cmd.Dir = runDir
		return startBackgroundProcess(cmd, program)
	}

	timeout := clampTimeout(obj, 30)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, resolved, stringArrayField(obj, "args")...) // no shell interpreter: argv passed directly, never through /bin/sh -c.
	cmd.Dir = runDir

	return runCapped(cmd, timeout, runCtx.Err())
}

// RunTestTool implements _ai_tool_run_test: an auto-detecting (or
// explicitly named) typed test-runner launch, restricted to each
// runner's "test" subcommand (or, for pytest/python -m pytest, its own
// fixed invocation shape) -- never an arbitrary shell command string.
type RunTestTool struct{}

func (RunTestTool) Name() string                      { return "run_test" }
func (RunTestTool) Capability() permission.Capability { return Registry["run_test"].Capability }

func (RunTestTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	obj := mustObject(args)
	path := ExtractPath(args)
	if path == "" {
		path = "."
	}

	runner, testArgs, err := resolveTestRunner(obj, path)
	if err != nil {
		return Result{}, err
	}
	if err := validateTestRunnerInvocation(runner, testArgs); err != nil {
		return Result{}, err
	}

	runDir, err := resolveRunDir(path)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: direktori test tidak bisa dikanonicalisasi")
	}
	if info, statErr := os.Stat(runDir); statErr != nil || !info.IsDir() {
		return Result{}, fmt.Errorf("ERROR: path test bukan direktori: %s", path)
	}

	resolved, err := resolveNonProjectExecutable(filepath.Base(runner))
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: %s", err.Error())
	}

	timeout := clampTimeout(obj, 60)
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(runCtx, resolved, testArgs...) // no shell interpreter here either.
	cmd.Dir = runDir

	header := fmt.Sprintf("Menjalankan test: %s %s\n", filepath.Base(runner), strings.Join(testArgs, " "))
	res, runErr := runCapped(cmd, timeout, runCtx.Err())
	if runErr != nil {
		return Result{}, fmt.Errorf("%s%s\n\nGAGAL: test exit dengan error", header, runErr.Error())
	}
	return Result{Output: fmt.Sprintf("%s%s\n\nOK: test selesai tanpa error (exit 0)", header, res.Output)}, nil
}

// resolveTestRunner mirrors the `cmd` / `runner` / auto-detect
// three-way branch at the top of _ai_tool_run_test. `cmd` (a
// compatibility-only, tokenized-not-shelled override) rejects any shell
// metacharacter outright, exactly like the zsh source's own pre-filter.
func resolveTestRunner(obj map[string]interface{}, path string) (runner string, testArgs []string, err error) {
	if cmd := stringField(obj, "cmd"); cmd != "" {
		if strings.ContainsAny(cmd, shellMetacharacters) || strings.Contains(cmd, "\n") {
			return "", nil, fmt.Errorf("ERROR: test command mengandung shell metacharacter; gunakan runner + args terstruktur.")
		}
		tokens := tokenizeShellLike(cmd)
		if len(tokens) == 0 {
			return "", nil, fmt.Errorf("ERROR: test command kosong")
		}
		return tokens[0], tokens[1:], nil
	}

	if runner = stringField(obj, "runner"); runner != "" {
		return runner, stringArrayField(obj, "args"), nil
	}

	// Auto-detect: fixed argv per marker file, never a shell string from
	// package.json's own "scripts.test" value (that would reintroduce
	// exactly the arbitrary-shell hazard this tool exists to avoid).
	if hasFile(path, "package.json") && commandExists("npm") && npmHasTestScript(path) {
		return "npm", []string{"test"}, nil
	}
	if hasFile(path, "Cargo.toml") && commandExists("cargo") {
		return "cargo", []string{"test"}, nil
	}
	if hasFile(path, "go.mod") && commandExists("go") {
		return "go", []string{"test", "./..."}, nil
	}
	if commandExists("pytest") && hasPytestFiles(path) {
		return "pytest", []string{"-v", "--tb=short"}, nil
	}
	return "", nil, fmt.Errorf("ERROR: Gak bisa auto-detect test runner. Gunakan runner + args terstruktur atau cmd kompatibel tanpa shell syntax.")
}

// validateTestRunnerInvocation mirrors the `case "${runner:t}" in ...`
// subcommand allowlist: only a real test-running invocation shape is
// permitted per runner, never a raw interpreter eval (`python -c`,
// `node -e`, ...).
func validateTestRunnerInvocation(runner string, testArgs []string) error {
	base := filepath.Base(runner)
	first := ""
	if len(testArgs) > 0 {
		first = testArgs[0]
	}
	switch base {
	case "npm", "pnpm", "yarn", "bun":
		if first != "test" {
			return fmt.Errorf("ERROR: %s hanya boleh menjalankan subcommand 'test'.", base)
		}
	case "cargo":
		if first != "test" {
			return fmt.Errorf("ERROR: cargo hanya boleh menjalankan subcommand 'test'.")
		}
	case "go":
		if first != "test" {
			return fmt.Errorf("ERROR: go hanya boleh menjalankan subcommand 'test'.")
		}
	case "pytest":
		// No further restriction, matches the zsh source's empty `pytest) ;;` arm.
	case "python", "python3":
		second := ""
		if len(testArgs) > 1 {
			second = testArgs[1]
		}
		if first != "-m" || second != "pytest" {
			return fmt.Errorf("ERROR: python runner hanya boleh menjalankan '-m pytest'.")
		}
	default:
		return fmt.Errorf("ERROR: test runner '%s' tidak diizinkan.", base)
	}
	return nil
}

func hasFile(dir, name string) bool {
	info, err := os.Stat(filepath.Join(dir, name))
	return err == nil && info.Mode().IsRegular()
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// npmHasTestScript mirrors the jq check against package.json's own
// `.scripts.test`, rejecting only npm's own placeholder
// (`echo "Error: no test specified" && exit 1"`), same as the zsh source.
func npmHasTestScript(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return false
	}
	var pkg struct {
		Scripts map[string]string `json:"scripts"`
	}
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	test := strings.TrimSpace(pkg.Scripts["test"])
	return test != "" && test != `echo "Error: no test specified" && exit 1`
}

// hasPytestFiles mirrors `find "$path" -maxdepth 3 \( -name 'test_*.py'
// -o -name '*_test.py' \) -print -quit`: a shallow (3 levels deep) scan
// for either pytest filename convention, stopping at the first match.
func hasPytestFiles(root string) bool {
	found := false
	var walk func(dir string, depth int)
	walk = func(dir string, depth int) {
		if found || depth > 3 {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if found {
				return
			}
			name := e.Name()
			if !e.IsDir() {
				if (strings.HasPrefix(name, "test_") || strings.HasSuffix(name, "_test.py")) && strings.HasSuffix(name, ".py") {
					found = true
					return
				}
				continue
			}
			walk(filepath.Join(dir, name), depth+1)
		}
	}
	walk(root, 0)
	return found
}

// RunCommandTool implements _ai_tool_run_command: the legacy,
// arbitrary-shell-command path -- kept for backward compatibility, hidden
// from the model-facing manifest by default (registry.go's Manifest),
// still dispatchable by name. Unlike exec_process/run_test, this really
// does interpret `command` through a shell, exactly like the zsh
// source's own `zsh -f -c -- "$command"` -- the session's regression
// check ("no `/bin/sh -c` in exec_process") is deliberately scoped to
// exec_process alone, not this tool, which is the one place in this
// package a shell interpreter is the intended, documented behavior.
type RunCommandTool struct{}

func (RunCommandTool) Name() string                      { return "run_command" }
func (RunCommandTool) Capability() permission.Capability { return Registry["run_command"].Capability }

func (RunCommandTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	command := ExtractField(args, "command", "cmd")
	if command == "" {
		return Result{}, fmt.Errorf("ERROR: run_command membutuhkan args.command (string non-empty). Diterima: %s", firstNChars(string(args), 200))
	}
	if IsDangerousCommand(command) {
		return Result{}, fmt.Errorf("ERROR: command diblokir sistem keamanan (destruktif)")
	}

	background := boolField(mustObject(args), "background")
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		if background {
			cmd = exec.Command("cmd", "/c", command)
			return startBackgroundProcess(cmd, "shell")
		}
		cmd = exec.CommandContext(ctx, "cmd", "/c", command)
	} else {
		if background {
			cmd = exec.Command("zsh", "-f", "-c", "--", command)
			if _, err := exec.LookPath("zsh"); err != nil {
				cmd = exec.Command("sh", "-c", command)
			}
			return startBackgroundProcess(cmd, "shell")
		}
		cmd = exec.CommandContext(ctx, "zsh", "-f", "-c", "--", command)
		if _, err := exec.LookPath("zsh"); err != nil {
			cmd = exec.CommandContext(ctx, "sh", "-c", command)
		}
	}

	out, err := cmd.CombinedOutput()
	capped := firstNChars(string(out), processOutputCap)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return Result{}, fmt.Errorf("ERROR (exit %d):\n%s", exitErr.ExitCode(), capped)
		}
		return Result{}, fmt.Errorf("ERROR: gagal menjalankan command: %w", err)
	}
	if strings.TrimSpace(capped) == "" {
		capped = "OK (exit 0, no output)"
	}
	return Result{Output: capped}, nil
}
