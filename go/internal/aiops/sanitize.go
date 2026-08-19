package aiops

import (
	"context"
	"os"
	"strings"
)

// SanitizePyCode mirrors _ai_sanitize_pycode (10-core/25-quick_chat.zsh):
// best-effort auto-repair of literal-\n leakage in LUNA-generated Python,
// by shelling out to scripts/ai_code_sanitize.py via runner. It is a
// no-op (never returns an error the caller must act on -- the zsh
// source's own contract is `return 0` always) when: the file doesn't
// exist, doesn't look like a .py file (allowing a ".fixed" suffix,
// matching aifix's own file naming), python3/the script aren't
// available, or runner is nil.
//
// extraArgs lets callers pass mode flags (aiproject's own call site
// uses "--normalize-markers" before the file argument).
func SanitizePyCode(ctx context.Context, runner CommandRunner, scriptPath, file string, extraArgs ...string) {
	if runner == nil || scriptPath == "" || file == "" {
		return
	}
	check := strings.TrimSuffix(file, ".fixed")
	if !strings.HasSuffix(check, ".py") {
		return
	}
	if _, err := os.Stat(file); err != nil {
		return
	}
	if _, err := os.Stat(scriptPath); err != nil {
		return
	}
	args := append([]string{scriptPath}, extraArgs...)
	args = append(args, file)
	_, _, _, _ = runner.Run(ctx, "python3", args...)
}
