package tools

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"testing"

	"github.com/monang404/luna-go/internal/permission"
)

// --- AC-01: Registry has the same 17(*) names/categories as AI_TOOL_REGISTRY ---
//
// (*) The session brief says "17 entri tool", but a literal count of
// 00-tool_registry.zsh's AI_TOOL_REGISTRY assoc array is 18 (read_file,
// list_dir, grep_search, glob_search, count_lines, write_file, edit_file,
// patch_file, run_command, exec_process, run_test, move_file,
// delete_file, git_status, git_diff, web_fetch, todo_write, todo_read).
// AC-01's own wording ("sama persis nama & kategorinya dengan
// AI_TOOL_REGISTRY lama") makes the actual source the ground truth, not
// the brief's count -- so this test asserts against a literal transcript
// of the zsh source's keys instead of a hardcoded "17", and documents the
// discrepancy here rather than silently under- or over-porting a tool to
// force the number to match.

func zshRegistryNamesAndLevels() map[string]permission.Level {
	// Transcribed 1:1 from 00-tool_registry.zsh's AI_TOOL_REGISTRY
	// (name -> "|"-suffix) and AI_TOOL_CAPABILITY (name -> capability,
	// cross-checked for the level each capability implies).
	return map[string]permission.Level{
		"read_file":     permission.LevelReadonly,
		"list_dir":      permission.LevelReadonly,
		"grep_search":   permission.LevelReadonly,
		"glob_search":   permission.LevelReadonly,
		"count_lines":   permission.LevelReadonly,
		"write_file":    permission.LevelWrite,
		"edit_file":     permission.LevelWrite,
		"patch_file":    permission.LevelWrite,
		"run_command":   permission.LevelShell,
		"exec_process":  permission.LevelProcess,
		"run_test":      permission.LevelProcess,
		"move_file":     permission.LevelWrite,
		"delete_file":   permission.LevelShell,
		"git_status":    permission.LevelReadonly,
		"git_diff":      permission.LevelReadonly,
		"web_fetch":     permission.LevelShell,
		"todo_write":    permission.LevelReadonly,
		"todo_read":     permission.LevelReadonly,
		"bash_output":   permission.LevelReadonly,
		"kill_shell":    permission.LevelProcess,
		"delegate_task": permission.LevelReadonly,
		"web_search":    permission.LevelShell,
	}
}

func zshRegistryCapabilities() map[string]permission.Capability {
	// Transcribed 1:1 from AI_TOOL_CAPABILITY.
	return map[string]permission.Capability{
		"read_file":     permission.CapFilesystemRead,
		"list_dir":      permission.CapFilesystemRead,
		"grep_search":   permission.CapFilesystemRead,
		"glob_search":   permission.CapFilesystemRead,
		"count_lines":   permission.CapFilesystemRead,
		"write_file":    permission.CapFilesystemWrite,
		"edit_file":     permission.CapFilesystemWrite,
		"patch_file":    permission.CapFilesystemWrite,
		"run_command":   permission.CapShellArbitrary,
		"exec_process":  permission.CapProcessExecute,
		"run_test":      permission.CapProcessTest,
		"move_file":     permission.CapFilesystemWrite,
		"delete_file":   permission.CapFilesystemDelete,
		"git_status":    permission.CapGitRead,
		"git_diff":      permission.CapGitRead,
		"web_fetch":     permission.CapNetworkPublic,
		"todo_write":    permission.CapSessionTodo,
		"todo_read":     permission.CapSessionTodo,
		"bash_output":   permission.CapProcessExecute,
		"kill_shell":    permission.CapProcessExecute,
		"delegate_task": permission.CapProcessExecute,
		"web_search":    permission.CapNetworkPublic,
	}
}

