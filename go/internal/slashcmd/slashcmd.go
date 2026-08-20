package slashcmd

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"

	"github.com/monang404/luna-go/internal/settings"
	"github.com/monang404/luna-go/internal/subagent"
)

// State exposes the necessary REPL state and controls to slash commands.
type State interface {
	Out() io.Writer
	Err() io.Writer
	ClearMessages()
	GetModel() string
	SetModel(string)
	GetSettings() *settings.Settings
	GetStats() (tokensIn, tokensOut, turns int)
	CompactHistory(ctx context.Context, instruction string)
	PrintPermissions()
	PrintStatus()
	RequestExit()
	RewindHistory(n int)
	LoadSession(id string) error
	InjectPrompt(ctx context.Context, prompt string)
}

// SlashCommand represents a command that can be run from the REPL via / prefix.
type SlashCommand interface {
	Name() string
	Aliases() []string
	Description() string
	Execute(ctx context.Context, args []string, state State) error
}

// Registry holds all registered slash commands and routes input to them.
type Registry struct {
	cmds map[string]SlashCommand
	list []SlashCommand
}

func NewRegistry() *Registry {
	return &Registry{
		cmds: make(map[string]SlashCommand),
	}
}

func (r *Registry) Register(cmd SlashCommand) {
	r.list = append(r.list, cmd)
	r.cmds[cmd.Name()] = cmd
	for _, alias := range cmd.Aliases() {
		r.cmds[alias] = cmd
	}
}

// Dispatch parses the input and executes the corresponding slash command.
// Returns true if a command was found and executed (even if it returned an error).
func (r *Registry) Dispatch(ctx context.Context, input string, state State) (bool, error) {
	if !strings.HasPrefix(input, "/") {
		return false, nil
	}

	parts := strings.Split(input, " ")
	cmdName := strings.ToLower(parts[0])

	cmd, ok := r.cmds[cmdName]
	if !ok {
		fmt.Fprintf(state.Out(), "Perintah tidak dikenal: %s. Ketik /help untuk bantuan.\n", cmdName)
		return true, nil
	}

	var args []string
	if len(parts) > 1 {
		args = parts[1:]
	}

	err := cmd.Execute(ctx, args, state)
	return true, err
}

func (r *Registry) Commands() []SlashCommand {
	return r.list
}

// -- Built-in commands --

func RegisterBuiltins(r *Registry) {
	r.Register(cmdExit{})
	r.Register(cmdClear{})
	r.Register(&cmdHelp{registry: r})
	r.Register(cmdModel{})
	r.Register(cmdCost{})
	r.Register(cmdTodo{})
	r.Register(cmdCompact{})
	r.Register(cmdPermissions{})
	r.Register(cmdInit{})
	r.Register(cmdResume{})
	r.Register(cmdRewind{})
	r.Register(cmdAgents{})
	r.Register(cmdStatus{})
}

type cmdExit struct{}

func (cmdExit) Name() string        { return "/exit" }
func (cmdExit) Aliases() []string   { return []string{"/quit", "/q"} }
func (cmdExit) Description() string { return "Keluar dari luna" }
func (cmdExit) Execute(_ context.Context, _ []string, state State) error {
	fmt.Fprintln(state.Out(), "Keluar dari luna.")
	state.RequestExit()
	return nil
}

type cmdClear struct{}

func (cmdClear) Name() string        { return "/clear" }
func (cmdClear) Aliases() []string   { return nil }
func (cmdClear) Description() string { return "Hapus context percakapan" }
func (cmdClear) Execute(_ context.Context, _ []string, state State) error {
	state.ClearMessages()
	fmt.Fprintln(state.Out(), "Context percakapan dihapus.")
	return nil
}

type cmdHelp struct{ registry *Registry }

func (cmdHelp) Name() string        { return "/help" }
func (cmdHelp) Aliases() []string   { return []string{"/h"} }
func (cmdHelp) Description() string { return "Tampilkan bantuan ini" }
func (c *cmdHelp) Execute(_ context.Context, _ []string, state State) error {
	fmt.Fprintln(state.Out(), "Slash Commands:")
	cmds := c.registry.Commands()
	// Sort by name for deterministic output
	sort.Slice(cmds, func(i, j int) bool { return cmds[i].Name() < cmds[j].Name() })
	for _, cmd := range cmds {
		names := []string{cmd.Name()}
		names = append(names, cmd.Aliases()...)
		fmt.Fprintf(state.Out(), "  %-18s %s\n", strings.Join(names, ", "), cmd.Description())
	}
	return nil
}

type cmdModel struct{}

func (cmdModel) Name() string        { return "/model" }
func (cmdModel) Aliases() []string   { return nil }
func (cmdModel) Description() string { return "Tampilkan/ganti model aktif" }
func (cmdModel) Execute(_ context.Context, args []string, state State) error {
	if len(args) > 0 {
		model := strings.Join(args, " ")
		state.SetModel(model)
		fmt.Fprintf(state.Out(), "Model diubah ke: %s\n", model)
	} else {
		fmt.Fprintf(state.Out(), "Model aktif: %s\n", state.GetModel())
	}
	return nil
}

