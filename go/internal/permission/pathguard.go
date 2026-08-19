package permission

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// ProjectRoot mirrors _ai_project_root (10-path_guard.zsh): prefer the
// already-resolved AgentContext.ProjectRoot (the Go equivalent of the
// AI_AGENT_PROJECT_ROOT env var an active agent loop exports, avoiding a
// `git rev-parse` shell-out on every single call -- see that function's
// PERF-003 comment), else resolve the nearest Git worktree root, else the
// physical (symlink-resolved) current working directory.
func ProjectRoot(ctx *AgentContext, cwd string) (string, error) {
	if ctx != nil && ctx.ProjectRoot != "" {
		return ctx.ProjectRoot, nil
	}
	if root, err := gitToplevel(cwd); err == nil && root != "" {
		return canonicalizeExisting(root)
	}
	return canonicalizeExisting(cwd)
}

// gitToplevel shells out to `git rev-parse --show-toplevel`, matching the
// `command -v git` + `git rev-parse --show-toplevel` fallback chain in
// _ai_project_root.
func gitToplevel(cwd string) (string, error) {
	if _, err := exec.LookPath("git"); err != nil {
		return "", err
	}
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// canonicalizeExisting resolves symlinks on a path that is expected to
// exist (project roots always do), matching the `cd -P -- "$root" && pwd
// -P` idiom _ai_project_root uses on both its git and non-git branches.
func canonicalizeExisting(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return resolved, nil
}

// CanonicalPath mirrors _ai_canonical_path: canonicalize a path even when
// its final path component does not exist yet (the security primitive
// every project-scoped filesystem tool relies on, e.g. write_file
// creating a brand new file). Go's filepath.EvalSymlinks fails outright on
// a nonexistent path, so this walks up to the nearest existing ancestor,
// resolves *that*, and re-appends the still-nonexistent tail -- equivalent
// to what `realpath` (and the python3 os.path.realpath fallback the zsh
// version also tries) does for a missing leaf component.
func CanonicalPath(target string) (string, error) {
	if target == "" {
		return "", fmt.Errorf("permission: empty path")
	}
	abs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}

	var tail []string
	cur := abs
	for {
		if resolved, err := filepath.EvalSymlinks(cur); err == nil {
			full := resolved
			for i := len(tail) - 1; i >= 0; i-- {
				full = filepath.Join(full, tail[i])
			}
			return filepath.Clean(full), nil
		}
		parent := filepath.Dir(cur)
		if parent == cur {
			// Reached the filesystem root and even it didn't resolve --
			// nothing left to walk up to.
			return "", fmt.Errorf("permission: cannot canonicalize %q", target)
		}
		tail = append(tail, filepath.Base(cur))
		cur = parent
	}
}

// PathWithinProject mirrors _ai_path_within_project: canonicalize both the
// project root and the target, then check exact match or prefix
// containment (`$canonical == $root` || `$canonical == $root/*`).
func PathWithinProject(ctx *AgentContext, cwd, target string) (bool, error) {
	root, err := ProjectRoot(ctx, cwd)
	if err != nil {
		return false, err
	}
	canonical, err := CanonicalPath(target)
	if err != nil {
		return false, err
	}
	return canonical == root || strings.HasPrefix(canonical, root+string(filepath.Separator)), nil
}

// IsPathAllowed mirrors the AC-01/AC-03 contract carved out of
// _ai_validate_project_path: is `target` allowed for filesystem access
// under `cfg`/`ctx`? Takes ctx/cwd/cfg in addition to the path (the
// session spec's `IsPathAllowed(path string) (bool, reason)` signature
// omits them, but path containment is meaningless without knowing *which*
// project root and override policy to check against -- same kind of
// deliberate, documented deviation SESSION-41 made for its own AC-04
// finding). AI_PERM_ALLOW_OUTSIDE_PROJECT short-circuits to allowed, same
// as the zsh source.
func IsPathAllowed(ctx *AgentContext, cfg PermConfig, cwd, target string) (bool, string) {
	if target == "" {
		return false, "path is empty"
	}
	if cfg.AllowOutsideProject {
		return true, "AI_PERM_ALLOW_OUTSIDE_PROJECT=1"
	}
	within, err := PathWithinProject(ctx, cwd, target)
	if err != nil {
		return false, err.Error()
	}
	if !within {
		return false, "path is outside project root"
	}
	return true, ""
}

// ValidateProjectPath mirrors _ai_validate_project_path, returning an
// error shaped like its stderr message when denied.
func ValidateProjectPath(ctx *AgentContext, cfg PermConfig, cwd, target, operation string) error {
	if target == "" {
		return fmt.Errorf("path kosong untuk %s", operation)
	}
	allowed, reason := IsPathAllowed(ctx, cfg, cwd, target)
	if !allowed {
		return fmt.Errorf("%s ditolak: path berada di luar project root: %s (%s)", operation, target, reason)
	}
	return nil
}
