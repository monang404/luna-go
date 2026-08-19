# A04 — Agent Intelligence & Decision Architecture Audit (AIDA)
**Target:** `zsh_bagas-main/.zsh_bagas/30-ai` (zsh-bagas AI Hub)
**Scope:** Decision Architecture only — bagaimana agent berpikir, memilih model, memilih tool, memanggil skill, membangun context, checkpoint, subagent, retry, fallback, stop.
**Metodologi:** Setiap klaim ditelusuri Decision Source → Caller → Runtime → Effect, langsung dari implementasi (bukan komentar/README).

---

## 1. Executive Summary

Sistem ini adalah ReAct-loop agent (`aiagent`) dengan lapisan tool registry + permission gate + retry/fallback provider yang **jauh lebih matang dari kebanyakan proyek serupa** — ada state machine eksplisit (PLAN→EXECUTE→VERIFY→COMPLETE/BLOCKED), checkpoint atomik dengan file-locking, circuit breaker per-provider/model, dan reject-loop yang menolak klaim `done:true` tanpa bukti eksekusi nyata.

Namun ada beberapa titik lemah nyata:
- **Subagent role `coder` sepenuhnya diimplementasikan tapi tidak pernah dipanggil di jalur mana pun** (dead route) — hanya role `researcher` yang punya caller.
- **`airun`/`aifix` beroperasi total di luar arsitektur tool-registry/permission/checkpoint** yang dipakai `aiagent` — bypass penuh terhadap guardrail utama.
- Tiga command-class (FAST/SMART/BIG/AGENT) punya urutan provider berbeda-beda **secara sengaja terdokumentasi**, tapi `aiask` diklasifikasikan FAST walau tugasnya (jawab pertanyaan atas konten arbitrer) sering butuh reasoning SMART — potential misclassification, bukan bug tapi decision inconsistency.
- Context engine 6-level (dokumentasi di `40-context_engine_docs.zsh`) **benar-benar diimplementasikan** di sysprompt (`00-sysprompt.zsh` baris 68–74) — bukan dead documentation seperti kecurigaan awal.
- Index (`aiindex`) murni manual/opsional — tool `grep_search`/`glob_search` **reuse pasif** kalau index fresh, tidak pernah trigger index sendiri.

Skor keseluruhan: **7.1/10** — arsitektur solid dengan disiplin evidence-based fixing (banyak `v-fix` inline yang menunjukkan histori bug nyata diperbaiki), tapi punya satu jalur mati signifikan (subagent coder) dan satu inkonsistensi arsitektural yang disengaja (airun/aifix di luar guardrail).

---

## 2. Cognitive Surface Mapping

| Komponen | File | Fungsi |
|---|---|---|
| Planner (step request) | `50-agent/42-execution/05-get_plan.zsh` | `_ai_agent_exec_get_plan` — minta 1 langkah JSON dari LLM tiap iterasi |
| Executor loop | `50-agent/42-execution/00-loop_main.zsh` | `_ai_agent_execute_loop` — bounded while-loop, orkestrasi phase |
| Runtime/orchestrator | `50-agent/40-runtime/30-aiagent.zsh` | `aiagent()` — entry point CLI |
| Tool router/dispatcher | `05-tools/05-tool_dispatch.zsh` | `_ai_tool_dispatch` — validasi schema → permission → eksekusi |
| Model router | `10-core/50-request_blocking.zsh` | `_ai_chat_request` — 2-lapis fallback (model-in-provider, provider-to-provider) |
| Skill loader/registry | `70-skills.zsh` | `_ai_load_skills`/`_ai_skill_match` — keyword-match, load-on-demand |
| Context builder | `45-project.zsh`, `50-agent/40-runtime/00-sysprompt.zsh` | project scan + tool manifest + skill injection ke sysprompt |
| Memory (session) | `10-core/60-session_trim.zsh` | `_ai_trim_session` — cap `AI_SESSION_MAX_MSGS` pesan, system msg dipertahankan |
| Checkpoint | `50-agent/10-state.zsh` | `_ai_agent_checkpoint_save` — atomik, file-lock, revisioned JSON |
| Resume | `50-agent/40-runtime/10-load_checkpoint.zsh` | `_ai_agent_load_checkpoint` — validasi schema_version==2 |
| Subagent | `55-subagent/20-run.zsh`, `15-run_step.zsh` | `_ai_subagent_run` — role-scoped bounded loop |
| Retry/fallback decision | `10-core/44-retry_decision.zsh` | `_ai_chat_retry_decision` — 413/429/404 branching |
| Circuit breaker | `10-core/40-circuit_breaker.zsh` | `_ai_breaker_is_open`/`_ai_breaker_record_fail` |
| Stop guard | `50-agent/39-agent-state-machine.zsh` + `42-execution/25-track_and_continue.zsh` | state machine + `AI_AGENT_MAX_STEPS`/`AI_AGENT_MAX_SAME_FAIL` |
| Permission gate | `06-permissions/15-permission_check.zsh` | `_ai_permission_check` — path containment → capability → level dispatch |
| Index (opsional) | `46-index.zsh` | `aiindex`, dipakai pasif oleh `grep_search`/`glob_search` |

### Diagram hubungan komponen
```
aiagent() ──> prepare_new_goal/load_checkpoint ──> print_header ──> run_execution
                                                                        │
                                                          battery/wakelock/context_begin
                                                                        │
                                                              _ai_agent_execute_loop
                                                                        │
                 ┌──────────────────────────────────────────────────────┴───────────────────────────────┐
                 │ get_plan (LLM call, state→PLAN)                                                        │
                 │   └─> _ai_agent_provider_request ─> _ai_chat_request ─> [provider×model fallback loop] │
                 │ check_done_rejections (state→VERIFY jika done)                                         │
                 │ run_tool (state→EXECUTE) ─> _ai_tool_dispatch ─> normalize→schema→permission→exec       │
                 │ log_and_notify ─> jsonl log + rate-limited notify                                       │
                 │ track_and_continue ─> same-fail counter, checkpoint_save, trim_session                  │
                 └──────────────────────────────────────────────────────┬───────────────────────────────┘
                                                                        │
                                                          state→COMPLETE|BLOCKED
                                                                        │
                                                              _ai_agent_finalize (report)
```

---

## 3. Decision Flow Mapping

