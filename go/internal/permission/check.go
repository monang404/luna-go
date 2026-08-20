package permission

import "fmt"

// Level is the permission tier a capability's ask policy is chosen from,
// mirroring the "|readonly", "|write", "|process", "|shell" suffix on
// every AI_TOOL_REGISTRY entry (05-tools/00-tool_registry.zsh) and the
// four-way `case "$level"` dispatch at the bottom of
// _ai_permission_check (15-permission_check.zsh).
type Level string

const (
	LevelReadonly Level = "readonly"
	LevelWrite    Level = "write"
	LevelProcess  Level = "process"
	LevelShell    Level = "shell"
)

// Decision is CheckPermission's result: whether the action is allowed to
// proceed, whether that required an interactive ask, and why.
type Decision struct {
	Allow  bool
	Asked  bool
	Reason string
}

// Request describes one permission decision to make. Path is optional --
// only filesystem-touching requests set it (path containment + the
// ask_once_per_file dedup key); Prompt overrides the default
// confirmation question for the resolved Level (matching each of
// _ai_perm_ask_write/_ai_perm_ask_process/_ai_perm_ask_shell's own
// hardcoded question text, e.g. "Delete this file?" vs "Run command?").
type Request struct {
	Level      Level
	Capability Capability
	Path       string
	Prompt     string
}

// defaultPrompt mirrors the fixed question text each
// _ai_perm_ask_write/_ai_perm_ask_process/_ai_perm_ask_shell call site
// passes to _ai_perm_ask when the caller didn't supply its own.
func defaultPrompt(level Level, cap Capability) string {
	switch {
	case level == LevelWrite:
		return "Write/edit this file?"
	case level == LevelProcess:
		return "Run process?"
	case level == LevelShell && cap == CapFilesystemDelete:
		return "Delete this file?"
	case level == LevelShell && cap == CapNetworkPublic:
		return "Fetch this URL?"
	case level == LevelShell && cap == CapShellArbitrary:
		return "Run command?"
	case level == LevelShell:
		return "Proceed?"
	default:
		return "Proceed?"
	}
}

// CheckPermission mirrors _ai_permission_check end-to-end: path
// containment, the YOLO capability gate, then level dispatch into the
// write/process/shell ask policies. `cwd` is the process's current
// directory (used only for ProjectRoot resolution when ctx.ProjectRoot is
// unset); `tracker` may be nil if Request.Level != LevelWrite or the
// caller doesn't want ask_once_per_file dedup; `ask` may be nil, in which
// case any path that would need an interactive confirmation is denied
// instead of asked (fail-closed, never fail-open).
func CheckPermission(ctx *AgentContext, cfg PermConfig, tracker *ApprovalTracker, ask AskFunc, cwd string, req Request) (Decision, error) {
	if ctx == nil {
		return Decision{}, fmt.Errorf("permission: nil AgentContext")
	}

	// ── Filesystem path containment ──────────────────────────────
	// Mirrors _ai_permission_check's per-tool `case` block: every request
	// that carries a Path gets canonical containment checked before the
	// interactive decision, so relative paths, "..", prefix collisions
	// and symlink escapes can never reach the ask/allow logic below.
	if req.Path != "" {
		if err := ValidateProjectPath(ctx, cfg, cwd, req.Path, string(req.Capability)); err != nil {
			return Decision{Allow: false, Reason: err.Error()}, nil
		}
	}

	// ── Role ceiling (AC-04) ────────────────────────────────────────
	// This has no direct zsh equivalent (the source has no role-scoped
	// capability check at this layer at all -- see context.go's doc
	// comment) and is intentionally a hard, unconditional block: a
	// RoleSubagent request for a subagentDeniedCaps capability is denied
	// before the YOLO gate and before level dispatch even get a chance to
	// ask, so an approved ask can never grant it either. Contrast with
	// the YOLO capability gate below, which *can* be satisfied by an ask
	// -- that gate is about a context temporarily lacking a capability it
	// is otherwise allowed to hold, not about a capability its role can
	// never hold at all.
	if req.Capability != "" && ctx.Role == RoleSubagent && subagentDeniedCaps[req.Capability] {
		return Decision{Allow: false, Reason: "capability outside subagent allowlist"}, nil
	}

	// ── Capability gate (YOLO mode) ───────────────────────────────
	// Mirrors: `if YOLO && capability set && !allowed(capability):
	// _ai_perm_ask(...)`. A missing capability under YOLO falls back to
	// an explicit one-off confirmation instead of an implicit privilege
	// escalation -- and for a RoleSubagent context whose capability is in
	// subagentDeniedCaps, CapabilityAllowed always reports false and
	// Grant refuses to flip it, so this ask can never actually succeed
	// into an escalation (AC-04).
	if ctx.YoloMode && req.Capability != "" && !ctx.CapabilityAllowed(req.Capability) {
		prompt := fmt.Sprintf("Agent meminta capability '%s'. Izinkan sekali?", req.Capability)
		allowed, asked, err := askOnce(ask, prompt)
		if err != nil {
			return Decision{}, err
		}
		if !allowed {
			return Decision{Allow: false, Asked: asked, Reason: "capability not granted"}, nil
		}
		if !ctx.Grant(req.Capability, true) {
			return Decision{Allow: false, Asked: asked, Reason: "capability escalation denied for this role"}, nil
		}
	}

	// ── Permission level dispatch ──────────────────────────────────
	switch req.Level {
	case LevelReadonly:
		return Decision{Allow: true}, nil
	case LevelWrite:
		return checkWrite(ctx, cfg, tracker, ask, req)
	case LevelProcess:
		return checkProcess(ctx, cfg, ask, req)
	case LevelShell:
		return checkShell(ctx, cfg, ask, req)
	default:
		return Decision{}, fmt.Errorf("permission: unknown level %q", req.Level)
	}
}

