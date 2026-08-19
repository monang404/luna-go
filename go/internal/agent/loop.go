// Ported from 30-luna/50-agent/42-execution/*.zsh (00-loop_main,
// 05-get_plan, 10-reject_checks, 15-run_tool, 20-log_and_notify,
// 25-track_and_continue) plus the provider/model fallback loop in
// 30-luna/10-core/50-request_blocking.zsh and its agent-facing wrapper
// 30-luna/50-agent/41-provider.zsh (_ai_agent_provider_request).
//
// The zsh source splits the loop body into six files that communicate
// through dynamically-scoped shell locals declared once in
// _ai_agent_execute_loop. Go has no equivalent to that scoping trick, so
// this port collapses those locals into one *runState value threaded
// explicitly through unexported methods (getPlan, rejectDoneChecks,
// runTool, trackAndContinue) that mirror the six files 1:1 in spirit,
// not the zsh multi-file-plus-dynamic-scope mechanism itself.
//
// SESSION-50 scope only (see docs/execution_sessions/50_port_agent_react_loop.yaml):
// subagent spawning (SESSION-51) and any step-by-step terminal rendering
// (SESSION-53) are deliberately not here. Deps.Log is the one hook this
// file offers a future renderer -- a plain string sink, not a UI.
//
// Known intentional deviations from the zsh source (documented here per
// FASE 16/17 rather than scattered across comments):
//
//   - No syntax-check-before-done gate (zsh's _ai_verify_touched_files /
//     py_compile-or-equivalent dispatcher). No Go port of that checker
//     exists yet anywhere in this repository (grep confirms no
//     internal/ package for it), and this session's own FASE 7 forbids
//     inventing a new verification system when "existing tooling"
//     doesn't already cover it. Only the "must have run at least one
//     tool before claiming done" reject-check is enforced here.
//   - No project-index invalidation after write_file/edit_file/
//     delete_file/move_file (zsh's 46-index.zsh integration). No Go
//     index package exists yet; nothing to invalidate.
//   - No dependency auto-install-and-retry on exit 127 (zsh's
//     02-tool_autodep.zsh hook). No Go port of that module exists yet.
//   - No desktop/system notification (zsh's _ai_notify_progress,
//     rate-limited). Presentation concern, deferred with the rest of
//     rendering to SESSION-53.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
	"github.com/monang404/luna-go/internal/tools"
)

// errFatalTransition marks the RC-012 policy tier-1 fatal path (see
// 39-agent-state-machine.zsh's own header comment for the full policy):
// a state transition that should always be legal per the loop's own
// control flow failed anyway (e.g. AgentState.Validate() started
// failing mid-run). RunLoop returns this error to the caller instead of
// silently swallowing it, exactly like _ai_agent_execute_loop's bare
// `return 1` at each of its three RC-012 tier-1 sites.
var errFatalTransition = errors.New("agent: fatal lifecycle transition failure")

