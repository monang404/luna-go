# DISCOVERED_DURING_SESSION — SESSION-38

**Filed by:** SESSION-38 ("Interrupt- and state-safety verification suite"), subtask 03
("For any FAIL result, file a new DISCOVERED_DURING_SESSION follow-up backlog note — do not
silently alter MASTER_BACKLOG.md").

**Status of this note:** this is a follow-up, not an edit to `MASTER_BACKLOG.md`. VERIFY-001
remains marked exactly as it was in `MASTER_BACKLOG.md`/`TRACEABILITY_MATRIX.md` before this
session — this note records the one FAIL result discovered while executing VERIFY-001's test
procedure, and proposes it as a new low-severity item for triage.

## What happened

VERIFY-001's test procedure (`scope.include`): "Trigger Ctrl+C exactly during `read -t N`
confirm prompt in aipatch/aiundo/aibakclean, inspect `$TMPDIR`." Per `AC-VERIFY-001`: PASS =
no leftover tmp file + exit 130; FAIL = stray temp file left on disk (Low severity if failed).

Live interrupt-timing tests were run against the real, unmodified shipped functions (harness:
`verify38/run_all.zsh` in this checkpoint), replicating each command's real call ordering:

| Command | Order of `mktemp` vs. confirm prompt (from source) | Result |
|---|---|---|
| **aipatch** (`35-files/10-aipatch.zsh:80-117`) | `tmpnew=$(mktemp)` (line 81) happens **before** `_ai_confirm` (line 117) | **FAIL** |
| aiundo (`35-files/15-aiundo.zsh`) | nothing created until *after* `_ai_confirm` returns 0 | PASS |
| aibakclean (`35-files/20-aibakclean.zsh`) | nothing created until *after* `_ai_confirm` returns 0 | PASS |

**aipatch FAIL, reproduced live, 3/3 runs stable:** sending `SIGINT` to a foreground `aipatch`
process while it is genuinely blocked inside `_ai_confirm`'s `read -t 60` (no `gum` in this
sandbox, so the `read` fallback path in `10-core/32-confirm.zsh` is exercised) kills the process
immediately (exit 130, matching the PASS half of `AC-VERIFY-001`) — but the `tmpnew` file
created at line 81, *before* the confirm prompt, is left behind on disk. Neither `aipatch()`
nor `_ai_confirm()` installs any `INT`/`TERM` trap, so there is no cleanup path for this window.

## Why aipatch differs from aiundo/aibakclean (root cause)

All three share the same `_ai_confirm` helper and the same confirm-then-act intent, but only
aipatch creates on-disk state (`tmpnew=$(mktemp)`, populated with the AI's proposed full-file
rewrite) *before* asking for confirmation — because it needs the temp file's content ready to
`diff` against the original for the confirm prompt's diff display (lines 96-107 print the diff
built from `$tmpnew` before the prompt is shown). aiundo/aibakclean have no equivalent need and
simply don't create anything until after the user says yes.

## Severity and scope

Per `AC-VERIFY-001`'s own acceptance criteria: **Low**. Impact is a stray, empty-of-secrets
(it's just the AI's proposed file rewrite, already a plaintext copy of content the user
explicitly sent to the AI) temp file left in `$TMPDIR` after an interrupted aipatch confirm —
no data loss, no corrupted state, `$file` itself is never touched at this point (confirmed by
this same test run: the original file was untouched in the separate VERIFY-019 window test).
Accumulation over repeated interrupted aipatch runs is the main downside (`$TMPDIR` clutter),
which is the same class of concern `aibakclean` already exists to clean up — the file would
simply need to survive until the next manual `aibakclean` run, or be cleaned by the OS's own
`$TMPDIR` reaping if any.

## Recommended fix (not implemented — verification-only session per `scope.exclude`)

Add a scoped `trap 'rm -f "$tmpnew" 2>/dev/null' INT TERM` immediately after `tmpnew=$(mktemp)`
in `aipatch()` (`35-files/10-aipatch.zsh:81`), cleared again after the confirm/apply logic
resolves (mirroring the existing `_ai_confirm` cancel-safety contract: "backup gak disentuh,
tmpnew dihapus, return 1" already documented in the comment at line 115, which currently only
covers the *explicit decline/timeout* paths, not a raw signal). This is a small, local,
single-function change — a natural fit for a future low-risk session in the same vein as
SESSION-33 ("low-risk security hardening: temp filenames").

## Evidence preserved

The harness (`verify38/run_all.zsh`, `verify38/README.md`) is preserved in this checkpoint at
the repo root, alongside `verify37/`, so a future fix-and-reverify session can rerun it directly
against a patched `10-aipatch.zsh` without rebuilding the test.
