// Ported from 30-luna/55-subagent/10-sysprompt.zsh
// (_ai_subagent_build_sysprompt). SESSION-51 scope only.
package subagent

import "fmt"

// BuildSysprompt returns the narrow system prompt for role given
// subGoal, a literal port of _ai_subagent_build_sysprompt's two case
// arms. termuxContext is injected verbatim at the end of the coder
// prompt only (mirrors $AI_TERMUX_CONTEXT / SESSION-10's SEC-006 fix --
// "Applied to coder ONLY, not researcher: researcher is readonly ...
// Termux-specific ... guidance is not actionable for it"); pass "" if
// the caller has no Termux context string to inject (e.g. non-Termux
// deployments, or tests). Returns "" for any role IsValidRole rejects --
// callers must reject an invalid role before reaching here (SpawnSubagent
// does).
func BuildSysprompt(role Role, subGoal, termuxContext string) string {
	switch role {
	case RoleResearcher:
		return fmt.Sprintf(`You are a readonly research subagent.

Goal:
%s

Investigate the repository using only readonly tools (read_file, list_dir, grep_search, glob_search, count_lines).

Do not modify files.
Do not run shell commands.
Do not execute tests.

Return concise findings when enough information has been gathered.

You must respond only as JSON:
{"thought": "...", "tool": "...", "args": {...}, "done": true|false}

When done is true, put your concise findings in "thought" -- that becomes the
summary returned to the caller, so make it self-contained and readable on its own.`, subGoal)
	case RoleCoder:
		prompt := fmt.Sprintf(`You are a coding subagent.

Goal:
%s

Inspect only what is necessary.
Implement the requested change.
Use existing tools.
Verify important changes when practical.

Respond only as JSON:
{"thought": "...", "tool": "...", "args": {...}, "done": true|false}

When done is true, put a concise description of what you changed in "thought" --
that becomes the summary returned to the caller, so make it self-contained and readable on its own.`, subGoal)
		if termuxContext != "" {
			prompt += "\n\n" + termuxContext
		}
		return prompt
	default:
		return ""
	}
}