// Deps bundles every external dependency RunLoop needs to drive one
// ReAct run. It is the Go equivalent of _ai_agent_execute_loop's
// positional parameters (state_dir, msgfile, checkpoint_file,
// run_slug, runs_logfile, max_step) plus the provider/tool/permission
// wiring that in zsh comes from already-sourced global config
// (AI_TASK_PROVIDER_ORDER_AGENT, AI_PROVIDERS, AI_MODELS, the tool
// dispatcher, the permission layer).
type Deps struct {
	// Limits supplies AgentMaxSteps, AgentMaxSameFail, AgentMaxToks,
	// SessionMaxMsgs, MaxRetries, and the other resilience-layer
	// thresholds (config.LoadLimits()).
	Limits config.Limits

	// ProviderOrder is the provider fallback order to try each PLAN
	// step, matching AI_TASK_PROVIDER_ORDER_AGENT. Defaults to
	// config.TaskProviderOrderAgent when nil.
	ProviderOrder []string

	// Breaker gates repeated attempts at a just-failed provider/model
	// pair (SESSION-46's circuit breaker). May be nil to skip breaker
	// gating entirely.
	Breaker *llmclient.BreakerStore

	// SystemPrompt seeds messages[0] as the "system" role on a fresh
	// (non-resumed) run. "" means no system message is added -- system
	// prompt *construction* (project context, tool schema, Termux
	// notes, ...) is a separate concern this session does not own; the
	// caller builds the final string and hands it in already-composed.
	SystemPrompt string

	// Dispatcher is the tool registry/dispatch pipeline (SESSION-43,
	// tools populated by SESSION-47/48) that RunTool calls through.
	Dispatcher *tools.Dispatcher
	// PermDeps is forwarded to every Dispatcher.Dispatch call
	// unchanged -- the same AgentContext/config/tracker/ask/cwd for
	// the whole run.
	PermDeps tools.PermDeps

	// Store persists a Checkpoint after every step that mutates
	// message history (mirrors _ai_agent_checkpoint_save's four call
	// sites). May be nil to disable checkpointing entirely (useful for
	// unit tests with fake dependencies).
	Store *Store
	// SessionID names the checkpoint file under Store.Dir. Required
	// whenever Store is non-nil.
	SessionID string

	// Log receives human-readable progress lines (step start, tool
	// result, retry, block/complete reasons) as plain strings, one per
	// call -- the "log/print sederhana" surface this session's own
	// why_not_more note asks for, deliberately not a rendering API. May
	// be nil to run silently.
	Log func(string)

	// Complete is the completion primitive getPlan calls once per PLAN
	// step. nil (every production caller's zero value) means "use
	// defaultComplete", the real provider/model fallback loop ported
	// from 50-request_blocking.zsh (see that function below). Tests
	// substitute a fake here -- the seam FASE 11's fake-dependency unit
	// tests need -- so the loop's control flow (plan -> guard ->
	// execute -> checkpoint -> repeat) can be driven deterministically
	// without any network access or configured API key.
	Complete func(ctx context.Context, deps Deps, messages []llmclient.Message) (llmclient.Response, error)
}

func (d Deps) log(format string, args ...any) {
	if d.Log != nil {
		d.Log(fmt.Sprintf(format, args...))
	}
}

// runState is the Go stand-in for _ai_agent_execute_loop's block of
// loop-scoped locals (step, reply, thought, tool, args, done_flag,
// output, commands_run, touched_files, changed_files,
// last_failed_tool/args, same_fail_count, block_reason), threaded
// explicitly through getPlan/rejectDoneChecks/runTool/trackAndContinue
// instead of zsh's dynamic scoping.
type runState struct {
	Deps

	state    *AgentState
	messages []llmclient.Message

	lastReply string
	tool      string
	args      json.RawMessage
	done      bool

	output   string
	toolErr  error
	filepath string

	commandsRun  int
	touchedFiles map[string]bool
	changedFiles map[string]bool

	lastFailedTool string
	lastFailedArgs string
	sameFailCount  int

	blockReason string
	cancelled   bool
}

