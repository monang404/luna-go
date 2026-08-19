// Traceability: 30-luna/60-ui/components/progress.zsh -> progress.go.
package components

import (
	"fmt"
	"strings"

	"github.com/monang404/luna-go/internal/ui"
)

// Progress is the port of ui_progress(current?, total?, message?).
func Progress(current, total int, message string, mode ui.Mode) string {
	t := mode.Tokens
	if total <= 0 {
		total = 1
	}
	if current < 0 {
		current = 0
	}
	if current > total {
		current = total
	}

	const barLen = 20
	filled := barLen * current / total
	empty := barLen - filled

	fillChar, emptyChar := "#", "."
	if mode.Unicode {
		fillChar, emptyChar = "█", "░"
	}
	bar := strings.Repeat(fillChar, filled) + strings.Repeat(emptyChar, empty)

	if message != "" {
		return fmt.Sprintf("%s●%s %s  %s[%d/%d]%s  %s%s%s\n",
			t.Info, t.Reset, message, t.Muted, current, total, t.Reset, t.OK, bar, t.Reset)
	}
	return fmt.Sprintf("%s[%d/%d]%s  %s%s%s\n",
		t.Muted, current, total, t.Reset, t.OK, bar, t.Reset)
}
