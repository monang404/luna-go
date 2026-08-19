// Package aiops is small shared command infrastructure used by the four
// SESSION-54 high-level command packages (internal/chat, internal/codeproject,
// internal/filepatch, internal/workflow). It is NOT a port of any single
// zsh file -- it exists because those four packages all need the same
// handful of small building blocks (an _ai_quick/_ai_chat_request-equivalent
// LLM call helper, the _ai_confirm-equivalent confirmation gate,
// clipboard/share adapters, and the _ai_ts/slugify helpers), and
// SESSION-54's own instructions (EXECUTION_CONTEXT.md §4 "Dependency
// inversion for interactive/external operations", §49 Step 1 "Shared
// command infrastructure") ask for exactly this: small helpers, injected
// via interfaces where the zsh source did something interactive or
// platform-specific, NOT a big internal/commands package.
//
// This package deliberately stays free of any command-specific behavior
// (no chat/codeproject/filepatch/workflow logic lives here) -- it only
// wraps internal/llmclient + internal/config into the shape the four
// command packages need, plus the injectable interfaces (ConfirmFunc,
// Clipboard, ShareFunc, CommandRunner) SESSION-54's own contract asks
// for.
package aiops
