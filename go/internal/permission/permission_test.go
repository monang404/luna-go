package permission

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestProject builds:
//
//	root/
//	  inside.txt
//	  subdir/nested.txt
//	  link_in       -> subdir            (symlink, stays inside root)
//	  link_out      -> <outside>         (symlink, escapes root)
//	outside/
//	  secret.txt
//
// and returns the canonicalized root and outside dirs.
func newTestProject(t *testing.T) (root, outside string) {
	t.Helper()
	base := t.TempDir()

	root = filepath.Join(base, "root")
	outside = filepath.Join(base, "outside")
	for _, d := range []string{root, filepath.Join(root, "subdir"), outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", d, err)
		}
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("setup: %v", err)
		}
	}
	must(os.WriteFile(filepath.Join(root, "inside.txt"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(root, "subdir", "nested.txt"), []byte("x"), 0o644))
	must(os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("x"), 0o644))
	must(os.Symlink(filepath.Join(root, "subdir"), filepath.Join(root, "link_in")))
	must(os.Symlink(outside, filepath.Join(root, "link_out")))

	canonRoot, err := canonicalizeExisting(root)
	must(err)
	canonOutside, err := canonicalizeExisting(outside)
	must(err)
	return canonRoot, canonOutside
}

// --- AC-03: >=10 table-driven path cases (relative, absolute, symlink, traversal) ---

func TestIsPathAllowed_TableDriven(t *testing.T) {
	root, outside := newTestProject(t)
	ctx := NewAgentContext("sess-1", root, false, RolePrimary)
	cfg := PermConfig{} // AllowOutsideProject: false

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"absolute path inside root", filepath.Join(root, "inside.txt"), true},
		{"absolute path nested inside root", filepath.Join(root, "subdir", "nested.txt"), true},
		{"relative path resolving inside root", filepath.Join(root, "./inside.txt"), true},
		{"relative traversal that lands back inside root", filepath.Join(root, "subdir", "..", "inside.txt"), true},
		{"path equal to root itself", root, true},
		{"symlink inside root pointing inside root", filepath.Join(root, "link_in", "nested.txt"), true},
		{"new file that does not exist yet, inside root", filepath.Join(root, "brand_new.txt"), true},
		{"traversal escaping root via ..", filepath.Join(root, "..", "outside", "secret.txt"), false},
		{"traversal escaping root via nested ../..", filepath.Join(root, "subdir", "..", "..", "outside", "secret.txt"), false},
		{"absolute path outside root entirely", filepath.Join(outside, "secret.txt"), false},
		{"symlink inside root escaping to outside", filepath.Join(root, "link_out", "secret.txt"), false},
		{"empty path", "", false},
	}

	if len(cases) < 10 {
		t.Fatalf("need >= 10 cases per AC-03, have %d", len(cases))
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, reason := IsPathAllowed(ctx, cfg, root, tc.path)
			if got != tc.want {
				t.Errorf("IsPathAllowed(%q) = %v (%s), want %v", tc.path, got, reason, tc.want)
			}
		})
	}
}

// --- AC-01: path traversal outside project root is rejected ---

func TestIsPathAllowed_RejectsTraversalOutsideRoot(t *testing.T) {
	root, _ := newTestProject(t)
	ctx := NewAgentContext("sess-1", root, false, RolePrimary)
	cfg := PermConfig{}

	traversal := filepath.Join(root, "..", "outside", "secret.txt")
	allowed, reason := IsPathAllowed(ctx, cfg, root, traversal)
	if allowed {
		t.Fatalf("traversal path %q should be rejected, reason=%q", traversal, reason)
	}
}

func TestIsPathAllowed_AllowOutsideProjectOverride(t *testing.T) {
	root, outside := newTestProject(t)
	ctx := NewAgentContext("sess-1", root, false, RolePrimary)
	cfg := PermConfig{AllowOutsideProject: true}

	allowed, _ := IsPathAllowed(ctx, cfg, root, filepath.Join(outside, "secret.txt"))
	if !allowed {
		t.Fatal("AI_PERM_ALLOW_OUTSIDE_PROJECT=1 should allow paths outside root")
	}
}

