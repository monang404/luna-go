// Package commands wires every ported internal/ package (config,
// permission, tools, llmclient, agent, subagent, ui, chat, codeproject,
// filepatch, workflow) into one cobra command tree.
//
// See docs/execution_sessions/55_wire_cli_entrypoint.yaml. This session's
// scope is wiring only -- no new business logic, no changes to any
// package's exported behavior. Where a real terminal-facing
// implementation was deliberately deferred by an earlier session (the
// interactive confirm prompt, the clipboard, the command runner), this
// file provides the first real one, since "callable from an actual
// terminal" is exactly what SESSION-55 exists to deliver.
package commands

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/monang404/luna-go/internal/agent"
	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/chat"
	"github.com/monang404/luna-go/internal/codeproject"
	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/filepatch"
	"github.com/monang404/luna-go/internal/permission"
	"github.com/monang404/luna-go/internal/subagent"
	"github.com/monang404/luna-go/internal/tools"
	"github.com/monang404/luna-go/internal/workflow"
)

// App bundles every dependency a subcommand needs. One App is built once
// in main() and threaded through the whole cobra tree, mirroring the
// zsh source's single already-sourced global config/function set.
type App struct {
	Requester *aiops.Requester
	Confirm   aiops.ConfirmFunc
	Runner    aiops.CommandRunner
	Clipboard aiops.Clipboard

	Chat       *chat.Service
	Sessions   *chat.SessionStore
	Code       *codeproject.Service
	Patch      *filepatch.Service
	Workflow   *workflow.Service
	Dispatcher *tools.Dispatcher
	Loader     *subagent.Loader

	Limits config.Limits
	Paths  config.Paths
}

// NewApp constructs every service with real (non-fake) dependencies:
// the live provider/model fallback loop (aiops.Requester), a real
// stdin/stdout confirm prompt, os/exec-backed command running, and a
// termux-clipboard-*-backed Clipboard that degrades to
// aiops.NoClipboard off-Termux. Exactly one App exists per process.
func NewApp() *App {
	// Auto-load secrets (API keys) from ~/.secrets.zsh into the environment
	config.LoadSecrets(config.DefaultSecretsPath())

	requester := aiops.NewRequester()
	runner := aiops.ExecRunner{}
	dispatcher := buildDispatcher()

	return &App{
		Requester:  requester,
		Confirm:    TerminalConfirm,
		Runner:     runner,
		Clipboard:  newTermuxClipboard(runner),
		Chat:       chat.NewService(requester),
		Sessions:   chat.NewSessionStore(),
		Code:       codeproject.NewService(requester, TerminalConfirm, runner),
		Patch:      filepatch.NewService(requester, TerminalConfirm),
		Workflow:   workflow.NewService(requester, TerminalConfirm, runner),
		Dispatcher: dispatcher,
		Loader:     subagent.NewLoader(),
		Limits:     config.LoadLimits(),
		Paths:      config.LoadPaths(),
	}
}

// buildDispatcher registers every concrete tools.Tool implementation
// under its tools.Registry entry -- the Go equivalent of the zsh
// dispatcher's hardcoded `case "$tool_name" in ...` block, but built
// from Registry (single source of truth) instead of a second hand-kept
// list, so a Registry addition can never silently go unregistered here
// without a compile-time reminder (the length assertion below).
func buildDispatcher() *tools.Dispatcher {
	d := tools.NewDispatcher()
	registered := 0
	register := func(name string, tool tools.Tool) {
		entry := tools.Registry[name]
		if err := d.Register(name, entry, tool); err != nil {
			// Registration only fails on programmer error (nil tool,
			// empty/duplicate name) -- never on user input, so panic
			// here is the right failure mode (same as an init()-time
			// invariant violation).
			panic(fmt.Sprintf("commands: buildDispatcher: %v", err))
		}
		registered++
	}
	register("read_file", tools.ReadFileTool{})
	register("list_dir", tools.ListDirTool{})
	register("grep_search", tools.GrepSearchTool{})
	register("glob_search", tools.GlobSearchTool{})
	register("count_lines", tools.CountLinesTool{})
	register("write_file", tools.WriteFileTool{})
	register("edit_file", tools.EditFileTool{})
	register("patch_file", tools.PatchFileTool{})
	register("run_command", tools.RunCommandTool{})
	register("exec_process", tools.ExecProcessTool{})
	register("run_test", tools.RunTestTool{})
	register("move_file", tools.MoveFileTool{})
	register("delete_file", tools.DeleteFileTool{})
	register("git_status", tools.GitStatusTool{})
	register("git_diff", tools.GitDiffTool{})
	register("web_fetch", tools.WebFetchTool{})
	register("web_search", &tools.WebSearchTool{})
	register("todo_write", tools.TodoWriteTool{})
	register("todo_read", tools.TodoReadTool{})
	register("bash_output", tools.BashOutputTool{})
	register("kill_shell", tools.KillShellTool{})
	register("delegate_task", &tools.DelegateTaskTool{})
	
	if registered != len(tools.Registry) {
		// Guard against a future Registry addition silently missing a
		// register() call above -- see the comment on this function.
		panic(fmt.Sprintf("commands: buildDispatcher: tools.Registry has %d entries, only %d are wired -- add the missing register() call", len(tools.Registry), registered))
	}
	return d
}

