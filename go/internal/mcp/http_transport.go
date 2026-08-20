package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

// HTTPTransport implements Transport over HTTP and Server-Sent Events (SSE).
type HTTPTransport struct {
	client     *http.Client
	postURL    string
	headers    map[string]string
	
	resp       *http.Response
	scanner    *bufio.Scanner
	
	mu         sync.RWMutex
	
	msgCh      chan *JSONRPCMessage
	errCh      chan error
	cancel     context.CancelFunc
}

// NewHTTPTransport connects to the given SSE URL and waits for the POST endpoint.
func NewHTTPTransport(ctx context.Context, url string, headers map[string]string) (*HTTPTransport, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "text/event-stream")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	ctx, cancel := context.WithCancel(ctx)
	
	t := &HTTPTransport{
		client:  client,
		headers: headers,
		resp:    resp,
		scanner: bufio.NewScanner(resp.Body),
		msgCh:   make(chan *JSONRPCMessage, 10),
		errCh:   make(chan error, 1),
		cancel:  cancel,
	}

	// We need to wait for the "endpoint" event to know where to POST
	endpointChan := make(chan string, 1)

	go t.readLoop(endpointChan)

	select {
	case endpoint := <-endpointChan:
		t.postURL = t.resolveURL(url, endpoint)
	case err := <-t.errCh:
		t.Close()
		return nil, fmt.Errorf("failed to receive endpoint event: %v", err)
	case <-ctx.Done():
		t.Close()
		return nil, ctx.Err()
	}

	return t, nil
}

func (t *HTTPTransport) resolveURL(base, endpoint string) string {
	// If endpoint is absolute, use it. Otherwise resolve relative to base.
	// For simplicity in this implementation, we assume basic relative or absolute resolution.
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	if strings.HasPrefix(endpoint, "/") {
		// extract origin
		// Not perfectly robust but works for typical cases
		idx := strings.Index(base, "://")
		if idx != -1 {
			originEnd := strings.Index(base[idx+3:], "/")
			if originEnd != -1 {
				origin := base[:idx+3+originEnd]
				return origin + endpoint
			}
		}
		return base + endpoint
	}
	if !strings.HasSuffix(base, "/") {
		base += "/"
	}
	return base + endpoint
}

func (t *HTTPTransport) readLoop(endpointChan chan<- string) {
	defer t.cancel()

	var currentEvent string
	var currentData string

	endpointSent := false

	for t.scanner.Scan() {
		line := t.scanner.Text()
		
		if line == "" {
			// Dispatch event
			if currentEvent == "endpoint" && !endpointSent {
				// The endpoint URL is in data
				endpointChan <- currentData
				endpointSent = true
			} else if currentEvent == "message" {
				var msg JSONRPCMessage
				if err := json.Unmarshal([]byte(currentData), &msg); err == nil {
					t.msgCh <- &msg
				}
			}
			
			currentEvent = "message" // default
			currentData = ""
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			field := parts[0]
			value := parts[1]
			if strings.HasPrefix(value, " ") {
				value = value[1:]
			}
			switch field {
			case "event":
				currentEvent = value
			case "data":
				currentData += value + "\n"
			}
		}
	}
	
	if err := t.scanner.Err(); err != nil {
		t.errCh <- err
	} else {
		t.errCh <- io.EOF
	}
}

// Send sends a JSON-RPC message via HTTP POST.
func (t *HTTPTransport) Send(ctx context.Context, msg *JSONRPCMessage) error {
	t.mu.RLock()
	postURL := t.postURL
	t.mu.RUnlock()
	
	if postURL == "" {
		return fmt.Errorf("HTTP POST endpoint not yet received")
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", postURL, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}

	resp, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("unexpected status code from POST: %d", resp.StatusCode)
	}

	return nil
}

// Receive returns the next message from the SSE stream.
func (t *HTTPTransport) Receive(ctx context.Context) (*JSONRPCMessage, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case msg := <-t.msgCh:
		return msg, nil
	case err := <-t.errCh:
		return nil, err
	}
}

// Close terminates the SSE connection.
func (t *HTTPTransport) Close() error {
	t.cancel()
	if t.resp != nil {
		return t.resp.Body.Close()
	}
	return nil
}
