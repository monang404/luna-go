package aiops

import "context"

// Clipboard is the injectable replacement for termux-clipboard-get/-set,
// used by aiclip (chat), aiprompt/aispec (workflow). Real Termux/Android
// clipboard access is a SESSION-55 (CLI wiring) concern.
type Clipboard interface {
	Get(ctx context.Context) (string, error)
	Set(ctx context.Context, content string) error
}

// NoClipboard is a Clipboard that reports "unavailable", matching the
// zsh source's `command -v termux-clipboard-get >/dev/null` guard
// failing (Termux API not installed / not on Termux at all).
type NoClipboard struct{}

// ErrClipboardUnavailable is returned by NoClipboard's methods.
var ErrClipboardUnavailable = clipboardUnavailableError{}

type clipboardUnavailableError struct{}

func (clipboardUnavailableError) Error() string { return "aiops: clipboard unavailable" }

func (NoClipboard) Get(context.Context) (string, error) { return "", ErrClipboardUnavailable }
func (NoClipboard) Set(context.Context, string) error   { return ErrClipboardUnavailable }

// ShareFunc is the injectable replacement for `termux-share -a send
// <file>` (aishare, internal/filepatch).
type ShareFunc func(ctx context.Context, path string) error

// CommandRunner is the injectable replacement for the handful of
// external processes the ported commands shell out to (git, python3,
// jq-equivalent-free JSON handling aside) -- used by internal/workflow's
// aicommit (git) and internal/codeproject's airun (python3)/autotest
// (python3 -m py_compile).
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (stdout string, stderr string, exitCode int, err error)
}
