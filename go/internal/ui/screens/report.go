// Traceability: 30-luna/60-ui/screens/report.zsh -> report.go.
// Blueprint v2: compact done report, no big box.
package screens

import (
	"github.com/monang404/luna-go/internal/ui"
	"github.com/monang404/luna-go/internal/ui/components"
)

// Report is the port of ui_report(files_changed?, runtime?, tools_used?,
// next_action?, summary_items...).
func Report(filesChanged, runtime, toolsUsed, nextAction string, items []string, mode ui.Mode) string {
	if filesChanged == "" {
		filesChanged = "0"
	}
	if runtime == "" {
		runtime = "?"
	}
	t := mode.Tokens

	doneSummary := "Files: " + filesChanged
	if toolsUsed != "" {
		doneSummary += "  Tools: " + toolsUsed
	}

	var b []byte
	b = append(b, components.StateDone(doneSummary, runtime, mode).Output...)

	for _, item := range items {
		b = append(b, "  "+t.OK+"✓"+t.Reset+" "+item+"\n"...)
	}

	if nextAction != "" {
		b = append(b, '\n')
		b = append(b, "  "+t.Muted+"Next:"+t.Reset+" "+nextAction+"\n"...)
	}

	b = append(b, '\n')
	return string(b)
}
