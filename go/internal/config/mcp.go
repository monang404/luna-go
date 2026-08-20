package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// MCPServerConfig represents the configuration for a single MCP server.
type MCPServerConfig struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Env     []string `json:"env,omitempty"`
}

// MCPConfig represents the overall mcp.json configuration file.
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// DefaultMCPConfigPath returns the default path for mcp.json (~/.luna/mcp.json).
func DefaultMCPConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".luna", "mcp.json")
}

// LoadMCPConfig reads the MCP configuration from the given path.
func LoadMCPConfig(path string) (*MCPConfig, error) {
	if path == "" {
		path = DefaultMCPConfigPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &MCPConfig{MCPServers: make(map[string]MCPServerConfig)}, nil
		}
		return nil, err
	}

	var config MCPConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	if config.MCPServers == nil {
		config.MCPServers = make(map[string]MCPServerConfig)
	}

	return &config, nil
}
