# Migration Traceability Matrix — zsh_bagas → Go

Tracks, per Go migration session (`SESSION-40` onward, see
`docs/execution_sessions/`), which zsh source file(s) map to which Go
package/file. Created in **SESSION-40**; updated by every subsequent
porting session (`06 — documentation` subtask in each session's checklist).

Unlike `docs/TRACEABILITY_MATRIX.md` (backlog-item → UX/bugfix session, for
`SESSION-01..39`), this file tracks the separate Go-rewrite effort
(`SESSION-40..56`) at the file/package level, per
`RENCANA_MIGRASI_GO_RUST.md`.

## Status legend

- `SKELETON` — package directory exists with a `doc.go` placeholder only, no logic ported.
- `IN PROGRESS` — porting started, not all acceptance criteria met yet.
- `PORTED` — acceptance criteria for the owning session met; zsh source retired from the active path.

## Session → package status

| Session | zsh source | Go package | Status | Notes |
|---|---|---|---|---|
| SESSION-40 | *(none — new infra)* | `cmd/zshbagas`, all `internal/*` dirs | SKELETON | `go.mod`, `Makefile`, CI, placeholder binary + per-package `doc.go`. See CHANGELOG SESSION-40 entry. |
| SESSION-41 | `30-ai/00-config/` (9 files, see below) | `internal/config` | PORTED | All 9 files ported, 16 unit tests, AC-01..04 verified. `internal/env` still SKELETON (secrets-file permission/env loading it would own overlapped enough with this session's `secrets.go` that the minimal loader was done here instead — `internal/env`'s own scope, if any remains, is TBD at SESSION-42+). See CHANGELOG SESSION-41 entry. |
| SESSION-42 | `30-ai/06-permissions/` | `internal/permission` | PORTED | All 7 files ported, 12 unit tests, AC-01..04 verified. `CheckPermission` takes level/capability/path directly (no tool-name dependency on the not-yet-ported `internal/tools`, SESSION-43). Real terminal ask UI and `.aiagent/permissions.zsh` project-config override deliberately not ported (see CHANGELOG SESSION-42 entry). |
| SESSION-43 | `30-ai/05-tools/00-tool_registry.zsh`, `02-tool_args_extract.zsh`, `02-tool_autodep.zsh`, `05-tool_dispatch.zsh` | `internal/tools` | PORTED | Registry (18 tools — brief said 17, source has 18, see CHANGELOG), args normalization, schema validation, generic `Dispatcher`, 24 unit tests, AC-01..04 verified. No concrete tool `Execute` implementations yet (SESSION-47/48); autodep's install-triggering half deliberately not ported (see CHANGELOG SESSION-43 entry). |
| SESSION-44 | `30-ai/10-core/48-http_call_blocking.zsh`, `43-payload_builder.zsh`, `41-provider_candidate.zsh` | `internal/llmclient` | PORTED | Blocking (non-streaming) request path: `BuildPayload`, `HasFallback`+`SelectProviderCandidate`, `CallBlocking`+`ParseResponse`, 26 unit tests, AC-01/03/04 verified by UNIT test; AC-02 (live-provider round trip) verified via an `httptest`-based round trip instead of a real provider call (no network egress to LLM providers or dev API key in this environment — see CHANGELOG SESSION-44 entry). SSE streaming (SESSION-45) and circuit breaker/retry/token budget/session trim (SESSION-46) not yet in this package. |
| SESSION-45 | `30-ai/10-core/55-request_streaming.zsh`, `56-sse_line_parser.zsh` | `internal/llmclient` | PORTED | Streaming request path: `parseSSELine`, `Event`, `CallStreaming`+`streamBody`, 28 unit tests, AC-01/02/03 verified by UNIT test; AC-04 (3-provider fixture replay) verified via representative synthetic fixtures instead of literal live-recorded network captures (no network egress to LLM providers or dev API key in this environment — see CHANGELOG SESSION-45 entry, same constraint as SESSION-44's AC-02). Circuit breaker/retry/token budget/session trim (SESSION-46) not yet in this package. |
| SESSION-46 | `30-ai/10-core/40-circuit_breaker.zsh`, `44-retry_decision.zsh`, `42-token_budget.zsh`, `60-session_trim.zsh` | `internal/llmclient` | PORTED | Resilience layer: `CircuitBreaker`+`BreakerStore`, `DecideHTTPRetry`+`ShouldRetry`, `ResolveMaxTokens`+`TemperatureForMode`+`IsReasoningModel`+`ReasoningEffortFor`, `TrimSession`, plus new (no zsh source) `EstimateTokens`/`TrimToFit` and the optional `CallWithRetry` wiring helper. 55 new test functions, AC-01..04 verified by UNIT test. `internal/llmclient` now has a complete request+resilience path (44+45+46) ready for SESSION-49. See CHANGELOG SESSION-46 entry. |
| SESSION-47 | `30-ai/05-tools/10-tool_fs_read.zsh`, `15-tool_search.zsh`, `20-tool_fs_write.zsh`, `25-tool_fs_patch_delete.zsh` | `internal/tools` | PORTED | Ten fs tools (`read_file`, `list_dir`, `count_lines`, `grep_search`, `glob_search`, `write_file`, `edit_file`, `move_file`, `patch_file`, `delete_file`), 46 new test functions, AC-01..05 verified. *(Row corrected in SESSION-48 — it was left at SKELETON/"Not started" despite the file-level detail section below and the CHANGELOG SESSION-47 entry both already documenting completed work; no functional change, table-row bug only.)* |
| SESSION-48 | `30-ai/05-tools/30-tool_process.zsh`, `35-tool_run_test.zsh`, `40-tool_git.zsh`, `45-tool_web_fetch.zsh`, `50-tool_todo.zsh` | `internal/tools` | PORTED | Remaining 7 named tools (`git_status`, `git_diff`, `web_fetch`, `todo_write`, `todo_read`, `exec_process`, `run_test`) plus `run_command` (legacy shell path), AC-01..04 targeted by UNIT/STATIC tests per the session YAML. All 18 `Registry` entries now have a real `Tool` implementation. **`go build`/`go vet`/`go test` could not actually be run in this session's environment** (no Go toolchain, no outbound network to install one) — verified by hand instead (import/usage cross-check, brace/paren balance with comments+strings stripped, interface-conformance re-read, dangerous-command regexes checked against real `grep -E`); see CHANGELOG SESSION-48 entry's "Verification note". Re-run the real toolchain before trusting this checkpoint the way SESSION-40..47's were trusted. |
| SESSION-49 | `30-ai/50-agent/00-policy.zsh` (transition matrix only), `10-state.zsh` (checkpoint-save subset only), `39-agent-state-machine.zsh`, `40-runtime/10-load_checkpoint.zsh` | `internal/agent` | PORTED (state/policy/checkpoint only) | Lifecycle state machine (`Phase`, `AgentState`), transition policy (`CanTransition`, `Transition`), and JSON checkpoint persistence (`Store.Save`/`Load`, `MigrateLegacyJSON`). 34 new test functions incl. exhaustive 5x5 transition matrix and terminal-state dead-end audit, AC-01..03 verified by UNIT test; AC-04 (migration against a real zsh checkpoint fixture) is **NOT VERIFIED** — no such fixture exists anywhere in this repository, and no evidence of a non-JSON legacy format was found either (see CHANGELOG SESSION-49 entry). ReAct execution loop, tool-call dispatch, and provider calls deliberately not ported — that's SESSION-50. Package status is `PORTED` for this session's narrow scope only; `internal/agent` as a whole stays incomplete until SESSION-50/51 land. |
| SESSION-50 | `30-ai/50-agent/42-execution/` (ReAct loop), `44-finalize.zsh` (data half) | `internal/agent` | PORTED (loop/plan/finalize only) | `RunLoop` drives the SESSION-49 state machine through PLAN→EXECUTE/VERIFY→COMPLETE/BLOCKED, calling `internal/llmclient` (provider/model fallback via `SelectProviderCandidate`+`CallWithRetry`) and `internal/tools` (`Dispatcher.Dispatch`) each iteration; checkpoints via SESSION-49's `Store`. 20 new test functions (fake LLM/fake tool, no network) incl. a checkpoint/resume double-execution audit; AC-02/AC-03 verified by UNIT test. AC-01 (real E2E) and AC-04 (real Ctrl-C interrupt-and-resume) are **NOT VERIFIED** — no provider API key in this environment (see CHANGELOG SESSION-50 entry). No syntax-verify-before-done gate, no project-index invalidation, no autodep retry, no notifications — all documented deviations (no Go port of the underlying checker exists yet for any of them). Subagent spawning (SESSION-51) and terminal rendering (SESSION-53) intentionally not here. |
| SESSION-51 | `30-ai/55-subagent/` | `internal/subagent` | PORTED | `SpawnSubagent` (researcher/coder) and `RunDebug` (`ai debug`), both reusing SESSION-50's `agent.RunLoop` with a role-scoped `tools.Dispatcher.Subset` and an isolated `permission.AgentContext` rather than a second loop. 14 new test functions incl. AC-01 (allowlist bypass attempt, call-counter proof)/AC-02 (parent-context non-mutation)/AC-04 (mutation tool rejected in debug mode) targeted cases, plus a multi-step integration scenario; AC-03 (real E2E against a live provider) is **NOT VERIFIED** — no provider API key/network egress in this environment, same constraint as SESSION-44/50 (see CHANGELOG SESSION-51 entry). No subagent checkpoint persistence (none exists in the zsh source to port). No live recursion path exists yet in `tools.Registry` for the depth guard to actually stop, so that guard is a forward guarantee, not an observed-and-closed one. |
| SESSION-52 | `00-ui_text.zsh`, `02-ui_colors.zsh`, `05-ui_box.zsh`, `06-ui_diff.zsh` | `internal/ui` | PORTED (tokens & primitives only) | Design tokens (`Tokens`, `ColorTokens`/`NoColorTokens`, `SupportsColor`/`ActiveTokens`), text wrap/width/unicode detection (`Wrap`, `Width`, `SupportsUnicode`), box drawing (`Box`, `BoxAccent`, approval+non-approval paths), diff colorizer (`DiffHeader`/`DiffFooter`/`ColorizeDiffBody`). 15 test functions, golden-file byte-parity against real `zsh`-executed fixtures (tokens, 20 text-wrap cases across 2 locale regimes, 10 box fixtures, 8 diff fixtures), AC-01..04 verified by UNIT test — `zsh`/`go` were both installed in this session's environment specifically to make this possible (see CHANGELOG SESSION-52 entry). Components/screens/router/command-registry deliberately not ported — that's SESSION-53. |
| SESSION-53 | `30-ai/60-ui/` (components & screens) | `internal/ui` | PORTED | *(Row corrected in SESSION-55 — same class of bug SESSION-48 fixed for SESSION-47's row: the package was fully built — `components/` (approval, cards, disclosure, header, palette, progress, state, timeline, verbosity), `screens/` (agent, home, palette, report), `registry.go` (`CommandRegistry`/`RegistryNames`/`RegistryRenderCategorized`/`RegistryFlatList`, 36 entries), `router.go`, 73 test functions across the package — but this table's row was left at SKELETON/"Not started" and no `### SESSION-53 —` CHANGELOG entry was ever written. SESSION-55 depends directly on `internal/ui.CommandRegistry` for its own `--help` output and found the gap that way; a full retroactive SESSION-53 CHANGELOG/file-level-detail writeup is still outstanding and is NOT done by this correction — only the status/summary above is fixed, so `--help` output can be trusted. `go test ./internal/ui/...` passes.)* |
| SESSION-54 | `30-ai/20-chat/`, `30-code/`, `35-files/`, `40-workflow/` | `internal/chat`, `internal/codeproject`, `internal/filepatch`, `internal/workflow` | PORTED | New shared package `internal/aiops` (not a 1:1 zsh port — small injected-dependency infra per this session's own §4 "dependency inversion" instruction: `Requester`/`Completer` wrapping `internal/llmclient`+`internal/config`'s provider/model fallback loop, `ConfirmFunc`, `Clipboard`/`ShareFunc`/`CommandRunner`, `UnifiedDiff`, `GuardDiff`, `SanitizePyCode`, `Slugify`/`Timestamp`). `internal/filepatch`: guards, `Cat`, `Patch`, `Undo`, `BakClean`, `Share` — 34 tests. `internal/chat`: `SplitReply`, `QuickChat`/`Aish`/`LongChat`, session store/ask/repl/mgmt (start/end/prune), `Clip` — 40 tests. `internal/codeproject`: `Code`, `Project`+5 helpers (`GenerateProject`/`SplitFiles`/`SalvageIfEmpty`/`FinishReport`/`Autotest`+`CheckCompleteness`), `Fix`+`FixApply`, `Run`, `Scrap` — 40 tests. `internal/workflow`: `Commit`, `Plan`, `Prompt`, `Spec`, `Build` (composes with `codeproject.Service`), `Review`+`ReviewDiffCore`, `Summarize` — 27 tests. All 141 new tests use fake `Completer`/`CommandRunner`/`ConfirmFunc` doubles, no live AI providers or real subprocess execution. `go build`/`go vet`/`go test ./...` all pass across the whole repo. Streaming, the `aiask` response cache, progress tickers, and battery/budget/data-saver/wakelock pre-flight checks deliberately not ported (see CHANGELOG SESSION-54 entry and file-level detail below for each). |
| SESSION-55 | *(new)* CLI entrypoint | `cmd/zshbagas`, `cmd/zshbagas/commands` | PORTED | One `cobra` subcommand per `ui.CommandRegistry` entry (36), each a legacy-alias-carrying thin wrapper around its SESSION-42..54 service method. First real (non-double) `aiops.ConfirmFunc` (`TerminalConfirm`), `aiops.Clipboard` (`termuxClipboard`), and `filepatch.ChooseFunc` (`terminalChoose`) implementations. `cobra`/`pflag` manually vendored from `codeload.github.com` (network-allowlist workaround, see CHANGELOG SESSION-55 entry). `scan`/`index`/`log`/`stats`/`dev` registered but return an explicit "not ported" error — their zsh source was never assigned to any SESSION-40..54, so flagged rather than fabricated. `deps`/`testmodels` implemented directly at the CLI layer (pure environment/network introspection, no `internal/` package to wire). 6 new test functions incl. bidirectional registry-parity checks and a wiring-regression test. `go build`/`go vet`/`go test ./... -race`/`gofmt -l` all pass; `make build`/`make build-termux` (SESSION-40's own targets, unmodified) both verified working end to end. AC-04 (manual E2E against a real provider) not verified — no API key/network egress to LLM providers in this environment, same constraint every session back to SESSION-44 notes. See CHANGELOG SESSION-55 entry for the full alias-mapping table and deviation list. |
| SESSION-56 | verify37/verify38/verify39 (\*.zsh), `source/install.sh` | `migration_verify/harness_go.go`, `source/install.sh` | PORTED (verification harness + install cutover only) | New standalone `migration_verify/harness_go.go` (not part of the `go/` module — stdlib only): (1) rebuilds `zshbagas`, runs `go vet`/`go test ./...`, (2) re-runs SESSION-52's `Test*_GoldenParity` suite as the one true byte-for-byte old-vs-new diff (UI rendering primitives — 41 golden sub-tests, 0 diffs against `golden/`), (3) re-runs `verify37/38/39` against the real unmodified zsh source to reconfirm the fallback path hasn't drifted, classifying each result as `KNOWN` (matches a `docs/audit/DISCOVERED_37/38/39_*.md` finding already on record) or `FAIL` (new/unexplained — none found), (4) captures deterministic no-key `zshbagas` CLI output to `migration_verify/go_cli_capture.txt` for manual AC-02/AC-03 review. AC-01 verified by UNIT (harness run, 0 unexplained diffs). `source/install.sh` rewritten: downloads a `zshbagas-<os>-<arch>` binary from GitHub Releases (`monang404/zsh_bagas-go`) into `~/.local/bin`, still clones/symlinks the old `.zsh_bagas` source unchanged (fallback `ai*` aliases, per this session's own `scope.exclude` — no deletion), and now manages a minimal `~/.zshrc` block (PATH + one `source` line) instead of symlinking the whole file to a repo template. AC-03 (fresh Termux install) is **NOT VERIFIED** — no Termux device in this environment, and `monang404/zsh_bagas-go` has no GitHub Release yet to download from (verified the graceful-failure branch instead: a real request to the release URL returns 404 and the script falls through to its documented manual-build message rather than aborting non-gracefully). AC-02 (1-week daily-driver usage) is **NOT VERIFIED** — no such usage period exists inside a single execution session; see CHANGELOG SESSION-56 entry. `.zsh_bagas/` source itself is untouched and still fully present — deletion stays out of scope per this session's `scope.exclude`, same as its own YAML says. |

## SESSION-56 file-level detail

| Source concern | New file | Symbols/role |
|---|---|---|
| `verify37/test_readme_fencing.zsh`, `verify38/run_all.zsh`, `verify39/run_all.zsh` (re-run, not modified) | `migration_verify/harness_go.go` | `runVerifyScript`, known-finding classifier tables (`known38`, `known39`) |
| SESSION-52's `internal/ui` `Test*_GoldenParity` (re-run, not modified) | `migration_verify/harness_go.go` | invoked via `go test ./internal/ui/... -run GoldenParity` |
| *(new — no zsh source, deterministic CLI capture)* | `migration_verify/harness_go.go`, `migration_verify/go_cli_capture.txt` (generated, not checked in as a "golden" file — see notes) | `captureGoCLI` |
| `source/install.sh` (rewritten, see CHANGELOG SESSION-56 entry) | `source/install.sh` | binary-release download + minimal `.zshrc` block, old-source clone/symlink unchanged |

**Not run this session:** `verify37/test_role_parity.zsh` — requires `jq`,
which `apt-get install jq` could not fetch in this environment
(`security.ubuntu.com` returned 404 for `jq`/`libjq1` specifically, while
`golang-go` and `zsh` installed fine from the same mirror set earlier in
the session — an intermittent/partial mirror gap, not a full network
block). Harness records this as `SKIP` with the reason rather than
silently omitting it; last known-good result is SESSION-37's own
checkpoint (`docs/audit/DISCOVERED_37_live_model_confirmation_blocked.md`
covers that session's own separate, unrelated finding).

## SESSION-55 file-level detail

### `cmd/zshbagas/commands` (new — wiring layer, not a 1:1 zsh port)

| Source concern | Go file | Go symbols |
|---|---|---|
| dependency wiring for every ported service | `app.go` | `App`, `NewApp`, `buildDispatcher`, `subagentDeps`, `agentDeps` |
| `10-core/32-confirm.zsh` (`_ai_confirm`, real terminal path) | `app.go` | `TerminalConfirm`, `terminalAsk` |
| `termux-clipboard-get`/`-set` calls (scattered call sites, e.g. `30-code/45-aiclip.zsh`) | `app.go` | `termuxClipboard`, `newTermuxClipboard` |
| root command tree, `20-menu.zsh`'s categorized listing | `root.go` | `NewRootCmd`, `Execute` |
| AC-03's literal description (not `90-selfcheck.zsh`'s actual behavior — see CHANGELOG) | `root.go` | `startupSelfCheck`, `noAPIKeyCommands` |
| AC-01 (registry ↔ cobra-tree parity) | `root.go` | `assertRegistryParity` |
| `20-chat/00-quick_chat.zsh`, `05-aiask.zsh`, `10/15/20-session_*.zsh`, `25-aiclip.zsh` | `chat.go` | `newChatCmd`, `newLongCmd`, `newShellCmd`, `newAskCmd`, `newClipCmd`, `newSessionCmd` |
| `30-code/05-code.zsh`, `10-aipatch.zsh` (edit alias), `05-aicat.zsh`, `45-fix.zsh`, `50-run.zsh`, `00-aicommit.zsh`, `review`, `55-scrap.zsh` | `code.go` | `newCodeCmd`, `newEditCmd`, `newViewCmd`, `newFixCmd`, `newRunCmd`, `newScrapCmd`, `newCommitCmd`, `newReviewCmd` |
| `35-files/15-aiundo.zsh`, `20-aibakclean.zsh`, `25-aishare.zsh` | `files.go` | `newUndoCmd`, `terminalChoose`, `newBakCleanCmd`, `newShareCmd`, `shareFile` |
| `40-workflow/` plan/prompt/spec/summarize | `workflow.go` | `newPlanCmd`, `newPromptCmd`, `newSpecCmd`, `newSummarizeCmd`, `resolveSummarizeSource`, `stripHTMLTags` |
| `30-code/` project/build; `45-project.zsh`/`46-index.zsh` (not ported, flagged) | `project.go` | `newProjectCmd`, `newBuildCmd`, `newScanCmd`, `newIndexCmd`, `errNoGoPort` |
| `50-agent/` (`aiagent`), `55-subagent/` (`aidebug`/`airesearch`/`aidelegate`) | `agent.go` | `newAgentCmd`, `newDebugCmd`, `newResearchCmd`, `newDelegateCmd`, `runStandaloneSubagent`, `printAgentResult` |
| `15-diagnostics.zsh` (CLI-layer reimplementation, not a byte-identical port), `10-help_stats.zsh`/`25-research_dev.zsh`'s `aidev` (not ported, flagged) | `util.go` | `newDepsCmd`, `newTestModelsCmd`, `newHCmd`, `newMenuCmd`, `newLogCmd`, `newStatsCmd`, `newDevCmd` |

**Vendored third-party (not zsh-sourced):** `go/vendor/github.com/spf13/cobra`
(v1.8.1, core package only — `doc/` subpackage and its `go-md2man`/
`yaml.v3` dependency chain excluded, `command_win.go` removed to drop
the Windows-only `mousetrap` dependency) and `go/vendor/github.com/spf13/pflag`
(v1.0.5, full package). See CHANGELOG SESSION-55 entry's "Dependency
fetch workaround" for why `go get` doesn't work in this sandbox and
what was done instead.

**Not ported (flagged, not fabricated):** `45-project.zsh` (`aiscan`),
`46-index.zsh` (`aiindex`), `60-ui/10-help_stats.zsh` (`aihist`/
`aistats`), and the `tmux`-workspace half of `60-ui/25-research_dev.zsh`
(`aidev`) — none of these files were ever assigned to a SESSION-40..54
YAML, so no `internal/` package implements them; `cmd/zshbagas/commands`
registers their subcommands (for `--help`/registry-parity purposes) but
each `RunE` returns an explicit error identifying the gap.

## SESSION-54 file-level detail

### `internal/filepatch`

| zsh source | Go file | Go symbols |
|---|---|---|
| `35-files/00-guards.zsh` (`_ai_is_secret_file`, `_ai_is_binary_file`) | `guards.go` | `IsSecretFile`, `IsBinaryFile` |
| `35-files/05-aicat.zsh` (`aicat`) | `cat.go` | `Cat`, `ErrBinaryFile`, `ErrFileNotFound` |
| `35-files/10-aipatch.zsh` (`aipatch`) | `patch.go` | `Service`, `Patch`, `PatchResult`, `stripCodeFences` |
| `35-files/15-aiundo.zsh` (`aiundo`) | `undo.go` | `Undo`, `ListBackups`, `ChooseFunc`, `UndoResult` |
| `35-files/20-aibakclean.zsh` (`aibakclean`) | `bakclean.go` | `BakClean`, `BakCleanResult` |
| `35-files/25-aishare.zsh` (`aishare`) | `share.go` | `Share` |

### `internal/chat`

| zsh source | Go file | Go symbols |
|---|---|---|
| `20-chat/01-chat_display.zsh` (`_ai_chat_split_reply`) | `chat_display.go` | `SplitReply` |
| `20-chat/00-quick_chat.zsh` (`aic`, `aicl`, `aish`) | `chat.go` | `Service`, `QuickChat`, `LongChat`, `Aish`, `LongChatStages` |
| `20-chat/05-aiask.zsh` (`aiask`) | `aiask.go` | `Ask`, `AskResult` |
| `20-chat/10-session_ask.zsh` (`_ai_session_ask`) | `session_ask.go` | `SessionAsk` |
| `20-chat/15-session_repl.zsh` (`_ai_session_repl`) | `session_repl.go` | `SessionRepl` |
| `20-chat/20-session_mgmt.zsh` (`_ai_session_prune`, `ai session start/end`) | `session_mgmt.go` | `SessionPrune`, `Start`, `End` |
| *(shared by all of the above)* | `session_store.go` | `SessionStore`, `Load`, `List`, `Clear` |
| `20-chat/25-aiclip.zsh` (`aiclip`, `_ai_clip_is_sensitive`) | `clip.go` | `Clip`, `IsClipSensitive` |

### `internal/codeproject`

| zsh source | Go file | Go symbols |
|---|---|---|
| `30-code/05-code.zsh` (`aicode`) | `code.go` | `Service`, `Code`, `CodeResult` |
| `30-code/10-project_generate.zsh` (`_ai_project_generate`) | `project_generate.go` | `GenerateProject`, `GenerateResult` |
| `30-code/15-project_split.zsh` (`_ai_project_split_files`, embedded python splitter) | `project_split.go` | `SplitFiles`, `SplitResult`, `safeJoin` |
| `30-code/20-project_salvage.zsh` (`_ai_project_salvage_if_empty`) | `project_salvage.go` | `SalvageIfEmpty`, `SalvageResult` |
| `30-code/25-project_report.zsh` (`_ai_project_finish_report`) | `project_report.go` | `FinishReport` |
| `30-code/30-project.zsh` (`aiproject`) | `project.go` | `Project`, `ProjectResult` |
| `30-code/35-project_autotest.zsh` (`_ai_project_autotest`) | `project_autotest.go` | `Autotest`, `AutotestResult`, `findPyFiles` |
| `30-code/40-project_completeness.zsh` (`_ai_project_check_completeness`) | `project_completeness.go` | `CheckCompleteness`, `CompletenessResult` |
| `30-code/45-fix.zsh` (`_ai_fix_apply`, `aifix`) | `fix.go` | `FixApply`, `Fix`, `FixResult` |
| `30-code/50-run.zsh` (`airun`) | `run.go` | `Run`, `RunResult` |
| `30-code/00-scrap.zsh` (`aiscrap`) | `scrap.go` | `Scrap`, `sniffStructure` (approximation, see notes below) |

### `internal/workflow`

| zsh source | Go file | Go symbols |
|---|---|---|
| `40-workflow/00-aicommit.zsh` (`aicommit`) | `commit.go` | `Service`, `Commit`, `CommitResult` |
| `40-workflow/05-aiplan.zsh` (`aiplan`) | `plan.go` | `Plan`, `PlanResult` |
| `40-workflow/10-aiprompt.zsh` (`aiprompt`) | `prompt.go` | `Prompt`, `PromptResult` |
| `40-workflow/15-aispec.zsh` (`aispec`) + `00-config/30-sysprompt_spec.zsh` (`AI_SPEC_SYSPROMPT`) | `spec.go` | `Spec`, `SpecResult`, `SpecSysPrompt` |
| `40-workflow/20-aibuild.zsh` (`aibuild`) | `build.go` | `Build`, `BuildResult` |
| `40-workflow/25-aireview.zsh` (`aireview`, `_ai_review_diff_core`) | `review.go` | `Review`, `ReviewDiffCore`, `ReviewResult` |
| `40-workflow/30-aisummarize.zsh` (`aisummarize`) | `summarize.go` | `Summarize`, `SummarizeResult`, `chunkByParagraph`, `splitParagraphs` |

### `internal/aiops` (new, shared infra — not a 1:1 zsh port)

| zsh source | Go file | Go symbols |
|---|---|---|
| `10-core/25-quick_chat.zsh` (`_ai_quick`, `_ai_chat_request` provider/model fallback loop) | `request.go` | `Requester`, `Completer`, `Complete`, `CompleteMessages` |
| `10-core/32-confirm.zsh` (`_ai_confirm` exit-code contract) | `confirm.go` | `Decision`, `ConfirmFunc`, `AutoApprove`, `AutoDecline` |
| *(new — injectable interfaces for terminal/platform-specific calls)* | `adapters.go`, `exec_runner.go` | `Clipboard`, `ShareFunc`, `CommandRunner`, `ExecRunner` |
| *(new — no single zsh source; a from-scratch `diff -u` equivalent, since no diff library was available to import in this environment)* | `diff.go` | `UnifiedDiff` |
| `10-core/*` `AI_DIFF_MAX_CHARS` guard (used inline by `aicommit`/`aireview`) | `guard_diff.go` | `GuardDiff`, `DiffMaxChars` |
| `10-core/25-quick_chat.zsh` (`_ai_sanitize_pycode`) | `sanitize.go` | `SanitizePyCode` |
| `10-core/25-quick_chat.zsh` (`_ai_ts`) + the `tr`/`cut` slug pipeline duplicated across `30-code/`/`40-workflow/` | `slug.go` | `Slugify`, `Timestamp`, `BackupPath` |

## File-level notes (SESSION-54 additions)

- Not ported: streaming/token-by-token terminal output for `aic`/`aish`/`_ai_session_ask` — every
  command in `internal/chat`/`internal/codeproject`/`internal/workflow` uses the blocking request
  path (`aiops.Completer`, itself backed by `internal/llmclient.CallBlocking`) instead of
  `CallStreaming`. Per this session's own `EXECUTION_CONTEXT.md` §0, exact terminal
  output/streaming is not required unless an existing UI contract explicitly owns it, and no such
  contract exists yet for these commands — SESSION-55 wires the CLI/UI layer these functions will
  stream through.
- Not ported: `aiask`'s response cache (`_ai_cache_key`/`_ai_cache_read`/`_ai_cache_write`,
  `10-core/`, not assigned to any session's `source_zsh_files`) — `chat.Ask` always performs a live
  request. Reimplementing an approximation of the cache without its actual key/storage primitives
  existing yet risked silently diverging from whatever exact semantics a future session ports; see
  `aiask.go`'s own doc comment.
- Not ported: `_ai_battery_check`/`_ai_budget_check`/`_ai_data_saver_check`/`_ai_wakelock_*`
  (`10-core/`, not part of this session's file list) — pre-flight guards `aiproject`/`aibuild` run
  before their expensive generate step. Callers that need those guards run them
  before/around calling `codeproject.Project`/`workflow.Build`; see each function's own doc comment.
- Not ported: `_ai_progress_tick_start`/`_ai_progress_tick_stop` (terminal-only cosmetic background
  ticker) — out of scope per this session's own note that `internal/ui` is not part of this session.
- Deliberate approximation, not a byte-identical port: `aiscrap`'s HTML structure sniff
  (`sniffStructure` in `codeproject/scrap.go`) reimplements just enough of the zsh source's
  BeautifulSoup-based anchor-tag/class/text extraction using a regexp-based scanner, since no
  third-party HTML parser Go module was reachable in this build environment (network policy blocks
  the Go module proxy). Functionally similar signal, not a byte-identical port.
- Deliberate scope note on AC-03 (`docs/execution_sessions/54_...yaml`, "project_split tetap
  menghormati aturan 150-baris per file"): no 150-line-per-file rule exists anywhere in the actual
  `30-code/*.zsh` source read for this session (`15-project_split.zsh` splits purely on `### FILE:`
  markers, with no line-count logic at all). This acceptance criterion does not correspond to any
  real behavior in the source of truth this session ported from; `SplitFiles`'s tests instead verify
  its actual behavior (marker-based splitting plus the path-containment guard against `..`/absolute-
  path escapes).
- Not ported: `aiscrap`/`aisummarize`'s exact `python3 $AI_SANITIZE_SCRIPT -` **stdin-piped**
  invocation mode — `aiops.CommandRunner` (this session's process-execution seam) only supports
  argument-based invocation, not piping arbitrary content through a subprocess's stdin. `Scrap`
  returns its reply unsanitized as a result; a caller wiring real stdin support (SESSION-55) can pipe
  it through the sanitize script itself.
- `internal/workflow.Build` (`aibuild`) is the one place in this session's four packages with a
  cross-package dependency: it composes its own `Spec` step with `internal/codeproject.Project`,
  taking a `*codeproject.Service` parameter — matching the zsh source's own `aispec`-then-`aiproject`
  call chain exactly (a one-way `workflow -> codeproject` dependency, consistent with the zsh load
  order where `30-code/` loads before `40-workflow/`).



| zsh source | Go file | Go symbols |
|---|---|---|
| `02-ui_colors.zsh` (`_ai_ui_supports_color`, `_ai_ui_colors_init`, `_ai_ui_c`, `_ai_ui_highlight_body`) | `tokens.go` | `Tokens`, `ColorTokens`, `NoColorTokens`, `SupportsColor`, `IsTerminal`, `ActiveTokens`, `Tokens.C`, `Tokens.HighlightBody` |
| `00-ui_text.zsh` (`_ai_ui_supports_unicode`, `_ai_ui_width`, `_ai_ui_wrap`) | `text.go` | `SupportsUnicode`, `Width`, `Wrap`, `wordLen`, `splitWord`, `parseNonNegativeInt` |
| `05-ui_box.zsh` (`_ai_ui_box_accent`, `_ai_ui_box`) | `box.go` | `Mode`, `DetectMode`, `BoxAccent`, `Box` |
| `06-ui_diff.zsh` (`_ai_ui_diff_header`, `_ai_ui_diff_footer`) + SESSION-25 body colorizer (`30-code/05-code.zsh`/`35-files/10-aipatch.zsh`, `sed` pattern) | `diff.go` | `DiffHeader`, `DiffFooter`, `ColorizeDiffBody` |

**Deliberate deviation — locale-coupled character width:** see CHANGELOG
SESSION-52 entry for the full explanation. `Wrap(text, width, unicode)`'s
`unicode` parameter reproduces zsh's rune-vs-byte `${#w}` semantics, which
in the real shell depend implicitly on the active `LC_*` locale (the same
condition `_ai_ui_supports_unicode` itself checks) rather than being a
Go-side invention.

**Not ported (out of session-52 scope, deferred to SESSION-53):**
`_ai_ui_line` (60-ui/05-ui_box.zsh, one-line icon helper — not in this
session's `target_go_files`), `ui_card_summary`
(`60-ui/components/cards.zsh`), `_ai_state_thinking`/`_sending`/`_acting`/
`_waiting`/`_done`/`_error` (`60-ui/components/state.zsh`) — all are
components/screens per `docs/RENDERING_CONTRACT.md` §2's helper table, not
primitives, and none of their source files are listed in this session's
`source_zsh_files`.

## SESSION-51 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `00-design_contract.zsh` (contract only, no functions) | `run.go` (package doc comment) | — |
| `05-tool_allowlist.zsh` (`_ai_subagent_tool_allowed`, `_ai_subagent_oneline`) | `allowlist.go` | `Role`, `IsValidRole`, `AllowedTools`, `ToolAllowed` |
| `10-sysprompt.zsh` (`_ai_subagent_build_sysprompt`) | `sysprompt.go` | `BuildSysprompt` |
| `15-run_step.zsh` (`_ai_subagent_step`) | `run.go` (via reused `agent.RunLoop`) | *(no direct port — see note below)* |
| `20-run.zsh` (`_ai_subagent_run`) | `run.go` | `Result`, `Status`, `Deps`, `SpawnSubagent`, `toResult`, `subagentContext`, `subagentMaxSteps` |
| `25-debug_allowlist.zsh` (`_ai_debug_tool_allowed`) | `allowlist.go` | `DebugToolAllowed` |
| `30-debug_step.zsh` (`_ai_debug_step`) | `debug.go` (via reused `agent.RunLoop`) | *(no direct port — see note below)* |
| `35-debug_report.zsh` (`_ai_debug_print_report`) | `debug.go` | `Report`, `toReport` |
| `40-debug.zsh` (`aidebug`) | `debug.go` | `RunDebug`, `debugSysprompt` |
| *(no direct source — new, required by AC-01)* | `../tools/dispatch.go` | `Dispatcher.Subset` |

**Why `15-run_step.zsh`/`30-debug_step.zsh` have no direct Go file:**
both are literally "one chat+tool step of the loop" — exactly what
SESSION-50's `runState.getPlan`/`runTool`/`trackAndContinue` already do.
Porting them again as a second implementation would be the "parallel
execution loop" the session brief explicitly forbids (FASE 9: "Gunakan
loop Session 50... Jangan membuat execution loop kedua"). Their
behavior is reached through `agent.RunLoop` itself, scoped down via
`Dispatcher.Subset` + a narrow `SystemPrompt` + a clamped
`Limits.AgentMaxSteps` — data narrowing the existing loop's inputs, not
new loop code.

**Deliberately NOT ported this session:**
- Subagent-specific persistent checkpoint — does not exist in the zsh
  source (`_ai_subagent_run` never calls
  `_ai_agent_checkpoint_save`), so none was invented here either (FASE
  12).
- `Report.Reproduction`'s per-call "tool: output" trail
  (`30-debug_step.zsh`'s `reproduction+=(...)`) — `agent.FinalResult`
  is a terminal summary, not a step-level transcript; reconstructing
  one would require changing SESSION-50's `RunLoop` return shape,
  which is out of this session's scope.
- Heuristic trigger / user-approval offer flow
  (`50-agent/40-runtime/05-subagent_offer.zsh`, design_contract.zsh
  §1/§2) and the `aidelegate`/`ai delegate` CLI command
  (`60-ui/25-research_dev.zsh`) — both are UI/CLI-wiring concerns
  (SESSION-52/53/55), not part of the orchestration primitive itself.

## SESSION-50 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `10-state.zsh` (`_ai_agent_parse`) | `plan.go` | `Plan`, `Plan.Empty`, `ParsePlan` |
| `00-loop_main.zsh` (`_ai_agent_execute_loop`) | `loop.go` | `Deps`, `RunLoop`, `runState`, `errFatalTransition` |
| `05-get_plan.zsh` (`_ai_agent_exec_get_plan`) | `loop.go` | `runState.getPlan`, `runState.requestCompletion` |
| `41-provider.zsh` (`_ai_agent_provider_request`), `50-request_blocking.zsh` (provider/model fallback loop) | `loop.go` | `defaultComplete` |
| `10-reject_checks.zsh` (`_ai_agent_exec_check_done_rejections`) | `loop.go` | `runState.rejectDoneChecks` |
| `15-run_tool.zsh` (`_ai_agent_exec_run_tool`) | `loop.go` | `runState.runTool`, `extractDestOrPath` |
| `25-track_and_continue.zsh` (`_ai_agent_exec_track_and_continue`); `20-log_and_notify.zsh`'s history-append fragment only | `loop.go` | `runState.trackAndContinue` |
| `39-agent-state-machine.zsh` (`_ai_agent_state_transition_or_warn`) | `loop.go` | `runState.transitionOrWarn` |
| `44-finalize.zsh` (data-derivation half only) | `finalize.go` | `FinalResult`, `Finalize` |
| *(no direct source — new, see CHANGELOG)* | `checkpoint.go` | `Store.Delete` |

**Deliberately NOT ported this session (see `loop.go`'s own package doc
comment for the full reasoning behind each):**
- Syntax-check-before-done gate (`_ai_verify_touched_files` and its
  per-extension `py_compile`/`zsh -n`/`node --check`/etc. dispatch,
  `10-reject_checks.zsh`) — no Go port of that checker exists anywhere
  in this repository yet.
- Project-index invalidation on successful `write_file`/`edit_file`/
  `delete_file`/`move_file` (`15-run_tool.zsh`'s `46-index.zsh`
  integration) — no Go index package exists yet.
- Exit-127 dependency auto-install-and-retry (`15-run_tool.zsh`'s
  `02-tool_autodep.zsh` hook) — no Go port of that module exists yet.
- The jsonl run-log write and rate-limited desktop notification
  (`20-log_and_notify.zsh`) — presentation/observability concerns,
  deferred to SESSION-53 with the rest of rendering; only that file's
  history-append fragment (folded into `trackAndContinue`) was ported.
- All of `44-finalize.zsh`'s box-drawing/diff/AI-review/`/details`
  output — UI, SESSION-53. Only the block-reason-default-fill and
  terminal-state→fields mapping were ported (`Finalize`).
- Provider spinner, `TRAPINT`/`TRAPTERM` shell-level Ctrl-C plumbing,
  `AI_CURRENT_PROVIDER`/`AI_CURRENT_MODEL` runtime state
  (`50-request_blocking.zsh`) — `defaultComplete` handles cancellation
  through Go's `context.Context` instead (see `RunLoop`'s own
  `ctx.Done()` check, which is the actual mechanism a future CLI's
  Ctrl-C handler would drive).

## SESSION-49 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `39-agent-state-machine.zsh` (`AI_AGENT_STATE_TRANSITIONS`, `_ai_agent_state_is_terminal`) | `state.go` | `Phase`, `PhasePlan`, `PhaseExecute`, `PhaseVerify`, `PhaseComplete`, `PhaseBlocked`, `Phase.IsTerminal`, `Phase.IsValid` |
| `39-agent-state-machine.zsh` (`_ai_agent_state_transition`) | `policy.go` | `CanTransition`, `Transition`, `InvalidTransitionError` |
| `10-state.zsh` (`_ai_agent_checkpoint_save`) | `checkpoint.go` | `Checkpoint`, `NewCheckpoint`, `Store`, `Store.Save`, `CurrentCheckpointSchemaVersion` |
| `40-runtime/10-load_checkpoint.zsh` (`_ai_agent_load_checkpoint`) | `checkpoint.go` | `Store.Load`, `decodeCheckpoint`, `Checkpoint.Validate` |
| *(no direct source — evidenced only by the loader's own `// 1` defensive fallback, see CHANGELOG)* | `checkpoint.go` | `MigrateLegacyJSON`, `LegacyCheckpoint` |
| `00-policy.zsh` (`_ai_agent_is_dangerous`, `AI_AGENT_DANGEROUS_PATTERNS`) | *(already ported, SESSION-48)* | `tools.IsDangerousCommand` — not re-touched this session; `00-policy.zsh` is listed as a SESSION-49 source file in the session YAML for the transition-matrix comment block only (`39-agent-state-machine.zsh` is where the matrix itself lives), the dangerous-command policy in the same file was already ported in SESSION-48's `tools/policy.go`. |

**Deliberately NOT ported this session (SESSION-50/53 concern, see `state.go`'s own doc comment and the CHANGELOG SESSION-49 entry for full rationale):**
- `_ai_agent_parse` (`10-state.zsh`) — LLM-reply JSON parsing/tool-call extraction. Belongs to the ReAct loop (SESSION-50), not the state machine.
- `_ai_agent_slug`, `_ai_subagent_should_offer`, `_ai_agent_project_name` (`10-state.zsh`) — presentation/subagent-offer/naming helpers, unrelated to lifecycle state or checkpoint schema.
- `00-loop_main.zsh`, `05-get_plan.zsh`, `10-reject_checks.zsh`, `15-run_tool.zsh`, `20-log_and_notify.zsh`, `25-track_and_continue.zsh` (all of `42-execution/`) — the actual ReAct loop driver that calls `_ai_agent_state_transition` repeatedly; this is exactly SESSION-50's `objective`. Read in full during this session's audit (step 2) to confirm `AgentState`'s field selection and the transition-call sites match, but no logic from them was ported.
- `44-finalize.zsh` — UI/summary rendering of the final BLOCKED/COMPLETE box; UI is SESSION-53.
- Ephemeral per-run loop locals (`last_failed_tool`, `last_failed_args`, `same_fail_count`, `reply`/`thought`/`tool`/`args` as raw per-iteration values) — documented explicitly as excluded from `AgentState` in `state.go`'s doc comment; these never reach `$state_dir` in the zsh source either.
- The `checkpoint_file.lock` mkdir-based concurrent-writer lock (`10-state.zsh`) — see CHANGELOG SESSION-49 "Deliberate deviations".

## SESSION-41 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `00-models.zsh` | `models.go` | `GroqModel`, `GroqReasoningEffort`, `TaskClass`, `Models`, `ModelsFor` |
| `05-provider_order.zsh` | `providers.go` | `TaskProviderOrderFast/Smart/Big/Agent`, `TaskProviderOrder` |
| `10-paths.zsh` | `limits.go` | `Paths`, `LoadPaths` |
| `15-limits.zsh` | `limits.go` | `Limits` (top half), `LoadLimits` |
| `20-runtime_guards.zsh` | `limits.go` | `Limits` (bottom half, threshold fields only) |
| `25-persona.zsh` | `persona.go` | `AsciiFallback`, `PersonaShort/Long`, `PersonaChatShort/Long`, `ChatAnswerMarker` |
| `30-sysprompt_spec.zsh` | `sysprompt.go` | `SpecSysprompt`, `TermuxContext` |
| `35-providers.zsh` | `providers.go` | `Provider`, `Providers`, `ProviderOrder`, `ActiveProviders`, `HasAnyKey` |
| `40-context_engine_docs.zsh` | `sysprompt.go` | `ContextEngineLevel`, `ContextEngineLevels` |

## SESSION-42 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `00-config.zsh` | `context.go` | `PermConfig`, `LoadPermConfig` |
| `05-agent_context.zsh` | `context.go` | `Role`, `Capability`, `AgentContext`, `NewAgentContext`, `(*AgentContext).CapabilityAllowed`, `(*AgentContext).Grant` |
| `10-path_guard.zsh` | `pathguard.go` | `ProjectRoot`, `CanonicalPath`, `PathWithinProject`, `IsPathAllowed`, `ValidateProjectPath` |
| `15-permission_check.zsh` | `check.go` | `Level`, `Decision`, `Request`, `CheckPermission` |
| `20-perm_ask.zsh` | `check.go`, `ask.go` | `checkProcess`, `AskFunc` |
| `25-perm_write.zsh` | `check.go`, `ask.go` | `checkWrite`, `ApprovalTracker` |
| `30-perm_shell.zsh` | `check.go` | `checkShell` |

## SESSION-43 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `00-tool_registry.zsh` (`AI_TOOL_REGISTRY`, `AI_TOOL_CAPABILITY`) | `registry.go` | `Entry`, `Registry`, `Names` |
| `00-tool_registry.zsh` (`AI_TOOL_SCHEMA`) | `schema.go` | `ValidateArgs`, `schemas` |
| `02-tool_args_extract.zsh` | `args.go` | `ExtractField`, `ExtractPath`, `NormalizeArgs` |
| `02-tool_autodep.zsh` (pure half only — see CHANGELOG) | `autodep.go` | `DetectPackageManager`, `CmdToPackage`, `ExtractMissingCmd` |
| `05-tool_dispatch.zsh` (`_ai_tool_manifest`) | `registry.go` | `Manifest` |
| `05-tool_dispatch.zsh` (`_ai_tool_validate_request`, `_ai_tool_dispatch`) | `dispatch.go` | `PermDeps`, `Dispatcher`, `(*Dispatcher).Register`, `(*Dispatcher).RegisterFromRegistry`, `(*Dispatcher).Dispatch` |
| *(none — new interface, no direct zsh equivalent)* | `tool.go` | `Result`, `Tool`, `NoopTool` |

## SESSION-46 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `40-circuit_breaker.zsh` (`_ai_breaker_record_fail`, `_ai_breaker_is_open`) | `circuitbreaker.go` | `State`, `CircuitBreaker`, `(*CircuitBreaker).Allow`, `(*CircuitBreaker).RecordSuccess`, `(*CircuitBreaker).RecordFailure`, `BreakerStore`, `DefaultBreakerThreshold` |
| `44-retry_decision.zsh` (`_ai_chat_retry_decision`) | `retry.go` | `RetryAction`, `HTTPRetryOutcome`, `DecideHTTPRetry` |
| *(new — see CHANGELOG)* | `retry.go` | `ShouldRetry` |
| `42-token_budget.zsh` (`_ai_resolve_max_toks`, `_ai_chat_temp_for_mode`, `_ai_is_reasoning_model`, `_ai_reasoning_effort_for`) | `tokenbudget.go` | `ResolveMaxTokens`, `TemperatureForMode`, `IsReasoningModel`, `ReasoningEffortFor`, `DeepseekReasoningEffortDefault` |
| *(none — new, no zsh equivalent, see CHANGELOG)* | `tokenbudget.go` | `EstimateTokens`, `TrimToFit` |
| `60-session_trim.zsh` (`_ai_trim_session`) | `sessiontrim.go` | `TrimSession` |
| *(none — new, optional wiring helper per scope.include, see CHANGELOG)* | `resilience.go` | `CallWithRetry` |

## SESSION-48 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `05-tools/40-tool_git.zsh` (`_ai_tool_git_status`) | `git.go` | `GitStatusTool` |
| `05-tools/40-tool_git.zsh` (`_ai_tool_git_diff`) | `git.go` | `GitDiffTool` |
| *(none — new, shared helper, see CHANGELOG)* | `git.go` | `gitAvailable`, `insideGitWorkTree` |
| `05-tools/45-tool_web_fetch.zsh` (`_ai_tool_web_fetch`) | `webfetch.go` | `WebFetchTool` |
| `05-tools/45-tool_web_fetch.zsh` (inline python3 SSRF pre-check) | `webfetch.go` | `resolveSafePublicAddr`, `isUnsafeIP` |
| `05-tools/45-tool_web_fetch.zsh` (inline python3 `strip_script`) | `webfetch.go` | `stripHTML` |
| `05-tools/50-tool_todo.zsh` (`_ai_tool_todo_write`) | `todo.go` | `TodoWriteTool` |
| `05-tools/50-tool_todo.zsh` (`_ai_tool_todo_read`) | `todo.go` | `TodoReadTool` |
| *(none — new, session-slug bridge, see CHANGELOG deviation note)* | `todo.go` | `todoSessionSlug`, `todoFilePath` |
| `50-agent/00-policy.zsh` (`_ai_agent_is_dangerous`, `AI_AGENT_DANGEROUS_PATTERNS`) | `policy.go` | `IsDangerousCommand`, `dangerousPatterns` |
| *(none — new, tokenizer helper, see CHANGELOG)* | `policy.go` | `tokenizeShellLike` |
| `05-tools/30-tool_process.zsh` (`_ai_tool_exec_process`) | `process.go` | `ExecProcessTool` |
| `05-tools/30-tool_process.zsh` (`_ai_tool_run_command`) | `process.go` | `RunCommandTool` |
| `05-tools/35-tool_run_test.zsh` (`_ai_tool_run_test`) | `process.go` | `RunTestTool` |
| *(none — new, shared helper, see CHANGELOG)* | `process.go` | `resolveNonProjectExecutable`, `resolveRunDir`, `clampTimeout`, `runCapped` |
| *(none — new, additive field, see CHANGELOG)* | `args.go` (SESSION-43) | `ExtractPath` (`"cwd"` added to its field list) |
| *(none — new, shared helper, see CHANGELOG)* | `fsguards.go` (SESSION-47) | `firstNChars` |

**Deliberately NOT ported (SESSION-48):** `_ai_yolo_shell_safe`'s fast-path allowlist tokenizer
(`05-tools/30-tool_process.zsh`) — a YOLO-mode ask-skipping optimization internal to the zsh
permission layer with no equivalent need in this Go port, where `permission.CheckPermission`
already makes the one permission decision before `Dispatcher.Dispatch` ever reaches `Execute`; see
`policy.go`'s own doc comment. Also not ported: the exit-127 autodep auto-install retry
`_ai_tool_exec_process`/`_ai_tool_run_command` both have in zsh — same boundary SESSION-43 already
drew around `autodep.go`'s install-triggering half, for the same reason (see CHANGELOG SESSION-48
entry).

## SESSION-47 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `30-ai/35-files/00-guards.zsh` (`_ai_is_secret_file`, `_ai_is_binary_file`) | `fsguards.go` | `IsSecretFile`, `IsBinaryFile` |
| `10-core/25-quick_chat.zsh` (`_ai_ts`) | `fsguards.go` | `timestampSuffix`, `backupPath` |
| `05-tools/10-tool_fs_read.zsh` (`_ai_tool_read_file`) | `fsread.go` | `ReadFileTool` |
| `05-tools/10-tool_fs_read.zsh` (`_ai_tool_list_dir`) | `fsread.go` | `ListDirTool` |
| `05-tools/10-tool_fs_read.zsh` (`_ai_tool_count_lines`) | `fsread.go` | `CountLinesTool` |
| `05-tools/15-tool_search.zsh` (`_ai_tool_grep_search`) *(see CHANGELOG deviation note)* | `fsread.go` | `GrepSearchTool` |
| `05-tools/15-tool_search.zsh` (`_ai_tool_glob_search`) *(see CHANGELOG deviation note)* | `fsread.go` | `GlobSearchTool` |
| `05-tools/20-tool_fs_write.zsh` (`_ai_tool_write_file`) | `fswrite.go` | `WriteFileTool` |
| `05-tools/20-tool_fs_write.zsh` (`_ai_tool_edit_file`) | `fswrite.go` | `EditFileTool` |
| `05-tools/20-tool_fs_write.zsh` (`_ai_tool_move_file`) | `fswrite.go` | `MoveFileTool` |
| `05-tools/25-tool_fs_patch_delete.zsh` (`_ai_tool_patch_file`) | `fspatch.go` | `PatchFileTool` |
| `05-tools/25-tool_fs_patch_delete.zsh` (`_ai_tool_delete_file`) | `fspatch.go` | `DeleteFileTool` |
| *(none — new, shared helper, see CHANGELOG)* | `fswrite.go` | `copyFile`, `writeAtomic` |
| *(none — new, shared helper, see CHANGELOG)* | `fsread.go` | `firstNLines`, `mustObject`, `stringField`, `numberFieldAsString`, `parsePositiveInt` |

## SESSION-45 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `56-sse_line_parser.zsh` (`_ai_sse_process_line`) | `sse.go` | `sseLine`, `sseDeltaWire`, `parseSSELine` |
| `55-request_streaming.zsh` (`_ai_chat_request_stream`) | `streaming.go` | `CallStreaming`, `streamBody`, `maxSSELineBytes` |
| *(new — see CHANGELOG)* | `streaming.go` | `Event` |

## SESSION-44 file-level detail

| zsh source | Go file | Go symbols |
|---|---|---|
| `43-payload_builder.zsh` (`_ai_build_chat_payload`) | `payload.go` | `Message`, `PayloadOptions`, `BuildPayload` |
| `41-provider_candidate.zsh` (`_ai_provider_has_fallback`) | `candidate.go` | `HasFallback` |
| *(new — see CHANGELOG)* | `candidate.go` | `Candidate`, `ErrNoProviderAvailable`, `SelectProviderCandidate` |
| `48-http_call_blocking.zsh` (`_ai_http_call_blocking`) | `blocking.go` | `ErrCancelled`, `CallBlocking`, `resolveTimeout` |
| `30-ai/scripts/ai_extract.py` (`extract`, `strip_leaked_trace`) | `blocking.go` | `Response`, `Usage`, `ParseResponse`, `stripLeakedTrace` |

## File-level notes

- Not ported: `.zsh_bagas/10-plugins/`, `.zsh_bagas/20-shell/` — interactive-shell config
  (zinit, aliases, prompt), explicitly out of scope per `RENCANA_MIGRASI_GO_RUST.md` §2
  ("murni konfigurasi shell interaktif — tidak perlu diporting").
- Not ported (SESSION-42): `_ai_perm_ask`'s real gum/read/`/dev/tty` terminal confirmation
  UI — `internal/permission` only defines the `AskFunc` hook it's called through; the real
  implementation is SESSION-52/53's job. Also not ported: `_ai_perm_load_project`'s
  `.aiagent/permissions.zsh` project-local override (sourcing arbitrary shell code that can
  overwrite the guardrail functions has no safe Go equivalent — see `context.go`'s
  `PermConfig` doc comment for the reasoning).
- Not ported (SESSION-43): `_ai_autodep_run_install`/`_ai_autodep_install_missing` (the
  parts of `02-tool_autodep.zsh` that actually shell out to `pkg install`/`apt-get
  install`/`pip3 install`) — only meaningful as a retry hook on a running shell/process
  tool's exit-127 handling, and that tool doesn't exist in Go until SESSION-47/48. The pure
  detection/mapping functions (`DetectPackageManager`, `CmdToPackage`, `ExtractMissingCmd`)
  are ported and ready for those sessions to wire up.
- Not ported (SESSION-44): the provider/model fallback orchestrator loop in
  `50-request_blocking.zsh` itself (the `for provider in ...` / `for model in
  model_list` loops, spinner/TRAPINT/TRAPTERM handling, `AI_CURRENT_PROVIDER`/
  `AI_CURRENT_MODEL` state) — that file isn't assigned to any session's
  `source_zsh_files` anywhere in `docs/execution_sessions/`; it becomes
  agent-loop wiring in SESSION-49/50 once `internal/llmclient`'s building
  blocks (this session + 45 + 46) all exist. `SelectProviderCandidate` in
  `candidate.go` is a new function synthesized this session (not a 1:1 port)
  covering only the provider-selection fragment of that loop — see its own
  doc comment and the CHANGELOG SESSION-44 entry for why.
- Not ported (SESSION-45): `_ai_chat_request_stream`'s own `for provider in
  ...` / `for model in model_list` / `while tries < max` retry loop, circuit
  breaker checks (`_ai_breaker_is_open`/`_ai_breaker_record_fail`), and
  `_ai_chat_retry_decision` call — all SESSION-46 (`port_llm_resilience_layer`)
  scope per this session's own `scope.exclude`. Also not ported: printing the
  model label / streamed deltas to a terminal (`printf "%s > "`, `printf '%s'
  "$content"`) — that's the chat UI, SESSION-52+. `CallStreaming` in
  `streaming.go` is the single-HTTP-attempt building block SESSION-46 will
  wrap in that loop, the same way SESSION-44's `CallBlocking` already is.
- Not ported (SESSION-44): reasoning-effort *resolution* (`_ai_is_reasoning_model`/
  `_ai_reasoning_effort_for`, `30-ai/10-core/42-token_budget.zsh`) — `BuildPayload`
  takes an already-resolved `ReasoningEffort` string; deciding *what* that
  value should be for a given provider/model is SESSION-46 (token budget layer).
- Not ported (SESSION-46): the multi-provider/multi-model fallback orchestrator loop itself (`for
  provider in ... { for model in model_list { while tries < max ... } }`, spinner, `AI_CURRENT_PROVIDER`/
  `AI_CURRENT_MODEL` state) in `50-request_blocking.zsh`/`55-request_streaming.zsh` — still SESSION-49/50
  scope, unchanged from SESSION-44/45's own handoff notes. `CallWithRetry` in `resilience.go` is the
  single-model retry primitive that loop will call in an outer per-model iteration, the same
  relationship `CallBlocking`/`CallStreaming` already have to it. Also not ported: file-backed
  persistence of circuit-breaker state across separate process invocations (`AI_CIRCUIT_BREAKER_FILE`)
  — `CircuitBreaker`/`BreakerStore` are in-memory only; see CHANGELOG SESSION-46 entry for the reasoning.
- Not ported (SESSION-47): the `aiindex` (`46-index.zsh`) JSON-index fast-path lookaside
  `_ai_tool_grep_search`/`_ai_tool_glob_search` try before falling back to `rg`/`fd`/`find` —
  `46-index.zsh` is not assigned to any session's `source_zsh_files` anywhere in
  `docs/execution_sessions/`, so there's no Go-side index reader to call into yet. Both
  `GrepSearchTool`/`GlobSearchTool` always take the fallback path the zsh source itself falls
  back to whenever the index is stale/missing/unparseable — functionally complete, just without
  that read optimization. `15-tool_search.zsh` (where these two functions actually live) is also
  not listed in SESSION-47's own `source_zsh_files` even though its `objective`/`scope.include`
  name both tools explicitly — see the CHANGELOG SESSION-47 entry's "Deliberate deviation" note
  for the reasoning behind porting them in this session anyway.
- Not ported (SESSION-48): `_ai_yolo_shell_safe`'s fast-path allowlist tokenizer and the exit-127
  autodep auto-install retry — see the SESSION-48 file-level detail section above and the
  CHANGELOG SESSION-48 entry for the reasoning behind both.
- Checkpoint convention: each session's checkpoint zip is named
  `agent-after-SESSION-<N>.zip` and contains the entire repository (both
  `source/.zsh_bagas/` unmodified and the in-progress `go/` tree), per each
  session YAML's `checkpoint:` block.
