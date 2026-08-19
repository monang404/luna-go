// Package chat ports 30-luna/20-chat/ (aic, aicl, aish, aiask, session
// management/REPL, aiclip, and the reasoning/answer splitter behind all
// of them) into Go, per SESSION-54.
//
// Streaming note: the zsh source streams tokens to the terminal for
// aic/aish (mode-dependent) and _ai_session_ask (always). This package
// uses the blocking request path (internal/aiops.Completer, itself
// backed by internal/llmclient.CallBlocking) throughout instead --
// SESSION-54's own contract (EXECUTION_CONTEXT.md §0) says exact
// terminal output/streaming is not required unless an existing UI
// contract explicitly owns it, and no such contract exists yet for
// these commands (SESSION-55 wires the CLI/UI). The *content* of a
// completed reply, and every guard/log/session-persistence side effect
// around it, is preserved; only the token-by-token terminal delivery is
// deferred to whatever SESSION-55 builds on top of this package.
package chat