```
Goal ─> Context Build (project scan + skill match + tool manifest, sysprompt) ─┐
                                                                                 ▼
                                              Model Selection (task_class="smart", order=AGENT)
                                                                                 ▼
                                              Planning (1 step per LLM call — bukan multi-step upfront plan)
                                                                                 ▼
                                              Tool Selection (LLM memilih dari tool_contracts di sysprompt)
                                                                                 ▼
                                              Permission (path containment → capability(YOLO) → level ask)
                                                                                 ▼
                                              Execution (_ai_tool_dispatch, autodep retry on exit127)
                                                                                 ▼
                                              Checkpoint (tiap iterasi, atomik+lock)
                                                                                 ▼
                                              Review (done:true → reject_checks: commands_run>0 + syntax verify)
                                                                                 ▼
                                              Done (state COMPLETE, checkpoint dihapus, auto-review diff)
```
**Catatan bukti:** "Planning" di sini BUKAN rencana multi-langkah upfront yang dieksekusi linear — setiap step re-plan dari nol berdasarkan histori pesan (`msgfile`). Satu-satunya artefak "rencana eksplisit" adalah `todo_write`/`todo_read` (tool session-scoped, opsional, LLM-driven, tidak divalidasi executor).

---

## 4. Tool Selection Audit

| Tool | Trigger | Caller | Fallback |
|---|---|---|---|
| `read_file` | LLM pilih tool `read_file` di JSON reply | `_ai_tool_dispatch` → `_ai_tool_read_file` | tidak ada — error jika file tak ada/secret/binary |
| `list_dir` | idem | idem | fallback `eza→ls→/bin/ls→/usr/bin/ls` |
| `grep_search` | idem | idem | index-symbol-lookup dulu (opsional) → `rg` → `find+grep` |
| `glob_search` | idem | idem | index file-list (opsional) → `fd` → `find` |
| `count_lines` | idem | idem | tidak ada |
| `write_file` | idem | idem | tolak jika file sudah ada (harus `edit_file`) |
| `edit_file` | idem | idem | Python search-replace exact-match; gagal jika 0 atau >1 match |
| `patch_file` | idem | idem | `patch -p0`; restore backup otomatis kalau gagal |
| `run_command` | idem, **hidden dari manifest default** (`AI_AGENT_EXPOSE_ARBITRARY_SHELL` harus =1) | idem | autodep-install-retry sekali kalau exit 127 |
| `exec_process` | idem | idem | allowlist program tetap (git/python/node/...); PATH-hijack guard |
| `run_test` | idem | idem | auto-detect runner (npm/cargo/go/pytest) kalau `runner` kosong |
| `move_file`/`delete_file` | idem | idem | delete selalu backup `.bak.<ts>` |
| `git_status`/`git_diff` | idem | idem | readonly, tanpa approval |
| `web_fetch` | idem | idem | SSRF guard (DNS-resolve + block privat/loopback/link-local), single-hop no-redirect |
| `todo_write`/`todo_read` | idem | idem | file JSON per-session-slug |

### Tool Decision Matrix
| Skenario | Tool Ideal | Tool Aktual (dibuktikan lewat sysprompt+registry) |
|---|---|---|
| Baca file kecil | `read_file` | `read_file` ✓ |
| Cari lokasi simbol | `grep_search`/index | `grep_search` (index-first jika fresh) ✓ |
| Edit blok unik | `edit_file` | `edit_file` (fallback manual ke `patch_file` diarahkan lewat sysprompt) ✓ |
| Jalankan test | `run_test` | `run_test`, bukan `run_command` — **dipaksa** karena `run_command` disembunyikan dari manifest default (baris `05-tool_dispatch.zsh:9`) |
| Command shell bebas | (seharusnya dihindari) | `run_command` — hanya terlihat model kalau `AI_AGENT_EXPOSE_ARBITRARY_SHELL=1`; default OFF ⇒ **tool decision by default excludes it**, konsisten dengan prinsip least-privilege |

**Temuan:** tidak ada tool yang benar-benar mati di jalur tool-registry (semua 18 punya implementasi + case di dispatcher). `run_command` bukan dead tapi *gated-hidden* (by design, bukan bug).

---

## 5. Model Routing Audit

| Task class | Provider order (const) | Dipakai oleh |
|---|---|---|
| FAST | `groq gemini cerebras deepseek` | `aic`/`aicl` chat cepat, `aicommit`, shell-helper, `aiask` (lihat catatan §12) |
| SMART | `deepseek cerebras gemini groq` | `aiplan`, `aiprompt`, `aireview`, `aisummarize` (final pass) |
| BIG | `deepseek cerebras gemini groq` | `aispec`, `aibuild`, `aiproject`/`project_generate`, `aiscrap`, `aifix`, `airun`(via aifix) |
| AGENT | `deepseek cerebras groq gemini` | `aiagent` (get_plan), subagent `researcher`/`coder` step, `aidebug` |

Model list per (provider,class) di `AI_MODELS` — fallback **dalam** provider dulu (kiri→kanan) sebelum lompat provider (`10-core/50-request_blocking.zsh:65-182`). Reasoning-effort field hanya dikirim untuk `groq/gpt-oss*` dan `deepseek-v4-*` (`42-token_budget.zsh:58-63`), model lain tidak dikirimi field itu — mencegah 400 error pada provider yang tidak mendukungnya (evidence: komentar `bug #5`).

### Model Escalation Matrix
| Task | Model Ideal | Model Aktual |
|---|---|---|
| Chat cepat/1-turn | model kecil, latency rendah | `groq_fast`/`gemini_fast` list (llama-3.1-8b-instant dst) ✓ |
| Code gen project multi-file | model besar, output panjang | BIG class, `AI_PROJECT_MAX_TOKS=3500` (sengaja diturunkan dari 9000 lama karena TPM limit — evidence `15-limits.zsh:23-29`) ✓ |
| Agent ReAct step | reasoning cepat + JSON-mode akurat | AGENT class (`deepseek cerebras groq gemini`), `AI_AGENT_MAX_TOKS=8000` ✓ |

**Tidak ditemukan** kasus "model mahal dipakai untuk tugas murah" — pemetaan task→class konsisten dengan dokumentasi di `05-provider_order.zsh`. **Fallback SELALU reachable** karena `_ai_provider_has_fallback` mengabaikan circuit breaker kalau provider itu satu-satunya kandidat tersisa (`41-provider_candidate.zsh`) — mencegah dead-end retry loop.

