package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os/exec"
)

// StdioTransport implements Transport over standard input/output of a subprocess.
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser

	scanner *bufio.Scanner
}

// NewStdioTransport starts an MCP server process and returns a Transport.
func NewStdioTransport(ctx context.Context, command string, args []string, env []string) (*StdioTransport, error) {
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

	return &StdioTransport{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		stderr:  stderr,
		scanner: bufio.NewScanner(stdout),
	}, nil
}

// Send sends a JSON-RPC message over stdout.
func (t *StdioTransport) Send(ctx context.Context, msg *JSONRPCMessage) error {
	reqBytes, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	reqBytes = append(reqBytes, '\n')

	_, err = t.stdin.Write(reqBytes)
	return err
}

// Receive blocks until a JSON-RPC message is received or an error occurs.
func (t *StdioTransport) Receive(ctx context.Context) (*JSONRPCMessage, error) {
	for t.scanner.Scan() {
		line := t.scanner.Bytes()
		var msg JSONRPCMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			// Skip malformed lines (could be raw logs)
			continue
		}
		return &msg, nil
	}
	if err := t.scanner.Err(); err != nil {
		return nil, err
	}
	return nil, io.EOF
}

// Close terminates the subprocess.
func (t *StdioTransport) Close() error {
	t.stdin.Close()
	return t.cmd.Process.Kill()
}
