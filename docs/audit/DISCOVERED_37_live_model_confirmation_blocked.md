# DISCOVERED_DURING_SESSION — SESSION-37

**Filed by:** SESSION-37 ("Live-provider verification: session-trim parity and README
prompt-injection"), subtask 03 ("For any FAIL result, file a new DISCOVERED_DURING_SESSION
follow-up backlog note — do not silently alter MASTER_BACKLOG.md").

**Status of this note:** this is a follow-up, not an edit to `MASTER_BACKLOG.md`. VERIFY-010
and VERIFY-011 remain open in `MASTER_BACKLOG.md`/`TRACEABILITY_MATRIX.md` exactly as before
this session — this note only records *why* and *what specifically* is still outstanding, so the
next attempt doesn't have to rediscover the constraint from scratch.

## What happened

SESSION-37 required, per its own `scope.include`:
- "Live API test with a real provider after a simulated 15+ turn `ail` session"
- "Adversarial README + live model test (requires authorized red-team scope)"

The execution environment available for this session has:
- Network egress restricted to a package-registry allowlist (`pypi.org`, `npmjs.org`,
  `github.com`/`codeload.github.com`/`raw.githubusercontent.com`, `crates.io`, Ubuntu archive
  mirrors, `api.anthropic.com`). **None of `zsh_bagas`'s actual providers (Groq, Gemini,
  Cerebras, DeepSeek) are reachable.**
- No API credentials of any kind provisioned for any provider, including `api.anthropic.com`
  (reachable at the network layer but no key available to authenticate a `/v1/messages` call).
- No installed instance of `zsh_bagas` itself with a real `ail` session against a live model to
  observe.

This is not a "we didn't have time" gap — it is a hard access constraint matching exactly what
the session's own `boundary_rationale` anticipated ("Both VERIFY items require live-provider/
authorized-red-team access").

## What was substituted, and what that does/doesn't prove

An automated harness was built and run against the **real, unmodified shipped functions**
(not reimplementations) to confirm the structural precondition each VERIFY item's semantic test
depends on. Full detail and results are in the SESSION-37 CHANGELOG entry; summary:

| Item | Structural half (tested, real code, this session) | Semantic half (needs live provider) |
|---|---|---|
| VERIFY-010 | PASS — role alternation holds across 16 turn-count/max-msgs/parity configurations, using the actual `_ai_trim_session` | **BLOCKED** — whether real model replies stay coherent/non-confused after these trims was not observed |
| VERIFY-011 | PASS — adversarial payload confirmed fenced/isolated and trust-hierarchy framing confirmed present, using the actual `aiscan` + `_ai_agent_build_sysprompt` output | **BLOCKED** — whether a real model, given this exact prompt, actually declines the injected instruction was not observed |

The structural PASS is meaningful evidence (it confirms the mechanism the fix relies on is
intact under real code, across more configurations than the session's own minimum), but it is
explicitly **not** a substitute for the semantic acceptance criteria as written in
`AC-VERIFY-010`/`AC-VERIFY-011` — those criteria are about observed model *behavior*, which
cannot be established without an actual model in the loop.

## Recommended next step

Re-run this session's `tests.targeted` (both items) in an environment with either:
1. Network access to at least one of `zsh_bagas`'s configured providers plus a valid API key
   for it, or
2. A working `zsh_bagas` install exercised through a real `ail` session end-to-end, or
3. Authorized red-team access to a live model for the adversarial-README half specifically
   (VERIFY-011's own wording already flags this as requiring "authorized red-team scope").

Until one of these runs and produces an observed-behavior PASS/FAIL, `AC-VERIFY-010` and
`AC-VERIFY-011` should continue to be treated as **open**, not closed, in `MASTER_BACKLOG.md` —
consistent with this note not having modified that file.

The harness itself (`test_role_parity.zsh`, `test_readme_fencing.zsh`, and the adversarial
README fixture) is preserved in this checkpoint under `verify37/` at the repo root so the next
attempt does not need to rebuild it from scratch — it only needs a live model to point it at.