func TestRegistry_MatchesZshSource1To1(t *testing.T) {
	wantLevels := zshRegistryNamesAndLevels()
	wantCaps := zshRegistryCapabilities()

	if len(Registry) != len(wantLevels) {
		gotNames, wantNames := Names(), sortedKeys(wantLevels)
		t.Fatalf("Registry has %d entries, zsh source has %d\ngot:  %v\nwant: %v",
			len(Registry), len(wantLevels), gotNames, wantNames)
	}
	for name, wantLevel := range wantLevels {
		entry, ok := Registry[name]
		if !ok {
			t.Errorf("Registry missing tool %q (present in zsh AI_TOOL_REGISTRY)", name)
			continue
		}
		if entry.Level != wantLevel {
			t.Errorf("Registry[%q].Level = %q, want %q", name, entry.Level, wantLevel)
		}
		if entry.Capability != wantCaps[name] {
			t.Errorf("Registry[%q].Capability = %q, want %q", name, entry.Capability, wantCaps[name])
		}
	}
	for name := range Registry {
		if _, ok := wantLevels[name]; !ok {
			t.Errorf("Registry has extra tool %q not present in zsh AI_TOOL_REGISTRY", name)
		}
	}
}

func sortedKeys(m map[string]permission.Level) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestManifest_HidesRunCommandByDefault(t *testing.T) {
	t.Setenv("AI_AGENT_EXPOSE_ARBITRARY_SHELL", "")
	m := Manifest()
	if contains(m, "run_command ") {
		t.Errorf("Manifest() should hide run_command by default, got:\n%s", m)
	}
}

