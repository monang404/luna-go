// Package ui is the Go port of the legacy zsh source's 30-luna/60-ui/ (design tokens
// AI_C_*, rendering contract, diff colorizer, screens, command registry).
//
// SESSION-40 created this package as an empty placeholder. SESSION-52
// ported the low-level rendering primitives only: design tokens
// (tokens.go), text wrap/width/unicode detection (text.go), box drawing
// (box.go), and the diff colorizer (diff.go). These are pure/near-pure
// functions ("input -> rendered string") with no component, screen,
// router, or command-registry code -- that composition layer is
// SESSION-53+ and deliberately not present here yet.
//
// Do not add screen/component/router logic to this package before
// SESSION-53 begins -- see docs/execution_sessions/53_port_ui_components_and_screens.yaml.
package ui
