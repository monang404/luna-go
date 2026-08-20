package mcp

import (
	"context"
)

// Transport defines the communication layer for MCP JSON-RPC messages.
type Transport interface {
	Send(ctx context.Context, msg *JSONRPCMessage) error
	Receive(ctx context.Context) (*JSONRPCMessage, error)
	Close() error
}
