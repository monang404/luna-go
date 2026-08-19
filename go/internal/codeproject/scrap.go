package codeproject

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/monang404/luna-go/internal/config"
)

// ErrScrapBadScheme mirrors aiscrap's URL-scheme guard.
var ErrScrapBadScheme = errors.New("codeproject: url must start with http:// or https://")

// anchorRE is a best-effort <a ...>...</a> matcher used to approximate
// the zsh source's BeautifulSoup-based structure sniff (see doc.go for
// why this is an approximation, not a byte-identical port: no
// third-party HTML parser module is available in this build
// environment). It captures the opening tag's attributes and the raw
// inner HTML.
var anchorRE = regexp.MustCompile(`(?is)<a\b([^>]*)>(.*?)</a>`)
var hrefRE = regexp.MustCompile(`(?i)href\s*=\s*["']([^"']*)["']`)
var classRE = regexp.MustCompile(`(?i)class\s*=\s*["']([^"']*)["']`)
var tagRE = regexp.MustCompile(`(?s)<[^>]*>`)

// sniffStructure mirrors aiscrap's inline python: for every `<a
// href=...>` tag, extract its class attribute and visible text; keep
// only entries with text longer than 30 characters, de-duplicated by
// class value, and cap the result at 10 lines (matching `_ai_head_n
// 10`). Each kept entry is rendered as "class: <class> | <text[:50]>",
// exactly matching the python source's print format.
func sniffStructure(html string) string {
	seen := map[string]bool{}
	var lines []string
	for _, m := range anchorRE.FindAllStringSubmatch(html, -1) {
		attrs, inner := m[1], m[2]
		if !hrefRE.MatchString(attrs) {
			continue
		}
		text := strings.TrimSpace(tagRE.ReplaceAllString(inner, ""))
		if len(text) <= 30 {
			continue
		}
		class := "None"
		if cm := classRE.FindStringSubmatch(attrs); cm != nil {
			class = cm[1]
		}
		if seen[class] {
			continue
		}
		seen[class] = true
		if len(text) > 50 {
			text = text[:50]
		}
		lines = append(lines, fmt.Sprintf("class: %s | %s", class, text))
		if len(lines) >= 10 {
			break
		}
	}
	return strings.Join(lines, "\n")
}

// Scrap mirrors aiscrap(url, task): fetch url, sniff its anchor-tag
// structure (see sniffStructure), then ask the model to write a Python
// scraper for it. The generated reply is returned to the caller
// unsanitized -- the zsh source pipes it through
// `python3 $AI_SANITIZE_SCRIPT -` (stdin mode) before printing, but
// aiops.CommandRunner (this package's process-execution seam) only
// supports argument-based invocation, not piping stdin through a
// subprocess; wiring stdin support is left to SESSION-55's CLI layer,
// which can pipe the returned string through the sanitize script itself
// if desired.
func (s *Service) Scrap(ctx context.Context, url, task string) (string, error) {
	if err := needAnyKeyBig(); err != nil {
		return "", err
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		preview := url
		if len(preview) > 40 {
			preview = preview[:40]
		}
		return "", fmt.Errorf("%w (got: %s)", ErrScrapBadScheme, preview)
	}

	structure := fetchAndSniff(ctx, url)

	sysprompt := fmt.Sprintf("Kamu programmer Python expert. Struktur HTML target: %s. Tulis kode langsung tanpa backtick. WAJIB pakai baris baru SUNGGUHAN buat pisah tiap statement di luar string.", structure)
	prompt := fmt.Sprintf("Buat scraper %s: %s", url, task)

	res, err := s.Requester.Complete(ctx, sysprompt, prompt, config.TaskSmart, config.TaskProviderOrderBig, 0)
	if err != nil || res.Content == "" {
		return "", fmt.Errorf("codeproject: scraper generation failed: %w", err)
	}
	return stripFences(res.Content), nil
}

// fetchAndSniff fetches url (15s timeout, matching requests.get's own
// timeout=15) and returns sniffStructure's result. Any fetch error
// yields an empty structure string -- matching the zsh source's `2>
// /dev/null` swallow (aiscrap continues with an empty $structure rather
// than failing outright when the target site is unreachable).
func fetchAndSniff(ctx context.Context, url string) string {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5*1024*1024))
	if err != nil {
		return ""
	}
	return sniffStructure(string(body))
}
