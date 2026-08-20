package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"sync/atomic"
)

// Client manages communication with a single MCP server over stdio.
type Client struct {
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	stderr    io.ReadCloser
	nextReqID int64

	mu       sync.Mutex
	pending  map[int64]chan *JSONRPCMessage
	loopDone chan struct{}
	err      error
}

// NewClient starts an MCP server process and returns a connected Client.
func NewClient(ctx context.Context, command string, args []string, env []string) (*Client, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	if len(env) > 0 {
		cmd.Env = append(cmd.Environ(), env...)
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	c := &Client{
		cmd:      cmd,
		stdin:    stdin,
		stdout:   stdout,
		stderr:   stderr,
		pending:  make(map[int64]chan *JSONRPCMessage),
		loopDone: make(chan struct{}),
	}

	go c.readLoop()

	return c, nil
}

// Close gracefully (or forcefully) stops the client.
func (c *Client) Close() {
	c.stdin.Close()
	c.cmd.Process.Kill()
	<-c.loopDone
}

func (c *Client) readLoop() {
	defer close(c.loopDone)
	scanner := bufio.NewScanner(c.stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		var msg JSONRPCMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			continue // skip malformed lines (could be raw logs)
		}

		if msg.ID != nil {
			c.mu.Lock()
			ch, ok := c.pending[*msg.ID]
			if ok {
				delete(c.pending, *msg.ID)
			}
			c.mu.Unlock()

			if ok {
				ch <- &msg
			}
		}
	}
	c.mu.Lock()
	c.err = fmt.Errorf("client disconnected")
	for _, ch := range c.pending {
		close(ch)
	}
	c.pending = make(map[int64]chan *JSONRPCMessage)
	c.mu.Unlock()
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
	reqBytes, _ := json.Marshal(req)
	reqBytes = append(reqBytes, '\n')

	if _, err := c.stdin.Write(reqBytes); err != nil {
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
	nb, _ := json.Marshal(notification)
	nb = append(nb, '\n')
	c.stdin.Write(nb)

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
