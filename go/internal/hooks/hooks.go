package hooks

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/monang404/luna-go/internal/settings"
)

// Runner manages the execution of hooks.
type Runner struct {
	hooks settings.HookSet
	cwd   string
	log   func(string)
}

// NewRunner creates a new Runner.
func NewRunner(hooks settings.HookSet, cwd string, log func(string)) *Runner {
	if log == nil {
		log = func(string) {}
	}
	return &Runner{
		hooks: hooks,
		cwd:   cwd,
		log:   log,
	}
}

// RunPreToolUse executes PreToolUse hooks matching the toolName.
func (r *Runner) RunPreToolUse(ctx context.Context, toolName string, toolInput string) {
	r.runMatching(ctx, "PreToolUse", r.hooks.PreToolUse, toolName, toolInput)
}

// RunPostToolUse executes PostToolUse hooks matching the toolName.
func (r *Runner) RunPostToolUse(ctx context.Context, toolName string, toolInput string) {
	r.runMatching(ctx, "PostToolUse", r.hooks.PostToolUse, toolName, toolInput)
}

func (r *Runner) runMatching(ctx context.Context, eventName string, defs []settings.HookDef, toolName, toolInput string) {
	for _, def := range defs {
		if !match(def.Matcher, toolName) {
			continue
		}

		r.log(fmt.Sprintf("Running %s hook (matcher: %s) for tool %s", eventName, def.Matcher, toolName))

		// The original PRD says: "Hooks will bypass interactive permission prompts since they are defined by the user in LUNA.md"
		// Execute via shell. On Windows, use pwsh or cmd. On Unix, use sh.
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.CommandContext(ctx, "cmd", "/c", def.Command)
		} else {
			cmd = exec.CommandContext(ctx, "sh", "-c", def.Command)
		}

		cmd.Dir = r.cwd
		cmd.Env = append(os.Environ(),
			"LUNA_EVENT="+eventName,
			"LUNA_TOOL_NAME="+toolName,
			"LUNA_TOOL_INPUT="+toolInput,
		)

		out, err := cmd.CombinedOutput()
		if err != nil {
			r.log(fmt.Sprintf("Hook %s failed: %v\nOutput: %s", eventName, err, string(out)))
		} else if len(out) > 0 {
			r.log(fmt.Sprintf("Hook %s output: %s", eventName, string(out)))
		}
	}
}

func match(pattern, name string) bool {
	if pattern == "*" {
		return true
	}
	// Simple glob matching
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, name)
		return matched
	}
	return pattern == name
}
