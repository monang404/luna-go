# COMMAND BEHAVIOR & CLI CONTRACT AUDIT — zsh_bagas

Repository: `zsh_bagas-main` (baseline.zip) · ~11.5k lines across 150 `.zsh`/`.py` files
Scope: `ai` dispatcher + 26 public `ai*` commands + 10 plain shell functions + aliases.
Legend: 🟢 Fakta (verified in code) · 🟡 Perlu Verifikasi (cannot be confirmed by static read) · 🔵 Rekomendasi

> **Coverage note (read first):** This pass gives full Contract/Consistency/Invariant coverage of the **public command surface** (dispatcher, all `ai*` entry points, aliases, completion, help). It does *not* re-derive a Contract Sheet for every one of the ~150 internal `_ai_*` helper files (tool layer, permission layer, agent execution internals) — those were read selectively, only where needed to verify a claim about a public command. That limitation is called out explicitly in the Evidence Coverage table (Phase 21) rather than papered over with unverified rows.

---

## 1. Executive Summary

The CLI surface is a single dispatcher (`ai <subcommand>`) plus a matching set of standalone functions (`aic`, `aipatch`, `aifix`, …) that can also be called directly, bypassing the dispatcher entirely. The dispatcher's subcommand list, the tab-completion, and the built-in help text are **provably synchronized** — all three are generated from (or checked against) the same `_AI_SUBCOMMANDS` array 🟢, and the shipped documentation (`CARA-PAKAI.md §15`) lists the identical 34 subcommands 🟢. That is a real strength: the most common source of CLI drift (help/completion/impl disagreeing) is structurally prevented here.

The main risk is not "missing command," it's **inconsistent destructive-action contracts between commands that look interchangeable**, and **dual entry points** (dispatcher word vs. bare function name) that don't get the same input handling. The two concrete, code-confirmed problems worth prioritizing:

1. **`airun` auto-applies `aifix`'s output without confirmation**, while every other file-mutating command in the same "file editing" group (`aipatch`, `aicode -o`) requires an explicit `gum confirm` / `read` prompt before overwriting. `aifix` alone is safe (writes to `.fixed`, never touches the original); `airun` removes that safety net silently. 🟢
2. **`airun`'s final fallback re-executes the target script a third time** (`python3 "$file"` after the 2-try loop exits) purely to print output — for a script with side effects (DB writes, network calls) this means up to 3 real executions from one `airun` invocation. 🟢

Beyond those two, the codebase shows unusually high self-awareness: many files contain inline `# v-fix (bug #NN audit)` comments documenting *prior* audit findings and their fixes (e.g., exit-code capture ordering in `aifix`, `mv` alias bypass in `aipatch`, timeout-guarded `read` prompts). This means several classic CLI bug classes (Ctrl+C hangs on non-TTY, silent no-op on dispatcher drift, alias interference on `mv`) have already been remediated and are now *invariants held*, not findings — noted as such below so this report doesn't re-flag already-fixed issues as new ones.

Overall scores are in §"Scoring."

---

## 2. Command Surface Mapping (Phase 1)

