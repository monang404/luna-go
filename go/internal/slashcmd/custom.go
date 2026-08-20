package slashcmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// customCommand represents a slash command loaded from a .md file.
type customCommand struct {
	name        string
	description string
	content     string
}

func (c *customCommand) Name() string        { return "/" + c.name }
func (c *customCommand) Aliases() []string   { return nil }
func (c *customCommand) Description() string { return c.description }

// Execute prints a notice and sends the file content as an instruction to the REPL agent loop.
// Note: Actual execution requires injecting the prompt into the REPL turn.
// For now, this just prints what it would do, since we need REPL support to run a turn from here.
// Wait, State interface doesn't have a RunTurn method yet.
func (c *customCommand) Execute(ctx context.Context, args []string, state State) error {
	prompt := c.content
	if len(args) > 0 {
		prompt += "\n\nArgumen tambahan: " + strings.Join(args, " ")
	}

	// Print a notice so the user knows what's happening.
	fmt.Fprintf(state.Out(), "[Menjalankan custom command: %s]\n", c.Name())

	// Pass it to the REPL. We need to add InjectPrompt to State.
	state.InjectPrompt(ctx, prompt)
	return nil
}

// LoadCustomCommands scans the given directory (e.g. .luna/commands) for .md files
// and registers them as custom slash commands.
func LoadCustomCommands(r *Registry, dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".md")

		// The first line can be the description if it starts with #
		content := string(data)
		description := "Custom command dari " + entry.Name()

		lines := strings.SplitN(content, "\n", 2)
		if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[0]), "# ") {
			description = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(lines[0]), "# "))
		}

		r.Register(&customCommand{
			name:        name,
			description: description,
			content:     content,
		})
	}

	return nil
}
