package tools

import (
	"encoding/json"
	"fmt"
)

// This file ports the AI_TOOL_SCHEMA half of 00-tool_registry.zsh: one jq
// predicate per tool name, each rejecting malformed args before
// permission checks or Execute ever run ("jq predicates are contracts,
// not sanitizers"). Go has no jq, so each predicate is reimplemented as a
// small validator function operating on the parsed args object; the goal
// is the same accept/reject boundary as the original jq expression, not
// byte-identical error text.

// validator checks one tool's already-normalized args object and returns
// a descriptive error if they don't satisfy that tool's schema.
type validator func(args map[string]interface{}) error

// schemas mirrors AI_TOOL_SCHEMA: one entry per Registry name (git_status
// and todo_read have no fields to check at all, matching the source's
// literal `'true'` predicate for both -- so they're simply absent here;
// ValidateArgs treats a missing entry as "nothing to validate").
var schemas = map[string]validator{
	"read_file": func(a map[string]interface{}) error {
		if err := requireNonEmptyString(a, "path"); err != nil {
			return err
		}
		if err := optionalNumber(a, "offset"); err != nil {
			return err
		}
		return optionalNumber(a, "limit")
	},
	"list_dir": func(a map[string]interface{}) error {
		return optionalString(a, "path")
	},
	"grep_search": func(a map[string]interface{}) error {
		if err := requireNonEmptyString(a, "pattern"); err != nil {
			return err
		}
		if err := optionalString(a, "path"); err != nil {
			return err
		}
		return optionalString(a, "glob")
	},
	"glob_search": func(a map[string]interface{}) error {
		return requireNonEmptyString(a, "pattern")
	},
	"count_lines": func(a map[string]interface{}) error {
		if err := requireNonEmptyString(a, "path"); err != nil {
			return err
		}
		return optionalString(a, "pattern")
	},
	"write_file": func(a map[string]interface{}) error {
		if err := requireNonEmptyString(a, "path"); err != nil {
			return err
		}
		return requireStringField(a, "content")
	},
	"edit_file": func(a map[string]interface{}) error {
		if err := requireNonEmptyString(a, "path"); err != nil {
			return err
		}
		if err := requireStringField(a, "old_str"); err != nil {
			return err
		}
		return requireStringField(a, "new_str")
	},
	"patch_file": func(a map[string]interface{}) error {
		if err := requireNonEmptyString(a, "path"); err != nil {
			return err
		}
		return requireStringField(a, "diff_content")
	},
	"run_command": func(a map[string]interface{}) error {
		return requireNonEmptyString(a, "command")
	},
	"exec_process": func(a map[string]interface{}) error {
		if err := requireNonEmptyString(a, "program"); err != nil {
			return err
		}
		if err := optionalNoNewlineStringArray(a, "args"); err != nil {
			return err
		}
		if err := optionalString(a, "cwd"); err != nil {
			return err
		}
		return optionalNumberInRange(a, "timeout", 1, 300)
	},
	"run_test": func(a map[string]interface{}) error {
		if err := optionalString(a, "cmd"); err != nil {
			return err
		}
		if err := optionalString(a, "runner"); err != nil {
			return err
		}
		if err := optionalNoNewlineStringArray(a, "args"); err != nil {
			return err
		}
		if err := optionalString(a, "path"); err != nil {
			return err
		}
		return optionalNumberInRange(a, "timeout", 1, 300)
	},
	"move_file": func(a map[string]interface{}) error {
		if err := requireNonEmptyString(a, "path"); err != nil {
			return err
		}
		return requireNonEmptyString(a, "dest")
	},
	"delete_file": func(a map[string]interface{}) error {
		return requireNonEmptyString(a, "path")
	},
	"git_diff": func(a map[string]interface{}) error {
		return optionalString(a, "path")
	},
	"web_fetch": func(a map[string]interface{}) error {
		return requireNonEmptyString(a, "url")
	},
	"web_search": func(a map[string]interface{}) error {
		return requireNonEmptyString(a, "query")
	},
	"todo_write": func(a map[string]interface{}) error {
		return validateTodoItems(a)
	},
	"bash_output": func(a map[string]interface{}) error {
		return requireNonEmptyString(a, "id")
	},
	"kill_shell": func(a map[string]interface{}) error {
		return requireNonEmptyString(a, "id")
	},
	"delegate_task": func(a map[string]interface{}) error {
		if err := requireNonEmptyString(a, "role"); err != nil {
			return err
		}
		return requireNonEmptyString(a, "goal")
	},
}

