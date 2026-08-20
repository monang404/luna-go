// Command luna is the entrypoint for LUNA (the Go rewrite of the
// original zsh plugin).
//
// SESSION-40 proved the build/cross-compile pipeline with an empty
// binary. SESSION-42..54 ported every command's business logic into
// internal/ packages behind injectable seams (aiops.Completer,
// aiops.ConfirmFunc, aiops.CommandRunner, aiops.Clipboard) so it could
// be unit tested without a real terminal or a live API key.
//
// SESSION-55 is the first session to wire all of that into something
// runnable: cmd/luna/commands builds one cobra subcommand per
// legacy alias (aiask, aiagent, aipatch, aicommit, aiundo, ...) on top
// of real terminal-facing implementations of those seams. See
// docs/execution_sessions/55_wire_cli_entrypoint.yaml and
// docs/MIGRATION_TRACEABILITY.md's SESSION-55 entry for the full
// mapping and any flagged deviations.
package main

import "github.com/monang404/luna-go/cmd/luna/commands"

// version is overridden at build time via:
//
//	go build -ldflags "-X main.version=$(git describe --tags --always --dirty)"
var version = "dev"

func main() {
	commands.Execute(version)
}
