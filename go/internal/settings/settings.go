// Package settings implements the hierarchical settings.json configuration
// system for luna, inspired by Claude Code's settings hierarchy:
//
//  1. ~/.luna/settings.json         (user-level / global defaults)
//  2. .luna/settings.json           (project-level, committed to repo)
//  3. .luna/settings.local.json     (project-level local override, gitignored)
//  4. Environment variables & CLI flags (highest priority, handled by caller)
//
// Each level overrides the previous: scalar fields (Model, DefaultMode)
// replace; array fields (Permissions.Allow, Permissions.Deny) are merged
// (union); map fields (Env, Hooks) are merged (later keys win).
//
// SESSION-57 scope: struct definition, loader, merge, unit tests.
package settings

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// PermissionMode controls how the permission layer handles tool calls.
type PermissionMode string

const (
	ModeDefault     PermissionMode = "default"           // ask for write/process/shell
	ModeAcceptEdits PermissionMode = "acceptEdits"       // auto-allow Edit/Write, ask for Bash/Shell
	ModePlan        PermissionMode = "plan"              // block ALL write/process/shell (read-only)
	ModeBypass      PermissionMode = "bypassPermissions" // allow everything (requires explicit flag)
)

// ValidMode reports whether m is a recognized PermissionMode.
func ValidMode(m PermissionMode) bool {
	switch m {
	case ModeDefault, ModeAcceptEdits, ModePlan, ModeBypass, "":
		return true
	}
	return false
}

// Permissions defines per-tool permission rules.
// Patterns follow the form "ToolName" or "ToolName(arg_pattern)".
type Permissions struct {
	// Allow lists tool patterns that are auto-allowed without asking.
	Allow []string `json:"allow,omitempty"`
	// Deny lists tool patterns that are unconditionally denied.
	Deny []string `json:"deny,omitempty"`
	// Ask lists tool patterns that always require interactive confirmation.
	Ask []string `json:"ask,omitempty"`
	// DefaultMode overrides the global DefaultMode for this config level.
	DefaultMode PermissionMode `json:"defaultMode,omitempty"`
}

// HookDef defines a single hook: a shell command to run when a matching
// event+tool combination fires.
type HookDef struct {
	// Matcher is the tool name pattern to match (e.g. "Bash", "Edit", "*").
	Matcher string `json:"matcher"`
	// Command is the shell command to execute. Receives context via env vars
	// ($LUNA_TOOL_NAME, $LUNA_TOOL_INPUT, $LUNA_EVENT).
	Command string `json:"command"`
}

// HookSet groups hook definitions by event type.
type HookSet struct {
	PreToolUse       []HookDef `json:"PreToolUse,omitempty"`
	PostToolUse      []HookDef `json:"PostToolUse,omitempty"`
	Notification     []HookDef `json:"Notification,omitempty"`
	Stop             []HookDef `json:"Stop,omitempty"`
	SubagentStop     []HookDef `json:"SubagentStop,omitempty"`
	UserPromptSubmit []HookDef `json:"UserPromptSubmit,omitempty"`
}

// Settings is the unified configuration schema, matching the proposed
// settings.json format. Each field is optional; absent fields are left
// at their zero value and filled by the merge process or by runtime
// defaults.
type Settings struct {
	// DefaultMode controls permission behavior globally.
	DefaultMode PermissionMode `json:"defaultMode,omitempty"`
	// Model is the default model alias or full name (e.g. "sonnet", "claude-sonnet-4-20250514").
	Model string `json:"model,omitempty"`
	// Permissions defines tool-level allow/deny/ask rules.
	Permissions Permissions `json:"permissions,omitempty"`
	// AdditionalDirectories lists extra paths the agent is allowed to access
	// beyond the current working directory.
	AdditionalDirectories []string `json:"additionalDirectories,omitempty"`
	// Hooks defines event-driven shell command hooks.
	Hooks HookSet `json:"hooks,omitempty"`
	// Env is a map of environment variables to set when the agent starts.
	// Keys already present in the process environment are NOT overridden
	// (env vars from the shell always win).
	Env map[string]string `json:"env,omitempty"`
}

// DefaultUserSettingsPath returns ~/.luna/settings.json.
func DefaultUserSettingsPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".luna", "settings.json")
}

// ProjectSettingsPath returns .luna/settings.json relative to projectRoot.
func ProjectSettingsPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".luna", "settings.json")
}

// ProjectLocalSettingsPath returns .luna/settings.local.json relative to projectRoot.
func ProjectLocalSettingsPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".luna", "settings.local.json")
}