**Inkonsistensi kecil (Perlu Verifikasi 🟡):** `aiask` (05-aiask.zsh:41) memakai `task_class="fast"` meski tugasnya "jawab pertanyaan berdasarkan konteks file/pipe arbitrer" — task yang secara semantik lebih dekat ke SMART daripada chat singkat. Ini konsisten dengan cache-key design (`_ai_cache_key` pakai `task_class`), tapi berpotensi menurunkan kualitas jawaban untuk pertanyaan kompleks atas konten panjang.

---

## 6. Skills System Audit

| Skill | Dipakai | Caller |
|---|---|---|
| `general` | selalu (unconditional) | `_ai_skill_match` selalu prepend "general" |
| `debugging`, `testing`, `git`, `termux` | keyword match | `_ai_load_skills` dipanggil dari `50-agent/40-runtime/15-prepare_new_goal.zsh:44` |
| `code_editing`, `error_recovery`, `file_ops`, `python`, `web_dev`, `javascript`, `shell_scripting` | keyword match | idem — **ditambahkan lewat fix eksplisit** (komentar `bug #66`: 5 file skill sempat *tidak pernah* ter-map ke `AI_SKILL_KEYWORDS`, jadi dead file sampai diperbaiki) |

### Skill Coverage
| Skill file | Coverage |
|---|---|
| Semua 12 file di `skills/*.md` | 100% — semua punya entry di `AI_SKILL_KEYWORDS` (diverifikasi: `70-skills.zsh` memetakan tepat 12 nama skill, cocok dengan `find .zsh_bagas/skills -name '*.md'` = 12 file) |

**Catatan arsitektur:** Skill loading **hanya terjadi di goal BARU** (`prepare_new_goal.zsh`), **tidak pernah di jalur `--resume`** (dikonfirmasi: `load_checkpoint.zsh` tidak memanggil `_ai_load_skills`). Ini konsisten dengan desain (checkpoint menyimpan sysprompt lengkap termasuk skill-content di dalam `messages[0]`), bukan bug — tapi berarti skill baru yang ditambahkan setelah checkpoint dibuat tidak akan pernah masuk ke sesi resume tsb.

Tidak ditemukan skill mati sisa (post-fix). Tidak ada duplicate/shadowed skill.

---

## 7. Planning Intelligence Audit

Sistem **tidak** melakukan upfront task-decomposition oleh executor — decomposition sepenuhnya didelegasikan ke LLM lewat instruksi `todo_write` di sysprompt ("Untuk task multi-langkah: Mulai dengan todo_write..."). `todo_write` **tidak divalidasi** terhadap eksekusi nyata: LLM bisa menulis todo list lalu tidak pernah mengupdate status — executor tidak pernah membaca `todo_write` untuk mengubah keputusan tool berikutnya (todo hanya readonly capability untuk *user visibility*, bukan input planner).

`Goal → Plan → Execution`: **Plan** = satu panggilan LLM per step (bukan artefak rencana terpisah dari execution). Ini bukan "planning trace" klasik — plan dan execution digabung jadi satu iterasi ReAct. Tidak ada bukti "planning terlalu panjang" atau "langkah diabaikan" karena tidak ada rencana eksplisit multi-step yang bisa disimpangi — desain ini secara struktural menghindari kelas bug tersebut (trade-off: tidak ada global plan untuk mendeteksi drift jangka panjang, dimitigasi oleh `todo_write` sebagai soft-plan opsional).

---

## 8. Context Management Audit

| Context Source | Digunakan |
|---|---|
| Project metadata (`_ai_project_context`/`aiscan`) | ✓ — auto-scan kalau belum pernah (`prepare_new_goal.zsh:43`) |
| Directory/file discovery (`list_dir`/`glob_search`) | ✓ — tool, dipanggil LLM sesuai instruksi progresif di sysprompt |
| Relevant file content (`read_file`) | ✓ |
| Relevant symbols (`grep_search`/index) | ✓ — index dipakai **pasif**, hanya jika `_ai_index_is_fresh` (mtime-based, lihat `46-index.zsh:94`) |
| Exact region (`read_file` offset/limit) | ✓ — schema sudah mendukung sejak awal, tidak ada tool baru dibutuhkan |
| Execution evidence (`run_test`/`run_command`) | ✓ |

**Context Budget (kualitatif):**
- **Static context**: sysprompt (persona + tool manifest + Termux constraints) dikirim ULANG PENUH tiap panggilan LLM (tidak ada prompt-caching — evidence eksplisit di `25-persona.zsh:18-28`: caching provider tidak diverifikasi aman jadi sengaja tidak diaktifkan, sebagai gantinya teks dipadatkan).
- **Dynamic context**: histori percakapan (`msgfile`) tumbuh linear per step, di-trim ke `AI_SESSION_MAX_MSGS=30` (system message dipertahankan) lewat `_ai_trim_session`, dipanggil di **setiap** akhir step (`track_and_continue.zsh:47`).
- **Reusable context**: index (opsional, invalidated otomatis begitu `write_file`/`edit_file`/`delete_file`/`move_file` sukses — `15-run_tool.zsh:97-101`) dan project-scan cache (per-folder, tidak pernah expired otomatis, hanya re-scan manual).

**Tidak ditemukan** context leak (index invalidation scoped ke slug project aktif saja) atau rebuild tak perlu (index re-scan selalu manual via `aiindex`, tidak otomatis).

---

## 9. Memory & Checkpoint Audit

**Checkpoint creation:** setiap iterasi loop yang mencapai `track_and_continue` (bukan hanya di akhir sesi) — `_ai_agent_checkpoint_save` dipanggil dari 3 titik: `get_plan` (saat gagal, agar progres tidak hilang), `reject_checks` (saat klaim ditolak), `track_and_continue` (jalur normal).

**Locking:** `mkdir`-based atomic lock dengan stale-lock detection (`kill -0` pada PID pemilik lock; jika proses sudah mati, lock dianggap stale dan diambil alih) — `10-state.zsh:107-117`. Ini **mencegah race condition** pada write concurrent tanpa memerlukan flock eksternal.

**Restore/resume:** `_ai_agent_load_checkpoint` memvalidasi `schema_version==2` secara eksplisit — checkpoint versi lama (schema 1 atau tanpa field) **ditolak**, bukan di-migrate diam-diam. Ini state diagram yang bersih: tidak ada "silent corruption" dari format lama.

