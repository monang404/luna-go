// Package repl implements the unified agentic REPL that serves as luna's
// default entry point — the single interactive loop where the model
// autonomously chooses tools, replacing the 30+ subcommand model.
//
// This is the Go equivalent of Claude Code's single `claude` entry point:
//   - `luna` (no args) → interactive REPL with agent loop
//   - `luna "prompt"` → send initial prompt then enter REPL
//   - `luna -p "prompt"` → headless mode, print result, exit
//
// The REPL reads user input, dispatches slash commands (/ prefix) to the
// slash command registry, and sends everything else through the agent loop
// (model decides which tools to call). Tool calls, permission prompts,
// and results are rendered inline.
//
// SESSION-60 scope: REPL core, agent loop integration, initial prompt,
// print mode stub.
package repl

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chzyer/readline"
	"github.com/pterm/pterm"

	"github.com/monang404/luna-go/internal/agent"
	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/hooks"
	"github.com/monang404/luna-go/internal/llmclient"
	"github.com/monang404/luna-go/internal/memory"
	"github.com/monang404/luna-go/internal/mcp"
	"github.com/monang404/luna-go/internal/permission"
	"github.com/monang404/luna-go/internal/settings"
	"github.com/monang404/luna-go/internal/slashcmd"
	"github.com/monang404/luna-go/internal/subagent"
	"github.com/monang404/luna-go/internal/tools"
)

// Options configures a REPL session.
type Options struct {
	// ProjectRoot is the working directory / project root.
	ProjectRoot string
	// InitialPrompt is sent as the first user message before entering
	// the interactive loop. Empty means no initial prompt.
	InitialPrompt string
	// PrintMode when true runs a single prompt and exits (no interactive loop).
	PrintMode bool
	// OutputFormat controls output in PrintMode: "text", "json", "stream-json".
	OutputFormat string
	// Model overrides the configured model for this session.
	Model string
	// PermissionMode overrides the configured permission mode.
	PermissionMode settings.PermissionMode
	// ResumeSessionID resumes a previous session by checkpoint ID.
	ResumeSessionID string
	// ContinueLast resumes the most recent session.
	ContinueLast bool
	// AdditionalDirs are extra directories the agent may access.
	AdditionalDirs []string
	// DangerouslySkipPermissions bypasses all permission checks (with warning).
	DangerouslySkipPermissions bool

	// In/Out allow injection for testing. nil defaults to os.Stdin/os.Stdout.
	In  io.Reader
	Out io.Writer
	Err io.Writer

	// Dispatcher is the tool dispatcher to use. nil builds the default.
	Dispatcher *tools.Dispatcher
	// Loader holds subagent definitions.
	Loader *subagent.Loader
	// Limits overrides config limits. Zero-value fields use defaults.
	Limits config.Limits
	// Paths overrides config paths.
	Paths config.Paths

	// Complete overrides the LLM completion function (for testing).
	Complete func(ctx context.Context, deps agent.Deps, messages []llmclient.Message) (llmclient.Response, error)
}

// REPL holds the state of a running interactive session.
type REPL struct {
	opts     Options
	settings *settings.Settings
	messages []llmclient.Message
	in       io.Reader
	out      io.Writer
	err      io.Writer

	// Agent dependencies
	dispatcher *tools.Dispatcher
	loader     *subagent.Loader
	limits     config.Limits
	paths      config.Paths
	agentCtx   *permission.AgentContext
	permConfig permission.PermConfig
	tracker    *permission.ApprovalTracker

	// runtime state
	sessionID      string
	totalTokensIn  int
	totalTokensOut int
	turnCount      int
	slashReg       *slashcmd.Registry
	exitRequested  bool
	spinner        *pterm.SpinnerPrinter
}