// Load reads and merges settings from all three levels:
//  1. ~/.luna/settings.json       (user)
//  2. <projectRoot>/.luna/settings.json       (project)
//  3. <projectRoot>/.luna/settings.local.json  (local override)
//
// A missing file at any level is silently skipped (not an error).
// A malformed JSON file IS an error (returned immediately, no partial merge).
// projectRoot may be "" to skip project-level files.
func Load(projectRoot string) (*Settings, error) {
	merged := &Settings{}

	// Level 1: user-level
	userPath := DefaultUserSettingsPath()
	if userSettings, err := loadFile(userPath); err != nil {
		return nil, fmt.Errorf("settings: user config %s: %w", userPath, err)
	} else if userSettings != nil {
		merged = Merge(merged, userSettings)
	}

	if projectRoot == "" {
		return merged, nil
	}

	// Level 2: project-level
	projPath := ProjectSettingsPath(projectRoot)
	if projSettings, err := loadFile(projPath); err != nil {
		return nil, fmt.Errorf("settings: project config %s: %w", projPath, err)
	} else if projSettings != nil {
		merged = Merge(merged, projSettings)
	}

	// Level 3: local override
	localPath := ProjectLocalSettingsPath(projectRoot)
	if localSettings, err := loadFile(localPath); err != nil {
		return nil, fmt.Errorf("settings: local config %s: %w", localPath, err)
	} else if localSettings != nil {
		merged = Merge(merged, localSettings)
	}

	return merged, nil
}

// loadFile reads a settings JSON file. Returns (nil, nil) if the file
// does not exist. Returns (nil, err) if the file exists but is malformed.
func loadFile(path string) (*Settings, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &s, nil
}

// Merge combines base and override into a new Settings. Rules:
//   - Scalar fields (DefaultMode, Model): override wins if non-zero.
//   - String slices (Allow, Deny, Ask, AdditionalDirectories): union (append, dedup).
//   - Hook arrays: override appends to base.
//   - Env map: override keys win over base keys.
func Merge(base, override *Settings) *Settings {
	if base == nil {
		base = &Settings{}
	}
	if override == nil {
		return base
	}

	out := *base // shallow copy

	// Scalars: override wins if non-empty
	if override.DefaultMode != "" {
		out.DefaultMode = override.DefaultMode
	}
	if override.Model != "" {
		out.Model = override.Model
	}

	// Permission scalars
	if override.Permissions.DefaultMode != "" {
		out.Permissions.DefaultMode = override.Permissions.DefaultMode
	}

	// Permission arrays: merge (union)
	out.Permissions.Allow = mergeStringSlices(base.Permissions.Allow, override.Permissions.Allow)
	out.Permissions.Deny = mergeStringSlices(base.Permissions.Deny, override.Permissions.Deny)
	out.Permissions.Ask = mergeStringSlices(base.Permissions.Ask, override.Permissions.Ask)

	// Additional directories: merge
	out.AdditionalDirectories = mergeStringSlices(base.AdditionalDirectories, override.AdditionalDirectories)

	// Hooks: append
	out.Hooks.PreToolUse = append(append([]HookDef(nil), base.Hooks.PreToolUse...), override.Hooks.PreToolUse...)
	out.Hooks.PostToolUse = append(append([]HookDef(nil), base.Hooks.PostToolUse...), override.Hooks.PostToolUse...)
	out.Hooks.Notification = append(append([]HookDef(nil), base.Hooks.Notification...), override.Hooks.Notification...)
	out.Hooks.Stop = append(append([]HookDef(nil), base.Hooks.Stop...), override.Hooks.Stop...)
	out.Hooks.SubagentStop = append(append([]HookDef(nil), base.Hooks.SubagentStop...), override.Hooks.SubagentStop...)
	out.Hooks.UserPromptSubmit = append(append([]HookDef(nil), base.Hooks.UserPromptSubmit...), override.Hooks.UserPromptSubmit...)

	// Env: merge maps (override wins)
	out.Env = mergeMaps(base.Env, override.Env)

	return &out
}

// mergeStringSlices returns the union of a and b, preserving order,
// deduplicating by exact match. a's entries come first.
func mergeStringSlices(a, b []string) []string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	seen := make(map[string]bool, len(a)+len(b))
	out := make([]string, 0, len(a)+len(b))
	for _, s := range a {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	for _, s := range b {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

// mergeMaps returns a new map containing all entries from a and b.
// On key collision, b's value wins.
func mergeMaps(a, b map[string]string) map[string]string {
	if len(a) == 0 && len(b) == 0 {
		return nil
	}
	out := make(map[string]string, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}