| Command | Kategori | File | Entry Point |
|---|---|---|---|
| `ai` (dispatcher) | Utility | `60-ui/40-dispatcher.zsh` | function `ai()` |
| `aic`, `aicl` | Chat | `20-chat/00-quick_chat.zsh` | function |
| `aish` | Chat | `20-chat/00-quick_chat.zsh` | function |
| `aiask` | Chat | `20-chat/05-aiask.zsh` | function |
| `aiclip` | Chat | `20-chat/25-aiclip.zsh` | function |
| `_ai_session` (via `session_mgmt.zsh`) | Chat/Session | `20-chat/20-session_mgmt.zsh` | function (dispatcher-only word `session`, no bare alias) |
| `aicode` | Code | `30-code/05-code.zsh` | function |
| `aiproject` | Code | `30-code/30-project.zsh` | function |
| `aifix` | Code | `30-code/45-fix.zsh` | function |
| `airun` | Code | `30-code/50-run.zsh` | function |
| `aiscrap` | Code | `30-code/00-scrap.zsh` | function |
| `aicat` | Files | `35-files/05-aicat.zsh` | function |
| `aipatch` | Files | `35-files/10-aipatch.zsh` | function |
| `aiundo` | Files | `35-files/15-aiundo.zsh` | function |
| `aibakclean` | Files | `35-files/20-aibakclean.zsh` | function |
| `aishare` | Files | `35-files/25-aishare.zsh` | function |
| `aicommit` | Workflow | `40-workflow/00-aicommit.zsh` | function |
| `aiplan`, `aiprompt`, `aispec`, `aibuild` | Workflow | `40-workflow/0*.zsh` | function |
| `aireview` | Workflow | `40-workflow/25-aireview.zsh` | function |
| `aisummarize` | Workflow | `40-workflow/30-aisummarize.zsh` | function |
| `aiscan` | Project | `45-project.zsh` | function |
| `aiindex` | Project | `46-index.zsh` | function |
| `aiagent` | Agent | `50-agent/40-runtime/30-aiagent.zsh` | function |
| `aidebug` | Agent | `55-subagent/40-debug.zsh` | function |
| `aih` | Utility | `60-ui/10-help_stats.zsh` | function (history search via `fzf`) |
| `aistats` | Utility | `60-ui/10-help_stats.zsh` | function |
| `ai_check_deps`, `ai_testmodels` | Utility | `60-ui/15-diagnostics.zsh` | function (dispatcher words `deps`, `testmodels`; **no bare `ai*`-prefixed alias**) |
| `airesearch`, `aidev` | Utility | `60-ui/25-research_dev.zsh` | function (dispatcher words `research`, `dev`) |
| `_ai_workspace`, `_ai_menu`, `_ai_help` | Utility | `60-ui/20-menu.zsh`, `10-help_stats.zsh` | internal, reached via `ai` / `ai menu` / `ai h` |
| `_ai_update_confirm_pull` | Utility | `60-ui/35-update_confirm.zsh` | dispatcher word `update` only |
| Shell functions: `mkcd`, `extract`, `bak`, `vf`, `gacp`, `ports`, `y`, `copy`, `proj`, `tm` | Shell | `20-shell/functions.zsh` | function, not under `ai` |
| Aliases: `ls`, `ll`, `la`, `..`, `...`, `c`, `v`, `rg`, `ff`, `gs`, `ga`, `gc`, `gp`, `gl`, `update`, `up`, `sshkey`, `pc` | Shell | `20-shell/aliases.zsh` | alias |

**No public command found without a file/entry point.** One naming note, not a bug: the dispatcher word `update` (→ `_ai_update_confirm_pull`, git-pulls the toolkit) and the plain shell **alias** `update` (→ `pkg update && pkg upgrade -y`, Termux OS packages) are two different commands with the same word, disambiguated only by prefix (`ai update` vs. bare `update`). 🟢 Flagged in Contradiction Hunt below.

---

## 3. Contract Sheets (Phase 2) — representative set

Full Contract Sheets for all 26 `ai*` commands would be exhaustive; the table below gives the sheet for the six commands that matter most for the Behavior Consistency Audit (Phase 6 target group), verified directly against source.

### `aipatch` — `35-files/10-aipatch.zsh`
| Properti | Nilai |
|---|---|
| Input | `[--force|--force-secret] <file> <instruksi...>` |
| Output | diff to stdout (colorized), status text |
| Exit Code | 1 on usage/missing-file/binary/secret/oversize/empty-reply/apply-fail; 0 on success or no-op |
| Confirm | ✅ `gum confirm` or `read -t 60` (default = cancel on timeout) |
| Backup | ✅ `$file.bak.$(timestamp)` before overwrite |
| Rollback | via separate `aiundo` command (not built-in) |
| Retry | ❌ none |
| Dry Run | not a flag, but diff-before-apply acts as implicit preview |
| Idempotent | Sebagian — re-running with same instruction re-queries the LLM; no-op only if model returns identical content |
| Side Effect | writes file, writes `.bak.*`, appends to log |

### `aicode -o <output>` — `30-code/05-code.zsh`
| Properti | Nilai |
|---|---|
| Confirm | ✅ (same pattern as `aipatch`, comment in source explicitly says it mirrors `aipatch`) |
| Backup | ✅ `.bak.$(timestamp)` |
| Rollback | via `aiundo` |
| Side Effect | writes file only when `-o` given; otherwise prints to stdout, no side effect |

### `aifix` — `30-code/45-fix.zsh`
| Properti | Nilai |
|---|---|
| Confirm | ❌ none needed — never touches the original file |
| Backup | ❌ none needed |
| Output artifact | `<file>.fixed` only |
| Side Effect | none on the original file; safe by construction |

### `airun` — `30-code/50-run.zsh`
| Properti | Nilai |
|---|---|
| Input | `<file.py>` |
| Confirm | ❌ **none** — calls `aifix`, then unconditionally `mv -f "${file}.fixed" "$file"` |
| Backup | ✅ `.bak.$(timestamp)` taken right before the overwrite |
| Retry | ✅ bounded loop, max 2 fix attempts |
| Side Effect | executes the target script up to 3 times (2 in-loop + 1 post-loop fallback); overwrites file up to 2 times without asking |
| Idempotent | Berbahaya — re-running on a script with side effects re-triggers those effects |

