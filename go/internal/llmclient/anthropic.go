package llmclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// anthropicMessage is the shape of a message in Anthropic's API.
type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// buildAnthropicPayload translates the OpenAI-style payload options into Anthropic's format.
func buildAnthropicPayload(messages []Message, opts PayloadOptions) ([]byte, error) {
	var system string
	var anthropicMessages []anthropicMessage

	for _, msg := range messages {
		if msg.Role == "system" {
			if system != "" {
				system += "\n" + msg.Content
			} else {
				system = msg.Content
			}
		} else {
			role := msg.Role
			if role == "tool" {
				role = "user"
			}
			anthropicMessages = append(anthropicMessages, anthropicMessage{
				Role:    role,
				Content: msg.Content,
			})
		}
	}

	if anthropicMessages == nil {
		anthropicMessages = []anthropicMessage{}
	}

	body := map[string]any{
		"model":      opts.Model,
		"messages":   anthropicMessages,
		"max_tokens": opts.MaxTokens,
	}

	if system != "" {
		body["system"] = system
	}

	if opts.Temperature > 0 {
		body["temperature"] = opts.Temperature
	}

	if opts.Stream {
		body["stream"] = true
	}

	return json.Marshal(body)
}

// anthropicResponse mirrors the JSON body returned by Anthropic.
type anthropicResponse struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"`
	Role       string           `json:"role"`
	Content    []map[string]any `json:"content"`
	StopReason string           `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// ParseAnthropicResponse decodes a raw HTTP body into a Response for Anthropic.
func ParseAnthropicResponse(httpStatus int, rawBody []byte) Response {
	resp := Response{HTTPStatus: httpStatus, RawBody: rawBody}

	var w anthropicResponse
	if err := json.Unmarshal(rawBody, &w); err != nil {
		return resp
	}

	resp.Usage = Usage{
		PromptTokens:     w.Usage.InputTokens,
		CompletionTokens: w.Usage.OutputTokens,
		TotalTokens:      w.Usage.InputTokens + w.Usage.OutputTokens,
	}

	if w.Error != nil {
		resp.ErrorMessage = w.Error.Message
	}

	if len(w.Content) > 0 {
		var content strings.Builder
		for _, block := range w.Content {
			if t, ok := block["type"].(string); ok && t == "text" {
				if text, ok := block["text"].(string); ok {
					content.WriteString(text)
				}
			}
		}
		resp.Content = strings.TrimSpace(stripLeakedTrace(content.String()))
	}

	resp.FinishReason = w.StopReason
	return resp
}

// callAnthropicBlocking sends a request to Anthropic's API.
func callAnthropicBlocking(req *http.Request, apiKey string) (Response, error) {
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	httpResp, err := sharedHTTPClient.Do(req)
	if err != nil {
		if req.Context().Err() != nil {
			return Response{}, ErrCancelled
		}
		return Response{}, fmt.Errorf("request failed: %w", err)
	}
	defer httpResp.Body.Close()

	body, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return Response{}, fmt.Errorf("reading response body: %w", err)
	}

	return ParseAnthropicResponse(httpResp.StatusCode, body), nil
}