// RunLoop drives the bounded ReAct execution loop for goal to
// completion, blockage, or context cancellation -- the Go port of
// _ai_agent_execute_loop. Pass resume (from Store.Load) to continue an
// interrupted run instead of starting fresh; resume's Goal/Step/Messages
// seed the new run's state exactly like 40-runtime/10-load_checkpoint.zsh
// feeding step_offset/msgfile back into the loop.
//
// The returned error is non-nil only on the RC-012 tier-1 fatal path
// (errFatalTransition, wrapped) -- every other outcome (COMPLETE,
// BLOCKED for any normal reason, context cancellation) is reported
// through the returned FinalResult with a nil error, matching
// _ai_agent_execute_loop always `return 0`ing on its normal exit paths
// and reserving `return 1` for the three fatal transition sites.
func RunLoop(ctx context.Context, deps Deps, goal string, resume *Checkpoint) (FinalResult, error) {
	rs := &runState{
		Deps:         deps,
		touchedFiles: map[string]bool{},
		changedFiles: map[string]bool{},
	}

	if resume != nil {
		if err := resume.Validate(); err != nil {
			return FinalResult{}, fmt.Errorf("agent: cannot resume: %w", err)
		}
		rs.state = NewState(resume.Goal)
		rs.state.Step = resume.Step
		rs.messages = append([]llmclient.Message(nil), resume.Messages...)
	} else {
		rs.state = NewState(goal)
		if deps.SystemPrompt != "" {
			rs.messages = []llmclient.Message{{Role: "system", Content: deps.SystemPrompt}}
		}
	}
	if err := rs.state.Validate(); err != nil {
		return FinalResult{}, fmt.Errorf("agent: invalid initial state: %w", err)
	}

	maxSteps := deps.Limits.AgentMaxSteps
	if maxSteps <= 0 {
		maxSteps = 15
	}

	for rs.state.Step < maxSteps {
		select {
		case <-ctx.Done():
			rs.blockReason = fmt.Sprintf("Agent dibatalkan (step %d)", rs.state.Step)
			rs.transitionOrWarn(PhaseBlocked)
			return rs.finish(), nil
		default:
		}

		brk, err := rs.getPlan(ctx)
		if err != nil {
			return rs.fatal(err)
		}
		if brk {
			break
		}

		reject, err := rs.rejectDoneChecks()
		if err != nil {
			return rs.fatal(err)
		}
		if reject {
			continue
		}

		if rs.done || rs.tool == "" {
			if !rs.done {
				rs.blockReason = fmt.Sprintf("Agent berhenti tanpa tool berikutnya dan tanpa declare selesai (step %d)", rs.state.Step)
				rs.transitionOrWarn(PhaseBlocked)
			}
			break
		}

		if terr := Transition(rs.state, PhaseExecute); terr != nil {
			rs.blockReason = fmt.Sprintf("State transition gagal di langkah EXECUTE (step %d)", rs.state.Step)
			return rs.fatal(errFatalTransition)
		}

		rs.log("step %d: %s(%s)", rs.state.Step, rs.tool, string(rs.args))
		rs.runTool(ctx)
		if rs.cancelled {
			rs.transitionOrWarn(PhaseBlocked)
			break
		}
		if rs.toolErr != nil {
			rs.log("  x %s -- %v", rs.tool, rs.toolErr)
		} else {
			rs.log("  v %s", rs.tool)
		}

		if giveUp := rs.trackAndContinue(); giveUp {
			rs.transitionOrWarn(PhaseBlocked)
			break
		}
	}

	if rs.state.Step >= maxSteps && rs.state.Phase != PhaseBlocked && rs.state.Phase != PhaseComplete {
		rs.log("[berhenti: sudah %d langkah, agent gak declare selesai]", maxSteps)
		if !rs.done {
			rs.blockReason = fmt.Sprintf("Sudah %d langkah, agent belum declare selesai (step %d)", maxSteps, rs.state.Step)
		}
		rs.transitionOrWarn(PhaseBlocked)
	}

	// Terminal lifecycle is authoritative, exactly like
	// 00-loop_main.zsh's trailing if/elif: a verified done:true (still
	// sitting in VERIFY, meaning nothing after it forced BLOCKED)
	// reaches COMPLETE; every other exit path becomes BLOCKED.
	if rs.done && rs.state.Phase == PhaseVerify {
		rs.transitionOrWarn(PhaseComplete)
	} else if rs.state.Phase != PhaseBlocked {
		rs.transitionOrWarn(PhaseBlocked)
	}

	result := rs.finish()

	// Mirrors 44-finalize.zsh: a checkpoint only exists to resume an
	// unfinished run, so delete it once the lifecycle verifiably
	// reaches COMPLETE. Best-effort, like the zsh `rm -f`.
	if rs.state.Phase == PhaseComplete && rs.Store != nil && rs.SessionID != "" {
		_ = rs.Store.Delete(rs.SessionID)
	}

	return result, nil
}

// fatal persists whatever step/done/block_reason evidence exists so far
// (mirrors the three RC-012 tier-1 sites in 00-loop_main.zsh, each of
// which writes $state_dir/{step,done,block_reason} before `return 1`)
// and returns the terminal FinalResult alongside err.
func (rs *runState) fatal(err error) (FinalResult, error) {
	rs.log("[peringatan: %s]", rs.blockReason)
	rs.transitionOrWarn(PhaseBlocked)
	return rs.finish(), err
}

// finish copies the loop's accumulated evidence into rs.state and
// returns Finalize(rs.state) -- the single place that mapping happens,
// used by every RunLoop exit path.
func (rs *runState) finish() FinalResult {
	rs.state.Done = rs.done
	rs.state.BlockReason = rs.blockReason
	rs.state.CommandsRun = rs.commandsRun
	rs.state.TouchedFiles = sortedKeys(rs.touchedFiles)
	rs.state.ChangedFiles = sortedKeys(rs.changedFiles)
	return Finalize(rs.state)
}

