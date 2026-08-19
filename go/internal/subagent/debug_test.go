package subagent

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/monang404/luna-go/internal/permission"
)

func TestRunDebug_Success(t *testing.T) {
	calls := 0
	deps := baseDeps(t, nil, countingTool{name: "run_test", cap: permission.CapProcessTest, calls: &calls})
	deps.Complete = scriptedComplete(
		`{"thought":"reproducing","tool":"run_test","args":{},"done":false}`,
		`{"thought":"root cause is a nil pointer in foo.go:42","tool":"","args":{},"done":true}`,
	)

	report, err := RunDebug(context.Background(), deps, "crash on startup")
	if err != nil {
		t.Fatalf("RunDebug error: %v", err)
	}
	if !report.Success {
		t.Fatalf("report.Success = false, want true (Error=%q)", report.Error)
	}
	if report.Diagnosis != "root cause is a nil pointer in foo.go:42" {
		t.Fatalf("Diagnosis = %q", report.Diagnosis)
	}
	if calls != 1 {
		t.Fatalf("run_test called %d times, want 1", calls)
	}
}

func TestRunDebug_EmptyProblem(t *testing.T) {
	deps := baseDeps(t, nil)
	report, err := RunDebug(context.Background(), deps, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Error == "" {
		t.Fatal("expected Error for empty problem")
	}
}

// AC-04: mutation tools are never reachable from `luna debug`, even if the
// model asks for one -- 25-debug_allowlist.zsh's guard.
func TestRunDebug_MutationToolRejected(t *testing.T) {
	calls := 0
	deps := baseDeps(t, nil, countingTool{name: "write_file", cap: permission.CapFilesystemWrite, calls: &calls})
	target := filepath.Join(deps.Cwd, "a.txt")
	deps.Complete = scriptedComplete(
		fmt.Sprintf(`{"thought":"fixing","tool":"write_file","args":{"path":%q},"done":false}`, target),
	)
	deps.Limits.AgentMaxSameFail = 1

	report, err := RunDebug(context.Background(), deps, "fix the bug")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report.Success {
		t.Fatal("report.Success = true, want false (write_file must never succeed in luna debug)")
	}
	if calls != 0 {
		t.Fatalf("write_file.Execute called %d times, want 0", calls)
	}
}

func TestDebugToolAllowed(t *testing.T) {
	cases := []struct {
		tool string
		want bool
	}{
		{"read_file", true},
		{"run_command", true},
		{"run_test", true},
		{"write_file", false},
		{"edit_file", false},
		{"delete_file", false},
	}
	for _, c := range cases {
		if got := DebugToolAllowed(c.tool); got != c.want {
			t.Errorf("DebugToolAllowed(%q) = %v, want %v", c.tool, got, c.want)
		}
	}
}
