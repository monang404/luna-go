package tools

import (
	"encoding/json"
	"fmt"
)

// This file ports 02-tool_args_extract.zsh. That file's own comment
// flags _ai_tool_extract_path/_ai_tool_extract_field as a "hidden SPOF"
// nearly every tool depends on -- the Go port keeps the same shape
// (small, dependency-free functions with no partial state) so the same
// kind of single point of failure can't reappear as a load-order bug the
// way it could in zsh; a Go compile failure here fails the whole build
// immediately rather than degrading silently at runtime.

// ExtractField returns the first non-empty string value found for any of
// the given field names, checking each field's top-level location, then
// ".parameters.<field>", then ".arguments.<field>" before moving to the
// next field name -- mirroring the "// "-chained jq expression
// _ai_tool_extract_field builds from its variadic argument list. Returns
// "" if argsJSON is empty, not a JSON object, or no field/alternative
// matched a non-empty string.
func ExtractField(argsJSON json.RawMessage, fields ...string) string {
	if len(argsJSON) == 0 {
		return ""
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(argsJSON, &obj); err != nil {
		return ""
	}
	for _, f := range fields {
		if v := lookupStringField(obj, f); v != "" {
			return v
		}
		if params, ok := obj["parameters"].(map[string]interface{}); ok {
			if v := lookupStringField(params, f); v != "" {
				return v
			}
		}
		if args, ok := obj["arguments"].(map[string]interface{}); ok {
			if v := lookupStringField(args, f); v != "" {
				return v
			}
		}
	}
	return ""
}

func lookupStringField(m map[string]interface{}, field string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[field]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// ExtractPath is the path-specific extraction _ai_tool_extract_path
// performs: a bare JSON string is treated as the path outright (Case 1);
// otherwise it tries the known path-ish field names path, file,
// filename, dir, directory (Case 2, via ExtractField).
//
// SESSION-48 addition: "cwd" is included in that field list so
// Dispatcher.Dispatch's unconditional `req.Path = ExtractPath(normalized)`
// (dispatch.go) automatically threads exec_process's args.cwd through
// permission.CheckPermission's path-containment guard, the same guard
// every path-bearing tool already gets for free -- without this,
// exec_process would need its own copy of the containment check with no
// AgentContext available inside Tool.Execute to run it against (Execute
// only ever receives already-normalized args + a context.Context, by
// design -- see tool.go's Tool interface doc comment). No other Registry
// tool's schema uses a field named "cwd", so this is additive and
// changes no other tool's ExtractPath result.
func ExtractPath(argsJSON json.RawMessage) string {
	if len(argsJSON) == 0 {
		return ""
	}
	var raw interface{}
	if err := json.Unmarshal(argsJSON, &raw); err != nil {
		return ""
	}
	if s, ok := raw.(string); ok {
		return s
	}
	return ExtractField(argsJSON, "path", "file", "filename", "dir", "directory", "cwd")
}

// pathFieldTools / patternFieldTools mirror the two `case "$tool_name"`
// arms in _ai_tool_normalize_args that share a fill rule (the
// path-bearing tools, and the pattern-bearing search tools), kept as
// named sets so NormalizeArgs's switch below reads the same way the zsh
// case statement does.
var pathFieldTools = map[string]bool{
	"read_file": true, "write_file": true, "edit_file": true,
	"count_lines": true, "delete_file": true, "list_dir": true,
	"patch_file": true, "move_file": true,
}

// NormalizeArgs mirrors _ai_tool_normalize_args: it tolerates the model
// placing fields at slightly wrong locations without ever inventing
// unknown fields or overwriting a field the caller already set.
//
//   - Empty argsJSON -> "{}" (matches the zsh function's own empty-input
//     short-circuit).
//   - A JSON value that fails to parse at all is a genuine malformed-args
//     error (unlike the zsh version, which never reaches this case
//     because jq's own parse failure inside _ai_tool_dispatch's later
//     -en/--argjson call is what actually rejects it -- see dispatch.go's
//     ValidateArgs, the Go equivalent of that failure point) -- returned
//     here so the caller (Dispatcher.Dispatch) can report it before
//     schema validation ever runs (AC-02).
//   - A bare JSON string is wrapped into the field the given tool name
//     expects (path/pattern/command/url), or "{}" for a tool with no
//     known single-field shape.
//   - Anything else that isn't a JSON object also normalizes to "{}"
//     (matches the zsh function's `[[ "$atype" != "object" ]]` branch).
//   - A JSON object gets its own known alternative field names folded
//     into the canonical field name only when the canonical name is
//     absent, exactly like the per-tool case arms in the zsh source.
func NormalizeArgs(argsJSON json.RawMessage, toolName string) (json.RawMessage, error) {
	if len(argsJSON) == 0 {
		return json.RawMessage("{}"), nil
	}

	var raw interface{}
	if err := json.Unmarshal(argsJSON, &raw); err != nil {
		return nil, fmt.Errorf("args is not valid JSON: %w", err)
	}

	switch v := raw.(type) {
	case string:
		return wrapBareString(v, toolName), nil
	case map[string]interface{}:
		return fillKnownAlternatives(v, toolName)
	default:
		return json.RawMessage("{}"), nil
	}
}

// wrapBareString implements the bare-string arm of
// _ai_tool_normalize_args's `case "$tool_name"`.
func wrapBareString(s, toolName string) json.RawMessage {
	if s == "" {
		return json.RawMessage("{}")
	}
	var field string
	switch {
	case pathFieldTools[toolName]:
		field = "path"
	case toolName == "grep_search" || toolName == "glob_search":
		field = "pattern"
	case toolName == "run_command":
		field = "command"
	case toolName == "web_fetch":
		field = "url"
	default:
		return json.RawMessage("{}")
	}
	out, err := json.Marshal(map[string]string{field: s})
	if err != nil {
		return json.RawMessage("{}")
	}
	return out
}

// fillKnownAlternatives implements the object arm: fold in a known
// alternative field name only when the canonical field is missing,
// mirroring each `has("...")` check in the zsh source exactly.
func fillKnownAlternatives(obj map[string]interface{}, toolName string) (json.RawMessage, error) {
	switch {
	case pathFieldTools[toolName]:
		fillIfMissing(obj, "path", "file", "filename", "dir", "directory")
	case toolName == "web_fetch":
		fillIfMissing(obj, "url", "link", "href")
	case toolName == "run_command":
		fillIfMissing(obj, "command", "cmd")
	case toolName == "grep_search":
		fillIfMissing(obj, "pattern", "query", "search", "regex")
	case toolName == "glob_search":
		fillIfMissing(obj, "pattern", "glob", "name", "filename")
	}
	out, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("args: failed to re-encode normalized object: %w", err)
	}
	return out, nil
}

// fillIfMissing sets obj[canonical] to the first non-empty string found
// among alternatives, but only when obj doesn't already have canonical --
// matching the zsh source's has("...") guard so an existing (even
// intentionally different) value is never clobbered.
func fillIfMissing(obj map[string]interface{}, canonical string, alternatives ...string) {
	if _, has := obj[canonical]; has {
		return
	}
	for _, alt := range alternatives {
		if v, ok := obj[alt].(string); ok && v != "" {
			obj[canonical] = v
			return
		}
	}
}
