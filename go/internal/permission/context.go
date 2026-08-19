// Package permission ports 30-luna/06-permissions/ (path guard, capability
// gate, and the write/process/shell permission-ask flow) into pure,
// I/O-free Go. Ported in SESSION-42.
//
// Deliberately excluded from this package (per the session's scope.exclude):
//   - the real interactive terminal confirmation UI (gum/read/tty), which
//     stays in the UI layer (SESSION-52/53) and is only referenced here
//     through the AskFunc hook;
//   - the tool-dispatch caller that decides *which* level/capability a
//     given tool call needs (SESSION-43/47/48) -- CheckPermission takes
//     the level/capability directly instead of a tool name, so this
//     package has no dependency on the not-yet-ported tool registry.
package permission

import "os"

// Role is the agent-loop identity a permission decision is made for,
// mirroring the "primary"/"subagent" distinction the session's scope calls
// out explicitly ("Agent context struct (role: primary/subagent, dsb)
// dipakai buat keputusan izin"). The zsh source (05-agent_context.zsh)
// does not itself thread a role through _ai_agent_context_begin -- capping
// a subagent's ceiling below primary's is new-but-faithful design work for
// this session (AC-04), consistent with the *separate* explicit-allowlist
// mechanism 55-subagent/05-tool_allowlist.zsh already uses at the
// tool-name level ("coder" role deliberately excludes run_command,
// exec_process, web_fetch -- see that file's comment block).
type Role string

const (
	RolePrimary  Role = "primary"
	RoleSubagent Role = "subagent"
)

// Capability mirrors the capability strings used by both
// AI_AGENT_CAPABILITIES (05-agent_context.zsh) and AI_TOOL_CAPABILITY
// (05-tools/00-tool_registry.zsh) in the zsh source.
type Capability string

const (
	CapFilesystemRead   Capability = "filesystem.read"
	CapFilesystemWrite  Capability = "filesystem.write"
	CapFilesystemDelete Capability = "filesystem.delete"
	CapGitRead          Capability = "git.read"
	CapSessionTodo      Capability = "session.todo"
	CapProcessTest      Capability = "process.test"
	CapProcessExecute   Capability = "process.execute"
	CapNetworkPublic    Capability = "network.public"
	CapShellArbitrary   Capability = "shell.arbitrary"
)

// subagentCeiling is the hard cap on what a RoleSubagent context can ever
// hold true, regardless of what a caller passes to NewAgentContext or
// later tries to Grant. It mirrors the union of every zsh subagent role's
// explicit allowlist (05-tool_allowlist.zsh's researcher ⊆ coder) minus
// the three highest-risk capabilities that allowlist deliberately never
// grants to any subagent role: shell.arbitrary, process.execute,
// network.public (that file's own comment: "TIDAK termasuk run_command,
// exec_process, web_fetch"). This is what makes AC-04 hold even before a
// per-role (researcher vs coder) allowlist exists in Go -- that finer
// grain is SESSION-51's job (internal/subagent).
var subagentDeniedCaps = map[Capability]bool{
	CapShellArbitrary: true,
	CapProcessExecute: true,
	CapNetworkPublic:  true,
}

// defaultCapabilities mirrors the literal AI_AGENT_CAPABILITIES assoc
// array _ai_agent_context_begin sets on every agent-loop start
// (05-agent_context.zsh): filesystem.read/git.read/session.todo/
// process.test start granted; everything else starts denied and must be
// escalated explicitly (YOLO capability gate) or via the write/process/
// shell ask flow.
func defaultCapabilities() map[Capability]bool {
	return map[Capability]bool{
		CapFilesystemRead:   true,
		CapGitRead:          true,
		CapSessionTodo:      true,
		CapProcessTest:      true,
		CapProcessExecute:   false,
		CapNetworkPublic:    false,
		CapFilesystemWrite:  false,
		CapFilesystemDelete: false,
		CapShellArbitrary:   false,
	}
}

