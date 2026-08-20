package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Definition represents a dynamically loaded subagent from .luna/agents/*.md
type Definition struct {
	Role        Role
	Description string
	Tools       []string
	System      string
}

var (
	registryMu sync.RWMutex
	registry   = make(map[Role]*Definition)
)

// LoadDefinitions scans the .luna/agents directory and loads all .md files.
// Files should have a simple frontmatter (---) containing:
// description: ...
// tools: tool1, tool2, tool3
//
// The remaining content is the System prompt.
func LoadDefinitions(agentsDir string) error {
	entries, err := os.ReadDir(agentsDir)
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

		path := filepath.Join(agentsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		role := Role(strings.TrimSuffix(entry.Name(), ".md"))
		def := parseDefinition(role, string(data))

		registryMu.Lock()
		registry[role] = def
		registryMu.Unlock()
	}
	return nil
}

func parseDefinition(role Role, content string) *Definition {
	def := &Definition{
		Role:  role,
		Tools: []string{}, // default empty
	}

	content = strings.TrimSpace(content)

	// Check for frontmatter
	if strings.HasPrefix(content, "---") {
		parts := strings.SplitN(content, "---", 3)
		if len(parts) >= 3 {
			frontmatter := parts[1]
			def.System = strings.TrimSpace(parts[2])

			lines := strings.Split(frontmatter, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				if strings.HasPrefix(line, "description:") {
					def.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
				} else if strings.HasPrefix(line, "tools:") {
					toolsStr := strings.TrimSpace(strings.TrimPrefix(line, "tools:"))
					toolsStr = strings.Trim(toolsStr, "[]")
					for _, t := range strings.Split(toolsStr, ",") {
						t = strings.TrimSpace(t)
						if t != "" {
							def.Tools = append(def.Tools, t)
						}
					}
				}
			}
			return def
		}
	}

	// No frontmatter, everything is system prompt
	def.System = content
	return def
}

// GetDefinition returns the loaded definition for the given role, if any.
func GetDefinition(role Role) *Definition {
	registryMu.RLock()
	defer registryMu.RUnlock()
	return registry[role]
}

// GetAllDefinitions returns all loaded definitions.
func GetAllDefinitions() []*Definition {
	registryMu.RLock()
	defer registryMu.RUnlock()

	var list []*Definition
	for _, def := range registry {
		list = append(list, def)
	}
	return list
}
