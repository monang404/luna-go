package mcp

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/monang404/luna-go/internal/config"
)

// DiscoveredTool represents a tool provided by an MCP server along with the client that owns it.
type DiscoveredTool struct {
	Tool   Tool
	Client *Client
}

// Manager manages multiple MCP clients based on the configuration.
type Manager struct {
	mu      sync.Mutex
	clients map[string]*Client
	tools   map[string]DiscoveredTool
}

// NewManager creates a new empty Manager.
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*Client),
		tools:   make(map[string]DiscoveredTool),
	}
}

// StartAndDiscover reads the MCP config, starts all servers, initializes them, and fetches their tools.
// This is typically called lazily or at startup.
func (m *Manager) StartAndDiscover(ctx context.Context, cfg *config.MCPConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for name, srvCfg := range cfg.MCPServers {
		if _, exists := m.clients[name]; exists {
			continue // Already running
		}

		var transport Transport
		var err error

		if srvCfg.URL != "" {
			transport, err = NewHTTPTransport(ctx, srvCfg.URL, srvCfg.Headers)
		} else if srvCfg.Command != "" {
			transport, err = NewStdioTransport(ctx, srvCfg.Command, srvCfg.Args, srvCfg.Env)
		} else {
			log.Printf("[MCP] Server %q has neither command nor url configured", name)
			continue
		}

		if err != nil {
			log.Printf("[MCP] Failed to start server %q: %v", name, err)
			continue
		}

		client := NewClient(transport)

		if err := client.Initialize(); err != nil {
			log.Printf("[MCP] Failed to initialize server %q: %v", name, err)
			client.Close()
			continue
		}

		tools, err := client.ListTools()
		if err != nil {
			log.Printf("[MCP] Failed to list tools for server %q: %v", name, err)
			client.Close()
			continue
		}

		m.clients[name] = client

		for _, t := range tools {
			// Prefix the tool name with server name to avoid collisions?
			// Actually, standard MCP usually exposes them as-is. Let's expose as-is for now,
			// or maybe <server>_<tool>. The roadmap didn't specify prefixing.
			// Claude Code prefixes with server name. Let's use serverName_toolName.
			prefixedName := fmt.Sprintf("%s_%s", name, t.Name)
			t.Name = prefixedName 
			m.tools[prefixedName] = DiscoveredTool{
				Tool:   t,
				Client: client,
			}
		}
	}
	return nil
}

// GetTools returns all discovered tools.
func (m *Manager) GetTools() map[string]DiscoveredTool {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Return a copy to avoid data races
	out := make(map[string]DiscoveredTool)
	for k, v := range m.tools {
		out[k] = v
	}
	return out
}

// Close gracefully stops all clients.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, client := range m.clients {
		client.Close()
	}
	m.clients = make(map[string]*Client)
	m.tools = make(map[string]DiscoveredTool)
}
