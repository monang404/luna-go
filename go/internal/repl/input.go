package repl

import (
	"github.com/chzyer/readline"
)

// newLineReader constructs a readline instance configured for the LUNA CLI.
// It provides history navigation, typical line-editing shortcuts, and
// distinguishes interrupts from EOFs.
func newLineReader(historyFile string) (*readline.Instance, error) {
	return readline.NewEx(&readline.Config{
		Prompt:          "\033[36m❯\033[0m ",
		HistoryFile:     historyFile, // Persists between sessions
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
}
