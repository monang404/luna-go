// Package tools ports 30-luna/05-tools/ (AI_TOOL_REGISTRY, args extraction,
// autodep, and the generic dispatcher) into Go. Ported in SESSION-43.
//
// Deliberately excluded from this package (per the session's
// scope.exclude): concrete implementations of the 17 individual tools
// (fs read/write/patch/delete, git, web_fetch, todo -- SESSION-47/48) and
// the model-facing JSON tool-schema fed to the LLM request payload (that
// lives in internal/llmclient/internal/agent, not here). What SESSION-43
// delivers is the stable interface + registry + dispatch skeleton those
// later sessions plug concrete tools into without redesigning any of it.
package tools

import (
	"context"
	"encoding/json"

	"github.com/monang404/luna-go/internal/permission"
)

// Result is what a successful Tool.Execute call returns: the text the
// model sees as the tool's output, mirroring what every
// `_ai_tool_*` zsh function prints to stdout for the agent loop to
// capture as `$output`.
type Result struct {
	Output string
}

// Tool is the interface every concrete tool (SESSION-47/48: read_file,
// write_file, git_status, web_fetch, ...) implements. It is deliberately
// small -- name + capability + one execute method -- because the
// permission Level, human-readable description, and JSON schema for each
// tool name already live in Registry (registry.go) as data, not as part
// of the interface. A concrete Tool only needs to know how to run itself;
// the Dispatcher (dispatch.go) is what wires Registry metadata,
// permission.CheckPermission, and Execute together.
//
// Capability is repeated here (rather than only living in Registry)
// so a Tool value is self-describing even before it's been registered --
// e.g. for a unit test that constructs a Tool directly and asserts on its
// declared capability without going through the Dispatcher at all.
type Tool interface {
	// Name is the tool name the model requests by (e.g. "read_file"),
	// matching one of the AI_TOOL_REGISTRY keys in registry.go.
	Name() string
	// Capability is the permission.Capability this tool needs to run,
	// mirroring the AI_TOOL_CAPABILITY entry for the same name.
	Capability() permission.Capability
	// Execute runs the tool against already-normalized, already
	// schema-validated args (see args.go/schema.go) and an
	// already-approved permission decision -- a Tool implementation
	// never itself calls permission.CheckPermission or re-validates
	// args; the Dispatcher guarantees both happened first.
	Execute(ctx context.Context, args json.RawMessage) (Result, error)
}

// NoopTool is a dummy Tool with no side effects, existing purely so the
// full dispatch pipeline (normalize -> validate -> permission check ->
// execute) can be exercised end-to-end in tests without depending on any
// real tool implementation landing first (SESSION-43's AC-04, and the
// session brief's own "Tool dummy 'noop'" scope line). It is exported
// (not test-only) because SESSION-47/48 and any future integration test
// of the wired-up Dispatcher benefit from the same fixture.
type NoopTool struct {
	// ToolName is returned by Name(). Defaults to "noop" if empty.
	ToolName string
	// Cap is returned by Capability(). Defaults to
	// permission.CapFilesystemRead (readonly) if empty, which lets a
	// caller pick a non-readonly capability (e.g. CapFilesystemWrite)
	// to specifically exercise the permission-ask path.
	Cap permission.Capability
	// Output is echoed back unchanged as the successful Result.
	Output string
}

func (n NoopTool) Name() string {
	if n.ToolName == "" {
		return "noop"
	}
	return n.ToolName
}

func (n NoopTool) Capability() permission.Capability {
	if n.Cap == "" {
		return permission.CapFilesystemRead
	}
	return n.Cap
}

func (n NoopTool) Execute(_ context.Context, _ json.RawMessage) (Result, error) {
	return Result{Output: n.Output}, nil
}
