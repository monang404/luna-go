package aiops

import "fmt"

// DiffMaxChars mirrors ${AI_DIFF_MAX_CHARS:-15000}.
const DiffMaxChars = 15000

// GuardDiff mirrors _ai_guard_diff(diff, statinfo): truncate an
// oversized diff before it's sent to the model, replacing the tail with
// a `git diff --stat`-style summary so the model still has SOME signal
// about what changed even when the raw diff itself is too large to
// send whole. Used by aicommit and aireview (both operate on `git diff`
// output, which can be arbitrarily large).
func GuardDiff(diff, statInfo string) string {
	if len(diff) <= DiffMaxChars {
		return diff
	}
	return fmt.Sprintf(
		"[diff kegedean (%d char), dipotong ke %d char. Ringkasan file yang berubah:]\n%s\n\n%s\n\n[...diff dipotong di sini, sisanya gak ditampilkan...]",
		len(diff), DiffMaxChars, statInfo, diff[:DiffMaxChars],
	)
}