**Cleanup:** checkpoint dihapus **hanya** jika lifecycle state == `COMPLETE` (`44-finalize.zsh:38-40`) — jika `BLOCKED` (termasuk max-steps tercapai atau max-same-fail), checkpoint **sengaja dipertahankan** agar bisa di-resume.

**State diagram (nyata, dari `39-agent-state-machine.zsh`):**
```
PLAN ──> EXECUTE ──> VERIFY ──> COMPLETE (terminal)
 │  ↖________________|            
 │                    └──> PLAN (retry setelah verify gagal)
 └──> BLOCKED (terminal, dari state manapun)
```
Transisi tidak valid (mis. `COMPLETE → EXECUTE`) di-reject oleh `_ai_agent_state_transition` dan mencetak error ke stderr — **tidak ada unreachable checkpoint atau inconsistent resume** yang ditemukan; satu potensi masalah kecil: kegagalan transisi state hanya `|| true` di sebagian besar caller (silent-continue), bukan hard-fail — artinya lifecycle state BISA stuck di state lama kalau transisi gagal karena race, walau efek fungsionalnya minimal karena keputusan akhir loop tetap berdasar `done_flag`, bukan murni state.

---

## 10. Subagent Delegation Audit

| Delegation | Status |
|---|---|
| `researcher` (readonly investigation) | ✓ **Hidup** — dipanggil dari `05-subagent_offer.zsh:36` (offer di awal `aiagent` baru jika goal match heuristik `*audit*`/`*refactor seluruh*`/dst) dan `60-ui/25-research_dev.zsh:19` (`airesearch` standalone command) |
| `coder` (mutasi lintas file) | 🔴 **DEAD** — sysprompt (`10-sysprompt.zsh:30-45`), tool allowlist (`05-tool_allowlist.zsh:27-29`), dan seluruh step-loop (`15-run_step.zsh`) sepenuhnya mendukung role ini, TAPI **tidak ada satu pun caller** yang memanggil `_ai_subagent_run coder ...` di seluruh codebase (diverifikasi via `grep -rn "_ai_subagent_run"` — hanya 2 call site, keduanya hardcode `researcher`) |
| `debug` subagent (`aidebug`, role diagnosis-only) | ✓ Hidup — arsitektur terpisah (`_ai_debug_step`/`_ai_debug_tool_allowed`), bukan bagian dari role researcher/coder di atas |

**Context loss saat handoff:** tidak ditemukan — hasil subagent disuntik sebagai **ringkasan terstruktur satu-pesan** (bukan transcript penuh) ke `$msgfile` main agent (`05-subagent_offer.zsh:100-103`), sesuai kontrak §5/§6 di `00-design_contract.zsh`. Tidak ada duplicate work: subagent offer hanya terjadi di jalur goal BARU, tidak pernah di `--resume` (dicegah eksplisit lewat percabangan `if/else` di `30-aiagent.zsh:59-63`).

---

## 11. Retry & Recovery Intelligence

| Retry | Status |
|---|---|
| HTTP 413 (request too large) | Bounded — halving `max_toks` sampai < 500, lalu pindah model. **Non-transient handling benar**: tidak retry payload identik. |
| HTTP 429/404 | **Langsung skip ke model berikutnya**, tanpa retry — benar secara arsitektural (429/404 pada request identik pasti gagal lagi). |
| Error lain / timeout | Retry dengan `sleep $AI_RETRY_DELAY` (2 detik), dibatasi `AI_MAX_RETRIES=1` (default) per model. |
| exit 127 (command not found, tool-level) | Auto-install-then-retry-once (`02-tool_autodep.zsh` + caller di `15-run_tool.zsh:28-44` dan `30-tool_process.zsh:107-121`) — retry dibatasi eksplisit "SEKALI", tidak berulang. |
| Same-tool-same-args gagal berulang | Berhenti (bukan retry) setelah `AI_AGENT_MAX_SAME_FAIL=3` (`25-track_and_continue.zsh`). |
| Circuit breaker | Window 30 detik per provider **dan** per (provider/model) — tapi diabaikan kalau kandidat itu satu-satunya API key yang tersedia (`_ai_provider_has_fallback`), mencegah dead-end. |

**Tidak ditemukan** retry yang tidak rasional (mis. retry pada 429, atau retry unbounded). Semua retry punya batas eksplisit dan alasan berbasis HTTP-semantics yang benar.

---

## 12. Stop Decision Audit

| Stop Condition | Status |
|---|---|
| Success (`done:true` + `commands_run>0` + syntax verify lolos) | ✓ → state `VERIFY`→`COMPLETE`, checkpoint dihapus |
| `done:true` tanpa tool call sesi ini | ✓ **Ditolak** — dipaksa kembali ke `PLAN` dengan pesan penolakan eksplisit (anti-hallucinated-success) |
| `done:true` tapi file yang disentuh gagal syntax check | ✓ Ditolak, kembali ke `PLAN` |
| Max steps (`AI_AGENT_MAX_STEPS=15`, +offset saat resume) | ✓ → `BLOCKED`, pesan eksplisit + saran `--resume` |
| Max same-fail (3x tool identik gagal) | ✓ → `BLOCKED` |
| Cancel (SIGINT/SIGTERM) | ✓ — trap kooperatif nulis flag `cancelled` ke state_dir, diperiksa di 2 titik loop (`00-loop_main.zsh:47`, `15-run_tool.zsh:18`) |
| LLM/provider request gagal total | ✓ → `BLOCKED` dengan detail asli dari stderr (bukan pesan generik "status 1" — evidence fix `bug #4`) |
| Invalid JSON dari LLM | ✓ → `BLOCKED` |

**Dead-loop guard:** tidak ditemukan potensi infinite loop — `while [ $step -lt $max_step ]` selalu bounded, dan `same_fail_count` mencegah stagnasi pada kegagalan berulang. Subagent (`_ai_subagent_run`) dan debug (`aidebug`) memakai pola bounded yang identik.

---

## 13. Decision Consistency Audit