// transitionOrWarn ports _ai_agent_state_transition_or_warn: a
// best-effort transition attempt that logs (never panics/errors to the
// caller) if the target is not actually reachable from the current
// phase -- used at every non-fatal transition site, mirroring which
// zsh call sites use `_or_warn` versus the plain fatal form.
func (rs *runState) transitionOrWarn(next Phase) {
	if err := Transition(rs.state, next); err != nil {
		rs.log("[peringatan: gagal transisi lifecycle state ke %s -- lifecycle_state mungkin tidak sinkron dengan block_reason/done]", next)
	}
}

// saveCheckpoint ports the checkpoint_save call sites shared by
// get_plan/reject_checks/track_and_continue: never fatal, always
// visible (via Log) on failure.
func (rs *runState) saveCheckpoint() {
	if rs.Store == nil || rs.SessionID == "" {
		return
	}
	cp := NewCheckpoint(rs.SessionID, rs.state.Goal, rs.state.Step, rs.messages)
	if err := rs.Store.Save(cp); err != nil {
		rs.log("[peringatan: checkpoint gagal disimpan step %d]", rs.state.Step)
	}
}

// ---------------------------------------------------------------------
// getPlan -- ports 42-execution/05-get_plan.zsh
// ---------------------------------------------------------------------

// getPlan asks the provider for the next plan and parses it into
// rs.tool/rs.args/rs.done/rs.lastReply/... Return values mirror
// 05-get_plan.zsh's own return-code contract: brk==true means the
// caller must `break` the outer loop (provider/model exhausted, or an
// unparseable reply); a non-nil err is the RC-012 tier-1 fatal path
// (state transition itself failed).
func (rs *runState) getPlan(ctx context.Context) (brk bool, err error) {
	if rs.state.Phase != PhasePlan {
		// Re-entering PLAN from PLAN would not be a valid matrix entry
		// (self-transitions are never listed); only attempt the
		// transition when actually coming from EXECUTE/VERIFY.
		if terr := Transition(rs.state, PhasePlan); terr != nil {
			rs.blockReason = fmt.Sprintf("State transition gagal di langkah PLAN (step %d)", rs.state.Step)
			return false, errFatalTransition
		}
	}
	rs.state.Step++
	llmclient.Debugf("agent step=%d phase=%s", rs.state.Step, rs.state.Phase)

	resp, reqErr := rs.requestCompletion(ctx)
	if errors.Is(reqErr, llmclient.ErrCancelled) {
		rs.blockReason = fmt.Sprintf("Agent dibatalkan (step %d)", rs.state.Step)
		rs.cancelled = true
		return true, nil
	}
	if reqErr != nil || resp.Content == "" {
		detail := resp.ErrorMessage
		if reqErr != nil {
			detail = reqErr.Error()
		}
		if detail == "" {
			detail = "tidak ada detail tambahan dari provider"
		}
		rs.log("[step %d] Gagal minta respons dari LUNA, berhenti. Detail: %s", rs.state.Step, detail)
		rs.saveCheckpoint()
		rs.blockReason = fmt.Sprintf("LLM/provider request gagal (cek API key atau jalankan 'luna deps'). Detail: %s", truncateStr(detail, 200))
		return true, nil
	}

	plan := ParsePlan(resp.Content)
	rs.lastReply = resp.Content
	rs.state.Thought = plan.Thought
	rs.tool = plan.Tool
	rs.args = plan.Args
	rs.done = plan.Done
	if plan.Compat != "" {
		rs.log("  (compat) %s", plan.Compat)
	}

	if plan.Empty() {
		rs.log("[error: agent balas format JSON gak valid, berhenti. Raw: %s]", resp.Content)
		rs.blockReason = fmt.Sprintf("LUNA balas format JSON gak valid (step %d)", rs.state.Step)
		return true, nil
	}
	return false, nil
}

// requestCompletion calls Deps.Complete (or defaultComplete when the
// caller left it nil) with the loop's current message history.
func (rs *runState) requestCompletion(ctx context.Context) (llmclient.Response, error) {
	complete := rs.Complete
	if complete == nil {
		complete = defaultComplete
	}
	return complete(ctx, rs.Deps, rs.messages)
}

