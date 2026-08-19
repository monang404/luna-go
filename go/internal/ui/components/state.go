// Traceability: 30-luna/60-ui/components/state.zsh -> state.go.
//
// Divergence note: verbosity level (AI_VERBOSITY in zsh) and the detail
// log (AI_LAST_DETAIL_LOG global string) are both passed/returned
// explicitly instead of touched via globals — see verbosity.go's doc
// comment and disclosure.go's DetailLog type for the rationale (pure
// component contract). Each function below returns a StateResult whose
// LogLine is exactly the line _ai_state_* would have appended to
// AI_LAST_DETAIL_LOG (without the trailing \n the shell's += adds — the
// caller/DetailLog owns line joining).
package components

import (
	"fmt"

	"github.com/monang404/luna-go/internal/ui"
)

// StateResult is what every _ai_state_* function produces: the text that
// would have been printed to stdout (empty string if verbosity gated it
// off, matching a silent shell function call) and the line to append to
// the detail log (always non-empty — the shell always appends regardless
// of what's printed).
type StateResult struct {
	Output  string
	LogLine string
}

func stateIconLine(unicode bool, icon, asciiPrefix, msg string, t ui.Tokens) string {
	if unicode {
		return t.Info + icon + t.Reset + " " + msg + "\n"
	}
	return asciiPrefix + " " + msg + "\n"
}

// StateThinking is the port of _ai_state_thinking(msg?).
func StateThinking(msg string, verbosity int, mode ui.Mode) StateResult {
	if msg == "" {
		msg = "Thinking..."
	}
	out := ""
	if VerboseGE(verbosity, 1) {
		out = stateIconLine(mode.Unicode, "●", "*", msg, mode.Tokens)
	}
	return StateResult{Output: out, LogLine: "[thinking] " + msg}
}

// StateSending is the port of _ai_state_sending(provider?).
func StateSending(provider string, verbosity int, mode ui.Mode) StateResult {
	msg := "Sending..."
	if provider != "" {
		msg = "Sending to " + provider + "..."
	}
	out := ""
	if VerboseGE(verbosity, 1) {
		out = stateIconLine(mode.Unicode, "●", "*", msg, mode.Tokens)
	}
	return StateResult{Output: out, LogLine: "[sending] " + msg}
}

// StateActing is the port of _ai_state_acting(tool, detail?).
func StateActing(tool, detail string, verbosity int, mode ui.Mode) StateResult {
	t := mode.Tokens
	out := ""
	if VerboseGE(verbosity, 1) {
		if mode.Unicode {
			if detail != "" {
				out = fmt.Sprintf("%s→%s %s  %s%s%s\n", t.Primary, t.Reset, tool, t.Muted, detail, t.Reset)
			} else {
				out = fmt.Sprintf("%s→%s %s\n", t.Primary, t.Reset, tool)
			}
		} else {
			// Shell always prints both %s slots even when detail=="" (a
			// literal double-space then nothing, byte-for-byte).
			out = fmt.Sprintf("> %s  %s\n", tool, detail)
		}
	}
	logLine := "[acting] " + tool
	if detail != "" {
		logLine += " | " + detail
	}
	return StateResult{Output: out, LogLine: logLine}
}

// StateWaiting is the port of _ai_state_waiting(cmd). Always shown
// (approval state), no verbosity gate.
func StateWaiting(cmd string, mode ui.Mode) StateResult {
	t := mode.Tokens
	var out string
	if mode.Unicode {
		out = t.Warn + "⚠" + t.Reset + "  Needs approval\n" + "  " + t.Bold + cmd + t.Reset + "\n"
	} else {
		out = "! Needs approval: " + cmd + "\n"
	}
	return StateResult{Output: out, LogLine: "[approval] " + cmd}
}

// StateDone is the port of _ai_state_done(summary?, runtime?). Always
// shown, no verbosity gate.
func StateDone(summary, runtime string, mode ui.Mode) StateResult {
	t := mode.Tokens
	line := "Done"
	if summary != "" {
		line += "  ·  " + summary
	}
	if runtime != "" {
		line += "  ·  " + runtime
	}
	var out string
	if mode.Unicode {
		out = t.OK + "✓" + t.Reset + " " + line + "\n"
	} else {
		out = "+ " + line + "\n"
	}
	return StateResult{Output: out, LogLine: "[done] " + line}
}

// StateError is the port of _ai_state_error(msg?). Always shown.
func StateError(msg string, mode ui.Mode) StateResult {
	if msg == "" {
		msg = "Error"
	}
	t := mode.Tokens
	var out string
	if mode.Unicode {
		out = t.Err + "✗" + t.Reset + " " + msg + "\n"
	} else {
		out = "x " + msg + "\n"
	}
	return StateResult{Output: out, LogLine: "[error] " + msg}
}

// StateStep is the port of _ai_state_step(msg) — level-1 verbosity.
func StateStep(msg string, verbosity int, mode ui.Mode) StateResult {
	out := ""
	if VerboseGE(verbosity, 1) {
		out = stateIconLine(mode.Unicode, "●", "*", msg, mode.Tokens)
	}
	return StateResult{Output: out, LogLine: "[step] " + msg}
}

// StateTool is the port of _ai_state_tool(tool, args) — level-2
// verbosity.
func StateTool(tool, args string, verbosity int, mode ui.Mode) StateResult {
	t := mode.Tokens
	out := ""
	if VerboseGE(verbosity, 2) {
		out = fmt.Sprintf("%sTool:%s %s  %s%s%s\n", t.Muted, t.Reset, tool, t.Muted, args, t.Reset)
	}
	return StateResult{Output: out, LogLine: "[tool] " + tool + " " + args}
}

// StateDebug is the port of _ai_state_debug(msg) — level-3 verbosity.
func StateDebug(msg string, verbosity int, mode ui.Mode) StateResult {
	t := mode.Tokens
	out := ""
	if VerboseGE(verbosity, 3) {
		out = fmt.Sprintf("%s[DEBUG] %s%s\n", t.Muted, msg, t.Reset)
	}
	return StateResult{Output: out, LogLine: "[debug] " + msg}
}