| Jalur | Tool routing | Model routing | Retry | Fallback | Permission |
|---|---|---|---|---|---|
| `aiagent` | via `_ai_tool_dispatch` (full registry, semua tool) | AGENT class | full (413/429/404 + same-fail) | full (breaker + capability YOLO) | full (`_ai_permission_check`, path containment, ask_once_per_file) |
| `aiagent` subagent researcher | via `_ai_tool_dispatch` **+ role allowlist tambahan** (`_ai_subagent_tool_allowed`, hanya 5 readonly tool) | AGENT class (sama) | full (sama `_ai_chat_request`) | full (sama) | full + role-gate ekstra sebelum dispatch |
| `aidebug` | via `_ai_tool_dispatch` **+ debug allowlist** (readonly + run_test/run_command, TANPA write) | AGENT class | full | full | full + debug-gate |
| `aifix`/`airun` | 🔴 **TIDAK via `_ai_tool_dispatch` sama sekali** — `_ai_quick`→`_ai_chat_request` langsung, hasil ditulis lewat `mv`/redirect shell biasa, **tanpa** permission check, **tanpa** path containment, **tanpa** secret-file guard | BIG class | full (di level `_ai_chat_request`) | full | 🔴 **tidak ada** — tidak lewat `_ai_permission_check` sama sekali |
| `aiplan`/`aispec`/`aibuild`/`aireview` | tidak pakai tool sama sekali (single-shot completion) | SMART/BIG (berbeda per fungsi, **terdokumentasi & konsisten** dengan tabel §5) | full | full | n/a (tidak menyentuh filesystem via tool) |

