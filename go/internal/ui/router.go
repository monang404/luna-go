// Traceability: 30-luna/60-ui/router.zsh + 30-luna/60-ui/40-dispatcher.zsh
// (the `luna()` case-statement's command->handler table only — the actual
// handler bodies are aic/aicode/aiagent/etc., which are LLM/tool
// execution and out of scope here per the session spec's exclude list)
// -> router.go.
//
// Scope note: router.zsh's ui_router() and 40-dispatcher.zsh's luna() both
// end by CALLING a handler function (aic, aicode, _ai_session, ...).
// SESSION-53 excludes "LLM / tool execution / stdin / readline" — so
// this file ports the pure mapping/decision half of both: given a
// command, which route/handler does it resolve to? Actually invoking
// that handler is CLI wiring (SESSION-55).
package ui

import "strings"

// ParseSlash is the port of the cmd/args split at the top of
// ui_router(): `cmd="${raw%% *}"`, `args="${raw#* }"` (only when a space
// is present), then `cmd="${cmd#/}"` (strip one leading slash).
func ParseSlash(raw string) (cmd string, args string) {
	cmd = raw
	if idx := strings.Index(raw, " "); idx >= 0 {
		cmd = raw[:idx]
		args = raw[idx+1:]
	}
	cmd = strings.TrimPrefix(cmd, "/")
	return cmd, args
}

// RouteKind enumerates ui_router()'s case-statement branches.
type RouteKind int

const (
	RouteChat RouteKind = iota
	RouteCode
	RouteFix
	RouteScan
	RouteAgent
	RouteIndex
	RouteCommit
	RouteReview
	RouteStats
	RouteDev
	RouteSession
	RouteDetails
	RouteConfig
	RoutePalette
	RouteHelp
	RouteUnknown
)

// Route is the port of ui_router()'s `case "$cmd" in ...` dispatch
// (mapping only — see file doc comment).
func Route(cmd string) RouteKind {
	switch cmd {
	case "chat":
		return RouteChat
	case "code":
		return RouteCode
	case "fix":
		return RouteFix
	case "scan":
		return RouteScan
	case "agent":
		return RouteAgent
	case "index":
		return RouteIndex
	case "commit":
		return RouteCommit
	case "review":
		return RouteReview
	case "stats":
		return RouteStats
	case "dev":
		return RouteDev
	case "session":
		return RouteSession
	case "details":
		return RouteDetails
	case "config":
		return RouteConfig
	case "", "?":
		return RoutePalette
	case "help", "h":
		return RouteHelp
	default:
		return RouteUnknown
	}
}

// ParseConfigArgs is the port of the `/config verbosity N` sub-parsing:
// `sub="${args%% *}"`, `val="${args#* }"`. Faithfully reproduces the
// shell's quirk that when args has no space, `${args#* }` leaves args
// UNCHANGED (the pattern simply doesn't match) — so val equals sub in
// that case, not "".
func ParseConfigArgs(args string) (sub string, val string) {
	if idx := strings.Index(args, " "); idx >= 0 {
		return args[:idx], args[idx+1:]
	}
	return args, args
}

// UnknownCommandMessage is the port of ui_router()'s `*)` branch text.
func UnknownCommandMessage(cmd string, t Tokens) string {
	var b strings.Builder
	b.WriteString(t.Warn + "Unknown slash command: /" + cmd + t.Reset + "\n")
	b.WriteString(t.Muted + "Coba /" + t.Reset + "help" + t.Reset + " atau tekan / untuk Command Palette\n")
	return b.String()
}

// UnknownConfigKeyMessage is the port of the `config)` branch's `*)`
// case (unrecognized config key).
func UnknownConfigKeyMessage(sub string, t Tokens) string {
	return t.Warn + "Unknown config key: " + sub + t.Reset + "\n"
}

// DispatcherHandlers is the port of the `luna()` case-statement's
// subcommand->handler-function-name table in 40-dispatcher.zsh. This is
// data (a handler *label*, not a callable — invoking aic/aicode/etc. is
// out of scope, see file doc comment) used to prove AC-03/AC-04: every
// registry command has exactly one handler and none are missing/orphaned
// (see registry_test.go).
var DispatcherHandlers = map[string]string{
	"chat":       "aic",
	"long":       "aicl",
	"code":       "aicode",
	"edit":       "aipatch",
	"view":       "aicat",
	"scan":       "aiscan",
	"index":      "aiindex",
	"fix":        "aifix",
	"run":        "airun",
	"build":      "aibuild",
	"project":    "aiproject",
	"scrap":      "aiscrap",
	"ask":        "aiask",
	"shell":      "aish",
	"commit":     "aicommit",
	"review":     "aireview",
	"debug":      "aidebug",
	"research":   "airesearch",
	"delegate":   "aidelegate",
	"plan":       "aiplan",
	"prompt":     "aiprompt",
	"spec":       "aispec",
	"summarize":  "aisummarize",
	"clip":       "aiclip",
	"session":    "_ai_session",
	"agent":      "aiagent",
	"stats":      "aistats",
	"log":        "aihist",
	"menu":       "_ai_menu",
	"deps":       "ai_check_deps",
	"dev":        "aidev",
	"testmodels": "ai_testmodels",
	"undo":       "aiundo",
	"bakclean":   "aibakclean",
	"share":      "aishare",
	"update":     "_ai_update_confirm_pull",
	"h":          "_ai_help",
}