// AgentContext is the Go equivalent of the process-local shell state
// _ai_agent_context_begin/_ai_agent_context_end install and tear down
// around one agent-loop run: AI_AGENT_SESSION_ID, AI_AGENT_PROJECT_ROOT,
// AI_AGENT_YOLO_MODE, AI_AGENT_CAPABILITIES. Unlike the zsh globals, this
// is an explicit value passed to every call in this package -- there is no
// package-level mutable state, which is also what makes it safe to run
// more than one AgentContext (e.g. primary + subagent, or two parallel
// sessions) in the same process.
type AgentContext struct {
	SessionID   string
	ProjectRoot string
	YoloMode    bool
	Role        Role

	capabilities map[Capability]bool
}

// NewAgentContext builds an AgentContext with the same starting
// capability set _ai_agent_context_begin installs, then -- for
// RoleSubagent only -- clamps the high-risk capabilities per
// subagentDeniedCaps regardless of the defaults above.
func NewAgentContext(sessionID, projectRoot string, yolo bool, role Role) *AgentContext {
	caps := defaultCapabilities()
	if role == RoleSubagent {
		for cap := range subagentDeniedCaps {
			caps[cap] = false
		}
	}
	return &AgentContext{
		SessionID:    sessionID,
		ProjectRoot:  projectRoot,
		YoloMode:     yolo,
		Role:         role,
		capabilities: caps,
	}
}

// CapabilityAllowed mirrors _ai_agent_capability_allowed: true only if the
// capability is present in the context's map with value true. A
// RoleSubagent context additionally can never report true for a
// subagentDeniedCaps entry, even if Grant was called for it (defense in
// depth against a caller mutating the map directly through a bug rather
// than through Grant).
func (c *AgentContext) CapabilityAllowed(cap Capability) bool {
	if c.Role == RoleSubagent && subagentDeniedCaps[cap] {
		return false
	}
	return c.capabilities[cap]
}

// Grant sets a capability's allow/deny state, mirroring the kind of
// escalation the YOLO capability gate in _ai_permission_check performs
// ("Agent meminta capability ... Izinkan sekali?" -> capability granted
// for the rest of the loop once approved). Returns false without changing
// state if the context's role is not allowed to ever hold that
// capability (AC-04) -- callers should treat that as a hard denial, not
// something a re-ask can override.
func (c *AgentContext) Grant(cap Capability, allow bool) bool {
	if c.Role == RoleSubagent && subagentDeniedCaps[cap] && allow {
		return false
	}
	c.capabilities[cap] = allow
	return true
}

// PermConfig mirrors the AI_PERM_* defaults from
// 30-luna/06-permissions/00-config.zsh. _ai_perm_load_project's
// project-local `.aiagent/permissions.zsh` override step is deliberately
// not ported here: sourcing arbitrary project-local shell code to
// overwrite the guardrail functions themselves has no safe Go equivalent
// (there is no "source this as code" primitive to opt into), and doing so
// would reintroduce exactly the supply-chain risk that file's own comment
// flags. If a future session wants an equivalent extension point, it
// should be a data file (e.g. JSON/YAML), not executable code.
type PermConfig struct {
	WriteMode           string // AI_PERM_WRITE_MODE: ask_once_per_file | auto | yolo
	ShellMode           string // AI_PERM_SHELL_MODE: ask_always | yolo
	ProcessMode         string // AI_PERM_PROCESS_MODE: ask_always | yolo
	AllowOutsideProject bool   // AI_PERM_ALLOW_OUTSIDE_PROJECT
}

// LoadPermConfig builds PermConfig from the current environment, matching
// the "${VAR:=default}" defaults in 00-config.zsh.
func LoadPermConfig() PermConfig {
	return PermConfig{
		WriteMode:           envOr("AI_PERM_WRITE_MODE", "ask_once_per_file"),
		ShellMode:           envOr("AI_PERM_SHELL_MODE", "ask_always"),
		ProcessMode:         envOr("AI_PERM_PROCESS_MODE", "ask_always"),
		AllowOutsideProject: envOr("AI_PERM_ALLOW_OUTSIDE_PROJECT", "0") == "1",
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
