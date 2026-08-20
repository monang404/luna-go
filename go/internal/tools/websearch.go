package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// WebSearchTool implements a zero-config web search using DuckDuckGo Lite.
type WebSearchTool struct {
	Deps PermDeps
}

func (t *WebSearchTool) Name() string { return "web_search" }
func (t *WebSearchTool) Description() string {
	return "Search the web for up-to-date information, news, or documentation."
}

func (t *WebSearchTool) Execute(ctx context.Context, args json.RawMessage) (string, error) {
	query := ExtractField(args, "query")
	if query == "" {
		return "", fmt.Errorf("web_search: parameter 'query' wajib diisi")
	}

	reqBody := "q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, "POST", "https://lite.duckduckgo.com/lite/", strings.NewReader(reqBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("gagal menghubungi search engine: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("search engine mengembalikan HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Strip HTML and decode entities
	text := StripHTML(string(body))
	if text == "" {
		return "Tidak ditemukan hasil atau gagal mem-parsing halaman.", nil
	}

	// We'll truncate it if it's too long, but usually DuckDuckGo Lite text is under 15k characters.
	if len(text) > 15000 {
		text = text[:15000] + "\n... (truncated)"
	}

	return fmt.Sprintf("=== Hasil Pencarian Web untuk '%s' ===\n%s", query, text), nil
}