### `aiundo` — `35-files/15-aiundo.zsh`
| Properti | Nilai |
|---|---|
| Confirm | ✅ `gum confirm` / `read -t 30` |
| Rollback-of-rollback | ✅ takes a safety backup `.bak.<ts>.before_undo` before restoring, so an accidental undo is itself undoable |
| Source selection | always the **latest** `.bak.*` by mtime — no way to pick an older backup from the CLI itself |

### `aibakclean` — `35-files/20-aibakclean.zsh`
| Properti | Nilai |
|---|---|
| Input | `[days]`, default via `zsh` glob qualifier |
| Dry Run | ✅ always lists files before asking |
| Confirm | ✅ `gum confirm` / `read -t 30` |
| Side Effect | deletes `.bak.*` and old cache files older than N days |

---

## 4. Input / Output / Exit-Code Audit (Phases 3–5)

- **Positional-vs-flag consistency:** `--force` / `--force-secret` on `aipatch` are explicitly kept as aliases of each other in code, with a comment noting this was a deliberate generalization to avoid a new flag per new guard. 🟢 Good practice, not a finding.
- **Empty/missing input:** every reviewed command (`aipatch`, `aifix`, `aicode`, `aicommit`, `aiplan`, `aispec`, `aibuild`, `aiprompt`) does an explicit `[ -z ... ]` / `[ $# -lt N ]` check before doing any work, returning 1 with a `Usage:` string. 🟢 Consistent pattern.
- **stdout/stderr discipline:** `aifix`'s hard-failure path (`GAGAL bikin perbaikan...`) is explicitly redirected to `>&2` 🟢 with a comment noting this was intentional. Most other commands' error/usage strings were **not confirmed to use `>&2`** (plain `echo` in `aipatch`, `aicommit`, `aiundo`, `aibakclean`) — 🟡 Perlu Verifikasi whether this is an intentional convention (only hard API failures go to stderr) or an oversight; worth a decision either way so it's consistent across the 26 commands rather than file-by-file.
- **Exit code 130 (Ctrl+C):** verified centralized — the low-level HTTP layer (`10-core/48-http_call_blocking.zsh`, `50-request_blocking.zsh`) explicitly `return 130` on SIGINT and every caller up the chain (`session_repl.zsh`) checks for `130` specifically to distinguish "user cancelled" from "real error." 🟢 This is the correct pattern and it's used consistently for the *API-call* phase.
- **Gap:** the `read -t N "confirm?..."` prompts used in `aipatch`/`aicode`/`aiundo`/`aibakclean`/`aicommit` are a *different* interrupt surface (confirmation phase, not API-call phase). No trap was found guarding that phase specifically. 🟡 Perlu Verifikasi (would need a live TTY test) whether Ctrl+C during a confirm prompt leaves a stray `mktemp` temp file (e.g. `aipatch`'s `$tmpnew`) on disk. Flagged in Interrupt Audit below as a recommendation, not a confirmed bug, since it can't be proven by static read alone.

---

## 5. Behavior Consistency Audit (Phase 6) — required comparison groups

| Kelompok | Confirm before mutating? | Backup? | Rollback path | Notes / Contradiction |
|---|---|---|---|---|
| `aifix` | N/A (writes `.fixed`, not original) | N/A | N/A | Safe by construction |
| `aipatch` | ✅ always | ✅ always | `aiundo` | Baseline pattern |
| `aicode -o` | ✅ always | ✅ always | `aiundo` | Matches `aipatch` (comment in source confirms this was intentional mirroring) |
| **`airun`** | **❌ never** | ✅ | `aiundo` (backup exists, but nothing tells the user it does) | **🟢 Contradiction:** `airun` calls `aifix` (safe) but then applies the result the way `aipatch`/`aicode` do (destructive) *without* the confirm step those two always use. A user who trusts "aifix is a preview step" is not protected when it's invoked through `airun`. |
| `aiplan` / `aispec` / `aibuild` / `aiprompt` (Workflow group) | N/A — read-only generation to stdout | N/A | N/A | Consistent with each other; none mutate files directly |
| `aicommit` (Git group) | ✅ | N/A (git itself is the audit trail) | `git reset` (external, not wrapped) | Consistent internally |
| `aireview` (Git group) | N/A — read-only | N/A | N/A | Consistent (declared read-only in `_ai_help` text and confirmed by code: no write tool calls) |
| Session group (`session`, `--resume`, checkpoint) | Checkpoint save/load doesn't prompt for confirm (by design — it's non-destructive state, not file mutation) | N/A | `--resume <slug>` | No contradiction found vs. the file-editing group, since session state and filesystem mutation are different risk classes |