// ValidateArgs looks up toolName's schema (if any) and runs it against
// argsJSON, mirroring _ai_tool_validate_request's
// `jq -en --argjson args "$args_json" "... and ($args | $schema)"` call:
// a JSON parse failure and a schema-predicate failure are both reported
// as validation errors here, exactly as both cases make that single jq
// invocation fail in the zsh source.
func ValidateArgs(toolName string, argsJSON json.RawMessage) error {
	var obj map[string]interface{}
	if err := json.Unmarshal(argsJSON, &obj); err != nil {
		return fmt.Errorf("ERROR: tool %q menerima arguments yang tidak sesuai schema (bukan JSON object): %w", toolName, err)
	}
	v, ok := schemas[toolName]
	if !ok {
		// git_status / todo_read (predicate 'true'), or a tool with no
		// registered schema at all -- nothing further to check.
		return nil
	}
	if err := v(obj); err != nil {
		return fmt.Errorf("ERROR: tool %q menerima arguments yang tidak sesuai schema: %w", toolName, err)
	}
	return nil
}

// --- field-level predicate helpers, each mirroring one jq fragment ---

// requireNonEmptyString mirrors `.field | type == "string" and length > 0`.
func requireNonEmptyString(a map[string]interface{}, field string) error {
	v, ok := a[field]
	if !ok {
		return fmt.Errorf("field %q is required", field)
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return fmt.Errorf("field %q must be a non-empty string", field)
	}
	return nil
}

// requireStringField mirrors `.field | type == "string"` (no length
// constraint -- content/old_str/new_str/diff_content may legitimately be
// empty strings).
func requireStringField(a map[string]interface{}, field string) error {
	v, ok := a[field]
	if !ok {
		return fmt.Errorf("field %q is required", field)
	}
	if _, ok := v.(string); !ok {
		return fmt.Errorf("field %q must be a string", field)
	}
	return nil
}

// optionalString mirrors `(.field // "<default>") | type == "string"`:
// the field may be absent (the default always satisfies the type check),
// but if present it must be a string.
func optionalString(a map[string]interface{}, field string) error {
	v, ok := a[field]
	if !ok {
		return nil
	}
	if _, ok := v.(string); !ok {
		return fmt.Errorf("field %q, if present, must be a string", field)
	}
	return nil
}

// optionalNumber mirrors `(.field // 0) | type == "number"` with no
// range constraint (read_file's offset/limit).
func optionalNumber(a map[string]interface{}, field string) error {
	v, ok := a[field]
	if !ok {
		return nil
	}
	if _, ok := v.(float64); !ok {
		return fmt.Errorf("field %q, if present, must be a number", field)
	}
	return nil
}

// optionalNumberInRange mirrors
// `(.field // default) | type == "number" and . >= min and . <= max`
// (exec_process/run_test's timeout).
func optionalNumberInRange(a map[string]interface{}, field string, min, max float64) error {
	v, ok := a[field]
	if !ok {
		return nil
	}
	n, ok := v.(float64)
	if !ok {
		return fmt.Errorf("field %q, if present, must be a number", field)
	}
	if n < min || n > max {
		return fmt.Errorf("field %q must be between %v and %v", field, min, max)
	}
	return nil
}

// optionalNoNewlineStringArray mirrors
// `(.field // []) | type == "array" and all(.[]; type == "string" and (contains("\n") | not))`
// (exec_process/run_test's args -- newline-free by construction so a
// single array element can never smuggle in a second shell token).
func optionalNoNewlineStringArray(a map[string]interface{}, field string) error {
	v, ok := a[field]
	if !ok {
		return nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return fmt.Errorf("field %q, if present, must be an array", field)
	}
	for i, item := range arr {
		s, ok := item.(string)
		if !ok {
			return fmt.Errorf("field %q[%d] must be a string", field, i)
		}
		for _, r := range s {
			if r == '\n' {
				return fmt.Errorf("field %q[%d] must not contain a newline", field, i)
			}
		}
	}
	return nil
}

// validTodoStatus mirrors `.status | IN("pending","doing","done")`.
var validTodoStatus = map[string]bool{"pending": true, "doing": true, "done": true}

// validateTodoItems mirrors todo_write's predicate:
// `.items | type == "array" and length > 0 and all(.[]; type == "object" and (.text | type == "string") and (.status | IN("pending","doing","done")))`.
func validateTodoItems(a map[string]interface{}) error {
	v, ok := a["items"]
	if !ok {
		return fmt.Errorf("field %q is required", "items")
	}
	arr, ok := v.([]interface{})
	if !ok || len(arr) == 0 {
		return fmt.Errorf("field %q must be a non-empty array", "items")
	}
	for i, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return fmt.Errorf("items[%d] must be an object", i)
		}
		if _, ok := obj["text"].(string); !ok {
			return fmt.Errorf("items[%d].text must be a string", i)
		}
		status, ok := obj["status"].(string)
		if !ok || !validTodoStatus[status] {
			return fmt.Errorf("items[%d].status must be one of pending|doing|done", i)
		}
	}
	return nil
}
