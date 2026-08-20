package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/permission"
)

// This file ports 30-luna/05-tools/45-tool_web_fetch.zsh (_ai_tool_web_fetch):
// fetch a URL, guard against SSRF, strip HTML down to plain text, cap the
// result. The zsh source shells out to curl plus an inline python3 SSRF
// pre-check and a second inline python3 HTML-strip script; this port
// replaces all three with net/http + net + a Go regexp-based stripHTML,
// so there is exactly one process (this Go binary) doing DNS resolution,
// the connection, and the text extraction -- no curl/python3 dependency
// at runtime, and no window between "resolve" and "connect" for a
// DNS-rebinding attacker to exploit (the resolved IP is reused directly
// for the dial, mirroring curl's own --resolve host:port:ip pinning).

// webFetchURLFields mirrors _ai_tool_extract_field's own field list for
// this tool (`url link href`) -- NormalizeArgs (args.go) already folds
// link/href into "url" before Execute ever runs for tool name
// "web_fetch", but ExtractField is used directly here too so this
// function is self-contained even if called outside the Dispatcher
// (e.g. a future direct unit test), same defensive posture as the fs
// tools in this package.
func webFetchURLFields(args json.RawMessage) string {
	return ExtractField(args, "url", "link", "href")
}

// WebFetchTool implements _ai_tool_web_fetch.
type WebFetchTool struct{}

func (WebFetchTool) Name() string                      { return "web_fetch" }
func (WebFetchTool) Capability() permission.Capability { return Registry["web_fetch"].Capability }

func (WebFetchTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	rawURL := webFetchURLFields(args)
	if rawURL == "" {
		return Result{}, fmt.Errorf("ERROR: web_fetch membutuhkan args.url (string non-empty). Diterima: %s", firstNChars(string(args), 200))
	}
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return Result{}, fmt.Errorf("ERROR: cuma skema http/https yang diizinkan")
	}

	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" || u.User != nil {
		return Result{}, fmt.Errorf("ERROR: DENY: invalid URL authority")
	}

	host := u.Hostname()
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	resolvedIP, err := resolveSafePublicAddr(host)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: %s", err.Error())
	}

	limits := config.LoadLimits()
	client := &http.Client{
		Timeout: time.Duration(limits.WebFetchTimeoutSec) * time.Second,
		// --max-redirs 0: never follow a redirect (a redirect response
		// itself is still returned to the caller as-is).
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
		Transport: &http.Transport{
			// Pin the actual TCP connection to the already-validated IP
			// (curl's --resolve host:port:ip), so nothing between the
			// SSRF check above and the request below can re-resolve the
			// hostname to a different (unvalidated) address.
			DialContext: func(dialCtx context.Context, network, addr string) (net.Conn, error) {
				_, reqPort, splitErr := net.SplitHostPort(addr)
				if splitErr != nil {
					reqPort = port
				}
				d := net.Dialer{}
				return d.DialContext(dialCtx, network, net.JoinHostPort(resolvedIP, reqPort))
			},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membangun request: %w", err)
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (agent-zsh)")

	resp, err := client.Do(req)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: curl gagal: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body) // a partial read (e.g. connection reset mid-body) still gets stripped/capped below rather than discarded outright.

	text := StripHTML(string(body))
	if text == "" {
		text = "(kosong setelah strip HTML -- mungkin bukan halaman HTML biasa)"
	}
	return Result{Output: firstNChars(text, limits.WebFetchMaxChars)}, nil
}

// --- SSRF guard (resolveSafePublicAddr) ---

// resolveSafePublicAddr mirrors the inline python3 SSRF pre-check:
// resolve host, reject if every candidate address is private, loopback,
// link-local, multicast, unspecified, or otherwise reserved; else return
// the first public address found (the zsh source's own `public[0]`).
func resolveSafePublicAddr(host string) (string, error) {
	ips, err := net.LookupIP(host)
	if err != nil || len(ips) == 0 {
		return "", fmt.Errorf("DENY: DNS resolution failed")
	}
	for _, ip := range ips {
		if isUnsafeIP(ip) {
			return "", fmt.Errorf("DENY: target resolves to a non-public address")
		}
	}
	return ips[0].String(), nil
}

// isUnsafeIP mirrors the Python ipaddress predicate combination
// `is_private or is_loopback or is_link_local or is_multicast or
// is_reserved or is_unspecified`. Go's net.IP has no single is_reserved
// equivalent, so IsPrivate (RFC 1918 + RFC 4193) plus the explicit
// checks below cover the same practical SSRF surface (loopback,
// link-local unicast/multicast, other multicast, unspecified/"any"); see
// this function's own doc for the one known gap this leaves relative to
// Python's broader is_reserved (e.g. some IETF-reserved-but-not-private
// ranges), noted rather than silently assumed equivalent.
func isUnsafeIP(ip net.IP) bool {
	return ip.IsPrivate() ||
		ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

// --- HTML stripping (stripHTML) ---

// scriptStyleTag mirrors the python strip_script's
// `<(script|style)[^>]*>.*?</\1>` -- RE2 (Go's regexp) has no
// backreference support, so the closing tag is matched against the same
// `(?:script|style)` alternation again rather than literally the same
// name captured by the opening tag. In the rare case of deliberately
// mismatched tags (`<script>...</style>`) this strips a little more
// than the python version would -- strictly more conservative for an
// HTML-to-text tool, never less.
var (
	scriptStyleTag = regexp.MustCompile(`(?is)<(script|style)[^>]*>.*?</\s*(?:script|style)\s*>`)
	anyTag         = regexp.MustCompile(`(?s)<[^>]+>`)
	runsOfSpaces   = regexp.MustCompile(`[ \t]+`)
	blankLines     = regexp.MustCompile(`\n\s*\n+`)
)

// stripHTML mirrors the zsh source's inline python3 strip_script
// exactly, step for step: drop <script>/<style> blocks (and their
// content) first, strip every remaining tag, HTML-entity-unescape,
// collapse runs of spaces/tabs to one, collapse blank-line runs to a
// single blank line, then trim.
func StripHTML(raw string) string {
	raw = scriptStyleTag.ReplaceAllString(raw, " ")
	raw = anyTag.ReplaceAllString(raw, " ")
	raw = html.UnescapeString(raw)
	raw = runsOfSpaces.ReplaceAllString(raw, " ")
	raw = blankLines.ReplaceAllString(raw, "\n\n")
	return strings.TrimSpace(raw)
}
