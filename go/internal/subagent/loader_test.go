package subagent

import (
	"strings"
	"testing"
)

func TestParseDefinition_HorizontalRuleInBody(t *testing.T) {
	content := "---\ndescription: test\ntools: read_file\n---\nSystem prompt.\n\n---\n\nBagian setelah horizontal rule harus tetap ada."
	def := parseDefinition("coder", content)
	if !strings.Contains(def.System, "Bagian setelah horizontal rule") {
		t.Errorf("System prompt terpotong: %q", def.System)
	}
}