**Kontradiksi nyata:** `aifix`/`airun` (dipanggil dari `airun`'s auto-fix loop dan dispatcher CLI `fix`/`run`) menulis file (`${file}.fixed`, lalu `mv` ke file asli) **tanpa** melewati guard yang sama seperti `write_file`/`edit_file` di `aiagent` (tidak ada secret-file check, tidak ada project-path containment, tidak ada approval prompt). Ini bukan bug tersembunyi — arsitekturnya memang beda (standalone command lama vs. agent framework baru) — tapi merupakan **inkonsistensi keputusan keamanan** yang nyata antara dua jalur yang sama-sama "AI menulis file pengguna".

---

## 14. Agent Efficiency Audit

| Inefficiency | Dampak |
|---|---|
| Sysprompt penuh (persona + tool manifest + Termux context + project context + skill) dikirim ULANG setiap panggilan LLM, tanpa prompt-caching | Token overhead berulang per step; **sengaja diterima** karena caching tidak diverifikasi aman lintas provider (evidence `25-persona.zsh:21-28`) — trade-off eksplisit, bukan oversight |
| `grep_search`/`glob_search` melakukan index-lookup **opsional** sebelum fallback ke `rg`/`fd` | Tambahan 1 jq-call kalau index fresh — dirancang untuk **mengurangi** filesystem read, bukan menambah; hemat net |
| `_ai_trim_session` dipanggil di setiap step (bukan hanya saat threshold terlampaui) | `jq 'length'` overhead kecil tiap step; guard `if len > MAX` membuat efeknya minimal |
| Checkpoint disimpan di **setiap** step (bukan setiap-N-step) | I/O disk per step (lock+write+atomic-move) — trade-off sengaja untuk daya tahan terhadap OOM-kill (evidence `bug #55`), bukan inefficiency yang tidak disadari |
| Tidak ada model escalation berlebihan yang ditemukan | AGENT/SMART/BIG/FAST classes dipetakan sesuai kompleksitas tugas |

Tidak ditemukan pola read/planning berulang yang tidak perlu — desain single-step ReAct secara struktural mencegah "planning berulang" (tidak ada planning terpisah dari eksekusi untuk diulang).

---

## 15. Dead Intelligence Audit

| Komponen | Status |
|---|---|
| Subagent role `coder` | 🔴 **MATI** — implementasi lengkap (sysprompt, allowlist, step-loop), nol caller. Dikonfirmasi lewat pencarian caller menyeluruh (`grep -rn "_ai_subagent_run"` di seluruh `30-ai/`) — hanya 2 call site, keduanya hardcode `role=researcher`. |
| Skill files (12 file) | ✓ Hidup (setelah fix `bug #66`) — histori menunjukkan 5 skill file SEMPAT mati (ada file, tidak ada keyword-map entry) sebelum diperbaiki; kondisi saat ini semua ter-map. |
| `run_command` tool | Bukan mati — gated-hidden by design (`AI_AGENT_EXPOSE_ARBITRARY_SHELL` default 0). Kalau di-enable, jalur eksekusinya lengkap dan reachable. |
| Model route `openrouter` | 🟡 **Perlu Verifikasi** — disebut di komentar `35-providers.zsh:32` sebagai "dicabut" dari `AI_PROVIDER_ORDER` karena entry-nya belum ada di `AI_PROVIDERS`; tidak ada sisa kode aktif yang mereferensikannya, jadi bukan dead route melainkan sudah dibersihkan sepenuhnya. |
| Model-model yang sengaja tidak dimasukkan (`gemini-2.5-pro`, `gemini-2.0-*`, dst) | Bukan dead code — didokumentasikan eksplisit di `00-models.zsh:31-39` sebagai hasil audit manual (429/404/format rusak), sengaja dikeluarkan dari daftar aktif. |
| Fallback breaker | ✓ Reachable — diverifikasi lewat 2 titik pemanggilan (provider-level & model-level) dan kondisi bypass saat kandidat terakhir. |

**Kesimpulan Dead Intelligence:** satu temuan signifikan (subagent `coder`), tidak ada yang lain.

---

## 16. Decision Invariant Audit

| Invariant | Status |
|---|---|
| Tool dipilih lewat router | ✓ Semua tool (termasuk subagent/debug) lewat `_ai_tool_dispatch`, KECUALI `aifix`/`airun` yang tidak lewat tool-router sama sekali (bukan "tool dipilih lewat router yang salah" — mereka **tidak memakai tool abstraction sama sekali**, arsitektur berbeda) |
| Retry bounded | ✓ Semua jalur retry (model, tool-autodep, agent-same-fail) punya batas eksplisit |
| Checkpoint reachable | ✓ Disimpan di 3 titik loop, dapat di-resume dengan validasi schema |
| Fallback reachable | ✓ Circuit breaker punya bypass kandidat-terakhir |
| Skill callable | ✓ Semua 12 skill file ter-map |
| Context cleaned | ✓ Session trim per step, index diinvalidasi otomatis saat file berubah |
| Planner menghasilkan langkah nyata | ✓ Setiap `get_plan` menghasilkan `{tool,args,done}` yang divalidasi non-kosong sebelum dieksekusi |
| Stop guard reachable | ✓ Max-steps, max-same-fail, cancel-flag, invalid-JSON — semua diperiksa tiap iterasi |

**Pelanggaran:** tidak ada pelanggaran invariant di jalur `aiagent`/subagent/debug. `aifix`/`airun` secara desain berada di luar cakupan invariant ini (mereka bukan bagian dari tool-router architecture) — dicatat sebagai inkonsistensi arsitektural (§13), bukan pelanggaran invariant per se.

---

## 17. Contradiction Hunt (minimal 10)

| # | Kontradiksi | Bukti | Severity |
|---|---|---|---|
| 1 | Subagent role `coder` diimplementasikan penuh tapi tak pernah dipanggil | 0 caller ditemukan via grep menyeluruh | High |
| 2 | `aifix`/`airun` menulis file tanpa permission-check/path-containment, padahal `aiagent` mewajibkannya | `45-fix.zsh` pakai `mv` langsung vs `_ai_tool_dispatch`→`_ai_permission_check` di agent | Medium |
| 3 | `aiask` diklasifikasikan FAST meski tugasnya bisa kompleks (QA atas konten arbitrer) | `05-aiask.zsh:41` `task_class="fast"` | Low |
| 4 | Sysprompt bilang "boleh loncat langsung" progressive-context tapi juga bilang "HEMAT TOKEN -- panduan urutan" — ambigu apakah wajib atau opsional | `00-sysprompt.zsh:68` | Low (disengaja, bukan bug — frasa eksplisit "bukan larangan keras") |
| 5 | Checkpoint disimpan tiap step untuk daya tahan OOM, tapi skill/project-context TIDAK di-reload saat resume | `10-load_checkpoint.zsh` tidak panggil `_ai_load_skills`/`_ai_project_context` | Low (by design, didokumentasikan) |
| 6 | `run_test` mengizinkan path validasi gagal secara silent (`|| true`) sementara tool filesystem lain (`read_file` dkk) mewajibkan | `15-permission_check.zsh:71` `_ai_validate_project_path "$fs_path" "run_test" || true` | Medium |
| 7 | Persona chat (`AI_PERSONA_CHAT_*`) beda total formatnya (marker `@@JAWABAN@@`) dari persona agent JSON-kontrak, meski dulunya reuse yang sama (fixed via `v-fix`) | `25-persona.zsh:59-70` | Low (sudah diperbaiki, historical) |
| 8 | `AI_PERM_ALLOW_OUTSIDE_PROJECT=1` bisa menonaktifkan SEMUA path containment untuk semua tool filesystem sekaligus, termasuk `delete_file` | `10-path_guard.zsh:51-53` dipakai `_ai_validate_project_path` universal | Medium (fitur eskalasi luas via satu toggle) |
| 9 | `_ai_tool_normalize_args` bisa mengubah bentuk args SEBELUM schema validation, artinya schema tidak lagi memvalidasi "apa yang model kirim" tapi "apa yang sudah dinormalisasi" | `05-tool_dispatch.zsh:49-55` | Low (disengaja untuk toleransi model-mistake) |
| 10 | `AI_AGENT_AUTO_NPM_CHECK` default OFF untuk alasan performa Termux, tapi `node --check`/`py_compile` (checker serupa) WAJIB & blocking secara default — inkonsistensi filosofi "opsional vs wajib" antar dua mekanisme verifikasi yang mirip | `00-config.zsh` vs `10-reject_checks.zsh` | Low (didokumentasikan sebagai keputusan sadar: satu blocking-required, satu informational-optional) |
| 11 | State-transition failure di sebagian besar caller di-swallow via `|| true`, tapi di `get_plan.zsh` kegagalan yang sama menyebabkan `return 2` (fatal) | Bandingkan `05-get_plan.zsh:32` vs `00-loop_main.zsh:49,59,99,127,135` | Medium |

---

## 18. Evidence Coverage Audit

| Area | Coverage |
|---|---|
| Planner | Penuh — `05-get_plan.zsh` dibaca utuh |
| Runtime | Penuh — `30-aiagent.zsh`, `25-execute_and_finalize.zsh` dibaca utuh |
| Tool Router | Penuh — `05-tool_dispatch.zsh` + semua 10 file implementasi tool dibaca utuh |
| Model Router | Penuh — `40-circuit_breaker.zsh`, `41-provider_candidate.zsh`, `42-token_budget.zsh`, `43-payload_builder.zsh`, `44-retry_decision.zsh`, `48-http_call_blocking.zsh`, `50-request_blocking.zsh` dibaca utuh |
| Skills | Penuh — `70-skills.zsh` dibaca utuh, caller diverifikasi |
| Checkpoint | Penuh — `10-state.zsh`, `39-agent-state-machine.zsh` dibaca utuh |
| Resume | Penuh — `10-load_checkpoint.zsh` dibaca utuh |
| Retry | Penuh — dicover bareng Model Router |
| Fallback | Penuh — dicover bareng Model Router + circuit breaker |
| Subagent | Penuh — semua file `55-subagent/*` dibaca, caller diverifikasi via grep |
| Context Builder | Penuh — `45-project.zsh` (sebagian, cukup untuk arsitektur), `46-index.zsh` (sebagian), `00-sysprompt.zsh` dibaca utuh |
| Permission | Penuh — semua 6 file `06-permissions/*` dibaca utuh |
| UI/streaming (di luar scope tapi disentuh utk verifikasi caller) | Parsial — `60-ui/40-dispatcher.zsh`, `25-research_dev.zsh` dibaca sebagian secukupnya untuk memverifikasi call-site |

**Belum 100% dibaca (di luar prioritas Decision Architecture):** `55-request_streaming.zsh`/`56-sse_line_parser.zsh` (jalur streaming untuk `aic`/`aish`) — tidak mempengaruhi kesimpulan agent-decision karena `aiagent` selalu memakai jalur blocking (`_ai_agent_provider_request` → `_ai_chat_request`, bukan streaming).

---

## 19. Decision Coverage Matrix (Anti-Blindspot)

| Decision Type | Implementasi Ditemukan | Sudah Diaudit | Coverage |
|---|---|---|---|
| Tool Selection | ✓ | ✓ | Penuh |
| Model Routing | ✓ | ✓ | Penuh |
| Skill Invocation | ✓ | ✓ | Penuh |
| Planning | ✓ (single-step ReAct, bukan multi-step planner) | ✓ | Penuh |
| Permission | ✓ | ✓ | Penuh |
| Retry | ✓ | ✓ | Penuh |
| Stop | ✓ | ✓ | Penuh |
| Checkpoint | ✓ | ✓ | Penuh |
| Resume | ✓ | ✓ | Penuh |
| Subagent | ✓ (2 dari 3 role reachable) | ✓ | Penuh |
| Context Build | ✓ | ✓ | Penuh |

Tidak ada Decision Type yang kosong.

---

## 20. Decision Trace Validation (8 skenario)

| # | Skenario | Trace: Goal → Planner → Model → Tool → Execution → Stop |
|---|---|---|
| 1 | Edit file existing | Goal masuk `prepare_new_goal` → sysprompt dibangun → LLM (AGENT class) balas `{tool:"edit_file",...}` → `_ai_permission_check` (path containment + ask_once_per_file) → `_ai_tool_edit_file` (python exact-match, backup) → `reject_checks` verifikasi syntax kalau `done:true` → `COMPLETE`/`PLAN` lanjut. **Tidak ada lompatan tanpa bukti.** |
| 2 | Review repo (banyak file) | Goal match heuristik subagent (`*review seluruh*`) → offer prompt → jika 'y', `_ai_subagent_run researcher` (readonly-only, tool allowlist terpisah) → ringkasan disuntik ke `$msgfile` → main loop lanjut dengan AGENT class model. **Tidak ada lompatan.** |
| 3 | Commit | `aicommit` (bukan `aiagent`) → `_ai_quick` FAST class → `git commit -m` langsung via shell biasa (bukan lewat tool registry, karena ini bukan bagian `aiagent`). **Trace valid tapi arsitektur berbeda dari agent utama** (dicatat sebagai temuan §13). |
| 4 | Debug | `aidebug` → `_ai_debug_step` (AGENT class model) → `_ai_debug_tool_allowed` (readonly + run_test/run_command, TANPA write) → `_ai_tool_dispatch` → hasil masuk `reproduction[]`/`affected_files{}` → laporan diagnosis. **Tidak ada lompatan.** |
| 5 | Resume | `ai agent --resume <slug>` → `_ai_agent_load_checkpoint` (validasi schema_version==2) → **skip** sysprompt/skill rebuild (pakai `messages` tersimpan) → lanjut `_ai_agent_run_execution` dari `step_offset`. **Trace valid**, tapi confirmed skill/project-context TIDAK di-refresh (temuan §17 no.5). |
| 6 | Build (aibuild) | `aibuild` → 2 LLM call berurutan: spec-generation (BIG class, sysprompt shared dari `30-sysprompt_spec.zsh`) → lalu `_ai_quick`/generate loop per-file (`10-project_generate.zsh`, BIG class, `AI_PROJECT_MAX_TOKS` override) — **tidak lewat tool registry sama sekali** (menulis file langsung). Trace valid tapi arsitektur non-agentic. |
| 7 | Research (standalone) | `airesearch` → `_ai_subagent_run researcher` langsung (tanpa main-agent loop di sekitarnya) → readonly tools only → return `status=/summary=/findings=` ke stdout. **Tidak ada lompatan.** |
| 8 | Patch (diff kompleks) | Agent memilih `patch_file` tool → `_ai_permission_check` (write level, ask_once_per_file) → `_ai_tool_patch_file` (`patch -p0`, restore backup otomatis kalau gagal, guard `AI_PATCH_MAX_CHARS`) → hasil masuk context → `reject_checks` syntax-verify. **Tidak ada lompatan.** |

Semua 8 skenario tertelusuri penuh; tidak ditemukan lompatan tanpa bukti runtime.

---

## 21. False Positive Challenge (untuk temuan High/Critical)

**Temuan #1 — Subagent `coder` dead route (High):**
- Caller lain? Dicek: tidak ada caller di `50-agent/`, `60-ui/`, `40-workflow/`, atau file mana pun selain 2 call site researcher yang sudah diidentifikasi.
- Runtime override? Tidak ada mekanisme dynamic dispatch (mis. `eval "_ai_subagent_run $role"` dengan `$role` dari variabel eksternal) yang bisa membuatnya reachable secara tidak langsung — semua call site hardcode string literal `"researcher"`.
- Env override? Tidak ada env var yang mengontrol role subagent yang dipanggil.
- Wrapper/dispatcher? `60-ui/40-dispatcher.zsh` tidak punya command `coder`/`subagent-coder` apa pun.
- **Kesimpulan: TETAP CONFIRMED dead — bukan false positive.** Severity dipertahankan High karena representasi kapabilitas yang terdokumentasi lengkap namun tak pernah dipakai adalah maintenance-risk (kode besar tak tertest secara praktik) dan indikasi fitur setengah-jalan (Task 6.2 selesai, tapi trigger untuk role kedua tidak pernah ditulis).

---

## 22. Duplicate Finding Consolidation

| Temuan Awal | Root Cause | Digabung |
|---|---|---|
| #2 (aifix/airun tanpa permission), #6 (run_test path-check silent `\|\| true`), #8 (AI_PERM_ALLOW_OUTSIDE_PROJECT global toggle) | **Permission enforcement tidak seragam di semua jalur penulisan file** | Digabung jadi 1 root cause (lihat §23 RC-2) |
| #3 (aiask FAST class), #10 (npm-check optional vs syntax-check wajib) | **Klasifikasi "seberapa penting/mahal" suatu operasi kadang tidak konsisten dengan skala LLM/verifikasi yang dipakai** | Digabung jadi 1 root cause (lihat §23 RC-3) |
| #5 (resume tidak reload skill/context), #4 (frasa progresif ambigu) | Bukan bug — desain sadar, tidak digabung ke root cause aksi (dicatat sebagai catatan desain) | — |

---

## 23. Root Cause Consolidation (maks 5)

| Gejala | Root Cause |
|---|---|
| Subagent `coder` tak terpakai; kapabilitas besar tak teruji di jalur nyata | **RC-1: Fitur diimplementasikan mengikuti kontrak desain (§00-design_contract) sampai selesai secara teknis, tapi trigger/UX untuk mengaktifkannya tidak pernah ditulis sebagai task terpisah.** |
| `aifix`/`airun` di luar guardrail; `run_test` path-check `\|\| true`; toggle global `AI_PERM_ALLOW_OUTSIDE_PROJECT` | **RC-2: Ada dua "generasi" arsitektur berdampingan — command lama single-shot (aifix/airun/aicommit/aibuild) yang mendahului tool-registry+permission framework, dan `aiagent` modern yang mengadopsi guardrail penuh. Keduanya tidak pernah disatukan.** |
| `aiask` FAST class utk task berpotensi kompleks; `AI_AGENT_AUTO_NPM_CHECK` optional vs syntax-check wajib | **RC-3: Klasifikasi biaya/kepentingan tugas (task_class, blocking-vs-informational) ditentukan manual per fungsi tanpa kriteria terpusat, sehingga bisa drift dari kompleksitas tugas aktualnya seiring waktu.** |
| State-transition failure kadang fatal (`get_plan` → return 2) kadang silent (`\|\| true` di banyak tempat lain) | **RC-4: State-machine enforcement ditambahkan belakangan (evidence: banyak `2>/dev/null \|\| true` sebagai "defensive fallback" alih-alih mengharuskan transisi valid), sehingga tidak semua caller memperlakukan kegagalan transisi dengan bobot yang sama.** |
| Sysprompt tidak di-cache, dikirim penuh tiap step | **RC-5: Keputusan sadar menghindari fitur provider (prompt caching) yang belum diverifikasi lintas-provider — trade-off token vs reliability, bukan oversight.** (Dicantumkan untuk kelengkapan, tapi ini BUKAN masalah yang perlu remediasi — sudah optimal untuk constraint yang dihadapi.) |

---

## 24. Counterfactual Decision Challenge (temuan High/Critical)

**RC-1 (Subagent coder dead):**
- Jalur alternatif jika gagal? Tidak ada — kalau task butuh delegasi mutasi lintas-file, sistem SELALU jatuh ke main-agent loop biasa (bukan pengganti yang setara, karena main-agent tidak run paralel/isolated).
- Runtime bisa ambil keputusan lain via fallback? Tidak — tidak ada logic apa pun yang mempertimbangkan pemanggilan `coder` role.
- Wrapper/dispatcher mengubah keputusan? Tidak.
- **Kesimpulan: severity High dikunci — tidak ada mitigasi tersembunyi.**

**RC-2 (aifix/airun di luar guardrail):**
- Jalur alternatif? Ya — `aiagent` tersedia sebagai jalur beryang lebih aman untuk task serupa (fix error di file), tapi `airun`/`aifix` adalah command terpisah yang **user bisa pilih langsung** tanpa melalui `aiagent` sama sekali.
- Runtime override? Tidak ada guard tambahan tersembunyi di `airun`/`aifix` — dikonfirmasi baca penuh kedua file.
- Wrapper mengubah keputusan? Dispatcher CLI (`fix`/`run` command) memanggil fungsi ini langsung, tanpa lapisan tambahan.
- **Kesimpulan: severity Medium dikunci (bukan Critical, karena scope terbatas — hanya path file yang eksplisit diberikan user sebagai argumen command, bukan path yang dipilih otonom oleh LLM seperti di `aiagent`).**

---

## 25. Self Review

- [x] Semua tool (18 tool `aiagent`) punya Decision Trace — ✓
- [x] Semua skill (12 file) punya caller — ✓
- [x] Model routing diverifikasi — ✓ (4 task class, circuit breaker, escalation matrix)
- [x] Planner diverifikasi — ✓ (single-step ReAct, todo_write soft-plan)
- [x] Runtime diverifikasi — ✓ (aiagent orchestrator penuh)
- [x] Checkpoint diverifikasi — ✓ (atomik, lock, schema-versioned)
- [x] Resume diverifikasi — ✓
- [x] Evidence Coverage selesai — ✓ (§18)
- [x] Decision Coverage Matrix selesai — ✓ (§19, tidak ada yang kosong)
- [x] Minimal 10 kontradiksi dicari — ✓ (11 ditemukan, §17)
- [x] False Positive Challenge selesai (untuk temuan High) — ✓ (§21)
- [x] Duplicate Finding Consolidation selesai — ✓ (§22)
- [x] Counterfactual Challenge selesai (untuk temuan High/Critical) — ✓ (§24)
- [x] Root Cause selesai (≤5) — ✓ (§23, 5 root cause)

Tidak ada item yang gagal — audit dapat ditutup.

---

## 26. Scoring

| Aspek | Skor |
|---|---|
| Tool Intelligence | 8/10 |
| Model Routing | 8/10 |
| Planning Quality | 6/10 (single-step ReAct valid tapi tanpa multi-step lookahead nyata) |
| Context Management | 8/10 |
| Checkpoint Reliability | 9/10 |
| Skill Architecture | 8/10 |
| Decision Consistency | 5/10 (dua-generasi arsitektur berdampingan, §13/§17) |
| Agent Intelligence (keseluruhan) | 7/10 |

**Skor akhir: 7.1/10**

---

## 27. Final Roadmap

| Dampak | Frekuensi | Effort | Sprint | Item |
|---|---|---|---|---|
| High | Medium | Low | Sprint 1 | RC-1: putuskan — hapus role `coder` (dead code) ATAU tambahkan trigger nyata (mis. heuristik "refactor lintas banyak file" yang eksplisit menawarkan role coder, bukan cuma researcher) |
| High | High | High | Sprint 2 | RC-2: satukan `aifix`/`airun`/`aicommit`/`aibuild` ke bawah payung permission/path-containment yang sama dengan `aiagent`, atau dokumentasikan eksplisit sebagai "trusted local commands" dengan scope berbeda by design |
| Medium | High | Low | Sprint 2 | RC-4: audit semua `_ai_agent_state_transition ... \|\| true` — putuskan mana yang boleh silent-continue vs harus fatal, buat kebijakan konsisten |
| Medium | Medium | Low | Sprint 1 | RC-3: buat kriteria eksplisit (mis. dokumen kecil) untuk memilih task_class per fungsi baru, cegah drift seperti `aiask`=FAST |
| Low | Low | Low | Sprint 3 | Tinjau ulang `run_test` path-validation yang silently `|| true` — pastikan ini benar-benar disengaja (path opsional) bukan sisa copy-paste dari fs_path lain |

---

*Audit ini dibangun murni dari pembacaan implementasi (`.zsh` di `30-ai/`), dengan verifikasi caller lewat pencarian menyeluruh (`grep -rn`) untuk setiap klaim "dipakai/tidak dipakai". Baris kode yang dirujuk dapat ditelusuri ulang di file aslinya sesuai path yang dicantumkan di setiap bagian.*