// defaultComplete ports 30-luna/10-core/50-request_blocking.zsh's
// provider/model fallback loop (`for provider { for model { ... } }`),
// using llmclient.SelectProviderCandidate for the provider-selection
// fragment and llmclient.CallWithRetry for the per-model retry
// primitive -- exactly the composition those two functions' own doc
// comments say they exist to support "once the agent loop lands".
// task_class is always "smart" (config.TaskSmart) and mode is always
// "json", matching _ai_agent_provider_request's own hardcoded call
// (`_ai_agent_provider_request "$msgfile" "json" smart ...`).
func defaultComplete(ctx context.Context, deps Deps, messages []llmclient.Message) (llmclient.Response, error) {
	order := deps.ProviderOrder
	if len(order) == 0 {
		order = config.TaskProviderOrderAgent
	}
	previousFailures := map[string]bool{}
	var lastResp llmclient.Response
	for {
		cand, err := llmclient.SelectProviderCandidate(order, previousFailures)
		if err != nil {
			return lastResp, llmclient.ExhaustionError(order, previousFailures, lastResp)
		}

		models := config.ModelsFor(cand.Name, config.TaskSmart)
		if len(models) == 0 {
			models = []string{cand.Provider.Model}
		}

		for idx, model := range models {
			maxTokens := llmclient.ResolveMaxTokens(idx+1, deps.Limits.AgentMaxToks)
			reasoningEffort := ""
			if llmclient.IsReasoningModel(cand.Name, model) {
				reasoningEffort = llmclient.ReasoningEffortFor(cand.Name)
			}
			temp := llmclient.TemperatureForMode("json")
			breakerKey := cand.Name + "/" + model

			buildPayload := func(mt int) ([]byte, error) {
				return llmclient.BuildPayload(messages, llmclient.PayloadOptions{
					Model:           model,
					MaxTokens:       mt,
					Temperature:     temp,
					ReasoningEffort: reasoningEffort,
					Stream:          false,
				})
			}

			modelCandidate := llmclient.Candidate{Name: cand.Name, Provider: cand.Provider}
			modelCandidate.Provider.Model = model

			resp, callErr := llmclient.CallWithRetry(ctx, modelCandidate, breakerKey, deps.Breaker, deps.Limits, maxTokens, buildPayload)
			llmclient.Debugf("provider=%s model=%s http_status=%d content_len=%d err=%v", cand.Name, model, resp.HTTPStatus, len(resp.Content), callErr)
			if callErr != nil {
				if errors.Is(callErr, llmclient.ErrCancelled) {
					return resp, callErr
				}
				lastResp = resp
				continue
			}
			lastResp = resp
			if resp.Content != "" {
				return resp, nil
			}
		}

		previousFailures[cand.Name] = true
	}
}

// ---------------------------------------------------------------------
// rejectDoneChecks -- ports 42-execution/10-reject_checks.zsh
// ---------------------------------------------------------------------

// rejectDoneChecks evaluates a done:true claim before it is allowed to
// end the loop. reject==true means the caller must `continue` the outer
// loop (claim rejected, a corrective user-turn was appended to
// rs.messages); a non-nil err is the RC-012 tier-1 fatal path.
func (rs *runState) rejectDoneChecks() (reject bool, err error) {
	if rs.done {
		if terr := Transition(rs.state, PhaseVerify); terr != nil {
			rs.blockReason = fmt.Sprintf("State transition gagal di langkah VERIFY (step %d)", rs.state.Step)
			return false, errFatalTransition
		}
	}

	if rs.done && rs.commandsRun == 0 {
		rs.log("  [ditolak: agent klaim selesai tapi belum pernah memanggil tool apa pun di sesi ini]")
		rs.messages = append(rs.messages,
			llmclient.Message{Role: "assistant", Content: rs.lastReply},
			llmclient.Message{Role: "user", Content: "Kamu klaim goal ini sudah selesai, tapi belum memanggil satu tool pun di sesi ini. Klaim tanpa verifikasi TIDAK DITERIMA. Jalankan tool nyata yang membuktikan goal ini tercapai (baca file terkait, jalankan test, dsb) sebelum declare done:true lagi."},
		)
		rs.saveCheckpoint()
		rs.done = false
		return true, nil
	}

	// NOTE: no syntax-verify-before-done gate here -- see package doc
	// comment for why (no Go port of the checker dispatcher exists).

	return false, nil
}

// ---------------------------------------------------------------------
// runTool -- ports 42-execution/15-run_tool.zsh
// ---------------------------------------------------------------------

