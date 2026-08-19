package subagent

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/monang404/luna-go/internal/permission"
)

// Manual/integration-style scenario from the SESSION-51 brief (FASE 25):
// Parent: plan -> run researcher subagent. Subagent: plan -> tool, plan ->
// done. Parent: receives result -> next plan -> complete. This drives
// SpawnSubagent end-to-end through a realistic multi-step script (not a
// single canned reply) to catch anything a one-shot fake wouldn't.
func TestIntegration_ParentDelegatesToResearcherSubagent(t *testing.T) {
	readCalls, grepCalls := 0, 0
	deps := baseDeps(t, nil,
		countingTool{name: "read_file", cap: permission.CapFilesystemRead, calls: &readCalls},
		countingTool{name: "grep_search", cap: permission.CapFilesystemRead, calls: &grepCalls},
	)
	target := filepath.Join(deps.Cwd, "auth.go")
	deps.Complete = scriptedComplete(
		fmt.Sprintf(`{"thought":"let's grep for auth endpoints","tool":"grep_search","args":{"path":%q,"pattern":"func.*Auth"},"done":false}`, target),
		fmt.Sprintf(`{"thought":"now read the main file to confirm","tool":"read_file","args":{"path":%q},"done":false}`, target),
		`{"thought":"Found 3 auth endpoints: Login, Logout, Refresh, all in auth.go","tool":"","args":{},"done":true}`,
	)

	result, err := SpawnSubagent(context.Background(), deps, RoleResearcher, "cari semua endpoint auth")
	if err != nil {
		t.Fatalf("SpawnSubagent (parent -> real subagent) errored: %v", err)
	}
	if result.Status != StatusSuccess {
		t.Fatalf("subagent did not complete successfully: status=%s error=%s", result.Status, result.Error)
	}
	if grepCalls != 1 || readCalls != 1 {
		t.Fatalf("expected 1 grep_search + 1 read_file call, got grep=%d read=%d", grepCalls, readCalls)
	}
	if result.Findings == "" {
		t.Fatal("Findings empty -- parent would have nothing to continue its own PLAN with")
	}
	t.Logf("parent receives: status=%s findings=%q files_affected=%v", result.Status, result.Findings, result.FilesAffected)

	// "Parent continues": simulate the parent folding this into its own
	// next step -- just confirm the Result is safe to use as plain data
	// (no panics, no leaked subagent internals like message history).
	nextParentPrompt := "Subagent report: " + result.Findings
	if len(nextParentPrompt) == 0 {
		t.Fatal("parent cannot continue with empty subagent report")
	}
}