func TestManifest_ExposesRunCommandWhenFlagSet(t *testing.T) {
	t.Setenv("AI_AGENT_EXPOSE_ARBITRARY_SHELL", "1")
	m := Manifest()
	if !contains(m, "run_command ") {
		t.Errorf("Manifest() should expose run_command when AI_AGENT_EXPOSE_ARBITRARY_SHELL=1, got:\n%s", m)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

// --- AC-02: dispatcher rejects malformed args before Execute ---

func newTestDispatcher(t *testing.T, entry Entry, tool Tool) *Dispatcher {
	t.Helper()
	d := NewDispatcher()
	if err := d.Register(tool.Name(), entry, tool); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return d
}

// AllowOutsideProject:true is used throughout this file's fixtures so
// dispatch tests can exercise normalize/validate/permission-call/execute
// wiring with plain relative test paths ("a.txt") without also having to
// stand up a real project-root directory tree matching the test binary's
// working directory -- IsPathAllowed/PathWithinProject's own containment
// logic already has dedicated, thorough coverage in
// internal/permission's own table-driven tests (SESSION-42's AC-03), so
// re-deriving that fixture here would just duplicate it.

func alwaysAllowDeps() PermDeps {
	return PermDeps{
		AgentCtx: permission.NewAgentContext("sess-1", "/tmp/project", true, permission.RolePrimary), // YOLO: write auto-allows at level dispatch
		Config:   permission.PermConfig{WriteMode: "yolo", ShellMode: "yolo", ProcessMode: "yolo", AllowOutsideProject: true},
		Tracker:  permission.NewApprovalTracker(),
		// Even under YOLO, CheckPermission's *capability* gate (distinct
		// from the level-dispatch YOLO checks above) still fires a
		// one-off ask for any capability not already granted by
		// defaultCapabilities() (filesystem.write/delete,
		// process.execute, network.public, shell.arbitrary all start
		// denied -- see internal/permission/context.go). An always-approve
		// Ask keeps this fixture genuinely "always allow" without every
		// test needing to pre-Grant the specific capability it exercises.
		Ask: func(string) (bool, error) { return true, nil },
		Cwd: "/tmp/project",
	}
}

type executedTool struct {
	name    string
	cap     permission.Capability
	execute func(ctx context.Context, args json.RawMessage) (Result, error)
	called  bool
}

func (e *executedTool) Name() string                      { return e.name }
func (e *executedTool) Capability() permission.Capability { return e.cap }
func (e *executedTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	e.called = true
	if e.execute != nil {
		return e.execute(ctx, args)
	}
	return Result{}, nil
}

func TestDispatch_RejectsMalformedJSON(t *testing.T) {
	tool := &executedTool{name: "read_file", cap: permission.CapFilesystemRead}
	d := newTestDispatcher(t, Entry{Level: permission.LevelReadonly, Capability: permission.CapFilesystemRead}, tool)

	_, err := d.Dispatch(context.Background(), alwaysAllowDeps(), "read_file", json.RawMessage(`{not valid json`))
	if err == nil {
		t.Fatal("Dispatch: expected error for malformed JSON args, got nil")
	}
	if tool.called {
		t.Error("Dispatch: Execute must not be called when args are malformed")
	}
}

func TestDispatch_RejectsSchemaViolation(t *testing.T) {
	tool := &executedTool{name: "read_file", cap: permission.CapFilesystemRead}
	d := newTestDispatcher(t, Entry{Level: permission.LevelReadonly, Capability: permission.CapFilesystemRead}, tool)

	// read_file requires a non-empty "path" string; this args object has none.
	_, err := d.Dispatch(context.Background(), alwaysAllowDeps(), "read_file", json.RawMessage(`{"offset": 3}`))
	if err == nil {
		t.Fatal("Dispatch: expected schema-validation error, got nil")
	}
	if tool.called {
		t.Error("Dispatch: Execute must not be called when schema validation fails")
	}
}

func TestDispatch_UnknownTool(t *testing.T) {
	d := NewDispatcher()
	_, err := d.Dispatch(context.Background(), alwaysAllowDeps(), "does_not_exist", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("Dispatch: expected error for unknown tool name, got nil")
	}
}

// --- AC-03: dispatcher calls permission.CheckPermission before Execute
//     for non-readonly tools (and honors a denial by never calling Execute) ---

func TestDispatch_NonReadonlyDeniedWithoutAsk(t *testing.T) {
	tool := &executedTool{name: "write_file", cap: permission.CapFilesystemWrite}
	d := newTestDispatcher(t, Entry{Level: permission.LevelWrite, Capability: permission.CapFilesystemWrite}, tool)

	deps := PermDeps{
		AgentCtx: permission.NewAgentContext("sess-1", "/tmp/project", false, permission.RolePrimary), // no YOLO
		Config:   permission.PermConfig{WriteMode: "ask_once_per_file", AllowOutsideProject: true},
		Tracker:  permission.NewApprovalTracker(),
		Ask:      nil, // no UI wired up -> fail-closed
		Cwd:      "/tmp/project",
	}

	_, err := d.Dispatch(context.Background(), deps, "write_file", json.RawMessage(`{"path":"a.txt","content":"x"}`))
	if err == nil {
		t.Fatal("Dispatch: expected permission denial (nil AskFunc, non-yolo write) to produce an error")
	}
	if tool.called {
		t.Error("Dispatch: Execute must not be called when permission.CheckPermission denies the request")
	}
}

func TestDispatch_NonReadonlyAllowedUnderYolo(t *testing.T) {
	tool := &executedTool{name: "write_file", cap: permission.CapFilesystemWrite}
	d := newTestDispatcher(t, Entry{Level: permission.LevelWrite, Capability: permission.CapFilesystemWrite}, tool)

	res, err := d.Dispatch(context.Background(), alwaysAllowDeps(), "write_file", json.RawMessage(`{"path":"a.txt","content":"x"}`))
	if err != nil {
		t.Fatalf("Dispatch: unexpected error under YOLO write mode: %v", err)
	}
	if !tool.called {
		t.Error("Dispatch: Execute should have been called once permission.CheckPermission allowed the request")
	}
	_ = res
}

func TestDispatch_ReadonlyNeverAsks(t *testing.T) {
	tool := &executedTool{name: "read_file", cap: permission.CapFilesystemRead}
	d := newTestDispatcher(t, Entry{Level: permission.LevelReadonly, Capability: permission.CapFilesystemRead}, tool)

	deps := PermDeps{
		AgentCtx: permission.NewAgentContext("sess-1", "/tmp/project", false, permission.RolePrimary),
		Config:   permission.PermConfig{AllowOutsideProject: true}, // no yolo anywhere
		Tracker:  permission.NewApprovalTracker(),
		Ask:      nil, // if a readonly dispatch ever tried to ask, this proves it by returning (false, nil) and denying
		Cwd:      "/tmp/project",
	}
	_, err := d.Dispatch(context.Background(), deps, "read_file", json.RawMessage(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatalf("Dispatch: readonly tool should never require an ask, got error: %v", err)
	}
	if !tool.called {
		t.Error("Dispatch: Execute should have been called for an allowed readonly tool")
	}
}

// --- AC-04: a dummy 'noop' tool can be registered & dispatched end-to-end ---

func TestDispatch_NoopToolEndToEnd(t *testing.T) {
	noop := NoopTool{Output: "ok"}
	d := NewDispatcher()
	if err := d.Register(noop.Name(), Entry{
		Description: "no-op test fixture",
		Level:       permission.LevelReadonly,
		Capability:  noop.Capability(),
	}, noop); err != nil {
		t.Fatalf("Register(noop): %v", err)
	}

	res, err := d.Dispatch(context.Background(), alwaysAllowDeps(), "noop", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("Dispatch(noop): unexpected error: %v", err)
	}
	if res.Output != "ok" {
		t.Errorf("Dispatch(noop): Output = %q, want %q", res.Output, "ok")
	}
}

func TestDispatch_NoopToolCanExerciseWritePermissionPath(t *testing.T) {
	// A noop configured with a write-level capability lets AC-03 and
	// AC-04 be exercised together: the same dummy tool proves both that
	// the full pipeline runs end-to-end AND that a non-readonly noop
	// still goes through permission.CheckPermission (denied here, since
	// no ask hook and no yolo).
	noop := NoopTool{ToolName: "noop", Cap: permission.CapFilesystemWrite, Output: "should not run"}
	d := NewDispatcher()
	if err := d.Register("noop", Entry{Level: permission.LevelWrite, Capability: permission.CapFilesystemWrite}, noop); err != nil {
		t.Fatalf("Register(noop): %v", err)
	}
	deps := PermDeps{
		AgentCtx: permission.NewAgentContext("sess-1", "/tmp/project", false, permission.RolePrimary),
		Config:   permission.PermConfig{WriteMode: "ask_once_per_file", AllowOutsideProject: true},
		Tracker:  permission.NewApprovalTracker(),
		Ask:      nil,
		Cwd:      "/tmp/project",
	}
	_, err := d.Dispatch(context.Background(), deps, "noop", json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("Dispatch(noop write-level, no ask): expected permission denial, got nil error")
	}
}

// --- Register / RegisterFromRegistry plumbing ---

func TestRegister_RejectsDuplicateName(t *testing.T) {
	d := NewDispatcher()
	tool := &executedTool{name: "noop", cap: permission.CapFilesystemRead}
	if err := d.Register("noop", Entry{Level: permission.LevelReadonly}, tool); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := d.Register("noop", Entry{Level: permission.LevelReadonly}, tool); err == nil {
		t.Fatal("second Register with the same name should have failed")
	}
}

func TestRegisterFromRegistry_UsesRegistryMetadata(t *testing.T) {
	d := NewDispatcher()
	tool := &executedTool{name: "read_file", cap: permission.CapFilesystemRead}
	if err := d.RegisterFromRegistry(tool); err != nil {
		t.Fatalf("RegisterFromRegistry: %v", err)
	}
	_, err := d.Dispatch(context.Background(), alwaysAllowDeps(), "read_file", json.RawMessage(`{"path":"a.txt"}`))
	if err != nil {
		t.Fatalf("Dispatch after RegisterFromRegistry: %v", err)
	}
}

func TestRegisterFromRegistry_RejectsUnknownName(t *testing.T) {
	d := NewDispatcher()
	tool := &executedTool{name: "not_a_real_tool", cap: permission.CapFilesystemRead}
	if err := d.RegisterFromRegistry(tool); err == nil {
		t.Fatal("RegisterFromRegistry: expected error for a name not in Registry")
	}
}

// --- args.go: ExtractField / ExtractPath / NormalizeArgs ---

func TestExtractPath_BareString(t *testing.T) {
	got := ExtractPath(json.RawMessage(`"src/main.go"`))
	if got != "src/main.go" {
		t.Errorf("ExtractPath(bare string) = %q, want %q", got, "src/main.go")
	}
}

func TestExtractPath_AlternativeFieldNames(t *testing.T) {
	cases := []struct {
		args json.RawMessage
		want string
	}{
		{json.RawMessage(`{"path":"a.txt"}`), "a.txt"},
		{json.RawMessage(`{"file":"b.txt"}`), "b.txt"},
		{json.RawMessage(`{"filename":"c.txt"}`), "c.txt"},
		{json.RawMessage(`{"dir":"d"}`), "d"},
		{json.RawMessage(`{"directory":"e"}`), "e"},
		{json.RawMessage(`{"parameters":{"path":"nested.txt"}}`), "nested.txt"},
	}
	for _, c := range cases {
		if got := ExtractPath(c.args); got != c.want {
			t.Errorf("ExtractPath(%s) = %q, want %q", c.args, got, c.want)
		}
	}
}

func TestNormalizeArgs_BareStringWrapsPerTool(t *testing.T) {
	got, err := NormalizeArgs(json.RawMessage(`"a.txt"`), "read_file")
	if err != nil {
		t.Fatalf("NormalizeArgs: %v", err)
	}
	var obj map[string]string
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("NormalizeArgs output not valid JSON: %v", err)
	}
	if obj["path"] != "a.txt" {
		t.Errorf("NormalizeArgs(bare string, read_file) = %s, want path=a.txt", got)
	}
}

func TestNormalizeArgs_FillsMissingCanonicalField(t *testing.T) {
	got, err := NormalizeArgs(json.RawMessage(`{"file":"b.txt"}`), "read_file")
	if err != nil {
		t.Fatalf("NormalizeArgs: %v", err)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal(got, &obj); err != nil {
		t.Fatalf("NormalizeArgs output not valid JSON: %v", err)
	}
	if obj["path"] != "b.txt" {
		t.Errorf("NormalizeArgs should fill path from file, got %s", got)
	}
}

func TestNormalizeArgs_NeverClobbersExistingField(t *testing.T) {
	got, err := NormalizeArgs(json.RawMessage(`{"path":"real.txt","file":"decoy.txt"}`), "read_file")
	if err != nil {
		t.Fatalf("NormalizeArgs: %v", err)
	}
	var obj map[string]interface{}
	_ = json.Unmarshal(got, &obj)
	if obj["path"] != "real.txt" {
		t.Errorf("NormalizeArgs should never overwrite an existing path, got %s", got)
	}
}

func TestNormalizeArgs_MalformedJSONIsAnError(t *testing.T) {
	_, err := NormalizeArgs(json.RawMessage(`{not json`), "read_file")
	if err == nil {
		t.Fatal("NormalizeArgs: expected error for unparseable JSON")
	}
}

func TestNormalizeArgs_EmptyArgsBecomesEmptyObject(t *testing.T) {
	got, err := NormalizeArgs(json.RawMessage(``), "list_dir")
	if err != nil {
		t.Fatalf("NormalizeArgs: %v", err)
	}
	if string(got) != "{}" {
		t.Errorf("NormalizeArgs(empty) = %s, want {}", got)
	}
}

// --- schema.go: ValidateArgs per tool ---

func TestValidateArgs_TableDriven(t *testing.T) {
	cases := []struct {
		name    string
		tool    string
		args    string
		wantErr bool
	}{
		{"read_file valid", "read_file", `{"path":"a.txt"}`, false},
		{"read_file missing path", "read_file", `{}`, true},
		{"read_file empty path", "read_file", `{"path":""}`, true},
		{"read_file bad offset type", "read_file", `{"path":"a.txt","offset":"x"}`, true},
		{"list_dir defaults ok", "list_dir", `{}`, false},
		{"list_dir bad type", "list_dir", `{"path":5}`, true},
		{"grep_search valid", "grep_search", `{"pattern":"foo"}`, false},
		{"grep_search empty pattern", "grep_search", `{"pattern":""}`, true},
		{"write_file valid", "write_file", `{"path":"a.txt","content":""}`, false},
		{"write_file missing content", "write_file", `{"path":"a.txt"}`, true},
		{"exec_process valid", "exec_process", `{"program":"ls","args":["-la"],"timeout":10}`, false},
		{"exec_process newline in args", "exec_process", `{"program":"ls","args":["-la\nrm -rf /"]}`, true},
		{"exec_process timeout out of range", "exec_process", `{"program":"ls","timeout":9999}`, true},
		{"exec_process missing program", "exec_process", `{}`, true},
		{"todo_write valid", "todo_write", `{"items":[{"text":"do thing","status":"pending"}]}`, false},
		{"todo_write empty items", "todo_write", `{"items":[]}`, true},
		{"todo_write bad status", "todo_write", `{"items":[{"text":"x","status":"bogus"}]}`, true},
		{"git_status always ok", "git_status", `{}`, false},
		{"todo_read always ok", "todo_read", `{}`, false},
		{"delete_file valid", "delete_file", `{"path":"a.txt"}`, false},
		{"move_file missing dest", "move_file", `{"path":"a.txt"}`, true},
		{"web_fetch valid", "web_fetch", `{"url":"https://example.com"}`, false},
		{"web_fetch missing url", "web_fetch", `{}`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateArgs(c.tool, json.RawMessage(c.args))
			if (err != nil) != c.wantErr {
				t.Errorf("ValidateArgs(%q, %s) error = %v, wantErr %v", c.tool, c.args, err, c.wantErr)
			}
		})
	}
}

// --- autodep.go: pure mapping/extraction ---

func TestExtractMissingCmd(t *testing.T) {
	got := ExtractMissingCmd("zsh: command not found: fooutil\nsome other line")
	if got != "fooutil" {
		t.Errorf("ExtractMissingCmd = %q, want %q", got, "fooutil")
	}
	if got := ExtractMissingCmd("no such line here"); got != "" {
		t.Errorf("ExtractMissingCmd(no match) = %q, want empty", got)
	}
}

func TestCmdToPackage_TermuxVsAPT(t *testing.T) {
	if got := CmdToPackage("python3", PkgManagerTermux); got != "python" {
		t.Errorf("CmdToPackage(python3, termux) = %q, want %q", got, "python")
	}
	if got := CmdToPackage("python3", PkgManagerAPT); got != "python3" {
		t.Errorf("CmdToPackage(python3, apt) = %q, want %q", got, "python3")
	}
	if got := CmdToPackage("jq", PkgManagerAPT); got != "jq" {
		t.Errorf("CmdToPackage(jq, apt) = %q, want %q", got, "jq")
	}
	if got := CmdToPackage("totally_unknown_cmd", PkgManagerAPT); got != "" {
		t.Errorf("CmdToPackage(unknown) = %q, want empty", got)
	}
}

var _ = errors.New // keep errors imported if future tests need errdefs
