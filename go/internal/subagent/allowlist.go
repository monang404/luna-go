// Ported from 30-luna/55-subagent/05-tool_allowlist.zsh
// (_ai_subagent_tool_allowed, _ai_subagent_oneline) and
// 30-luna/55-subagent/25-debug_allowlist.zsh (_ai_debug_tool_allowed).
//
// SESSION-51 scope only. Both zsh sources describe the allowlist as a
// deliberately explicit, manually-maintained list per role (not a
// filter derived from tools.Registry's readonly tag) -- "researcher ->
// HANYA 5 tool readonly ... coder -> 5 tool readonly yang sama +
// write_file/edit_file/patch_file/move_file/delete_file + run_test +
// git_status/git_diff + todo_write/todo_read. TIDAK termasuk
// run_command, exec_process, web_fetch." This file ports that literal
// list, not a derivation from it, so a future change to Registry's
// tags can never silently widen (or narrow) what a subagent role may
// call.
package subagent

// Role is a subagent's role, mirroring the two roles 00-design_contract.zsh
// ships in this round ("Round pertama Fase 6 CUMA 2 role ini
// (researcher, coder)"). Any other Role value is rejected by
// SpawnSubagent exactly like _ai_subagent_run's own `case "$role" in
// researcher|coder) ;; *) ... ;; esac` guard.
type Role string

const (
	// RoleResearcher: readonly investigation, context gathering,
	// inventory, analysis. Never modifies the project.
	RoleResearcher Role = "researcher"
	// RoleCoder: implements changes -- the full researcher tool set
	// plus file mutation, test running, readonly git context, and
	// todo tracking. Deliberately excludes run_command, exec_process,
	// and web_fetch (SESSION-10/SEC-006 hardening).
	RoleCoder Role = "coder"
)

// IsValidRole reports whether role is one of the two roles this round
// of the subagent system supports.
func IsValidRole(role Role) bool {
	switch role {
	case RoleResearcher, RoleCoder:
		return true
	default:
		return false
	}
}

// researcherTools is the literal 5-tool readonly allowlist from
// 05-tool_allowlist.zsh's researcher case arm.
var researcherTools = []string{
	"read_file", "list_dir", "grep_search", "glob_search", "count_lines",
}

// coderTools is researcher's 5 tools plus the mutation/test/readonly-git/
// todo tools from the coder case arm. Order matches the zsh comment's own
// listing; duplicated here (rather than "append researcherTools") so the
// full list is visible at a glance for audit, matching how the zsh case
// statement itself spells every tool name out explicitly.
var coderTools = []string{
	"read_file", "list_dir", "grep_search", "glob_search", "count_lines",
	"write_file", "edit_file", "patch_file", "move_file", "delete_file",
	"run_test",
	"git_status", "git_diff",
	"todo_write", "todo_read",
}

// AllowedTools returns the literal tool-name allowlist for role, or nil
// for any role IsValidRole rejects. The returned slice is owned by this
// package -- callers (run.go's Dispatcher.Subset call) must not mutate
// it.
func AllowedTools(role Role) []string {
	switch role {
	case RoleResearcher:
		return researcherTools
	case RoleCoder:
		return coderTools
	default:
		return nil
	}
}

// ToolAllowed mirrors _ai_subagent_tool_allowed(role, tool) directly, for
// callers (and tests) that want a yes/no answer without going through
// Dispatcher.Subset.
func ToolAllowed(role Role, tool string) bool {
	for _, t := range AllowedTools(role) {
		if t == tool {
			return true
		}
	}
	return false
}

// debugTools is the literal allowlist from 25-debug_allowlist.zsh's
// _ai_debug_tool_allowed: every readonly inspection tool plus run_test
// and run_command (diagnosis may need to reproduce a bug by actually
// running something) -- but never a mutation tool. aidebug()'s own
// caller-side guard (40-debug.zsh) additionally forces
// AI_PERM_SHELL_MODE=ask_always and AI_AGENT_YOLO_MODE=0 around
// run_command so this allowlist is not the only thing standing between
// diagnosis and an unreviewed shell command; RunDebug (debug.go) ports
// that same forced-safe-defaults behavior.
var debugTools = []string{
	"read_file", "list_dir", "grep_search", "glob_search", "count_lines",
	"git_status", "git_diff", "todo_read",
	"run_test", "run_command",
}

// DebugToolAllowed mirrors _ai_debug_tool_allowed.
func DebugToolAllowed(tool string) bool {
	for _, t := range debugTools {
		if t == tool {
			return true
		}
	}
	return false
}
