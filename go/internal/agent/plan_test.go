package agent

import (
	"encoding/json"
	"testing"
)

func TestParsePlan_ValidToolCall(t *testing.T) {
	reply := `Here's my plan: {"thought":"baca file dulu","tool":"read_file","args":{"path":"main.py"},"done":false}`
	p := ParsePlan(reply)
	if p.Empty() {
		t.Fatalf("ParsePlan() unexpectedly empty: %+v", p)
	}
	if p.Tool != "read_file" {
		t.Errorf("Tool = %q, want %q", p.Tool, "read_file")
	}
	if p.Done {
		t.Errorf("Done = true, want false")
	}
	var args map[string]string
	if err := json.Unmarshal(p.Args, &args); err != nil {
		t.Fatalf("Args not valid JSON: %v", err)
	}
	if args["path"] != "main.py" {
		t.Errorf("Args[path] = %q, want %q", args["path"], "main.py")
	}
}

func TestParsePlan_DonePlan(t *testing.T) {
	p := ParsePlan(`{"thought":"selesai semua","done":true}`)
	if !p.Done {
		t.Errorf("Done = false, want true")
	}
	if p.Tool != "" {
		t.Errorf("Tool = %q, want empty", p.Tool)
	}
	if p.Empty() {
		t.Errorf("Empty() = true for a done:true plan, want false")
	}
}

func TestParsePlan_InvalidJSON(t *testing.T) {
	p := ParsePlan("saya rasa perlu baca file tapi lupa format JSON-nya, oops {not valid")
	if !p.Empty() {
		t.Errorf("ParsePlan() on malformed JSON = %+v, want Empty()", p)
	}
	if string(p.Args) != "{}" {
		t.Errorf("Args = %q, want \"{}\"", p.Args)
	}
}

func TestParsePlan_MissingRecognizedFields(t *testing.T) {
	// A syntactically valid JSON object that has none of
	// tool/command/thought/done is not a plan candidate at all.
	p := ParsePlan(`{"unrelated":"value"}`)
	if !p.Empty() {
		t.Errorf("ParsePlan() on object with no recognized fields = %+v, want Empty()", p)
	}
}

func TestParsePlan_WrongFieldType(t *testing.T) {
	// "done" as a non-empty string is still truthy per python's bool();
	// this asserts ParsePlan never panics on an unexpected JSON shape
	// and degrades gracefully rather than erroring.
	p := ParsePlan(`{"thought":123,"done":"yes"}`)
	if p.Thought != "123" {
		t.Errorf("Thought = %q, want %q (numeric thought stringified)", p.Thought, "123")
	}
	if !p.Done {
		t.Errorf("Done = false, want true (non-empty string is truthy)")
	}
}

func TestParsePlan_LegacyCommandFormat(t *testing.T) {
	p := ParsePlan(`{"thought":"jalankan ls","command":"ls -la"}`)
	if p.Tool != "run_command" {
		t.Fatalf("Tool = %q, want %q", p.Tool, "run_command")
	}
	if p.Compat == "" {
		t.Errorf("Compat = %q, want non-empty legacy-format note", p.Compat)
	}
	var args map[string]string
	if err := json.Unmarshal(p.Args, &args); err != nil {
		t.Fatalf("Args not valid JSON: %v", err)
	}
	if args["command"] != "ls -la" {
		t.Errorf("Args[command] = %q, want %q", args["command"], "ls -la")
	}
}

func TestParsePlan_HoistsRootFieldsIntoArgs(t *testing.T) {
	p := ParsePlan(`{"thought":"baca","tool":"read_file","path":"main.py"}`)
	var args map[string]string
	if err := json.Unmarshal(p.Args, &args); err != nil {
		t.Fatalf("Args not valid JSON: %v", err)
	}
	if args["path"] != "main.py" {
		t.Errorf("Args[path] = %q, want %q (hoisted from root)", args["path"], "main.py")
	}
	if p.Compat == "" {
		t.Errorf("Compat = %q, want non-empty hoist note", p.Compat)
	}
}

func TestParsePlan_PicksLastCandidateObject(t *testing.T) {
	// Mirrors the python source's reversed-scan behavior: an earlier
	// stray '{' in explanatory text must not shadow the real, later
	// tool-call JSON.
	reply := `Let me explain {this is not json` + "\n" + `{"tool":"read_file","args":{"path":"x.py"}}`
	p := ParsePlan(reply)
	if p.Tool != "read_file" {
		t.Fatalf("Tool = %q, want %q", p.Tool, "read_file")
	}
}

func TestParsePlan_NeverPanics(t *testing.T) {
	inputs := []string{
		"",
		"{",
		"}",
		"{{{{{{{{",
		`{"tool": {"nested": "not a string"}}`,
		`{"args": "not an object"}`,
		`{"args": [1,2,3], "tool":"x"}`,
	}
	for _, in := range inputs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("ParsePlan(%q) panicked: %v", in, r)
				}
			}()
			ParsePlan(in)
		}()
	}
}