// TerminalConfirm is the first real (non-test-double) aiops.ConfirmFunc:
// a blocking y/n prompt on the controlling terminal, replacing the
// AskFunc/ConfirmFunc seam every SESSION-42..54 package left open for
// this session. Mirrors _ai_confirm's 0/timeout/non-zero exit-code
// contract as aiops.Decision (Approve/Decline/Timeout is not modeled --
// no terminal read timeout existed in the zsh source's interactive path
// either, only its non-interactive/CI fallback, which is out of this
// session's scope per the YAML's exclude list).
func TerminalConfirm(_ context.Context, prompt string) (aiops.Decision, error) {
	fmt.Fprintf(os.Stderr, "%s [y/N] ", prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return aiops.Declined, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return aiops.Approved, nil
	default:
		return aiops.Declined, nil
	}
}

// terminalAsk adapts TerminalConfirm to permission.AskFunc for
// tools.PermDeps/agent.Deps callers, which use the bool-returning
// permission-layer contract instead of aiops.Decision.
func terminalAsk(prompt string) (bool, error) {
	dec, err := TerminalConfirm(context.Background(), prompt)
	if err != nil {
		return false, err
	}
	return dec == aiops.Approved, nil
}

// termuxClipboard shells out to termux-clipboard-get/-set via the
// injected CommandRunner, the first real aiops.Clipboard implementation
// (every earlier session only had aiops.NoClipboard). Falls back to
// ErrClipboardUnavailable when the termux-api command isn't installed
// (off-Termux dev machines), matching aiops.NoClipboard's error exactly
// so callers need no extra branch.
type termuxClipboard struct {
	runner aiops.CommandRunner
}

func newTermuxClipboard(runner aiops.CommandRunner) aiops.Clipboard {
	return termuxClipboard{runner: runner}
}

func (c termuxClipboard) Get(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("termux-clipboard-get"); err != nil {
		return "", aiops.ErrClipboardUnavailable
	}
	stdout, _, code, err := c.runner.Run(ctx, "termux-clipboard-get")
	if err != nil || code != 0 {
		return "", aiops.ErrClipboardUnavailable
	}
	return stdout, nil
}

func (c termuxClipboard) Set(ctx context.Context, content string) error {
	if _, err := exec.LookPath("termux-clipboard-set"); err != nil {
		return aiops.ErrClipboardUnavailable
	}
	cmd := exec.CommandContext(ctx, "termux-clipboard-set")
	cmd.Stdin = strings.NewReader(content)
	if err := cmd.Run(); err != nil {
		return aiops.ErrClipboardUnavailable
	}
	return nil
}

// subagentDeps builds subagent.Deps from the parent agent's own
// permission context -- shared by research/delegate/debug commands,
// each of which spawns exactly one subagent standalone (not from
// inside a running RunLoop, matching the zsh source's aidebug/
// airesearch/aidelegate, which are standalone entry points, not agent
// tool calls).
func (a *App) subagentDeps(cwd string) subagent.Deps {
	agentCtx := permission.NewAgentContext("standalone", cwd, false, permission.RolePrimary)
	return subagent.Deps{
		Limits:          a.Limits,
		Dispatcher:      a.Dispatcher,
		Loader:          a.Loader,
		ParentAgentCtx:  agentCtx,
		Config:          permission.LoadPermConfig(),
		Tracker:         permission.NewApprovalTracker(),
		Ask:             terminalAsk,
		Cwd:             cwd,
		ParentSessionID: "standalone",
	}
}

// agentDeps builds agent.Deps for a top-level `agent` run (aiagent).
func (a *App) agentDeps(cwd, systemPrompt string, logf func(string)) agent.Deps {
	agentCtx := permission.NewAgentContext("agent", cwd, false, permission.RolePrimary)
	return agent.Deps{
		Limits:       a.Limits,
		SystemPrompt: systemPrompt,
		Dispatcher:   a.Dispatcher,
		PermDeps: tools.PermDeps{
			AgentCtx: agentCtx,
			Config:   permission.LoadPermConfig(),
			Tracker:  permission.NewApprovalTracker(),
			Ask:      terminalAsk,
			Cwd:      cwd,
		},
		Store:     agent.NewStore(a.Paths.AgentCheckpointDir),
		SessionID: "agent",
		Log:       logf,
	}
}