// New creates a REPL with the given options.
func New(opts Options) *REPL {
	in := opts.In
	if in == nil {
		in = os.Stdin
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	errW := opts.Err
	if errW == nil {
		errW = os.Stderr
	}

	limits := opts.Limits
	if limits.AgentMaxSteps <= 0 {
		limits = config.LoadLimits()
	}
	paths := opts.Paths
	if paths.Luna == "" {
		paths = config.LoadPaths()
	}

	r := &REPL{
		opts:       opts,
		in:         in,
		out:        out,
		err:        errW,
		dispatcher: opts.Dispatcher,
		limits:     limits,
		paths:      paths,
		tracker:    permission.NewApprovalTracker(),
		sessionID:  fmt.Sprintf("repl-%d", time.Now().Unix()),
		slashReg:   slashcmd.NewRegistry(),
		loader:     opts.Loader,
	}
	slashcmd.RegisterBuiltins(r.slashReg, r.loader)
	return r
}

// Run is the main entry point. It:
//  1. Loads settings (hierarchical)
//  2. Loads memory files (LUNA.md)
//  3. Builds the system prompt
//  4. If InitialPrompt set: runs one agent turn
//  5. If PrintMode: exits after step 4
//  6. Otherwise: enters interactive loop
func (r *REPL) Run(ctx context.Context) error {
	// Load hierarchical settings
	s, err := settings.Load(r.opts.ProjectRoot)
	if err != nil {
		fmt.Fprintf(r.err, "luna: peringatan: gagal load settings: %v\n", err)
		s = &settings.Settings{}
	}
	r.settings = s

	// Load custom slash commands
	customCmdDir := filepath.Join(r.opts.ProjectRoot, ".luna", "commands")
	if err := slashcmd.LoadCustomCommands(r.slashReg, customCmdDir); err != nil {
		fmt.Fprintf(r.err, "luna: peringatan: gagal load custom commands: %v\n", err)
	}

	// Load subagents
	subagentsDir := filepath.Join(r.opts.ProjectRoot, ".luna", "agents")
	if r.loader != nil {
		if err := r.loader.LoadDefinitions(subagentsDir); err != nil {
			fmt.Fprintf(r.err, "luna: peringatan: gagal load subagents: %v\n", err)
		}
	}

	// Load MCP Config and start servers
	mcpCfg, err := config.LoadMCPConfig("")
	if err == nil && len(mcpCfg.MCPServers) > 0 {
		mgr := mcp.NewManager()
		if err := mgr.StartAndDiscover(ctx, mcpCfg); err != nil {
			fmt.Fprintf(r.err, "luna: peringatan: gagal start MCP manager: %v\n", err)
		} else {
			if r.dispatcher != nil {
				tools.RegisterMCPTools(r.dispatcher, mgr)
			}
		}
		defer mgr.Close()
	}

	// Configure subagent spawning
	r.dispatcher.ConfigureDelegateTask(func(ctx context.Context, role string, goal string) (string, error) {
		deps := subagent.Deps{
			Limits:          r.limits,
			ProviderOrder:   nil, // Handled internally or from settings if we implement it. For now, nil uses default LLM logic
			Breaker:         nil, // Handled internally
			Dispatcher:      r.dispatcher,
			Loader:          r.loader,
			ParentAgentCtx:  r.agentCtx,
			Config:          r.permConfig,
			Tracker:         r.tracker,
			Ask:             r.terminalAsk,
			Cwd:             r.opts.ProjectRoot,
			ParentSessionID: r.sessionID,
			Log:             r.logAgent,
			Complete:        r.opts.Complete,
		}
		res, err := subagent.SpawnSubagent(ctx, deps, subagent.Role(role), goal)
		if err != nil {
			return "", err
		}
		if res.Status == subagent.StatusSuccess {
			return fmt.Sprintf("Status: %s\nSummary: %s\nFindings/Changes: %s", res.Status, res.Summary, res.Findings+res.Changes), nil
		}
		return fmt.Sprintf("Status: %s\nError: %s\nSummary: %s", res.Status, res.Error, res.Summary), nil
	})

	// Apply settings env vars
	for k, v := range s.Env {
		if os.Getenv(k) == "" {
			os.Setenv(k, v)
		}
	}

	// Determine permission mode
	mode := r.opts.PermissionMode
	if mode == "" {
		mode = s.DefaultMode
	}
	if mode == "" {
		mode = settings.ModeDefault
	}
	if r.opts.DangerouslySkipPermissions {
		fmt.Fprintln(r.err, "⚠️  luna: --dangerously-skip-permissions aktif. SEMUA tool diizinkan tanpa konfirmasi.")
		mode = settings.ModeBypass
	}

	// Build permission context
	yolo := mode == settings.ModeBypass
	r.agentCtx = permission.NewAgentContext(r.sessionID, r.opts.ProjectRoot, yolo, permission.RolePrimary)

	cfg := permission.LoadPermConfig()
	switch mode {
	case settings.ModeAcceptEdits:
		cfg.WriteMode = "yolo"
	case settings.ModePlan:
		cfg.WriteMode = "block"
		cfg.ProcessMode = "block"
		cfg.ShellMode = "block"
	}
	r.permConfig = cfg

	// Load memory files
	userConfigDir := config.DefaultConfigDir()
	memoryContent, memWarnings, _ := memory.LoadMemoryFiles(r.opts.ProjectRoot, userConfigDir)
	for _, w := range memWarnings {
		fmt.Fprintf(r.err, "luna: %s\n", w)
	}

	// Build system prompt
	sysPrompt := r.buildSystemPrompt(memoryContent)

	if r.opts.ResumeSessionID != "" {
		store := agent.NewStore(r.paths.AgentCheckpointDir)
		cp, err := store.Load(r.opts.ResumeSessionID)
		if err != nil {
			return fmt.Errorf("luna: gagal resume session %s: %v", r.opts.ResumeSessionID, err)
		}
		r.sessionID = r.opts.ResumeSessionID // Override generated sessionID
		r.messages = cp.Messages

		if r.opts.OutputFormat != "json" {
			fmt.Fprintf(r.err, "Resumed session %s\n", r.sessionID)
		}
	} else {
		r.messages = []llmclient.Message{{Role: "system", Content: sysPrompt}}
	}

	// Print mode header
	if !r.opts.PrintMode && r.opts.OutputFormat != "json" {
		r.printHeader(mode)
	}

	// Handle initial prompt
	if r.opts.InitialPrompt != "" {
		if err := r.runTurn(ctx, r.opts.InitialPrompt); err != nil {
			return err
		}
		if r.opts.PrintMode {
			return nil // single turn complete
		}
	} else if r.opts.PrintMode {
		return fmt.Errorf("luna: -p/--print memerlukan prompt (argument atau stdin)")
	}

	// Interactive loop
	return r.interactiveLoop(ctx)
}

// interactiveLoop reads user input and processes each line.
func (r *REPL) interactiveLoop(ctx context.Context) error {
	var (
		cancelTurn context.CancelFunc
		mu         sync.Mutex
	)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	var lastInterrupt time.Time
	go func() {
		for range sigCh {
			if time.Since(lastInterrupt) < 2*time.Second {
				fmt.Println()
				pterm.Warning.Println("Keluar (interrupt ganda)")
				os.Exit(130)
			}
			lastInterrupt = time.Now()
			
			mu.Lock()
			if cancelTurn != nil {
				cancelTurn()
			}
			mu.Unlock()
		}
	}()

	var rl *readline.Instance
	if r.in == os.Stdin || r.in == nil {
		historyFile := filepath.Join(config.DefaultConfigDir(), "history")
		var err error
		rl, err = newLineReader(historyFile)
		if err != nil {
			fmt.Fprintf(r.err, "luna: peringatan: gagal inisialisasi readline: %v\n", err)
		} else {
			defer rl.Close()
		}
	}

	var scanner *bufio.Scanner
	if rl == nil {
		scanner = bufio.NewScanner(r.in)
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		r.printStatusLine()
		var input string
		
		if rl != nil {
			line, err := rl.Readline()
			if err == readline.ErrInterrupt {
				select {
				case sigCh <- os.Interrupt:
				default:
				}
				continue
			} else if err == io.EOF {
				return nil
			} else if err != nil {
				fmt.Fprintln(r.err, err)
				return err
			}
			input = line
		} else {
			fmt.Fprint(r.out, "\n> ")
			if !scanner.Scan() {
				fmt.Fprintln(r.out)
				return nil // EOF
			}
			input = scanner.Text()
		}

		input = strings.TrimSpace(input)

		// Multi-line support: if line ends with '\', read more
		for strings.HasSuffix(input, "\\") {
			input = strings.TrimSuffix(input, "\\")
			if rl != nil {
				rl.SetPrompt("..> ")
				more, err := rl.Readline()
				rl.SetPrompt("\033[36m❯\033[0m ")
				if err != nil {
					break
				}
				input += "\n" + strings.TrimSpace(more)
			} else {
				fmt.Fprint(r.out, "..> ")
				if !scanner.Scan() {
					break
				}
				input += "\n" + strings.TrimSpace(scanner.Text())
			}
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Slash commands
		if strings.HasPrefix(input, "/") {
			if r.handleSlashCommand(ctx, input) {
				return nil // exit requested
			}
			continue
		}

		// Agent turn
		turnCtx, cancel := context.WithCancel(ctx)
		mu.Lock()
		cancelTurn = cancel
		mu.Unlock()

		err := r.runTurn(turnCtx, input)

		mu.Lock()
		cancelTurn = nil
		mu.Unlock()

		if err != nil {
			if turnCtx.Err() == context.Canceled && ctx.Err() == nil {
				fmt.Fprintln(r.err, "\n[Dibatalkan]")
				continue // Turn cancelled by user, but session continues
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			fmt.Fprintf(r.err, "luna: agent error: %v\n", err)
		}
	}
}

// handleSlashCommand processes a /-prefixed command. Returns true if the
// REPL should exit.
func (r *REPL) handleSlashCommand(ctx context.Context, input string) bool {
	r.slashReg.Dispatch(ctx, input, r)
	return r.exitRequested
}

// -- slashcmd.State implementation --

func (r *REPL) Out() io.Writer { return r.out }
func (r *REPL) Err() io.Writer { return r.err }

func (r *REPL) ClearMessages() {
	if len(r.messages) > 0 && r.messages[0].Role == "system" {
		r.messages = r.messages[:1]
	} else {
		r.messages = nil
	}
	r.turnCount = 0
}

func (r *REPL) GetModel() string {
	model := r.opts.Model
	if model == "" && r.settings != nil {
		model = r.settings.Model
	}
	if model == "" {
		model = "(default provider)"
	}
	return model
}

func (r *REPL) SetModel(m string) {
	r.opts.Model = m
}

func (r *REPL) GetSettings() *settings.Settings {
	return r.settings
}

func (r *REPL) GetStats() (int, int, int) {
	in, out := 0, 0
	for _, msg := range r.messages {
		toks := llmclient.EstimateTokens(msg.Content)
		if msg.Role == "assistant" {
			out += toks
		} else {
			in += toks
		}
	}
	r.totalTokensIn = in
	r.totalTokensOut = out
	return r.totalTokensIn, r.totalTokensOut, r.turnCount
}

func (r *REPL) CompactHistory(ctx context.Context, instruction string) {
	r.compactHistory(ctx, instruction)
}

func (r *REPL) PrintPermissions() {
	r.printPermissions()
}

func (r *REPL) PrintStatus() {
	r.printStatusLine()
}

func (r *REPL) RewindHistory(n int) {
	if n <= 0 {
		return
	}

	// Ensure we don't rewind past the system prompt
	if len(r.messages) <= 1 {
		fmt.Fprintln(r.err, "History kosong, tidak ada yang di-rewind.")
		return
	}

	keep := len(r.messages) - n
	if keep < 1 {
		keep = 1 // At least keep system prompt
	}

	r.messages = r.messages[:keep]
	fmt.Fprintf(r.err, "Berhasil rewind %d pesan (sisa: %d).\n", n, keep)
}

func (r *REPL) LoadSession(id string) error {
	store := agent.NewStore(r.paths.AgentCheckpointDir)
	cp, err := store.Load(id)
	if err != nil {
		return err
	}
	r.sessionID = id
	r.messages = cp.Messages
	r.GetStats() // refresh token count
	return nil
}

func (r *REPL) RequestExit() {
	r.exitRequested = true
}

func (r *REPL) InjectPrompt(ctx context.Context, prompt string) {
	err := r.runTurn(ctx, prompt)
	if err != nil {
		fmt.Fprintf(r.err, "luna: agent error: %v\n", err)
	}
}

// runTurn sends one user message through the agent loop and displays the result.
func (r *REPL) runTurn(ctx context.Context, userMsg string) error {
	r.messages = append(r.messages, llmclient.Message{Role: "user", Content: userMsg})
	r.turnCount++

	// Build agent deps for this turn
	deps := agent.Deps{
		Limits:       r.limits,
		SystemPrompt: "", // already in messages[0]
		Dispatcher:   r.dispatcher,
		PermDeps: tools.PermDeps{
			AgentCtx: r.agentCtx,
			Config:   r.permConfig,
			Tracker:  r.tracker,
			Ask:      r.terminalAsk,
			Cwd:      r.opts.ProjectRoot,
		},
		Store:       agent.NewStore(r.paths.AgentCheckpointDir),
		SessionID:   r.sessionID,
		Log:         r.logAgent,
		Complete:    r.opts.Complete,
		HooksRunner: hooks.NewRunner(r.settings.Hooks, r.opts.ProjectRoot, r.logAgent),
	}

	// Run agent loop with current messages as a "resume" checkpoint
	// This allows the agent to make multiple tool calls in one turn
	cp := &agent.Checkpoint{
		SchemaVersion: agent.CurrentCheckpointSchemaVersion,
		SessionID:     r.sessionID,
		Goal:          userMsg,
		Step:          0,
		Messages:      r.messages,
	}

	result, err := agent.RunLoop(ctx, deps, userMsg, cp)
	
	if r.spinner != nil {
		r.spinner.Stop()
		r.spinner = nil
	}

	if err != nil {
		return err
	}

	// Persist updated messages from agent's checkpoint (including tool calls and assistant replies)
	r.messages = cp.Messages

	if r.opts.OutputFormat == "json" {
		r.RunJSON(ctx, result)
	} else {
		// Display the result in text mode
		if result.Thought != "" {
			fmt.Fprintf(r.out, "\n%s\n", result.Thought)
		} else if result.BlockReason != "" {
			fmt.Fprintf(r.out, "\n[blocked: %s]\n", result.BlockReason)
		}
	}

	// Update stats and trigger auto-compact if limits exceeded
	r.GetStats() // computes r.totalTokensIn, r.totalTokensOut
	maxMsgs := r.limits.SessionMaxMsgs
	if maxMsgs == 0 {
		maxMsgs = 50 // Default fallback
	}
	maxToks := r.limits.AgentMaxToks
	if maxToks == 0 {
		maxToks = 50000 // Safe default
	}

	if len(r.messages) > maxMsgs || (r.totalTokensIn+r.totalTokensOut) > maxToks {
		if r.opts.Complete != nil {
			r.compactHistory(ctx, "Auto-compact karena batas sesi atau token LLM hampir tercapai.")
		}
	}

	return nil
}

// terminalAsk provides interactive permission confirmation.
func (r *REPL) terminalAsk(prompt string) (bool, error) {
	fmt.Fprintf(r.err, "%s [y/N] ", prompt)
	reader := bufio.NewReader(r.in)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false, nil
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// logAgent prints agent progress to stderr using animated spinners.
func (r *REPL) logAgent(msg string) {
	if r.opts.OutputFormat == "json" {
		return
	}
	
	msg = strings.TrimSpace(msg)
	
	// Create spinner if it doesn't exist yet for this turn
	if r.spinner == nil {
		r.spinner, _ = pterm.DefaultSpinner.WithRemoveWhenDone(true).Start()
	}

	// Update text depending on step
	if strings.HasPrefix(msg, "[berpikir]") {
		r.spinner.UpdateText("Sedang berpikir...")
	} else if strings.HasPrefix(msg, "step ") {
		r.spinner.UpdateText(msg)
	} else if strings.HasPrefix(msg, "v ") || strings.HasPrefix(msg, "✓ ") || strings.HasPrefix(msg, "x ") || strings.HasPrefix(msg, "✗ ") {
		// Just update the text to show success/fail briefly before next thinking phase
		r.spinner.UpdateText(msg)
	} else if strings.HasPrefix(msg, "[") {
		// Stop spinner entirely to print the warning, then leave it nil
		r.spinner.Warning(msg)
		r.spinner = nil
	} else {
		r.spinner.UpdateText(msg)
	}
}

// buildSystemPrompt assembles the system prompt from components.
func (r *REPL) buildSystemPrompt(memoryContent string) string {
	var parts []string

	parts = append(parts, "Kamu adalah LUNA, asisten pengembangan berbasis AI. "+
		"Kamu punya akses ke berbagai tool untuk membaca, menulis, dan mengedit file, "+
		"menjalankan command, mencari file/isi file, dan lainnya. "+
		"Pilih tool yang tepat untuk menyelesaikan permintaan user. "+
		"Selalu gunakan tool untuk memverifikasi pekerjaan kamu sebelum menyatakan selesai.\n\n"+
		"PENTING: Kamu WAJIB selalu merespons dengan struktur JSON object murni. "+
		"Jangan tambahkan teks pengantar apapun di luar JSON block.\n"+
		"Gunakan format ini untuk memanggil tool:\n"+
		"{\n"+
		"  \"thought\": \"<pemikiran kamu, langkah logis>\",\n"+
		"  \"tool\": \"<nama tool>\",\n"+
		"  \"args\": {\"<kunci>\": \"<nilai>\"}\n"+
		"}\n\n"+
		"Jika tugas sudah selesai, kamu WAJIB memberikan jawaban akhirmu ke user di dalam field `response`:\n"+
		"{\n"+
		"  \"thought\": \"<pemikiran kamu>\",\n"+
		"  \"response\": \"<jawaban lengkap, ringkasan, atau penjelasan akhir untuk user>\",\n"+
		"  \"done\": true\n"+
		"}")

	// Add memory content
	if memoryContent != "" {
		parts = append(parts, "\n\n---\nProject & User Memory:\n\n"+memoryContent)
	}

	// Add tool manifest
	if r.dispatcher != nil {
		parts = append(parts, "\n\n---\nAvailable Tools:\n"+tools.Manifest())
	}

	// Add working directory context
	if r.opts.ProjectRoot != "" {
		parts = append(parts, fmt.Sprintf("\n\nWorking directory: %s", r.opts.ProjectRoot))
	}

	return strings.Join(parts, "")
}

// printHeader displays the REPL startup banner.
func (r *REPL) printHeader(mode settings.PermissionMode) {
	fmt.Fprintln(r.out, "")
	pterm.DefaultHeader.WithFullWidth().WithMargin(1).Println("LUNA (Go) - Agentic AI Dev Assistant")

	model := r.opts.Model
	if model == "" && r.settings != nil {
		model = r.settings.Model
	}
	if model == "" {
		model = "(auto)"
	}
	
	pterm.Info.Printfln("Model: %s | Mode: %s", model, mode)

	if r.opts.ProjectRoot != "" {
		pterm.Info.Printfln("Project: %s", r.opts.ProjectRoot)
	}

	fmt.Fprintln(r.out, "")
	pterm.ThemeDefault.SuccessMessageStyle.Println("Ketik pesan untuk mulai. /help untuk bantuan, /exit untuk keluar.")
	fmt.Fprintln(r.out, "")
}

// printPermissions shows active permission configuration.
func (r *REPL) printPermissions() {
	fmt.Fprintln(r.out, "Permission Configuration:")
	fmt.Fprintf(r.out, "  Write mode:   %s\n", r.permConfig.WriteMode)
	fmt.Fprintf(r.out, "  Shell mode:   %s\n", r.permConfig.ShellMode)
	fmt.Fprintf(r.out, "  Process mode: %s\n", r.permConfig.ProcessMode)

	if r.settings != nil {
		if len(r.settings.Permissions.Allow) > 0 {
			fmt.Fprintf(r.out, "  Allow: %s\n", strings.Join(r.settings.Permissions.Allow, ", "))
		}
		if len(r.settings.Permissions.Deny) > 0 {
			fmt.Fprintf(r.out, "  Deny:  %s\n", strings.Join(r.settings.Permissions.Deny, ", "))
		}
		if len(r.settings.Permissions.Ask) > 0 {
			fmt.Fprintf(r.out, "  Ask:   %s\n", strings.Join(r.settings.Permissions.Ask, ", "))
		}
	}
}

// printStatus shows diagnostic information.
func (r *REPL) printStatus() {
	fmt.Fprintln(r.out, "Luna Status:")
	fmt.Fprintf(r.out, "  Project root: %s\n", r.opts.ProjectRoot)
	fmt.Fprintf(r.out, "  Config dir:   %s\n", config.DefaultConfigDir())

	model := r.opts.Model
	if model == "" && r.settings != nil {
		model = r.settings.Model
	}
	if model == "" {
		model = "(auto)"
	}
	fmt.Fprintf(r.out, "  Model:        %s\n", model)
	fmt.Fprintf(r.out, "  Turns:        %d\n", r.turnCount)

	// Provider status
	active := config.ActiveProviders(config.TaskProviderOrder)
	if len(active) > 0 {
		fmt.Fprintf(r.out, "  Providers:    %s\n", strings.Join(active, ", "))
	} else {
		fmt.Fprintln(r.out, "  Providers:    ⚠️  tidak ada API key terdeteksi")
	}

	// Tools
	if r.dispatcher != nil {
		fmt.Fprintf(r.out, "  Tools:        %d terdaftar\n", len(r.dispatcher.Names()))
	}
}

// compactHistory is a stub for context compaction (full implementation in SESSION-69).
// compactHistory handles context compaction.
func (r *REPL) compactHistory(ctx context.Context, instruction string) {
	if len(r.messages) <= 4 {
		fmt.Fprintln(r.out, "History terlalu pendek untuk di-compact.")
		return
	}

	fmt.Fprintln(r.out, "Melakukan compact history...")

	// Extract messages to summarize (everything except system prompt and the last 2 turns)
	keepRecent := 2 // Keep last 2 messages (usually user/assistant pair)
	if len(r.messages) <= keepRecent+1 {
		return
	}

	startIndex := 0
	if r.messages[0].Role == "system" {
		startIndex = 1
	}

	endIndex := len(r.messages) - keepRecent
	messagesToSummarize := r.messages[startIndex:endIndex]

	var historyText string
	for _, msg := range messagesToSummarize {
		historyText += fmt.Sprintf("[%s]: %s\n\n", msg.Role, msg.Content)
	}

	prompt := "Tolong buat ringkasan padat dari riwayat percakapan berikut. Pertahankan keputusan penting, cuplikan kode krusial, dan konteks task yang sedang berjalan.\n"
	if instruction != "" {
		prompt += "Instruksi tambahan: " + instruction + "\n"
	}
	prompt += "\nHistory:\n" + historyText

	// Call LLM for completion
	deps := agent.Deps{
		Limits:   r.limits,
		Log:      r.logAgent,
		Complete: r.opts.Complete,
	}
	complete := r.opts.Complete
	if complete == nil {
		complete = agent.Complete
	}
	resp, err := complete(ctx, deps, []llmclient.Message{{Role: "user", Content: prompt}})
	if err != nil {
		fmt.Fprintf(r.err, "Gagal melakukan compact history: %v\n", err)
		return
	}
	summary := resp.Content

	// Rebuild messages
	newMessages := make([]llmclient.Message, 0, keepRecent+2)
	if startIndex == 1 {
		newMessages = append(newMessages, r.messages[0])
	}
	newMessages = append(newMessages, llmclient.Message{
		Role:    "user",
		Content: "[Context compact] History sebelumnya telah diringkas sebagai berikut:\n" + summary,
	})
	newMessages = append(newMessages, r.messages[endIndex:]...)
	r.messages = newMessages
	fmt.Fprintf(r.out, "History berhasil di-compact. Tersisa %d pesan.\n", len(r.messages))
}

// RunJSON outputs the result as JSON (for -p --output-format json).
func (r *REPL) RunJSON(ctx context.Context, result agent.FinalResult) error {
	output := map[string]any{
		"thought":       result.Thought,
		"done":          result.Done,
		"block_reason":  result.BlockReason,
		"commands_run":  result.CommandsRun,
		"touched_files": result.TouchedFiles,
		"changed_files": result.ChangedFiles,
	}
	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return err
	}
	fmt.Fprintln(r.out, string(data))
	return nil
}
func (r *REPL) printStatusLine() {
	if r.opts.PrintMode || r.opts.OutputFormat == "json" {
		return
	}

	r.GetStats()
	totalToks := r.totalTokensIn + r.totalTokensOut
	toksStr := fmt.Sprintf("%d", totalToks)
	if totalToks >= 1000 {
		toksStr = fmt.Sprintf("%.1fk", float64(totalToks)/1000.0)
	}

	modelName := "default"
	if r.settings != nil && r.settings.Model != "" {
		modelName = r.settings.Model
	}

	modeStr := string(r.opts.PermissionMode)
	if modeStr == "" {
		modeStr = "plan" // default
	}

	fmt.Fprintf(r.out, "\n[Mode: %s] [Model: %s] [Tokens: %s]", modeStr, modelName, toksStr)
}
