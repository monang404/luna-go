package screens

import (
	"strings"
	"testing"

	"github.com/monang404/luna-go/internal/ui"
)

func TestOptionsIncludesEveryRegistryCommandAndHardcodedExtras(t *testing.T) {
	opts := Options()
	if len(opts) != len(ui.CommandRegistry)+5 {
		t.Fatalf("len(Options()) = %d, want %d", len(opts), len(ui.CommandRegistry)+5)
	}
	joined := strings.Join(opts, "\n")
	for _, e := range ui.CommandRegistry {
		if !strings.Contains(joined, e.Name) {
			t.Fatalf("Options() missing registry command %q", e.Name)
		}
	}
	for _, extra := range []string{"details", "config verbosity 0", "config verbosity 1", "config verbosity 2", "config verbosity 3"} {
		if !strings.Contains(joined, extra) {
			t.Fatalf("Options() missing hardcoded extra %q", extra)
		}
	}
}

func TestCommandExtractsBeforeDoubleSpace(t *testing.T) {
	cases := map[string]string{
		"agent         Agent full akses: ...":            "agent",
		"config verbosity 2   Output detail (tool+file)": "config verbosity 2",
		"details       Tampilkan detail log terakhir":    "details",
		"chat": "chat",
	}
	for in, want := range cases {
		if got := Command(in); got != want {
			t.Fatalf("Command(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRouteRegistryCommandGoesToDispatcher(t *testing.T) {
	if got := Route("agent"); got != RouteToDispatcher {
		t.Fatalf("Route(agent) = %v, want RouteToDispatcher", got)
	}
	if got := Route("chat"); got != RouteToDispatcher {
		t.Fatalf("Route(chat) = %v, want RouteToDispatcher", got)
	}
}

func TestRouteNonRegistryGoesToRouter(t *testing.T) {
	if got := Route("details"); got != RouteToRouter {
		t.Fatalf("Route(details) = %v, want RouteToRouter", got)
	}
	if got := Route("config verbosity 2"); got != RouteToRouter {
		t.Fatalf("Route(config verbosity 2) = %v, want RouteToRouter", got)
	}
	if got := Route("bogus"); got != RouteToRouter {
		t.Fatalf("Route(bogus) = %v, want RouteToRouter", got)
	}
}

// End-to-end sanity: every registry option, run through Command(), must
// Route() to the dispatcher (this is the palette's actual selection
// pipeline for registry items).
func TestSelectionPipelineForEveryRegistryOption(t *testing.T) {
	for _, e := range ui.CommandRegistry {
		line := ""
		for _, opt := range Options() {
			if strings.HasPrefix(opt, e.Name) {
				line = opt
				break
			}
		}
		if line == "" {
			t.Fatalf("no option line found for registry command %q", e.Name)
		}
		cmd := Command(line)
		if cmd != e.Name {
			t.Fatalf("Command(%q) = %q, want %q", line, cmd, e.Name)
		}
		if got := Route(cmd); got != RouteToDispatcher {
			t.Fatalf("Route(%q) = %v, want RouteToDispatcher", cmd, got)
		}
	}
}
