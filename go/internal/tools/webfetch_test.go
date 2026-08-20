package tools

import (
	"context"
	"net"
	"strings"
	"testing"
)

// --- AC-02: StripHTML against 2+ real-shaped HTML fixtures ---

func TestStripHTML_SimplePage(t *testing.T) {
	html := `<!DOCTYPE html>
<html>
<head><title>Hello</title></head>
<body>
  <h1>Welcome</h1>
  <p>This is a <b>simple</b> paragraph with a <a href="/x">link</a>.</p>
</body>
</html>`

	got := StripHTML(html)
	if strings.Contains(got, "<") || strings.Contains(got, ">") {
		t.Errorf("StripHTML left raw tag markers: %q", got)
	}
	for _, want := range []string{"Welcome", "simple", "paragraph", "link"} {
		if !strings.Contains(got, want) {
			t.Errorf("StripHTML(simple page) missing %q, got: %q", want, got)
		}
	}
}

func TestStripHTML_ScriptAndStyleStripped(t *testing.T) {
	html := `<html><head>
<style>body { color: red; font-family: "Comic Sans"; }</style>
<script>var secret = "should not appear"; alert(1);</script>
</head>
<body>
<p>Visible text.</p>
<script type="text/javascript">
  function evil() { document.cookie = "steal me"; }
</script>
</body></html>`

	got := StripHTML(html)
	if strings.Contains(got, "should not appear") || strings.Contains(got, "evil") || strings.Contains(got, "steal me") {
		t.Errorf("StripHTML should have removed <script> content entirely, got: %q", got)
	}
	if strings.Contains(got, "Comic Sans") || strings.Contains(got, "color: red") {
		t.Errorf("StripHTML should have removed <style> content entirely, got: %q", got)
	}
	if !strings.Contains(got, "Visible text.") {
		t.Errorf("StripHTML should keep non-script/style text, got: %q", got)
	}
}

func TestStripHTML_UnescapesEntitiesAndCollapsesWhitespace(t *testing.T) {
	html := "<p>Tom &amp; Jerry &lt;3</p>\n\n\n\n<p>next</p>"
	got := StripHTML(html)
	if !strings.Contains(got, "Tom & Jerry <3") {
		t.Errorf("StripHTML should unescape HTML entities, got: %q", got)
	}
	if strings.Contains(got, "\n\n\n") {
		t.Errorf("StripHTML should collapse runs of blank lines, got: %q", got)
	}
}

func TestStripHTML_EmptyAfterStrip(t *testing.T) {
	got := StripHTML("<script>window.x = 1;</script>")
	if got != "" {
		t.Errorf("StripHTML of a script-only document should be empty, got: %q", got)
	}
}

// --- SSRF guard: isUnsafeIP / resolveSafePublicAddr, pure/offline ---

func TestIsUnsafeIP_TableDriven(t *testing.T) {
	cases := []struct {
		ip     string
		unsafe bool
	}{
		{"127.0.0.1", true},             // loopback
		{"::1", true},                   // loopback (v6)
		{"10.0.0.5", true},              // RFC 1918 private
		{"172.16.0.1", true},            // RFC 1918 private
		{"192.168.1.1", true},           // RFC 1918 private
		{"169.254.1.1", true},           // link-local
		{"224.0.0.1", true},             // multicast
		{"0.0.0.0", true},               // unspecified
		{"8.8.8.8", false},              // public
		{"93.184.216.34", false},        // public (example.com-ish)
		{"2001:4860:4860::8888", false}, // public v6 (Google DNS)
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("net.ParseIP(%q) failed", c.ip)
		}
		if got := isUnsafeIP(ip); got != c.unsafe {
			t.Errorf("isUnsafeIP(%s) = %v, want %v", c.ip, got, c.unsafe)
		}
	}
}

func TestResolveSafePublicAddr_RejectsLoopbackLiteral(t *testing.T) {
	_, err := resolveSafePublicAddr("127.0.0.1")
	if err == nil {
		t.Fatal("expected resolveSafePublicAddr to reject a loopback literal")
	}
}

func TestResolveSafePublicAddr_RejectsPrivateLiteral(t *testing.T) {
	_, err := resolveSafePublicAddr("192.168.0.1")
	if err == nil {
		t.Fatal("expected resolveSafePublicAddr to reject a private-range literal")
	}
}

// --- WebFetchTool.Execute: input validation paths that don't need network ---

func TestWebFetchTool_RequiresURL(t *testing.T) {
	_, err := WebFetchTool{}.Execute(context.Background(), argsJSON(t, map[string]string{}))
	if err == nil {
		t.Fatal("expected error when args.url is missing")
	}
}

func TestWebFetchTool_RejectsNonHTTPScheme(t *testing.T) {
	_, err := WebFetchTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"url": "ftp://example.com/file"}))
	if err == nil {
		t.Fatal("expected error for a non-http(s) scheme")
	}
}

func TestWebFetchTool_SSRFGuardBlocksLoopbackTarget(t *testing.T) {
	_, err := WebFetchTool{}.Execute(context.Background(), argsJSON(t, map[string]string{"url": "http://127.0.0.1:9/"}))
	if err == nil {
		t.Fatal("expected the SSRF guard to reject a loopback target before any request is made")
	}
}

func TestWebFetchTool_AcceptsAlternativeFieldNames(t *testing.T) {
	// NormalizeArgs (args.go) folds link/href into "url" for web_fetch
	// before Execute is ever called through the Dispatcher; Execute
	// itself also accepts them directly (webFetchURLFields), matching
	// the zsh source's own `_ai_tool_extract_field ... url link href`.
	got := webFetchURLFields([]byte(`{"link":"http://example.com"}`))
	if got != "http://example.com" {
		t.Errorf("webFetchURLFields(link) = %q, want http://example.com", got)
	}
	got = webFetchURLFields([]byte(`{"href":"http://example.com"}`))
	if got != "http://example.com" {
		t.Errorf("webFetchURLFields(href) = %q, want http://example.com", got)
	}
}
