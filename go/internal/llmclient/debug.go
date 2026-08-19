// debug.go wires up the AIOPS_DEBUG diagnostics surface described in
// this repo's audit brief (docs/execution_sessions -- "debug mode
// tidak menjelaskan root cause" / P3 finding): AIOPS_DEBUG=1 was
// referenced by users and by the CLI's own help text but was never
// actually read anywhere in the Go source (grep for AIOPS_DEBUG across
// internal/ and cmd/ before this file returned zero hits). This file
// is the single place that env var is read; every other package that
// wants debug output goes through Debugf/DebugEnabled below instead of
// re-reading the env var itself, so the on/off decision has one source
// of truth.
//
// Hard rule (matches the audit brief's own section 9/22 instructions):
// never print an API key value. Debugf callers pass "configured=true"/
// "configured=false", never the key itself -- see DescribeKey below,
// the one helper that is allowed to look at a key's presence.
package llmclient

import (
	"fmt"
	"os"
)

// DebugEnabled reports whether AIOPS_DEBUG is set to a truthy value.
// Matches the shell convention the zsh source and this repo's own CLI
// examples use (AIOPS_DEBUG=1 luna ...): "" and "0" are off,
// anything else is on.
func DebugEnabled() bool {
	v := os.Getenv("AIOPS_DEBUG")
	return v != "" && v != "0"
}

// Debugf writes one diagnostic line to stderr, prefixed consistently,
// but only when AIOPS_DEBUG is enabled. Safe to call unconditionally
// from hot paths (candidate selection, HTTP call sites) since it's a
// no-op otherwise.
func Debugf(format string, args ...any) {
	if !DebugEnabled() {
		return
	}
	fmt.Fprintf(os.Stderr, "[aiops-debug] "+format+"\n", args...)
}

// DescribeKey reports "configured" or "missing" for a provider's key
// var without ever touching the key's value -- the only shape of API
// key information this package's debug output is allowed to emit (see
// this file's package doc and the audit brief's section 9/22).
func DescribeKey(keyVar string) string {
	if os.Getenv(keyVar) != "" {
		return "configured"
	}
	return "missing"
}
