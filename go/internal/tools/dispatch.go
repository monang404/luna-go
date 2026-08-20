package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/monang404/luna-go/internal/permission"
)

// This file ports 05-tool_dispatch.zsh's _ai_tool_dispatch (the manifest
// builder itself moved to registry.go's Manifest, since it's pure
// Registry-derived presentation text with no dispatch behavior).

// PermDeps bundles everything Dispatch needs to reach
// permission.CheckPermission, so callers don't have to thread five
// separate parameters through every Dispatch call. One PermDeps is built
// per agent-loop run in the not-yet-ported internal/agent
// (SESSION-49/50), the same lifetime as the AgentContext and
// ApprovalTracker it holds.
type PermDeps struct {
	AgentCtx *permission.AgentContext
	Config   permission.PermConfig
	Tracker  *permission.ApprovalTracker
	Ask      permission.AskFunc
	// Cwd is the process's current directory, forwarded to
	// permission.CheckPermission for ProjectRoot resolution when
	// AgentCtx.ProjectRoot is unset.
	Cwd string
}

// registration pairs one concrete Tool with the Registry metadata that
// governs how the Dispatcher is allowed to call it.
type registration struct {
	entry Entry
	tool  Tool
}

// Dispatcher holds the set of concrete tools wired up for one running
// agent (the Go equivalent of the `case "$tool_name" in ...` block at the
// bottom of _ai_tool_dispatch, but as data instead of a hardcoded
// switch, so SESSION-47/48 register tools instead of editing this file).
// Safe for concurrent use.
type Dispatcher struct {
	mu    sync.RWMutex
	tools map[string]registration
}

// NewDispatcher returns an empty Dispatcher ready for Register calls.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{tools: make(map[string]registration)}
}

// Register adds a concrete tool implementation under name with explicit
// metadata, for tools that aren't (or aren't yet) one of the 17 names in
// the package-level Registry -- e.g. NoopTool in tests, or a future tool
// this package's Registry hasn't been updated for yet. Returns an error
// if tool is nil, name is empty, or name is already registered (a
// dispatcher never silently overwrites one tool's wiring with another's,
// unlike the zsh `case` statement, where a duplicate arm would be a
// silent shadow bug caught only by manual review).
func (d *Dispatcher) Register(name string, entry Entry, tool Tool) error {
	if tool == nil {
		return errors.New("tools: cannot register a nil Tool")
	}
	if name == "" {
		return errors.New("tools: cannot register a tool with an empty name")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.tools[name]; exists {
		return fmt.Errorf("tools: %q is already registered", name)
	}
	d.tools[name] = registration{entry: entry, tool: tool}
	return nil
}

// RegisterFromRegistry is the convenience path SESSION-47/48's concrete
// tools are expected to use: it looks up name's Level/Capability/
// Description from the package-level Registry so a real tool
// implementation never has to restate that metadata itself, and can't
// accidentally drift from it. Returns an error if tool.Name() isn't one
// of the 17 known Registry entries.
func (d *Dispatcher) RegisterFromRegistry(tool Tool) error {
	if tool == nil {
		return errors.New("tools: cannot register a nil Tool")
	}
	name := tool.Name()
	entry, ok := Registry[name]
	if !ok {
		return fmt.Errorf("tools: %q is not a known Registry tool name", name)
	}
	return d.Register(name, entry, tool)
}

// Names returns the sorted list of tool names currently registered on
// this Dispatcher (not the full package-level Registry -- see Registry
// vs. a specific Dispatcher's subset, which may be smaller in any
// session before all 17 real tools exist).
func (d *Dispatcher) Names() []string {
	d.mu.RLock()
	defer d.mu.RUnlock()
	names := make([]string, 0, len(d.tools))
	for name := range d.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// Subset returns a new Dispatcher containing only the registrations named
// in names, copied from d. A name with no matching registration on d is
// silently skipped (not an error) -- the caller (internal/subagent,
// SESSION-51) builds names from a fixed per-role allowlist that may
// legitimately be broader than what any given Dispatcher instance has
// registered so far (e.g. in tests).
//
// This is how SESSION-51's per-role tool allowlist is enforced: a tool
// absent from the returned Dispatcher is indistinguishable, from
// Dispatch's perspective, from a tool that was never registered at all --
// the same "tool tidak dikenal" rejection either way, decided before the
// subagent's loop ever reaches Dispatch, exactly mirroring
// 15-run_step.zsh's ordering ("tool valid -> role permission -> dispatch").
func (d *Dispatcher) Subset(names []string) *Dispatcher {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := NewDispatcher()
	for _, name := range names {
		if reg, ok := d.tools[name]; ok {
			out.tools[name] = reg
		}
	}
	return out
}

// ConfigureDelegateTask injects a subagent spawner into any registered
// DelegateTaskTool instances. This breaks the circular dependency between
// tools and subagent execution.
func (d *Dispatcher) ConfigureDelegateTask(spawner SubagentSpawner) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, reg := range d.tools {
		if dt, ok := reg.tool.(*DelegateTaskTool); ok {
			dt.Spawner = spawner
		}
	}
}

// Dispatch mirrors _ai_tool_dispatch end-to-end: unknown-tool check,
// args normalization, schema validation, permission check, then Execute
// -- in that exact order, so nothing downstream of a rejection at any
// step ever runs.
func (d *Dispatcher) Dispatch(ctx context.Context, deps PermDeps, toolName string, argsJSON json.RawMessage) (Result, error) {
	d.mu.RLock()
	reg, ok := d.tools[toolName]
	d.mu.RUnlock()
	if !ok {
		return Result{}, fmt.Errorf("ERROR: tool '%s' gak dikenal. Tool valid: %s", toolName, strings.Join(d.Names(), ", "))
	}

	// ── Normalize args before validation ─────────────────────────
	// Mirrors _ai_tool_dispatch calling _ai_tool_normalize_args before
	// _ai_tool_validate_request, so the schema below sees the corrected
	// shape (bare-string args, alternative field names) rather than
	// rejecting input a human would consider obviously fine.
	normalized, err := NormalizeArgs(argsJSON, toolName)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: tool '%s' menerima arguments yang tidak sesuai schema: %w", toolName, err)
	}

	// AC-02: malformed/schema-invalid args are rejected here, before
	// permission.CheckPermission or Execute are ever reached.
	if err := ValidateArgs(toolName, normalized); err != nil {
		return Result{}, err
	}

	// ── Permission check ──────────────────────────────────────────
	// AC-03: called unconditionally, for every tool including readonly
	// ones -- exactly like _ai_tool_dispatch, which calls
	// _ai_permission_check for every tool_name with no readonly
	// short-circuit of its own; the readonly fast-allow lives inside
	// permission.CheckPermission's own level dispatch (see
	// internal/permission/check.go), not here. Path is populated from
	// the normalized args for every tool -- harmless no-op for tools
	// with no path-shaped field, and required for path-containment
	// checking on every tool that has one (including readonly reads).
	req := permission.Request{
		Level:      reg.entry.Level,
		Capability: reg.entry.Capability,
		Path:       ExtractPath(normalized),
	}
	decision, err := permission.CheckPermission(deps.AgentCtx, deps.Config, deps.Tracker, deps.Ask, deps.Cwd, req)
	if err != nil {
		return Result{}, err
	}
	if !decision.Allow {
		reason := decision.Reason
		if reason == "" {
			reason = fmt.Sprintf("ERROR: ditolak permission model buat tool '%s'", toolName)
		}
		return Result{}, errors.New(reason)
	}

	return reg.tool.Execute(ctx, normalized)
}
