package mcp

import "encoding/json"

// JSONRPCMessage represents a base JSON-RPC 2.0 message structure.
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// InitializeParams represents parameters for the "initialize" request.
type InitializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct {
		Roots map[string]interface{} `json:"roots,omitempty"`
	} `json:"capabilities"`
	ClientInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"clientInfo"`
}

// InitializeResult represents the expected result of an "initialize" request.
type InitializeResult struct {
	ProtocolVersion string `json:"protocolVersion"`
	Capabilities    struct {
		Tools map[string]interface{} `json:"tools,omitempty"`
	} `json:"capabilities"`
	ServerInfo struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"serverInfo"`
}

// Tool represents a single tool returned by "tools/list".
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolsListResult represents the result of a "tools/list" request.
type ToolsListResult struct {
	Tools []Tool `json:"tools"`
}

// CallToolParams represents parameters for "tools/call".
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// CallToolResultContent represents one content block in the "tools/call" result.
type CallToolResultContent struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// CallToolResult represents the result of a "tools/call" request.
type CallToolResult struct {
	Content []CallToolResultContent `json:"content"`
	IsError bool                    `json:"isError,omitempty"`
}
