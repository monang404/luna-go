package subagent

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Definition represents a dynamically loaded subagent from .luna/agents/*.md
type Definition struct {
	Role        Role
	Description string
	Tools       []string
	System      string
}

// Loader manages the loading and retrieval of subagent definitions.
type Loader struct {
	mu       sync.RWMutex
	registry map[Role]*Definition
}

// NewLoader creates a new Loader instance.
func NewLoader() *Loader {
	return &Loader{
		registry: make(map[Role]*Definition),
	}
}

// LoadDefinitions scans the .luna/agents directory and loads all .md files.
// Files should have a simple frontmatter (---) containing:
// description: ...
// tools: tool1, tool2, tool3
//
// The remaining content is the System prompt.
func (l *Loader) LoadDefinitions(agentsDir string) error {
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

		l.mu.Lock()
		l.registry[role] = def
		l.mu.Unlock()
	}
	return nil
}

func parseDefinition(role Role, content string) *Definition {
	def := &Definition{
		Role:  role,
		Tools: []string{}, // default empty
	}

	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		def.System = content
		return def
	}

	lines := strings.Split(content, "\n")
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}

	if closeIdx == -1 {
		// No valid closing frontmatter, treat everything as system prompt
		def.System = content
		return def
	}

	frontmatter := strings.Join(lines[1:closeIdx], "\n")
	
	type Frontmatter struct {
		Description string   `yaml:"description"`
		Tools       []string `yaml:"tools"`
	}
	var fm Frontmatter
	if err := yaml.Unmarshal([]byte(frontmatter), &fm); err == nil {
		def.Description = fm.Description
		def.Tools = fm.Tools
	}

	if closeIdx+1 < len(lines) {
		def.System = strings.TrimSpace(strings.Join(lines[closeIdx+1:], "\n"))
	} else {
		def.System = ""
	}

	return def
}

// GetDefinition returns the loaded definition for the given role, if any.
func (l *Loader) GetDefinition(role Role) *Definition {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.registry[role]
}

// GetAllDefinitions returns all loaded definitions.
func (l *Loader) GetAllDefinitions() []*Definition {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var list []*Definition
	for _, def := range l.registry {
		list = append(list, def)
	}
	return list
}
