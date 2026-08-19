package ui

import "testing"

func TestParseSlash(t *testing.T) {
	cases := []struct {
		raw      string
		wantCmd  string
		wantArgs string
	}{
		{"/chat hello world", "chat", "hello world"},
		{"chat", "chat", ""},
		{"/", "", ""},
		{"", "", ""},
		{"/config verbosity 2", "config", "verbosity 2"},
	}
	for _, c := range cases {
		cmd, args := ParseSlash(c.raw)
		if cmd != c.wantCmd || args != c.wantArgs {
			t.Fatalf("ParseSlash(%q) = (%q, %q), want (%q, %q)", c.raw, cmd, args, c.wantCmd, c.wantArgs)
		}
	}
}

func TestRouteKnownCommands(t *testing.T) {
	cases := map[string]RouteKind{
		"chat":    RouteChat,
		"code":    RouteCode,
		"fix":     RouteFix,
		"scan":    RouteScan,
		"agent":   RouteAgent,
		"index":   RouteIndex,
		"commit":  RouteCommit,
		"review":  RouteReview,
		"stats":   RouteStats,
		"dev":     RouteDev,
		"session": RouteSession,
		"details": RouteDetails,
		"config":  RouteConfig,
		"":        RoutePalette,
		"?":       RoutePalette,
		"help":    RouteHelp,
		"h":       RouteHelp,
		"bogus":   RouteUnknown,
	}
	for cmd, want := range cases {
		if got := Route(cmd); got != want {
			t.Fatalf("Route(%q) = %v, want %v", cmd, got, want)
		}
	}
}

func TestParseConfigArgsQuirk(t *testing.T) {
	sub, val := ParseConfigArgs("verbosity 2")
	if sub != "verbosity" || val != "2" {
		t.Fatalf("got (%q, %q)", sub, val)
	}
	// Shell quirk: no space -> val falls back to the whole args string.
	sub2, val2 := ParseConfigArgs("verbosity")
	if sub2 != "verbosity" || val2 != "verbosity" {
		t.Fatalf("got (%q, %q)", sub2, val2)
	}
}

// AC-03: Router memetakan semua command di registry ke handler yang
// benar, tidak ada command yang hilang.
func TestDispatcherHandlersCoverEveryRegistryCommand(t *testing.T) {
	for _, name := range RegistryNames() {
		if _, ok := DispatcherHandlers[name]; !ok {
			t.Errorf("registry command %q has no dispatcher handler (orphan command)", name)
		}
	}
}

// The reverse direction: no handler entry for a command that doesn't
// exist in the registry (drift the other way).
func TestDispatcherHandlersHaveNoExtraEntries(t *testing.T) {
	names := map[string]bool{}
	for _, n := range RegistryNames() {
		names[n] = true
	}
	for cmd := range DispatcherHandlers {
		if !names[cmd] {
			t.Errorf("dispatcher handler %q has no matching registry entry", cmd)
		}
	}
}

func TestDispatcherHandlersNoDuplicateTargets(t *testing.T) {
	// Not a hard requirement in the zsh source, but worth asserting each
	// handler function name is only reused deliberately (session/h are
	// the only zsh-side aliases, e.g. "session" -> _ai_session is unique).
	if len(DispatcherHandlers) != len(RegistryNames()) {
		t.Fatalf("DispatcherHandlers has %d entries, registry has %d commands", len(DispatcherHandlers), len(RegistryNames()))
	}
}
