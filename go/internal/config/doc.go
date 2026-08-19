// Package config ports 30-luna/00-config/ (nine zsh files) into type-safe Go:
// provider endpoints/models/keys, per-task-class provider fallback order,
// per-provider multi-model fallback lists, retry/timeout/session/diff/
// patch/file-size/agent guards, persona text, the aispec/aibuild sysprompt,
// the progressive context-engine level mapping, and a minimal
// ~/.secrets.zsh-compatible loader.
//
// Ported in SESSION-41 (docs/execution_sessions/41_port_config_layer.yaml).
// Source zsh files (30-luna/00-config/ unless noted):
//
//	00-models.zsh            -> models.go
//	05-provider_order.zsh    -> providers.go
//	10-paths.zsh             -> limits.go (Paths)
//	15-limits.zsh            -> limits.go (Limits)
//	20-runtime_guards.zsh    -> limits.go (Limits, guard fields)
//	25-persona.zsh           -> persona.go
//	30-sysprompt_spec.zsh    -> sysprompt.go
//	35-providers.zsh         -> providers.go
//	40-context_engine_docs.zsh -> sysprompt.go (ContextEngineLevels)
//
// Deliberately out of scope for this package (see the session's
// scope.exclude): any HTTP call to a provider (internal/llmclient,
// SESSION-44/45/46) and any runtime guard that touches the OS/process
// directly, e.g. battery/network checks (internal/permission, SESSION-42) --
// only the *thresholds* those guards read live here.
package config
