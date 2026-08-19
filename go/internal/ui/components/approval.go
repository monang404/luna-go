// Traceability: 30-luna/60-ui/components/approval.zsh -> approval.go.
//
// Scope note: ui_approve() in zsh is a print-then-read function (renders
// the box, then blocks on `gum choose` / `read -r` for the actual
// approve/deny decision). Terminal input handling is explicitly out of
// scope for SESSION-53 (see scope.exclude in the session spec — that's
// SESSION-55 CLI wiring). ApprovalPrompt below ports only the pure
// rendering half; the caller is responsible for obtaining the decision
// and does not belong in this package.
package components

import "github.com/monang404/luna-go/internal/ui"

// ApprovalPrompt is the port of the `_ai_ui_box "Command requires
// approval" "$command_to_run"` call inside ui_approve(). Uses the
// SESSION-52 Box primitive directly (no new rendering added here).
func ApprovalPrompt(commandToRun string, mode ui.Mode) string {
	return ui.Box("Command requires approval", []string{commandToRun}, mode)
}