**Bottom line for Phase 6:** one real cross-command contract contradiction (`airun`), confirmed by direct code comparison, not by pattern-matching or assumption.

---

## 6. Idempotency & Side Effects (Phases 7–8)

| Command | Idempotent | Risiko | Side Effect surface |
|---|---|---|---|
| `aipatch` | Sebagian (re-run re-queries LLM; converges once diff is empty, code explicitly checks `diff -q` and no-ops) | Low | File write, `.bak.*`, log |
| `aicode -o` | Sebagian, same pattern as `aipatch` | Low | File write, `.bak.*`, log |
| `aifix` | Aman (only ever writes `.fixed`) | None | `.fixed` file only |
| `airun` | **Berbahaya** | High for any script with I/O/network/DB effects — confirmed re-execution up to 3x per invocation | File write (no confirm), `.bak.*`, script side effects |
| `aiundo` | Aman (extra `.before_undo` safety backup taken first) | Low | File overwrite, extra backup |
| `aibakclean` | Aman (dry-run list, then confirm) | Low | Deletes old `.bak.*`/cache |
| `aicommit` | Aman — re-run just re-generates a message; the actual `git commit` is still gated by its own confirm | Low | `git commit` |
| Read-only group (`aiplan`, `aispec`, `aibuild`, `aiprompt`, `aireview`, `aicat`, `aiscan`) | Aman | None | stdout / log only |

---

## 7. Retry, Recovery, Rollback & Interrupt Audit (Phases 9–10)

- **Retry:** confirmed bounded in two places — `airun`'s fix loop (`tries < 2`) 🟢 and the HTTP/provider layer's circuit breaker (`10-core/40-circuit_breaker.zsh`, `44-retry_decision.zsh`) 🟢. No unbounded retry loop found in the reviewed files.
- **Rollback reachability:** every command that takes a backup (`aipatch`, `aicode -o`, `airun`) is reachable via the same generic `aiundo <file>` — 🟢 satisfies the Invariant Audit's "Rollback → Reachable" rule, including for `airun` even though `airun` itself never mentions `aiundo` to the user (a documentation gap, not a functional one — see Recommendations).
- **Interrupt safety:** SIGINT/SIGTERM handling is explicit and cooperative in the agent execution path (`50-agent/40-runtime/25-execute_and_finalize.zsh` sets a `cancelled` flag file via `trap ... INT TERM`, checked by `42-execution/00-loop_main.zsh` and `15-run_tool.zsh` between steps) 🟢. This is a genuinely good pattern for a long-running multi-step agent loop — it won't kill mid-tool-call, it finishes the current tool then stops cleanly.
- **Gap already noted above:** confirm-prompt-phase interrupts (`aipatch` et al.) are not covered by the same trap mechanism. 🟡 Perlu Verifikasi / 🔵 Recommendation: either reuse the same trap pattern around the confirm block, or explicitly document that a leftover `mktemp` temp file is expected/harmless (it lives in `$TMPDIR`, not the project tree, so impact is low — but should be a stated decision, not silent).

---

## 8. Command Composition (Phase 12)

Confirmed real compositions by reading the actual call chains (not assumed from naming):

