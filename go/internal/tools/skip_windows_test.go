package tools
import "runtime"
import "testing"
func skipOnWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("skipping test that requires POSIX tools (ls, grep, find, sh)")
	}
}

