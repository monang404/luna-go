// Ported from the parsing half of 30-luna/50-agent/10-state.zsh
// (_ai_agent_parse), which shells out to an inline python3 script. This
// file is a literal behavioral port of that script's logic, using
// encoding/json instead of python's json module.
//
// Algorithm (unchanged from the python source): scan the raw reply for
// every '{' byte, and -- starting from the LAST one and working
// backwards -- try to decode one JSON value starting at that position
// (trailing garbage after the value is fine and ignored, matching
// json.JSONDecoder.raw_decode). The first candidate (rightmost-first)
// that decodes successfully AND is a JSON object containing at least one
// of "tool"/"command"/"thought"/"done" is accepted; everything before it
// in the scan order is never even looked at. This intentionally picks
// the LAST plausible JSON object in the reply, since a model's raw text
// sometimes contains earlier stray '{' (e.g. inside an explanation) with
// the real tool-call JSON coming after it.
package agent

import (
	"encoding/json"
	"strconv"
	"strings"
)

// Plan is the typed result of parsing one LLM reply: either a tool call
// (Tool != "", Done == false), a completion claim (Done == true), or
// empty (neither -- Empty() reports true), which the caller (loop.go)
// treats as "LUNA balas format JSON gak valid" exactly like the zsh
// source's post-parse check.
type Plan struct {
	// Thought is the model's reasoning text for this step, "" if absent
	// or falsy (mirrors python's `str(obj.get('thought') or '')`).
	Thought string
	// Tool is the tool name to call next. "" together with Done==false
	// means Empty() -- an unparseable/contentless reply.
	Tool string
	// Args is the tool call's arguments, always valid JSON (defaults to
	// the two-byte object "{}" when the model supplied none), safe to
	// hand directly to tools.Dispatcher.Dispatch without a nil check.
	Args json.RawMessage
	// Done mirrors `bool(obj.get('done', False))`: whether the model
	// claims the goal is complete. Never implies Tool == "" by itself --
	// a plan can (rarely) carry both; the loop's own guard (VERIFY
	// entry) is what treats Done as authoritative once set.
	Done bool
	// Compat is a human-readable note the caller may print at
	// AI_VERBOSE=1, set when a legacy/malformed shape was normalized
	// (root "command" mapped to run_command, or root fields hoisted
	// into args). "" when no normalization happened.
	Compat string
}

// Empty reports whether p carries no usable content at all -- the same
// condition 05-get_plan.zsh checks right after parsing
// (`[ -z "$thought" ] && [ -z "$tool" ] && [ "$done_flag" != "true" ]`)
// to decide the reply was not valid JSON in the expected shape.
func (p Plan) Empty() bool {
	return p.Thought == "" && p.Tool == "" && !p.Done
}

// hoistFields mirrors the python script's hoist_fields list verbatim:
// well-known argument field names that get pulled from the JSON root
// into args when the model put them at the wrong nesting level.
var hoistFields = []string{
	"path", "file", "filename", "command", "cmd",
	"url", "pattern", "content", "old_str", "new_str",
	"diff_content", "dest", "program", "offset", "limit",
	"glob", "items", "runner", "timeout",
}

// ParsePlan parses reply into a Plan. It never panics and never returns
// an error: a reply with no valid/matching JSON object anywhere in it
// simply yields an empty Plan (Empty() == true, Args == "{}"), exactly
// like the python source's all-branches-fail defaults
// (thought, tool, args, done, compat_msg = ”, ”, '{}', False, ”).
func ParsePlan(reply string) Plan {
	var idxs []int
	for i := 0; i < len(reply); i++ {
		if reply[i] == '{' {
			idxs = append(idxs, i)
		}
	}
	for k := len(idxs) - 1; k >= 0; k-- {
		i := idxs[k]
		var obj map[string]any
		dec := json.NewDecoder(strings.NewReader(reply[i:]))
		if err := dec.Decode(&obj); err != nil {
			continue
		}
		if obj == nil {
			continue
		}
		_, hasTool := obj["tool"]
		_, hasCommand := obj["command"]
		_, hasThought := obj["thought"]
		_, hasDone := obj["done"]
		_, hasResponse := obj["response"]
		if !hasTool && !hasCommand && !hasThought && !hasDone && !hasResponse {
			continue
		}
		return planFromObject(obj)
	}
	return Plan{Args: json.RawMessage("{}")}
}

// planFromObject builds a Plan from a matched JSON object, mirroring the
// python script's body from `thought = str(...)` through the final
// `done = bool(...)`.
func planFromObject(obj map[string]any) Plan {
	p := Plan{Thought: truthyString(obj["thought"])}
	if resp := truthyString(obj["response"]); resp != "" {
		p.Thought = resp
	}

	if _, hasTool := obj["tool"]; !hasTool {
		if cmdVal, hasCommand := obj["command"]; hasCommand {
			// Auto-map legacy command format directly to run_command,
			// same as the python source's special-case branch.
			p.Tool = "run_command"
			args, err := json.Marshal(map[string]string{"command": truthyString(cmdVal)})
			if err != nil {
				args = []byte("{}")
			}
			p.Args = args
			p.Compat = "Legacy tool format detected. Normalized to run_command."
			p.Done = truthyBool(obj["done"])
			return p
		}
	}

	p.Tool = truthyString(obj["tool"])
	rawArgs, ok := obj["args"].(map[string]any)
	if !ok || rawArgs == nil {
		rawArgs = map[string]any{}
	}
	for _, field := range hoistFields {
		if _, exists := rawArgs[field]; exists {
			continue
		}
		if v, ok := obj[field]; ok {
			rawArgs[field] = v
			if p.Compat == "" {
				p.Compat = "Legacy tool format detected. Normalized root fields into args."
			}
		}
	}
	args, err := json.Marshal(rawArgs)
	if err != nil {
		args = []byte("{}")
	}
	p.Args = args
	p.Done = truthyBool(obj["done"])
	return p
}

// truthyString mirrors python's `str(v or ”)`: nil/false/0/""/empty
// containers all collapse to "", anything else is stringified.
func truthyString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		if !t {
			return ""
		}
		return "true"
	case float64:
		if t == 0 {
			return ""
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}

// truthyBool mirrors python's `bool(v)` truthiness rules for the JSON
// value types that can appear in a decoded object.
func truthyBool(v any) bool {
	switch t := v.(type) {
	case nil:
		return false
	case bool:
		return t
	case string:
		return t != ""
	case float64:
		return t != 0
	case []any:
		return len(t) != 0
	case map[string]any:
		return len(t) != 0
	default:
		return false
	}
}
