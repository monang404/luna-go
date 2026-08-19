package aiops

import "context"

// Decision mirrors _ai_confirm's three-way exit code contract
// (10-core/32-confirm.zsh, referenced throughout 35-files/10-aipatch.zsh,
// 30-code/05-code.zsh, 30-code/45-fix.zsh, 40-workflow/00-aicommit.zsh,
// 35-files/15-aiundo.zsh, 35-files/20-aibakclean.zsh): 0 = approved, 2 =
// timed out (treated as cancel, but reported distinctly to the user), any
// other non-zero = explicit decline.
type Decision int

const (
	// Approved mirrors exit code 0.
	Approved Decision = iota
	// Declined mirrors any non-zero, non-2 exit code (explicit "no").
	Declined
	// TimedOut mirrors exit code 2 -- callers print a distinct "timeout,
	// dianggap batal" message rather than the plain "Dibatalkan." used
	// for Declined, so this is kept separate from Declined rather than
	// collapsed into one generic "not approved" value.
	TimedOut
)

// ConfirmFunc is the injectable replacement for _ai_confirm(prompt): the
// interactive gum/read confirmation prompt every destructive operation
// in 35-files/ and 40-workflow/00-aicommit.zsh goes through. Real
// terminal confirmation is a SESSION-55 (CLI wiring) concern; this
// package only defines the contract so command logic can be exercised
// (and unit tested) without a real terminal.
type ConfirmFunc func(ctx context.Context, prompt string) (Decision, error)

// AutoApprove is a ConfirmFunc that always approves -- useful for
// non-interactive callers (tests, --force-style automation) that have
// already decided to proceed.
func AutoApprove(_ context.Context, _ string) (Decision, error) {
	return Approved, nil
}

// AutoDecline is a ConfirmFunc that always declines -- useful as a safe
// default/test double for code paths that must never apply a change
// unless a real confirmation function is wired in.
func AutoDecline(_ context.Context, _ string) (Decision, error) {
	return Declined, nil
}
