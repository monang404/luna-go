package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

// Client manages communication with a single MCP server over a Transport.
type Client struct {
	transport Transport
	nextReqID int64

	mu       sync.Mutex
	pending  map[int64]chan *JSONRPCMessage
	loopDone chan struct{}
	err      error
}

// NewClient creates a new Client from a Transport.
func NewClient(transport Transport) *Client {
	c := &Client{
		transport: transport,
		pending:   make(map[int64]chan *JSONRPCMessage),
		loopDone:  make(chan struct{}),
	}
	go c.readLoop()
	return c
}

// Close gracefully (or forcefully) stops the client.
func (c *Client) Close() {
	c.transport.Close()
	<-c.loopDone
}

func (c *Client) readLoop() {
	defer close(c.loopDone)
	for {
		msg, err := c.transport.Receive(context.Background())
		if err != nil {
			c.mu.Lock()
			c.err = fmt.Errorf("client disconnected: %v", err)
			for _, ch := range c.pending {
				close(ch)
			}
			c.pending = make(map[int64]chan *JSONRPCMessage)
			c.mu.Unlock()
			return
		}

		if msg.ID != nil {
			c.mu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.mu.Unlock()

			if ok {
				ch <- msg
			}
		}
	}
}

// sendRequest sends a JSON-RPC request and waits for the response.
func (c *Client) sendRequest(method string, params interface{}) (*JSONRPCMessage, error) {
	c.mu.Lock()
	if c.err != nil {
		c.mu.Unlock()
		return nil, c.err
	}
	reqID := atomic.AddInt64(&c.nextReqID, 1)
	ch := make(chan *JSONRPCMessage, 1)
	c.pending[reqID] = ch
	c.mu.Unlock()

	paramsRaw, _ := json.Marshal(params)
	req := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      &reqID,
		Method:  method,
		Params:  paramsRaw,
	}

	if err := c.transport.Send(context.Background(), &req); err != nil {
		c.mu.Lock()
		delete(c.pending, reqID)
		c.mu.Unlock()
		return nil, err
	}

	resp := <-ch
	if resp == nil {
		return nil, fmt.Errorf("connection closed while waiting for response")
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("RPC Error %d: %s", resp.Error.Code, resp.Error.Message)
	}
	return resp, nil
}

// Initialize performs the MCP initialization handshake.
func (c *Client) Initialize() error {
	params := InitializeParams{
		ProtocolVersion: "2024-11-05", // recent MCP version
	}
	params.ClientInfo.Name = "luna-go"
	params.ClientInfo.Version = "0.1.0"

	resp, err := c.sendRequest("initialize", params)
	if err != nil {
		return err
	}
	
	// Notify initialized
	notification := JSONRPCMessage{
		JSONRPC: "2.0",
		Method:  "notifications/initialized",
	}
	c.transport.Send(context.Background(), &notification)

	var res InitializeResult
	return json.Unmarshal(resp.Result, &res)
}

// ListTools retrieves the list of tools from the server.
func (c *Client) ListTools() ([]Tool, error) {
	resp, err := c.sendRequest("tools/list", map[string]interface{}{})
	if err != nil {
		return nil, err
	}
	var res ToolsListResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		return nil, err
	}
	return res.Tools, nil
}

// CallTool invokes a specific tool on the server.
func (c *Client) CallTool(name string, args json.RawMessage) (string, error) {
	params := CallToolParams{
		Name:      name,
		Arguments: args,
	}
	resp, err := c.sendRequest("tools/call", params)
	if err != nil {
		return "", err
	}
	var res CallToolResult
	if err := json.Unmarshal(resp.Result, &res); err != nil {
		return "", err
	}
	if res.IsError {
		// Attempt to format the error
		if len(res.Content) > 0 {
			return "", fmt.Errorf("tool execution failed: %s", res.Content[0].Text)
		}
		return "", fmt.Errorf("tool execution failed")
	}

	// Combine all text content
	out := ""
	for _, cnt := range res.Content {
		if cnt.Type == "text" {
			out += cnt.Text
		}
	}
	return out, nil
}