// checkWrite mirrors _ai_perm_ask_write.
func checkWrite(ctx *AgentContext, cfg PermConfig, tracker *ApprovalTracker, ask AskFunc, req Request) (Decision, error) {
	if cfg.WriteMode == "block" {
		return Decision{Allow: false, Reason: "blocked by plan mode"}, nil
	}
	if ctx.YoloMode || cfg.WriteMode == "yolo" {
		return Decision{Allow: true, Reason: "yolo"}, nil
	}
	if cfg.WriteMode == "auto" {
		return Decision{Allow: true, Reason: "auto"}, nil
	}
	if cfg.WriteMode == "ask_once_per_file" && req.Path != "" && tracker != nil {
		if tracker.IsApproved(ctx.SessionID, req.Path) {
			return Decision{Allow: true, Reason: "already approved this session"}, nil
		}
	}

	prompt := req.Prompt
	if prompt == "" {
		prompt = defaultPrompt(LevelWrite, req.Capability)
	}
	allowed, asked, err := askOnce(ask, prompt)
	if err != nil {
		return Decision{}, err
	}
	if allowed && cfg.WriteMode == "ask_once_per_file" && req.Path != "" && tracker != nil {
		tracker.Approve(ctx.SessionID, req.Path)
	}
	return Decision{Allow: allowed, Asked: asked}, nil
}

// checkProcess mirrors _ai_perm_ask_process.
func checkProcess(ctx *AgentContext, cfg PermConfig, ask AskFunc, req Request) (Decision, error) {
	if cfg.ProcessMode == "block" {
		return Decision{Allow: false, Reason: "blocked by plan mode"}, nil
	}
	if ctx.YoloMode || cfg.ProcessMode == "yolo" {
		return Decision{Allow: true, Reason: "yolo"}, nil
	}
	prompt := req.Prompt
	if prompt == "" {
		prompt = defaultPrompt(LevelProcess, req.Capability)
	}
	allowed, asked, err := askOnce(ask, prompt)
	if err != nil {
		return Decision{}, err
	}
	return Decision{Allow: allowed, Asked: asked}, nil
}

// checkShell mirrors _ai_perm_ask_shell. AC-02: shell.arbitrary must
// never auto-allow, even under YOLO/ShellMode=="yolo" -- the zsh source
// enforces this the same way (_ai_yolo_shell_safe still gates
// run_command specifically under YOLO; every other "shell"-level tool
// bypasses under YOLO). _ai_yolo_shell_safe itself lives in the
// not-yet-ported tool layer (05-tools/30-tool_process.zsh, SESSION-47/48),
// so this package can't replicate its command-safety heuristic -- it
// takes the strictly safer position of always asking for
// shell.arbitrary rather than silently allowing everything through
// while that heuristic doesn't exist yet.
func checkShell(ctx *AgentContext, cfg PermConfig, ask AskFunc, req Request) (Decision, error) {
	if cfg.ShellMode == "block" {
		return Decision{Allow: false, Reason: "blocked by plan mode"}, nil
	}
	yolo := ctx.YoloMode || cfg.ShellMode == "yolo"
	if yolo && req.Capability != CapShellArbitrary {
		return Decision{Allow: true, Reason: "yolo"}, nil
	}
	prompt := req.Prompt
	if prompt == "" {
		prompt = defaultPrompt(LevelShell, req.Capability)
	}
	allowed, asked, err := askOnce(ask, prompt)
	if err != nil {
		return Decision{}, err
	}
	return Decision{Allow: allowed, Asked: asked}, nil
}

// askOnce calls ask if provided, else fails closed (never fails open):
// a nil AskFunc means no UI layer is wired up yet, so anything requiring
// interactive confirmation must be denied rather than silently allowed.
func askOnce(ask AskFunc, prompt string) (allowed bool, asked bool, err error) {
	if ask == nil {
		return false, false, nil
	}
	ok, err := ask(prompt)
	if err != nil {
		return false, true, err
	}
	return ok, true, nil
}
