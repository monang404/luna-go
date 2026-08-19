// Traceability: 30-luna/60-ui/screens/agent.zsh -> agent.go.
// Blueprint v2 §4: agent state machine (Idle -> Thinking -> Acting ->
// Approval -> Done/Error), compact — no hero box.
//
// Screen contract (per session scope): rendering only. No LLM calls, no
// tool execution, no stdin/filesystem — composes components + primitives
// from data the caller already gathered.
package screens

import (
	"fmt"
	"strings"

	"github.com/monang404/luna-go/internal/ui"
	"github.com/monang404/luna-go/internal/ui/components"
)

// AgentStart is the port of ui_agent_start(goal?, total?).
func AgentStart(goal, total string, verbosity int, mode ui.Mode) string {
	if goal == "" {
		goal = "Running..."
	}
	var b strings.Builder
	b.WriteString(components.StateStep(goal, verbosity, mode).Output)
	if total != "?" && total != "" {
		b.WriteString("  " + mode.Tokens.Muted + "Steps: " + total + mode.Tokens.Reset + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// nonEmptyLineCount mirrors `printf '%s\n' "$steps_str" | grep -c '.'`:
// count of lines containing at least one character.
func nonEmptyLineCount(stepsStr string) int {
	n := 0
	for _, l := range strings.Split(stepsStr, "\n") {
		if l != "" {
			n++
		}
	}
	return n
}

// AgentDashboard is the port of ui_agent_dashboard(action, steps_str,
// current_idx?, output?, next_action?). next_action is accepted for
// signature parity with the zsh source but is unused there too (dead
// parameter in the original — preserved as-is, not a Go-side omission).
func AgentDashboard(action, stepsStr string, currentIdx int, output, nextAction string, verbosity int, mode ui.Mode) string {
	t := mode.Tokens
	var b strings.Builder
	b.WriteString(components.StateStep(action, verbosity, mode).Output)

	if stepsStr != "" {
		total := nonEmptyLineCount(stepsStr)
		if total == 0 {
			total = 1
		}
		fmt.Fprintf(&b, "  Progress %d/%d\n", currentIdx, total)

		idx := 1
		for _, stepLine := range strings.Split(stepsStr, "\n") {
			if stepLine == "" {
				continue // no idx++ — matches the shell's `continue` before `(( idx++ ))`
			}
			switch {
			case idx < currentIdx:
				b.WriteString("  " + t.OK + "✓" + t.Reset + " " + stepLine + "\n")
			case idx == currentIdx:
				b.WriteString("  " + t.Info + "●" + t.Reset + " " + stepLine + "\n")
			default:
				b.WriteString("  " + t.Muted + "○" + t.Reset + " " + stepLine + "\n")
			}
			idx++
		}
	}

	if output != "" {
		b.WriteString("\n")
		b.WriteString("  " + t.Muted + output + t.Reset + "\n")
	}

	b.WriteString("\n")
	return b.String()
}

// AgentDone is the port of ui_agent_done(files_changed?, runtime?,
// summary_items...).
func AgentDone(filesChanged, runtime string, items []string, mode ui.Mode) string {
	if filesChanged == "" {
		filesChanged = "0"
	}
	if runtime == "" {
		runtime = "?"
	}
	t := mode.Tokens
	var b strings.Builder
	b.WriteString(components.StateDone("Files: "+filesChanged, runtime, mode).Output)
	for _, item := range items {
		b.WriteString("  " + t.OK + "✓" + t.Reset + " " + item + "\n")
	}
	b.WriteString("\n")
	return b.String()
}

// AgentError is the port of ui_agent_error(reason?).
func AgentError(reason string, mode ui.Mode) string {
	if reason == "" {
		reason = "Unknown error"
	}
	return components.StateError(reason, mode).Output + "\n"
}
