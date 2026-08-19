// Package env is a placeholder introduced by SESSION-40 (Go skeleton +
// build pipeline). No logic has been ported yet -- this file exists only so
// the package compiles and has a documented home for the future port.
//
// Package env corresponds to the legacy zsh source's .zsh_bagas/00-core/ (locale, history, shell opts). Most of this is N/A in a standalone Go binary, but env-var loading (~/.secrets.zsh style export lines) remains relevant and will be ported here. See SESSION-41.
//
// Do not add real logic here until the corresponding migration session
// (see docs/execution_sessions/) begins.
package env