type cmdCost struct{}

func (cmdCost) Name() string        { return "/cost" }
func (cmdCost) Aliases() []string   { return nil }
func (cmdCost) Description() string { return "Tampilkan estimasi token & biaya" }
func (cmdCost) Execute(_ context.Context, _ []string, state State) error {
	in, out, turns := state.GetStats()
	fmt.Fprintf(state.Out(), "Token input: ~%d | Token output: ~%d | Turns: %d\n", in, out, turns)
	return nil
}

type cmdTodo struct{}

func (cmdTodo) Name() string        { return "/todo" }
func (cmdTodo) Aliases() []string   { return nil }
func (cmdTodo) Description() string { return "Tampilkan todo list sesi" }
func (cmdTodo) Execute(_ context.Context, _ []string, state State) error {
	fmt.Fprintln(state.Out(), "(todo list belum dimuat — gunakan tool todo_read via prompt)")
	return nil
}

type cmdCompact struct{}

func (cmdCompact) Name() string        { return "/compact" }
func (cmdCompact) Aliases() []string   { return nil }
func (cmdCompact) Description() string { return "Ringkas history lama (manual trigger)" }
func (cmdCompact) Execute(ctx context.Context, args []string, state State) error {
	state.CompactHistory(ctx, strings.Join(args, " "))
	return nil
}

type cmdPermissions struct{}

func (cmdPermissions) Name() string        { return "/permissions" }
func (cmdPermissions) Aliases() []string   { return nil }
func (cmdPermissions) Description() string { return "Tampilkan aturan permission aktif" }
func (cmdPermissions) Execute(_ context.Context, _ []string, state State) error {
	state.PrintPermissions()
	return nil
}

type cmdInit struct{}

func (cmdInit) Name() string        { return "/init" }
func (cmdInit) Aliases() []string   { return nil }
func (cmdInit) Description() string { return "Generate LUNA.md dari project (segera hadir)" }
func (cmdInit) Execute(_ context.Context, _ []string, state State) error {
	fmt.Fprintln(state.Out(), "Fitur /init (generate LUNA.md) akan tersedia di SESSION-63.")
	return nil
}

type cmdResume struct{}

func (cmdResume) Name() string        { return "/resume" }
func (cmdResume) Aliases() []string   { return nil }
func (cmdResume) Description() string { return "Lanjutkan sesi sebelumnya" }
func (cmdResume) Execute(_ context.Context, args []string, state State) error {
	if len(args) == 0 {
		fmt.Fprintln(state.Out(), "Penggunaan: /resume <session_id>")
		return nil
	}
	err := state.LoadSession(args[0])
	if err != nil {
		fmt.Fprintf(state.Out(), "Gagal resume: %v\n", err)
	} else {
		fmt.Fprintf(state.Out(), "Berhasil resume session %s.\n", args[0])
	}
	return nil
}

type cmdRewind struct{}

func (cmdRewind) Name() string        { return "/rewind" }
func (cmdRewind) Aliases() []string   { return nil }
func (cmdRewind) Description() string { return "Hapus N pesan terakhir (default 1)" }
func (cmdRewind) Execute(_ context.Context, args []string, state State) error {
	n := 1
	if len(args) > 0 {
		if parsed, err := strconv.Atoi(args[0]); err == nil {
			n = parsed
		}
	}
	state.RewindHistory(n)
	return nil
}

type cmdAgents struct{}

func (cmdAgents) Name() string        { return "/agents" }
func (cmdAgents) Aliases() []string   { return nil }
func (cmdAgents) Description() string { return "Lihat daftar subagent yang tersedia" }
func (cmdAgents) Execute(_ context.Context, _ []string, state State) error {
	defs := subagent.GetAllDefinitions()
	if len(defs) == 0 {
		fmt.Fprintln(state.Out(), "Tidak ada subagent yang dimuat.")
		return nil
	}

	fmt.Fprintln(state.Out(), "Subagents:")
	for _, def := range defs {
		toolsStr := "readonly"
		if len(def.Tools) > 0 {
			toolsStr = strings.Join(def.Tools, ", ")
		}
		fmt.Fprintf(state.Out(), "\n  Role: %s\n", def.Role)
		fmt.Fprintf(state.Out(), "  Desc: %s\n", def.Description)
		fmt.Fprintf(state.Out(), "  Tools: %s\n", toolsStr)
	}
	return nil
}

type cmdStatus struct{}

func (cmdStatus) Name() string        { return "/status" }
func (cmdStatus) Aliases() []string   { return []string{"/doctor"} }
func (cmdStatus) Description() string { return "Diagnostik konfigurasi" }
func (cmdStatus) Execute(_ context.Context, _ []string, state State) error {
	state.PrintStatus()
	return nil
}