// runTool dispatches rs.tool/rs.args through Deps.Dispatcher, records
// the result, and tracks touched/changed files. It never returns an
// error itself -- a dispatch failure is recorded into rs.toolErr for
// trackAndContinue to act on, exactly like the zsh source capturing
// $exit_status rather than aborting the loop on a failed tool call.
func (rs *runState) runTool(ctx context.Context) {
	result, err := rs.Dispatcher.Dispatch(ctx, rs.PermDeps, rs.tool, rs.args)

	if ctx.Err() != nil {
		rs.blockReason = fmt.Sprintf("Agent dibatalkan oleh context setelah tool '%s' (step %d)", rs.tool, rs.state.Step)
		rs.cancelled = true
		return
	}

	rs.commandsRun++
	rs.toolErr = err
	if err != nil {
		rs.output = truncateStr(err.Error(), 3000)
	} else {
		rs.output = truncateStr(result.Output, 3000)
	}

	rs.filepath = extractDestOrPath(rs.args)
	if rs.toolErr == nil && rs.filepath != "" {
		rs.touchedFiles[rs.filepath] = true
		if rs.tool == "write_file" || rs.tool == "edit_file" {
			rs.changedFiles[rs.filepath] = true
		}
	}
}

// ---------------------------------------------------------------------
// trackAndContinue -- ports 42-execution/25-track_and_continue.zsh
// (log_and_notify's non-UI half -- the jsonl run log and desktop
// notification are presentation concerns out of scope here -- is
// folded in here rather than kept as a separate no-op file)
// ---------------------------------------------------------------------

// trackAndContinue updates the same-failure counter, appends the tool
// turn to message history, trims/checkpoints, and reports whether the
// loop must give up (giveUp==true: the same (tool, args) pair has now
// failed AgentMaxSameFail times in a row with no progress).
func (rs *runState) trackAndContinue() (giveUp bool) {
	if rs.toolErr != nil {
		if rs.tool == rs.lastFailedTool && string(rs.args) == rs.lastFailedArgs {
			rs.sameFailCount++
		} else {
			rs.sameFailCount = 1
			rs.lastFailedTool = rs.tool
			rs.lastFailedArgs = string(rs.args)
		}
		maxSameFail := rs.Limits.AgentMaxSameFail
		if maxSameFail <= 0 {
			maxSameFail = 3
		}
		if rs.sameFailCount >= maxSameFail {
			rs.log("[berhenti: panggilan tool yang sama gagal %d kali berturut-turut, gak ada progress. Cek manual.]", rs.sameFailCount)
			rs.blockReason = fmt.Sprintf("Tool '%s' gagal %d kali berturut-turut (step %d)", rs.tool, rs.sameFailCount, rs.state.Step)
			return true
		}
	} else {
		rs.sameFailCount = 0
		rs.lastFailedTool = ""
		rs.lastFailedArgs = ""
	}

	rs.messages = append(rs.messages,
		llmclient.Message{Role: "assistant", Content: rs.lastReply},
		llmclient.Message{Role: "user", Content: "Output:\n" + rs.output},
	)
	sessionMax := rs.Limits.SessionMaxMsgs
	if sessionMax > 0 {
		rs.messages = llmclient.TrimSession(rs.messages, sessionMax)
	}
	rs.saveCheckpoint()
	return false
}

// ---------------------------------------------------------------------
// small helpers
// ---------------------------------------------------------------------

// truncateStr caps s at n bytes, matching _ai_head_c's byte-oriented
// truncation (used for tool output capped at 3000 chars, and the
// block_reason detail snippet capped at 200).
func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// extractDestOrPath mirrors 15-run_tool.zsh's
// `jq -r '.dest // .path // empty'`: prefer "dest" (move_file's
// destination -- the source path is gone after a successful move, so
// tracking "dest" is what keeps the *resulting* file's syntax
// verifiable), fall back to "path" for every other tool.
func extractDestOrPath(args json.RawMessage) string {
	var obj map[string]any
	if err := json.Unmarshal(args, &obj); err != nil {
		return ""
	}
	if v, ok := obj["dest"].(string); ok && v != "" {
		return v
	}
	if v, ok := obj["path"].(string); ok && v != "" {
		return v
	}
	return ""
}

// sortedKeys returns m's keys sorted, so AgentState.TouchedFiles/
// ChangedFiles (and thus FinalResult) are deterministic across runs --
// the zsh source's own iteration order over an associative array has no
// defined ordering either, so imposing one here is strictly an
// improvement, not a behavior change worth flagging as a deviation.
func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