- `ai run` → `airun` → **`aifix`** (internal call, not via dispatcher) → writes `.fixed` → `airun` applies it. **Format is compatible** (both operate on the same file path convention) but the confirm-contract breaks here as documented in §5.
- `ai fix` (standalone `aifix`) → produces `<file>.fixed` → user is expected to manually `diff` and apply. **No automatic hand-off command** exists from bare `aifix` to `aipatch`/apply — the user has to `mv` it themselves or re-run `aipatch` with the diff as instructions. 🟡 Recommendation candidate: `aifix` output could feed `aipatch`'s apply path (confirm+backup) for a safer manual flow, instead of leaving a raw `mv` as the only next step.
- `ai patch` → `aiundo` is the documented undo path (`aipatch`'s success message literally prints "undo cepat: aiundo ...")  🟢 — good, self-documenting composition.
- `aiagent` → internally can call the same tool layer as `aipatch`/`aifix` (`05-tools/20-tool_fs_write.zsh`, `25-tool_fs_patch_delete.zsh`) but through the **permission layer** (`06-permissions/*`), which is a materially different contract (agent asks once per session under `--yolo`, or per-action otherwise) than the interactive commands' per-call confirm. This is expected/by-design (agent mode vs. interactive mode are different UX contracts, documented as such in `_ai_help`), not flagged as a contradiction.

---

## 9. Pipeline & Completion Compatibility (Phases 13–14)

- **Completion vs. dispatcher:** `_AI_SUBCOMMANDS` array is the single source for both the `case` statement in `ai()` and `_ai_complete()`'s `_describe`. 🟢 No possible drift — they're the same variable, not two copies.
- **Help vs. dispatcher:** `_ai_help()` prints `${_AI_SUBCOMMANDS[*]}` directly rather than a hand-maintained list. 🟢 Same guarantee.
- **Documentation vs. implementation:** `CARA-PAKAI.md §15` lists all 34 words matching `_AI_SUBCOMMANDS` exactly (`chat long code edit view scan fix run build project scrap ask shell commit review debug research plan prompt spec summarize clip session agent stats log menu deps dev testmodels undo bakclean share index update h`). 🟢 Verified by direct comparison, not assumed.
- **Dispatcher self-defense already implemented:** the `*)` fallback case in `ai()` explicitly handles the scenario where `_AI_SUBCOMMANDS` and the `case` block drift apart (a word present in the array but missing its `case` branch) by printing an explicit internal-bug message instead of silently no-op'ing — this is documented in-code as a fix for a prior audit finding (bug #28). 🟢 Confirmed still in place; not re-flagged as a new issue.
- **Typo handling:** unrecognized subcommands go through a Levenshtein-distance suggestion (`ai comit` → "did you mean commit?") before falling back to treating the input as a chat message — also documented in-code as a fix for a prior finding (bug #29, to prevent silent accidental API calls on typos). 🟢 Confirmed in place.
- **Pipe/redirect:** not exhaustively tested per-command (would require a live shell), but structurally, commands that print machine-relevant output (`aicat`, `aiplan`, `aispec`) write straight to stdout with no interactive prompt in their default path, so they compose with `|`/`>` fine. Commands with mandatory interactive confirm (`aipatch`, `aiundo`, `aibakclean`, `aicommit`) **will hang or auto-cancel** in a non-interactive pipeline — this is by design given the `read -t N` timeout defaults to "cancel," which is the safe failure mode, but it does mean these commands are **not pipeline-composable** by design. 🟢 (confirmed from the timeout-cancel code) / not a bug, but worth stating explicitly since Fase 13 asks for it.

---

## 10. Contradiction Hunt (Phase 19) — required minimum 10

1. **`airun` vs `aipatch`/`aicode` confirm contract** (detailed in §5). Severity: High.
2. **`airun`'s post-loop fallback re-executes the target script a third time**, contradicting the in-code comment on the same file explaining that the *in-loop* double-execution bug was already fixed to prevent exactly this class of problem. Severity: Medium (the loop itself is fixed; the trailing fallback wasn't covered by that fix).
3. **Two different commands named `update`**: `ai update` (git-pulls the toolkit itself) vs. bare `update` alias (`pkg update && pkg upgrade -y`, OS packages). Not a functional bug — different namespaces (`ai <word>` vs. shell alias) — but a real naming collision a user could type expecting the wrong effect. Severity: Low/Medium (user-confusion risk, not a code defect).
4. **`aih` naming overload**, self-documented in-code: `aih()` means "search AI conversation history via fzf," which is easy to expect to mean "ai help" given `ai h` exists as a *different* subcommand for actual help text. The code comment explicitly acknowledges this is intentional and not a bug — included here per the audit's contradiction-hunt requirement, but downgraded to Low since it's a deliberate, documented decision, not drift.
5. **`aifix` is described (correctly) as non-mutating/preview-only in isolation**, but the CLI as a whole does not surface anywhere in `_ai_help` that calling `ai fix` directly is safe while the effectively-equivalent `ai run` on a failing script is not. Severity: Medium — documentation gap that could lead a user to assume both are equally safe.
6. **No bare `ai*`-prefixed function exists for `deps`/`testmodels`/`research`/`dev`/`update`** — they're reachable only via `ai <word>`, unlike almost every other subcommand which has both a dispatcher word and a same-behavior bare function (`aic`, `aipatch`, `aifix`, etc.). Not a bug (these are lower-frequency utility commands), but an inconsistency in the "two entry points per command" pattern the rest of the CLI follows. Severity: Low.
7. **stderr routing inconsistency** noted in §4 — `aifix`'s failure path explicitly writes to `stderr`, most sibling commands' failure paths write to stdout via plain `echo`. Severity: Low (cosmetic/pipe-composability), but real and code-confirmed.
8. **`aiundo` only ever offers the latest backup**, while `aibakclean` can delete backups older than N days — if a user's most recent edit is bad but they want to go back two edits, no CLI path supports that (must inspect `.bak.*` manually). Severity: Low — functionality gap, not a contradiction between two commands' stated contracts, but relevant to the Rollback invariant.
9. **`ai code` (dispatcher word `code`) maps to `aicode`**, while `ai edit` maps to `aipatch` — two commands with overlapping "modify a file via AI" purpose are exposed under semantically distinct dispatcher words (`code` vs `edit`) rather than being unified or clearly differentiated in `_ai_help`'s one-line summaries (which only describe `chat`/`code`/`agent`/`review`/`debug`/`research`, not `edit`/`fix`/`run` explicitly). Severity: Low/Medium — discoverability gap.
10. **Session group vs. file-editing group confirm philosophy differs by design** (checkpoints don't need user confirm; file overwrites do) — included per the audit's requirement to compare the Session group, and explicitly noted as **not** a contradiction once the different risk classes are accounted for. Included for completeness of the required comparison, downgraded to informational.

*(10 items provided as required; items 4 and 10 are included because the phase mandates the search but are explicitly downgraded — they reflect deliberate, documented design decisions rather than defects.)*

---

## 11. Dead Behavior Audit (Phase 20)

Checked specifically for the classic pattern (setter with no reader, flag with no effect) by cross-referencing every dispatcher word against its function definition (§2) and every function against at least one caller:

- No dispatcher word was found pointing at an undefined function — all 34 words in `_AI_SUBCOMMANDS` resolve to a real, defined function (verified directly, not assumed; see §2/§9). 🟢
- `--force-secret` on `aipatch`: confirmed still functional (kept as an alias of `--force`, not silently dropped) despite being effectively superseded by `--force`. 🟢 Not dead, just legacy-compatible.
- Did not have budget in this pass to trace every flag inside the agent/subagent/tool layer (`50-agent/*`, `55-subagent/*`, `05-tools/*`) for unused setters — flagged honestly as unaudited in Evidence Coverage rather than reported as "no dead code found" there. 🟡 Perlu Verifikasi.

---

## 12. Evidence Coverage Audit (Phase 21)

| Area | File Relevan | Sudah Dicek | Coverage |
|---|---|---|---|
| Dispatcher | `60-ui/40-dispatcher.zsh` | ✅ full read | High |
| Completion | `60-ui/45-completion.zsh` | ✅ full read | High |
| Help/Docs | `10-help_stats.zsh`, `CARA-PAKAI.md` | ✅ full read + diffed against `_AI_SUBCOMMANDS` | High |
| File-editing group (`aipatch`/`aifix`/`aicode`/`airun`) | `35-files/10-*.zsh`, `30-code/45-*.zsh`, `30-code/50-*.zsh`, `30-code/05-*.zsh` | ✅ full read | High |
| Backup/rollback group (`aiundo`/`aibakclean`) | `35-files/15-*.zsh`, `35-files/20-*.zsh` | ✅ full read | High |
| Workflow group (`aiplan`/`aispec`/`aibuild`/`aiprompt`/`aicommit`) | `40-workflow/*.zsh` | ✅ grep'd for confirm/backup/exit patterns, not full line-by-line read | Medium |
| Interrupt/exit-code plumbing | `10-core/48-*`, `10-core/50-*`, `50-agent/40-runtime/25-*`, `50-agent/42-execution/*` | ✅ targeted read | Medium-High |
| Permission layer | `06-permissions/*` | ❌ not read this pass | Low |
| Tool layer internals (`05-tools/*`) | 12 files | ❌ not read this pass, only referenced | Low |
| Agent state machine / checkpoint internals | `50-agent/10-state.zsh`, `39-agent-state-machine.zsh` | ❌ not read this pass | Low |
| Subagent/debug internals | `55-subagent/*` (9 files) | ❌ not read this pass | Low |
| Shell functions (non-AI) | `20-shell/functions.zsh`, `aliases.zsh` | ✅ full read | High |

**Honest statement per audit rules:** this pass does not meet the strict Definition-of-Done bar of "no command analyzed from only one file" for the Permission, Tool-internals, Agent-state, and Subagent areas — those were referenced but not independently traced caller-by-caller. If a full engineering-grade sign-off on those specific areas is needed, they warrant a dedicated follow-up pass rather than being asserted clean here.

---

## 13. Negative Behavior Matrix (Phase 22) — spot-checked, not exhaustive

| Skenario | Expected | Status |
|---|---|---|
| `aipatch` tanpa argumen | Usage message, exit 1 | 🟢 Confirmed (`[ $# -lt 2 ]` check) |
| `aipatch` pada file kosong | Diproses normal (content kosong dikirim ke AI) | 🟡 Not explicitly guarded — no empty-file check found separate from the missing-file check |
| `aipatch` pada file binary | Ditolak eksplisit | 🟢 Confirmed (`_ai_is_binary_file` guard) |
| `aipatch` pada file secret (.env dll) | Ditolak kecuali `--force` | 🟢 Confirmed (`_ai_is_secret_file` guard) |
| `aipatch` pada file > `AI_FILE_MAX_CHARS` | Ditolak kecuali `--force` | 🟢 Confirmed |
| `airun` pada file non-Python | Tidak divalidasi ekstensi — akan gagal di `python3` dengan pesan interpreter, bukan pesan CLI yang jelas | 🟡 Perlu Verifikasi — no `.py` extension check found in `airun` despite the `Usage: airun <file.py>` string implying one |
| `aiundo` tanpa backup tersedia | Pesan jelas, exit 1 | 🟢 Confirmed |
| Dependency (`gum`) tidak ada | Fallback ke `read -t N` | 🟢 Confirmed consistently across all 5 confirm-using commands checked |
| Dispatcher argumen tidak dikenal (typo) | Saran koreksi via Levenshtein | 🟢 Confirmed |
| `ai` tanpa argumen | Buka workspace/menu | 🟢 Confirmed (`_ai_workspace`) |

---

## 14. Validation Matrix (Phase 23) — items that need live-shell verification

| Skenario | Expected | Status |
|---|---|---|
| Ctrl+C tepat saat `read -t N` confirm prompt di `aipatch`/`aiundo`/`aibakclean` | Ideally: cleanup temp files, consistent exit 130 | 🟡 Perlu Verifikasi — no trap found scoped to this specific window |
| `airun` pada script Python dengan efek samping (network/DB), gagal 2x | Confirmed by code: side effect runs up to 3 times | 🟡 Behavior is code-confirmed, but real-world impact severity depends on the script — needs a live test with a representative script to size the actual risk |
| Terminal sempit / `NO_COLOR` | Not reviewed this pass | 🟡 Perlu Verifikasi |
| Resume `aiagent --resume` setelah cancel pertengahan | Cooperative cancel via trap+state-file, confirmed in code; live resume flow not traced end-to-end this pass | 🟡 Perlu Verifikasi |

---

## 15. Root Cause Consolidation (Phase 24)

| Gejala | Root Cause |
|---|---|
| `airun` skips confirm that its sibling `aipatch`/`aicode` always require | `airun` was built to internally reuse `aifix` + a raw `mv -f`, but did not reuse `aipatch`'s confirm+backup **apply** step — the two commands independently implement "apply AI output to a file" instead of sharing one apply function. |
| `airun`'s triple-execution risk | The 2-try retry loop is correctly bounded, but the trailing "show the user the final error" step was implemented as a third real execution instead of reusing the already-captured `$output` from the last loop iteration. |
| Naming collisions (`update` word vs alias; `aih` vs `ai h`) | No single naming registry across dispatcher words, bare functions, and plain shell aliases — each layer picked names independently over time. |
| Inconsistent stderr routing | No documented convention for which failure paths must use `>&2`; each command's error handling was written independently (evidenced by the fact that `aifix` alone has an explicit comment calling out its own stderr choice). |
| Confirm-phase interrupt handling gap | The interrupt-safety pattern (trap + cooperative check) was built specifically for the agent execution loop, and was not generalized into a shared helper for the simpler `read`-based confirm prompts used elsewhere. |

Five root causes account for essentially every non-cosmetic finding in this audit — the codebase's actual defect surface is narrow, not broad.

---

## 16. False Positive Challenge (Phase 25)

Applied to the two High/Medium findings before locking severity:

**Finding: `airun` skips confirm.**
- Checked caller: `airun` is only ever called directly by the user or via `ai run`; not called internally by `aiagent`'s tool layer (which has its own separate permission-gated write tool) — confirmed this isn't secretly double-gated elsewhere. Still valid.
- Checked fallback: no `AI_YOLO`/env-var override found that would explain the missing confirm as an intentional "fast path" flag — it's unconditional in the code, not env-gated. Still valid.
- Checked documentation: `CARA-PAKAI.md` does not mention `airun` skipping confirmation as a documented, intentional behavior. Still valid — not downgraded.
- **Verdict: kept as High**, since it's an unconditional, undocumented gap, not a deliberate documented fast path.

**Finding: `airun` triple-execution.**
- Checked whether the target scripts are expected to be side-effect-free (e.g., pure computation) by convention — no such convention documented anywhere in `CARA-PAKAI.md` or code comments; `airun` is generic "run and auto-fix any `.py` file." Still valid.
- Checked whether the fallback `python3 "$file"` might be intentionally re-running to get *fresh* output after the last fix attempt (i.e., arguably necessary, not a bug) — this is plausible as *intent*, but the loop already captures `$output`/`$exit_code` from the last iteration, so re-running is not actually necessary to show the final state; it could reuse the captured value instead.
- **Verdict: kept as Medium** (real, but lower severity than the confirm gap since it only matters for scripts with side effects, and only on the failure path after 2 already-failed fix attempts).

---

## 17. Self Review (Phase 26)

| Checklist item | Status |
|---|---|
| Semua command publik punya Contract Sheet | Partial — full sheets for the 6-command consistency-comparison set; abbreviated coverage table for the rest (§2) |
| Semua caller lintas-file ditelusuri | Partial — done for file-editing group and interrupt plumbing; not done for permission/tool-internals/subagent layers |
| Dispatcher diperiksa | ✅ |
| Completion diperiksa | ✅ |
| Help diperiksa | ✅ |
| Dokumentasi dibandingkan | ✅ (`CARA-PAKAI.md §15` vs `_AI_SUBCOMMANDS`, exact match confirmed) |
| Evidence Coverage penuh | ❌ — explicitly partial, documented in §12 |
| Minimal 10 kontradiksi dicari | ✅ (§10, with 2 explicitly downgraded to informational per findings) |
| Negative Behavior Matrix selesai | Partial (§13, spot-checked) |
| State Transition Audit selesai | ❌ not performed this pass (agent state machine internals unread) |
| Invariant Audit selesai | Partial — checked Delete→Confirm, Ctrl+C→130 (API layer), Help→Documented, Completion→Dispatcher, Rollback→Reachable; did not check Retry→Bounded exhaustively outside the two areas reviewed |
| Root Cause Consolidation selesai | ✅ (§15) |
| Semua temuan High/Critical melewati False Positive Challenge | ✅ (§16, both High/Medium findings) |
| Validation Matrix selesai | Partial — items listed, not live-tested (§14) |

**Per the audit's own Definition of Done, this report is not a fully closed audit** — Evidence Coverage, State Transition, and full Validation Matrix items remain open. It is presented as a complete, honestly-scoped first pass on the public command surface, with the specific gaps enumerated above rather than hidden.

---

## 18. Scoring

| Aspek | Skor | Alasan singkat |
|---|---|---|
| Contract Consistency | 7/10 | One real cross-command gap (`airun`), otherwise strong shared patterns |
| Exit Code Discipline | 7/10 | 130-on-cancel correctly centralized for API calls; stderr routing inconsistent |
| Input Validation | 8/10 | Consistent usage/empty/binary/secret/size guards across file-editing group |
| Output Predictability | 6/10 | Confirm-blocking commands are intentionally not pipe-composable (fine), but not clearly documented as such |
| Composition | 7/10 | `aipatch`↔`aiundo` is well composed and self-documenting; `aifix`→apply has no guided path |
| Documentation Accuracy | 9/10 | Dispatcher/completion/help/docs verified in exact agreement |
| CLI Professionalism | 7/10 | Evidence of prior audits being taken seriously and fixed (in-code bug-fix comments); remaining gaps are narrow and root-caused |

**Overall: ~7.3/10** — a CLI with real engineering discipline (self-documented fixes, centralized interrupt handling, doc/completion/help kept in lockstep) undermined by one clear, fixable contract inconsistency in the highest-risk command (`airun`).

---

## 19. Final Roadmap

| Dampak | Frekuensi | Effort | Prioritas | Item |
|---|---|---|---|---|
| High | Medium | Low | **Sprint 1** | Add the same confirm step `aipatch`/`aicode` use before `airun`'s `mv -f "${file}.fixed" "$file"` |
| Medium | Low | Low | **Sprint 1** | Reuse the last-loop-iteration `$output`/`$exit_code` in `airun`'s post-loop fallback instead of re-executing the script a third time |
| Low | Medium | Low | Sprint 2 | Standardize error-output routing (stdout vs stderr) across all 26 commands and document the convention |
| Medium | Low | Medium | Sprint 2 | Extend the SIGINT/SIGTERM cooperative-cancel pattern already used in the agent loop to the plain `read -t N` confirm prompts in `aipatch`/`aiundo`/`aibakclean`/`aicommit` |
| Low | Low | Low | Sprint 2 | Document in `CARA-PAKAI.md`/`_ai_help` that `aifix` alone is non-destructive but `airun` is not, and that confirm-gated commands are not pipeline-safe by design |
| Low | Low | High | Sprint 3 | Full Evidence-Coverage close-out: trace permission layer, tool-layer internals, agent state machine, and subagent internals to the same depth as the file-editing group (needed before this audit can claim the strict Definition-of-Done) |

---

*This report is scoped to what was directly verified in `baseline.zip`. Any table cell not backed by a quoted file/line-level check above is marked 🟡 rather than asserted as fact, per the audit's own evidence standard.*
