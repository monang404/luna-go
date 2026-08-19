# DISCOVERED_DURING_SESSION — SESSION-39

**Filed by:** SESSION-39 ("airun/aipatch edge-case and side-effect verification"), subtask 03
("For any FAIL result, file a new DISCOVERED_DURING_SESSION follow-up backlog note — do not
silently alter MASTER_BACKLOG.md").

**Status of this note:** this is a follow-up, not an edit to `MASTER_BACKLOG.md`. VERIFY-017 and
VERIFY-018 remain marked exactly as they were in `MASTER_BACKLOG.md`/`TRACEABILITY_MATRIX.md`
before this session — this note records the two FAIL results discovered while executing their
test procedures and proposes them as new low-severity items for triage.

Evidence for both was produced live (harness: `verify39/run_all.zsh` in this checkpoint),
against the real, unmodified shipped functions, 3/3 stable reruns. Full detail in the
SESSION-39 CHANGELOG entry; summary below.

## Finding 1 — VERIFY-017: aipatch permanently rejects legitimate 0-byte files (Low)

`_ai_is_binary_file` (`35-files/00-guards.zsh:44-59`) delegates classification to
`file --mime-encoding -b`. Reproduced directly: `file --mime-encoding -b` on any 0-byte input
reports `binary` (an artifact of the `file` utility's own empty-input handling, not a bug in
this codebase's wrapper). Consequently `aipatch` on any 0-byte file always prints
`"[$file] kelihatan file biner (bukan teks) -- aipatch cuma buat file teks/kode. Ditolak."`
and returns 1 — before ever reaching the AI call.

**Why this is worth flagging despite being a "clean, defined" rejection:** the binary-file guard
in `aipatch()` (`35-files/10-aipatch.zsh:41-44`) is checked unconditionally, with no `--force`
bypass — unlike the two guards immediately after it (secret-file, file-size), which both
explicitly check `[ "$force" -ne 1 ]`. An empty file is trivially valid text (the empty string),
and there is no way — not even `aipatch --force` — to ever aipatch a 0-byte file. This is a
real, reproducible gap for a legitimate use case (e.g. a freshly-`touch`'d placeholder file, an
empty `__init__.py`), not a crash or data-corruption risk.

**Recommended fix (not implemented — verification-only session):** special-case size-0 files to
skip the binary check (an empty file cannot contain binary content by definition), or extend the
`--force` bypass to also cover the binary-file guard. Small, local, single-function change.

## Finding 2 — VERIFY-018: airun has no upfront `.py`-extension validation (Low)

`airun()` (`30-code/50-run.zsh:22-90`) only checks `[ -z "$1" ]` (missing argument) before
calling `python3 "$file"` unconditionally — confirmed by source read, no `case`/`[[ ... == *.py
]]` extension check exists anywhere in the function, despite the `Usage: airun <file.py>` text
implying one. Reproduced live: `airun somefile.txt` (containing plain prose, not Python) runs
`python3` on it, which raises a real `SyntaxError`; that raw interpreter traceback is exactly
what `airun` captures as `$output` and — once its 2-try auto-fix retry loop is exhausted (both
proposed "fixes" accepted, script still fails) — echoes verbatim to the user via the post-loop
fallback added in SESSION-17/CLI-002 (`50-run.zsh:88`, `[ -n "$output" ] && echo "$output"`).
This confirms `AC-VERIFY-018`'s FAIL condition exactly: **"raw Python interpreter error surfaces"**
rather than a clean CLI-level rejection before `python3` is ever invoked.

Secondary observation from the same test run (not a separate backlog item, just context): if the
user *declines* the first proposed fix instead of accepting it, `airun` returns immediately
(`_ai_fix_apply` failure triggers a direct `return 1` in the loop body) *without* ever reaching
the fallback that would print `$output` — so in that path the raw error is silently swallowed
entirely rather than shown. Either way (accept-through-exhaustion or decline-early), the user
never gets a clean, immediate "this isn't a Python file" message.

**Recommended fix (not implemented — verification-only session):** add an early
`case "$file" in *.py) ;; *) echo "airun cuma buat file .py: $file"; return 1 ;; esac`-style
check (matching the pattern the `Usage` text already promises) right after the existing
empty-argument check in `airun()`, before the retry loop's first `python3 "$file"` call. Small,
local, single-function change — same shape/scope as Finding 1 above.

## Evidence preserved

The harness (`verify39/run_all.zsh`, `verify39/README.md`) is preserved in this checkpoint at
the repo root, alongside `verify37/`/`verify38/`, so a future fix-and-reverify session can rerun
it directly against patched `10-aipatch.zsh`/`50-run.zsh` without rebuilding the test.