func TestValidateProjectPath_ErrorsOnTraversal(t *testing.T) {
	root, _ := newTestProject(t)
	ctx := NewAgentContext("sess-1", root, false, RolePrimary)
	cfg := PermConfig{}

	err := ValidateProjectPath(ctx, cfg, root, filepath.Join(root, "..", "outside", "secret.txt"), "write_file")
	if err == nil {
		t.Fatal("expected error for path outside project root")
	}
}

// --- AC-02: shell.arbitrary always requires ask, never auto-allow ---

func TestCheckPermission_ShellArbitraryNeverAutoAllows(t *testing.T) {
	root, _ := newTestProject(t)
	cfg := PermConfig{ShellMode: "yolo"}
	tracker := NewApprovalTracker()

	modes := []struct {
		name     string
		yoloMode bool
	}{
		{"yolo agent context", true},
		{"yolo shell mode config", false},
	}

	for _, m := range modes {
		t.Run(m.name, func(t *testing.T) {
			ctx := NewAgentContext("sess-1", root, m.yoloMode, RolePrimary)
			req := Request{Level: LevelShell, Capability: CapShellArbitrary}

			// No ask handler wired up -> must fail closed, not auto-allow.
			decision, err := CheckPermission(ctx, cfg, tracker, nil, root, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision.Allow {
				t.Fatalf("shell.arbitrary must never auto-allow under %s, got Allow=true", m.name)
			}

			// Ask handler that approves -> now it can be allowed, but only
			// because it was actually asked.
			asked := false
			approve := func(prompt string) (bool, error) {
				asked = true
				return true, nil
			}
			decision, err = CheckPermission(ctx, cfg, tracker, approve, root, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !asked {
				t.Fatalf("shell.arbitrary must always ask under %s", m.name)
			}
			if !decision.Allow {
				t.Fatalf("expected allow after explicit approval under %s", m.name)
			}
		})
	}
}

func TestCheckPermission_ShellNonArbitraryBypassesUnderYolo(t *testing.T) {
	root, _ := newTestProject(t)
	ctx := NewAgentContext("sess-1", root, true, RolePrimary)
	// network.public isn't in the default-granted set, so grant it first --
	// this test is about the shell-level dispatch's own YOLO bypass, not
	// about the separate YOLO capability-escalation gate that runs before it.
	ctx.Grant(CapNetworkPublic, true)
	cfg := PermConfig{}
	req := Request{Level: LevelShell, Capability: CapNetworkPublic}

	decision, err := CheckPermission(ctx, cfg, nil, nil, root, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allow || decision.Asked {
		t.Fatalf("expected silent allow under yolo for non-arbitrary shell capability, got %+v", decision)
	}
}

// --- capability decision matrix: readonly/write/shell x primary/subagent ---

func TestCheckPermission_DecisionMatrix(t *testing.T) {
	root, _ := newTestProject(t)
	target := filepath.Join(root, "inside.txt")
	approve := func(string) (bool, error) { return true, nil }

	cases := []struct {
		name       string
		role       Role
		level      Level
		capability Capability
		cfg        PermConfig
		wantAllow  bool
	}{
		{"primary readonly always allowed", RolePrimary, LevelReadonly, CapFilesystemRead, PermConfig{}, true},
		{"subagent readonly always allowed", RoleSubagent, LevelReadonly, CapFilesystemRead, PermConfig{}, true},
		{"primary write auto mode allowed", RolePrimary, LevelWrite, CapFilesystemWrite, PermConfig{WriteMode: "auto"}, true},
		{"primary write ask_always needs ask (approved)", RolePrimary, LevelWrite, CapFilesystemWrite, PermConfig{WriteMode: "ask_once_per_file"}, true},
		{"subagent write auto mode allowed", RoleSubagent, LevelWrite, CapFilesystemWrite, PermConfig{WriteMode: "auto"}, true},
		{"primary shell.arbitrary with approval allowed", RolePrimary, LevelShell, CapShellArbitrary, PermConfig{ShellMode: "yolo"}, true},
		{"subagent shell.arbitrary never allowed even with approval", RoleSubagent, LevelShell, CapShellArbitrary, PermConfig{ShellMode: "yolo"}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := NewAgentContext("sess-1", root, false, tc.role)
			tracker := NewApprovalTracker()
			req := Request{Level: tc.level, Capability: tc.capability, Path: target}
			decision, err := CheckPermission(ctx, tc.cfg, tracker, approve, root, req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision.Allow != tc.wantAllow {
				t.Errorf("Allow = %v, want %v (decision=%+v)", decision.Allow, tc.wantAllow, decision)
			}
		})
	}
}

// --- AC-04: subagent context cannot escalate to a capability outside its allowlist ---

func TestSubagentContext_CannotEscalateHighRiskCapabilities(t *testing.T) {
	root, _ := newTestProject(t)

	for cap := range subagentDeniedCaps {
		cap := cap
		t.Run(string(cap), func(t *testing.T) {
			ctx := NewAgentContext("sess-1", root, true /* yolo */, RoleSubagent)

			if ctx.CapabilityAllowed(cap) {
				t.Fatalf("subagent context should not start with %s allowed", cap)
			}

			// Attempting to Grant it directly must be refused.
			if ok := ctx.Grant(cap, true); ok {
				t.Fatalf("Grant(%s, true) should be refused for a subagent context", cap)
			}
			if ctx.CapabilityAllowed(cap) {
				t.Fatalf("%s must still be denied after a refused Grant", cap)
			}

			// The YOLO capability-gate ask path in CheckPermission must not
			// be able to escalate it either, even if the user "approves".
			approve := func(string) (bool, error) { return true, nil }
			decision, err := CheckPermission(ctx, PermConfig{}, nil, approve, root, Request{
				Level:      LevelReadonly,
				Capability: cap,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if decision.Allow {
				t.Fatalf("CheckPermission should not allow a subagent to escalate %s, got %+v", cap, decision)
			}
		})
	}
}

func TestPrimaryContext_CanEscalateViaYoloCapabilityGate(t *testing.T) {
	root, _ := newTestProject(t)
	ctx := NewAgentContext("sess-1", root, true /* yolo */, RolePrimary)

	if ctx.CapabilityAllowed(CapProcessExecute) {
		t.Fatal("process.execute should not start granted")
	}

	approve := func(string) (bool, error) { return true, nil }
	decision, err := CheckPermission(ctx, PermConfig{}, nil, approve, root, Request{
		Level:      LevelReadonly,
		Capability: CapProcessExecute,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allow {
		t.Fatalf("primary context should be able to escalate process.execute via YOLO capability gate, got %+v", decision)
	}
	if !ctx.CapabilityAllowed(CapProcessExecute) {
		t.Fatal("capability should now be granted for the rest of the session")
	}
}

// --- ask_once_per_file dedup (25-perm_write.zsh) ---

func TestCheckPermission_WriteAskOncePerFileDedup(t *testing.T) {
	root, _ := newTestProject(t)
	ctx := NewAgentContext("sess-1", root, false, RolePrimary)
	cfg := PermConfig{WriteMode: "ask_once_per_file"}
	tracker := NewApprovalTracker()
	target := filepath.Join(root, "inside.txt")

	askCount := 0
	approve := func(string) (bool, error) {
		askCount++
		return true, nil
	}
	req := Request{Level: LevelWrite, Capability: CapFilesystemWrite, Path: target}

	d1, err := CheckPermission(ctx, cfg, tracker, approve, root, req)
	if err != nil || !d1.Allow {
		t.Fatalf("first write should be allowed after approval: %+v, err=%v", d1, err)
	}
	if askCount != 1 {
		t.Fatalf("expected exactly 1 ask on first write, got %d", askCount)
	}

	d2, err := CheckPermission(ctx, cfg, tracker, approve, root, req)
	if err != nil || !d2.Allow {
		t.Fatalf("second write to same file should be allowed: %+v, err=%v", d2, err)
	}
	if askCount != 1 {
		t.Fatalf("second write to same file should NOT re-ask, askCount=%d", askCount)
	}
	if d2.Asked {
		t.Fatal("second write should be allowed without asking (dedup)")
	}
}

// --- readonly is never asked, even without an AskFunc ---

func TestCheckPermission_ReadonlyNeverAsks(t *testing.T) {
	root, _ := newTestProject(t)
	ctx := NewAgentContext("sess-1", root, false, RolePrimary)

	decision, err := CheckPermission(ctx, PermConfig{}, nil, nil, root, Request{
		Level:      LevelReadonly,
		Capability: CapFilesystemRead,
		Path:       filepath.Join(root, "inside.txt"),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !decision.Allow || decision.Asked {
		t.Fatalf("readonly should allow without asking, got %+v", decision)
	}
}
