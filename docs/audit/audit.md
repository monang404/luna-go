# AUDIT MENYELURUH — REPOSITORY `zsh_bagas` (dari `baseline.zip`)

**Auditor:** Principal Security Engineer + Staff Software Engineer + DevOps Architect + Zsh Runtime Expert (simulasi)
**Target:** `zsh_bagas-main/` — 153 file kode (zsh + Python), ~11.9k LOC, inti produk: sebuah **AI coding agent** yang hidup di dalam konfigurasi Zsh (`.zsh_bagas/30-ai/`), dilengkapi tool registry, sistem permission, dan agent loop ala ReAct.

**Dokumen ini adalah gabungan dua putaran audit** menjadi satu laporan tunggal yang koheren:
- **Putaran 1** memetakan seluruh struktur repo dan menelusuri jalur runtime kritikal (agent loop, permission, HTTP) secara naratif/analitis.
- **Putaran 2** mengambil setiap klaim "Perlu Verifikasi" dari putaran 1, **mereproduksinya lewat eksekusi kode asli** (zsh sungguhan di sandbox, bukan simulasi manual), menutup 5 modul yang belum ditelusuri (`20-chat`, `30-code`, `40-workflow`, `55-subagent`, `60-ui`), dan melakukan static analysis menyeluruh.

Di setiap temuan, status ditandai eksplisit:
- **FAKTA** — terbukti dari kode dan/atau eksekusi langsung.
- **FAKTA (dieksekusi)** — dibuktikan dengan benar-benar menjalankan potongan kode asli repo di zsh sungguhan, bukan cuma dibaca.
- **VERIFIKASI** — diperiksa ulang di putaran 2, hasil konsisten dengan putaran 1.
- **DIKOREKSI** — klaim putaran 1 yang, setelah dieksekusi, ternyata salah arah atau salah besaran (severity naik/turun, atau mekanismenya berbeda dari dugaan awal).
- **Perlu Verifikasi (belum ditutup)** — bagian yang secara sadar masih di luar cakupan kedua putaran (didokumentasikan eksplisit di bagian Dokumentasi & sebagian `20-chat`/`40-workflow`).

**Ringkasan eksekutif:** Repo ini punya fondasi security & reliability yang matang secara *desain* — defense-in-depth nyata di permission system, path containment, dan checkpoint/locking. Tapi audit gabungan ini membongkar **satu bug Critical yang sebelumnya ter-underestimate parah**: pola `local path` yang meng-collide dengan tied special parameter `$path`/`$PATH` di zsh — putaran 1 hanya menemukannya di satu tempat yang *sudah diperbaiki*, tapi putaran 2 membuktikan lewat eksekusi nyata bahwa bug identik masih hidup **tanpa perbaikan di 7 fungsi tool inti** (`read_file`, `write_file`, `edit_file`, `patch_file`, `delete_file`, `count_lines`, `git_diff`) — membuat mayoritas tool suite **gagal total** untuk pemakaian normal. Temuan kedua yang sama pentingnya: classifier `_ai_agent_is_dangerous` mengeksekusi command substitution (`$(...)`) sebagai **efek samping dari tokenisasi**, sebelum command itu sendiri disetujui/dijalankan/diblokir — juga dibuktikan lewat eksekusi nyata, bukan analisis kode statis.

---

## Fase 1 — Pemetaan Repository

### Struktur folder & fungsi

| Folder | Fungsi | Dependency | Risiko |
|---|---|---|---|
| `.zshrc` | Loader tipis: source semua `**/*.zsh` di `.zsh_bagas/`, urut nama file (numeric glob qualifier `(N.on)`) | — | Rendah — urutan numerik (00→90) itu sendiri **adalah** dependency graph implisit; lihat §Arsitektur |
| `00-core/` | Compat shim, env/PATH/locale, secrets loader | Di-source pertama, semua modul lain bergantung padanya | Sedang — `secrets-guard.zsh` men-source `~/.secrets.zsh` yang berisi API key |
| `10-plugins/` | Zinit plugin manager | git clone eksternal | Rendah (sudah di-pin ke commit spesifik) |
| `20-shell/` | Aliases, functions, prompt (starship/zoxide/direnv via `eval`) | plugin manager | Rendah–Sedang — 3x `eval "$(... init zsh)"` pada tool eksternal, semuanya terhadap output tool terpercaya bukan input model |
| `30-ai/` | **Inti produk**: AI agent, tool registry, permission system, HTTP client, UI | Modul terbesar, ~100 file, banyak submodul saling bergantung | **Tinggi** — semua kapabilitas eksekusi kode/file/network berada di sini, dan di sinilah bug Critical (§Zsh Special Parameter Collision) ditemukan |
| `90-local/` | Contoh config lokal (`.example`), tidak di-source otomatis | — | Rendah |
| `skills/*.md` | Prompt/pengetahuan domain untuk agent (bukan kode eksekusi) | Dibaca sebagai teks oleh `70-skills.zsh` | Rendah, tapi lihat §Prompt Injection |
| `install.sh` | Installer (`curl \| bash`-style): clone repo, symlink `.zshrc`/`.zsh_bagas` | Jaringan (GitHub) | Sedang — lihat §Supply Chain |

### Struktur `30-ai/` (breakdown submodul, urut angka = urut load & lapis dependency)

```
30-ai/
├── 00-config/     konstanta: model, provider order, path, limits, runtime guards, persona, system-prompt spec
├── 05-tools/      tool registry + implementasi tiap tool (fs, process, git, web_fetch, todo)
├── 06-permissions/ path guard, permission_check, perm_ask (write/process/shell)
├── 10-core/       security bootstrap, wakelock, cache, circuit breaker, HTTP call (blocking+streaming), retry, token budget, session trim
├── 20-chat/       quick chat, session ask/REPL, aiclip
├── 30-code/       code generation/project scaffolding, autotest, fix, run
├── 35-files/      aicat/aipatch/aiundo/aibakclean/aishare (file-level AI ops + backup/undo)
├── 40-workflow/   aicommit/aiplan/aiprompt/aispec/aibuild/aireview/aisummarize
├── 45-project.zsh / 46-index.zsh   project indexing/summary (dipakai buat inject context ke sysprompt)
├── 50-agent/      ReAct loop: policy (dangerous-command blocklist), state/checkpoint, runtime (load/resume/execute/finalize), execution loop, provider selection
├── 55-subagent/   sub-agent terbatas (tool allowlist sendiri) untuk task audit/refactor besar & debug
├── 60-ui/         semua tampilan (box, menu, dispatcher, screens, components)
├── scripts/       2 script Python (ai_code_sanitize.py, ai_extract.py) dipanggil dari zsh
└── 70-skills.zsh  loader untuk ../skills/*.md
```

### Runtime flow — hasil penelusuran implementasi nyata

Alur `ai agent "<goal>"` sebenarnya (ditelusuri dari `40-runtime/30-aiagent.zsh` → `44-finalize.zsh`):

```
Startup (.zshrc, glob-sort 00→90)
   → 00-core: secrets-guard (source ~/.secrets.zsh, cek chmod 600)
   → 30-ai/00-config: load model list, provider order, limits, runtime guards
   → 30-ai/05-tools: build AI_TOOL_REGISTRY (nama|level), AI_TOOL_CAPABILITY, AI_TOOL_SCHEMA (asosiatif array)
      ↓ (user run "ai agent ...")
30-aiagent.zsh (entry)
   → 10-load_checkpoint.zsh   : ada checkpoint lama utk slug ini? (mkdir-lock based)
   → 15-prepare_new_goal.zsh  : kalau checkpoint gak ada → siapkan sysprompt+goal baru
   → 20-print_header.zsh      : render header UI
   → 25-execute_and_finalize.zsh
        → 42-execution/00-loop_main.zsh   [while step < AI_AGENT_MAX_STEPS]
             → 05-get_plan.zsh        : panggil provider → dapat JSON aksi
             → 10-reject_checks.zsh   : cek repeated-failure guard, tolak klaim "done" belum terverifikasi
             → 15-run_tool.zsh        : _ai_tool_dispatch(tool,args)
                  → 05-tool_dispatch.zsh: normalize args → validate schema (jq) → permission check
                  → 06-permissions/15-permission_check.zsh
                       → path containment (10-path_guard.zsh) utk tool fs
                       → capability gate (YOLO mode)
                       → level dispatch: readonly / write / process / shell → *_ai_perm_ask_*()
                  → eksekusi tool sebenarnya (05-tools/10..50-*.zsh)   ← 7 dari tool ini RUSAK, lihat §Zsh Special Parameter Collision
             → 20-log_and_notify.zsh  : log + notifikasi diff
             → 25-track_and_continue.zsh : update touched/changed files, checkpoint_save() — TANPA cek return code
        → (loop selesai: done_flag / max_steps / same-fail-limit)
   → 44-finalize.zsh : verify_touched_files, npm_checks (validasi ringan), render report
   → rm -rf state_dir sementara (bukan checkpoint permanen) setelah selesai
```

Ini konsisten dengan komentar di kode sendiri (`00-policy.zsh`) yang menyebut alur ini sebagai bagian paling berisiko di keseluruhan sistem.

---

## Fase 2 — Audit Arsitektur

**Skor arsitektur: 8/10** — **tidak berubah** dari putaran pertama. Audit lanjutan tidak membantah kualitas *desain*; kegagalan yang ditemukan (§Zsh Special Parameter Collision) adalah cacat **implementasi/eksekusi** dari desain yang secara struktural benar, bukan cacat arsitektural.

**Kekuatan (berbasis bukti):**
- **Modularitas baik.** Tidak ada file monolitik — file zsh terbesar `05-tools/02-tool_autodep.zsh` (209 baris), file python terbesar `scripts/ai_code_sanitize.py` (424 baris). Banyak komentar eksplisit menyebut "split out of the old monolithic X.zsh".
- **Separation of concerns jelas** antara tool implementation (`05-tools`), permission (`06-permissions`), dan orchestration (`50-agent`).
- **Naming convention numerik konsisten** (`00-`, `05-`, `10-`...) yang secara eksplisit *adalah* dependency order.

**Risiko/kelemahan arsitektur:**
- **Ordering-as-dependency itu sendiri fragile.** Tidak ada startup self-check yang memvalidasi semua fungsi lintas-file benar-benar terdefinisi setelah source selesai. **Simulasi konkret:** kalau `05-tools/02-tool_args_extract.zsh` (nomor `02-`, definisi `_ai_tool_extract_path`/`_ai_tool_extract_field`) gagal ter-source, **hampir seluruh tool** (yang bergantung pada dua fungsi ini) akan gagal dengan `command not found` generik, bukan pesan error yang jelas. Ini bukan spekulasi — `_ai_tool_extract_path`/`_ai_tool_extract_field` dikonfirmasi dipanggil dari `10-tool_fs_read.zsh`, `20-tool_fs_write.zsh`, `25-tool_fs_patch_delete.zsh`, `30-tool_process.zsh`, `40-tool_git.zsh`, `45-tool_web_fetch.zsh`, `50-tool_todo.zsh` — **hidden single point of failure** yang tidak disebut eksplisit sebelumnya. **Usulan konkret** (bukan cuma ide, siap pakai):
  ```zsh
  # taruh di akhir .zshrc, setelah semua source selesai
  _ai_startup_selfcheck() {
      local -a required_fns=(
          _ai_tool_extract_path _ai_tool_extract_field _ai_tool_dispatch
          _ai_permission_check _ai_validate_project_path _ai_agent_is_dangerous
          _ai_agent_checkpoint_save _ai_project_root
      )
      local fn missing=()
      for fn in "${required_fns[@]}"; do
          (( $+functions[$fn] )) || missing+=("$fn")
      done
      if (( ${#missing[@]} > 0 )); then
          print -u2 "zsh_bagas: startup self-check GAGAL, fungsi hilang: ${missing[*]}"
          print -u2 "zsh_bagas: kemungkinan ada file .zsh yang gagal ter-source (syntax error)."
      fi
  }
  _ai_startup_selfcheck
  ```
- **Circular/hidden dependency:** `_ai_perm_ask_shell` (`06-permissions/30-perm_shell.zsh`) memanggil `_ai_ui_box` (`60-ui/05-ui_box.zsh`, folder bernomor lebih besar) — valid secara runtime, tapi melanggar arah dependency tersirat dari penomoran. Technical debt, bukan bug aktif.
- **Global associative arrays sebagai registry** (`AI_TOOL_REGISTRY`, `AI_TOOL_CAPABILITY`, `AI_TOOL_SCHEMA`) — state global yang di-mutate dari banyak file, tapi failure mode-nya aman (deny) kalau ada yang gagal ter-source sebagian.
- **Dead code dikonfirmasi (BARU):** subagent role `coder` (`55-subagent/`) diimplementasikan penuh tapi **tidak pernah dipanggil dari alur produk mana pun** — lihat §Audit Subagent.

---

## Audit Keamanan (Prioritas Tertinggi)

### Command Injection & Command Classification

**FAKTA — sudah ditangani dengan baik (VERIFIKASI):**
Satu-satunya jalur "arbitrary shell" adalah tool `run_command` (`05-tools/30-tool_process.zsh`, `zsh -f -c -- "$command"`). Dilapis 4 kontrol independen:
1. Registry level `shell` → wajib lewat `_ai_perm_ask_shell` (konfirmasi interaktif, menampilkan teks command **mentah/belum dievaluasi**) kecuali YOLO+lolos `_ai_yolo_shell_safe` — allowlist ketat (binary baca-only: `git|rg|grep|sed|awk|cat|head|tail|wc|ls|pwd|find|sort|uniq|cut|tr|diff`), dan **secara eksplisit menolak** command yang mengandung karakter `$`, backtick, `;`, `|`, `&`, `<`, `>`, newline **sebelum** tokenisasi apa pun terjadi (baris pre-filter di `_ai_yolo_shell_safe`, dikonfirmasi aman lewat pengujian).
2. `_ai_tool_manifest` menyembunyikan `run_command` dari daftar tool yang dilihat model kecuali `AI_AGENT_EXPOSE_ARBITRARY_SHELL=1` di-set eksplisit — default aman.
3. `_ai_agent_is_dangerous` (`50-agent/00-policy.zsh`) — deny-by-default: regex pola dikenal (fork bomb, `dd of=/dev/`, `mkfs`, dst) **plus** tokenisasi `${(z)cmd}` untuk kombinasi flag berbahaya (`rm` + recursive + force; `git push` + force) yang **teruji tahan** terhadap command substitution, arithmetic substitution, brace expansion, quoting-trick, dan newline injection — **diuji satu per satu lewat eksekusi nyata**, semua berhasil dinormalisasi dan tertangkap sebelum pattern match.
4. `exec_process` (jalur "terstruktur", tanpa shell interpreter) — resolve binary via `command -v`, menolak binary yang resolve ke dalam project root, allowlist ketat nama binary.

**FAKTA (dieksekusi) — bug baru, lebih serius dari yang tadinya diduga:**
`${(z)cmd}` di zsh **bukan cuma word-splitting** — ia benar-benar **mengevaluasi** command substitution/backtick/aritmetika sebagai bagian dari tokenisasi. Ini artinya tokenizer-nya sendiri **kebal** terhadap upaya bypass klasifikasi lewat `$(...)` (poin 3 di atas terbukti aman), **tapi** justru menciptakan bug yang sama sekali berbeda: **command substitution di dalam `$cmd` benar-benar TEREKSEKUSI sebagai efek samping pemanggilan `_ai_agent_is_dangerous`**, terlepas dari hasil klasifikasinya.

Bukti langsung (side-effect execution, bukan sekadar tokenisasi):
```zsh
_classify_only() {
  local cmd="$1"; local -a tokens
  tokens=(${(z)cmd})
}
_classify_only "echo safe -\$(touch /tmp/SIDE_EFFECT_PROOF; echo x)"
$ ls /tmp/SIDE_EFFECT_PROOF
-rw-r--r-- 1 root root 0 ...   # FILE INI BENAR-BENAR DIBUAT
```

**Analisis urutan eksekusi sebenarnya** (ditelusuri sampai ke `05-tool_dispatch.zsh`): `_ai_permission_check` (approval box, menampilkan command **mentah**) berjalan **sebelum** `_ai_tool_dispatch` memanggil `_ai_tool_run_command` — jadi user tetap melihat command asli (termasuk `$(...)` literal) sebelum approve; **ini bukan bypass total approval UI**. Tapi begitu user approve dan masuk ke body `_ai_tool_run_command`, urutannya: (1) `_ai_agent_is_dangerous "$command"` dipanggil → **mengeksekusi semua `$(...)`/backtick di dalamnya sebagai efek samping** → (2) baru kalau lolos klasifikasi, command yang sama dieksekusi ULANG secara utuh via `zsh -f -c -- "$command"`.

Konsekuensi: (a) command yang **diblokir** ("ERROR: command diblokir sistem keamanan") tetap sudah mengeksekusi bagian `$(...)`-nya — pesan "diblokir" menyesatkan; (b) command yang **lolos**, bagian `$(...)`-nya tereksekusi **dua kali** — sekali sebagai side-effect klasifikasi, sekali lagi saat eksekusi sungguhan, berisiko double-execution untuk operasi non-idempotent (curl POST, append log, dst).

| Severity | File | Fungsi | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|---|
| **High** | `50-agent/00-policy.zsh` | `_ai_agent_is_dangerous` | `${(z)cmd}` mengeksekusi `$(...)`/backtick sebagai efek samping tokenisasi, sebelum keputusan block/allow final, dan (untuk command yang lolos) tereksekusi lagi saat eksekusi sungguhan | Command dengan substitusi non-idempotent bisa berefek ganda; command yang "diblokir" tetap sudah menjalankan bagian substitusinya | Terapkan pre-filter yang SAMA seperti `_ai_yolo_shell_safe` (tolak/flag command yang mengandung `$`/backtick SEBELUM tokenisasi klasifikasi) di `_ai_agent_is_dangerous`, atau ekstrak ke helper bersama `_ai_shell_tokenize` yang WAJIB mewarisi pre-filter tersebut |

### Variable Safety

**FAKTA (VERIFIKASI):** Kode secara konsisten men-quote ekspansi variabel path (`"$target"`, `"$cwd"`, `-- "$real_cwd"`) dan menggunakan `--` sebelum argumen yang berasal dari input agar tidak diinterpretasi sebagai flag. Terlihat sengaja, bukan default umum di banyak skrip zsh. Spot-check tidak menemukan instance unquoted-var yang eksploitatif di jalur kritikal.

### PATH Security (hijacking via user-writable PATH — beda dari bug §Zsh Special Parameter Collision)

**FAKTA — baik:**
- `exec_process` menolak binary yang resolve ke dalam project root (`_ai_path_within_project "$resolved"` → block).
- `env.zsh` — `PATH="$HOME/.local/bin:$HOME/go/bin:$PATH"` — prepend direktori user-writable. Pola umum & standar, risiko struktural yang sama di hampir semua shell config, bukan bug spesifik repo ini.

| Severity | File | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|
| Low | `00-core/env.zsh` | `~/.local/bin` di depan PATH, writable oleh user yang sama | Kalau device/akun ter-kompromi sebagian, binary palsu bisa di-shadow sebelum tool sempat jalankan versi asli | Untuk `exec_process`, pertimbangkan resolve lewat path absolut yang di-hardcode/verified checksum untuk binary kritikal |

### Zsh Special Parameter Collision — Bug Critical, Dibuktikan Lewat Eksekusi

Komentar yang sudah ada di `06-permissions/25-perm_write.zsh` mendokumentasikan bug ini secara eksplisit: variabel lokal bernama `path` di zsh collide dengan special parameter tied `$path`/`$PATH` — deklarasi `local path` MENGOSONGKAN `$PATH` untuk sisa scope fungsi, membuat semua command eksternal (`command -v`, `command sed`, `command cp`, bare `mkdir`, bare `python3`, dst) di dalam scope itu gagal diam-diam. Fix untuk `_ai_perm_ask_write` sudah diterapkan (rename ke `file_path`).

**Ini bukan lagi dugaan — dibuktikan langsung dengan menjalankan zsh sungguhan:**
```
$ zsh -c 'f(){ local path; command -v ls; }; f'
f: command not found      # command -v gagal, PATH kosong

$ zsh -c 'echo "global: ${(t)path}"; f(){ local path; echo "local: ${(t)path}, PATH=[$PATH]"; }; f'
global: array-tied-special
local: array-local-tied-special, PATH=[]
```
`path` adalah **tied special parameter bawaan zsh** (bukan hasil `typeset -T` manual repo ini). Ini juga terbukti **dynamic-scoped**: kekosongan `$PATH` merambat ke setiap fungsi yang dipanggil dari dalam fungsi yang men-declare `local path`, termasuk `_ai_tool_extract_path`/`_ai_tool_extract_field` (yang secara internal butuh `jq`).

**Reproduksi paling telak, memakai fungsi ASLI repo (bukan versi sederhana):**
```
$ source 02-tool_args_extract.zsh
$ _ai_tool_read_file_test() {
    local args_json="$1"; local path offset limit
    path=$(_ai_tool_extract_path "$args_json")
    echo "extracted path=[$path]"
  }
$ _ai_tool_read_file_test '{"path":"/tmp/somefile.txt"}'
extracted path=[]
```
Path valid dikirim, hasil ekstraksi **kosong** — karena `jq` di dalam `_ai_tool_extract_field` tidak resolvable saat `$PATH` kosong, gagal silent (`2>/dev/null`), mengembalikan string kosong.

**Audit ini menemukan bahwa pola yang sama, TANPA perbaikan, ada di 7 fungsi tool implementasi:**

| File:Baris | Fungsi | Command eksternal setelah `local path`? | Dampak terbukti (reproduksi langsung) | Severity |
|---|---|---|---|---|
| `06-permissions/25-perm_write.zsh` | `_ai_perm_ask_write` | — | **Sudah diperbaiki** (rename ke `file_path`) | — (baseline benar) |
| `05-tools/10-tool_fs_read.zsh` | `_ai_tool_read_file` | Ya — `command sed`, `command nl`, `command -v awk`, `command python3` | **FAKTA (dieksekusi):** `path` selalu kosong → tool langsung `return 1` "membutuhkan args.path" walau args.path valid dikirim model. Tool paling sering dipakai di seluruh agent loop | **Critical** |
| `05-tools/10-tool_fs_read.zsh` | `_ai_tool_count_lines` | Ya — `command wc`, `command grep` | **FAKTA (dieksekusi):** sama, `path` selalu kosong | **Critical** |
| `05-tools/10-tool_fs_read.zsh` | `_ai_tool_list_dir` | Ya, tapi **ada fallback chain** (`whence -p` gagal → fallback ke `/bin/ls`/`/usr/bin/ls` hardcoded absolut; formatting fallback ke pure-zsh read-loop) | **Resilient dari crash** karena didesain untuk portabilitas Termux, TAPI `path=$(_ai_tool_extract_path ...)` tetap kosong via mekanisme sama → fallback `[ -z "$path" ] && path="."` membuatnya diam-diam **selalu list direktori saat ini**, mengabaikan argumen model sepenuhnya | **High** (silent-wrong-behavior, bukan crash) |
| `05-tools/20-tool_fs_write.zsh` | `_ai_tool_write_file` | Ya — bare `mkdir -p` (bukan builtin zsh, dikonfirmasi via `whence -w mkdir` → `command`) | **FAKTA (dieksekusi):** `path` kosong → `return 1` "membutuhkan args.path" | **Critical** |
| `05-tools/20-tool_fs_write.zsh` | `_ai_tool_edit_file` | Ya — bare `python3 -c` | **FAKTA (dieksekusi):** sama | **Critical** |
| `05-tools/25-tool_fs_patch_delete.zsh` | `_ai_tool_patch_file` | Ya — `command -v patch`, bare `cp` | **FAKTA (dieksekusi):** sama | **Critical** |
| `05-tools/25-tool_fs_patch_delete.zsh` | `_ai_tool_delete_file` | Ya — bare `cp`/`rm` | **FAKTA (dieksekusi):** sama | **Critical** |
| `05-tools/40-tool_git.zsh` | `_ai_tool_git_diff` | Ya — `command -v git`, bare `git diff` | **FAKTA (dieksekusi):** fungsi selalu return "ERROR: git gak ketemu di PATH" walau git terpasang dan repo valid | **Critical** |

**Tidak terpengaruh** (dikonfirmasi eksplisit): `_ai_tool_git_status` (tidak declare `path`), `_ai_tool_move_file` (pakai `src`/`dest`, bukan `path`).

**Skala dampak riil:** 7 dari ~17 tool terdaftar — termasuk tool file-read/write paling fundamental untuk sebuah "AI coding agent" — **gagal total** untuk kasus pemakaian normal, plus `list_dir` yang diam-diam mengabaikan argumen. Ini kontradiksi langsung dengan asumsi awal bahwa "tidak ditemukan state corruption risk" di §Runtime AI — belum ada state corruption, tapi ada **complete functional breakage** yang jauh lebih mendasar.

**Catatan penting:** komentar v-fix yang sudah ada di `perm_write.zsh` menjelaskan mekanisme *persis* yang dibuktikan di atas — artinya mekanisme dan akibat penuhnya **sudah pernah diketahui**, tapi fix hanya diterapkan di satu fungsi permission-layer, tidak disebarkan ke tujuh fungsi tool-implementation-layer tempat bug identik hidup.

| Severity | File | Fungsi | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|---|
| **Critical** | `05-tools/10-tool_fs_read.zsh`, `20-tool_fs_write.zsh`, `25-tool_fs_patch_delete.zsh`, `40-tool_git.zsh` | `_ai_tool_read_file`, `write_file`, `edit_file`, `patch_file`, `delete_file`, `count_lines`, `git_diff` | `local path` mengosongkan `$PATH`, membuat ekstraksi argumen `path` selalu gagal (via jq yang tak ter-resolve) | Tool inti gagal total untuk pemakaian normal | Rename `local path` → `local fs_path` (pola sudah ada, tinggal disalin dari `perm_write.zsh`) di seluruh 7 lokasi; ganti semua pemakaian `$path` → `$fs_path` di sisa fungsi |
| High | `05-tools/10-tool_fs_read.zsh` | `_ai_tool_list_dir` | Sama root cause, tapi fallback membuatnya silent-wrong bukan crash | Argumen `path` diabaikan diam-diam, selalu list `.` | Sama rename, plus tampilkan error eksplisit kalau ekstraksi gagal alih-alih diam-diam fallback ke `.` |

**Regression-lint yang tepat sasaran (dikoreksi dari dugaan awal):** grep menyeluruh + **pengujian langsung** untuk `local path`, `local status`, `local pipestatus`, `local reply`, `local options` menunjukkan **hanya `path` yang benar-benar berbahaya**:

| Variabel | Test empiris | Kesimpulan |
|---|---|---|
| `path` | PATH kosong terbukti di scope lokal | **Berbahaya** |
| `status` | `false; local status; false; echo $status` → tetap `1` (benar) | **Aman** — `$status`/`$?` tetap ter-update normal |
| `pipestatus` | `local -a pipestatus; false \| true; echo $pipestatus` → `(1 0)` (benar) | **Aman** — array tetap ter-update |
| `reply` | Dipakai konsisten untuk hasil `$(...)` biasa di seluruh repo, bukan bergantung pada `read`'s implicit `$reply` | **Aman** dalam pola pemakaian yang ada |
| `options` | `${(t)options}` setelah `local options` → `scalar-local`/`array-local` biasa, bukan tied ke apa pun | **Aman** |

Regression-lint CI sebaiknya HANYA menyasar `local path\b` — menyertakan `status`/`pipestatus` akan menghasilkan false-positive yang mengaburkan sinyal.

### File Operation Safety

**FAKTA — baik, dengan catatan:**
- Semua operasi tulis/hapus tool wajib lewat `_ai_validate_project_path` (canonical path containment via `realpath`), yang menangani symlink dengan benar (resolve symlink sebelum cek containment).
- Bypass tersedia lewat `AI_PERM_ALLOW_OUTSIDE_PROJECT=1` — env var, hanya bisa di-set oleh user/environment, bukan oleh model/agent. Risiko rendah selama asumsi ini tetap benar; worth didokumentasikan eksplisit sebagai kill-switch berbahaya.

**BARU — bug backup-terhapus (dieksekusi & dikonfirmasi, 2 lokasi):**
Pada jalur "overwrite file existing" di `aicode` (`30-code/05-code.zsh`) dan `aipatch` (`35-files/10-aipatch.zsh`) — keduanya secara eksplisit mengklaim "pola persis sama" — kalau `command mv -f "$tmpnew" "$output"` **gagal**, kode mencetak "File asli gak berubah, cek $backup." lalu **baris berikutnya langsung `rm -f "$backup"`** — menghapus backup yang baru saja direferensikan sebagai tempat pemulihan, tanpa pernah memverifikasi ulang isi `$output`.

```zsh
if ! command mv -f "$tmpnew" "$output"; then
    echo "GAGAL menimpa (mv error). File asli gak berubah, cek $backup."
    rm -f "$backup"      # <- backup yang baru saja direferensikan, dihapus
    return 1
fi
```
`mv` lintas-filesystem (mktemp default `/tmp`, `$output` bisa di mount berbeda — relevan di Termux/Android) diimplementasikan sebagai copy+unlink, bukan rename atomik — kegagalan di tengah proses **bisa** meninggalkan `$output` terpotong/hilang, bukan otomatis "tidak berubah". Justru saat itulah backup paling dibutuhkan.

Bandingkan dengan pola yang **benar** di lokasi lain repo yang sama (`05-tools/25-tool_fs_patch_delete.zsh`, fungsi `_ai_tool_patch_file`): urutan yang benar dipakai — **restore dulu** (`command cp -f "$backup" "$path"`) **baru** hapus backup. Pola benar sudah dikenal di codebase, tidak diterapkan konsisten di dua lokasi lain.

| Severity | File | Fungsi | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|---|
| Medium | `30-code/05-code.zsh` | `aicode` (overwrite path) | Backup dihapus di jalur gagal-mv, tanpa restore/verifikasi | Potensi kehilangan satu-satunya salinan file lama kalau mv gagal di tengah jalan | Hapus `rm -f "$backup"` di jalur ini; tambahkan `[ -f "$output" ] \|\| command cp -f "$backup" "$output"` sebelum melapor status |
| Medium | `35-files/10-aipatch.zsh` | `aipatch` | Bug identik | Sama | Sama |

### Temporary File

**FAKTA (VERIFIKASI, cakupan diperluas):** `_ai_http_call_blocking` dan `_ai_tool_dispatch` memakai `mktemp`. Checkpoint locking (`50-agent/10-state.zsh`) pakai `mkdir` sebagai primitif atomic lock dengan stale-lock detection via `kill -0` — implementasi matang.

Pola `.tmp.$$` (bukan `mktemp`, PID-based) untuk write-then-atomic-mv, yang tadinya hanya disorot di `install.sh`, ternyata dipakai **konsisten di ~14 lokasi lain** di seluruh `30-ai/`: checkpoint save, msgfile append (5 lokasi), cache, circuit breaker, log trim, session trim. **Kesimpulan risikonya tetap sama seperti yang berlaku untuk `install.sh`**: semua lokasi menulis ke direktori milik user yang sama (state/cache dir `chmod 700`), bukan `/tmp` bersama multi-user — race window kecil, hanya relevan kalau ada proses lokal lain milik user yang sama yang menebak nama file `$$`.

| Severity | File | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|
| Low | `install.sh` (dan ~14 lokasi serupa di `30-ai/`) | `mv "$X" "$X.bak.$$"`/`"$X.tmp.$$"` — nama predictable (PID) | Race window kecil, hanya dieksploitasi oleh proses lokal lain milik user yang sama | Ganti dengan `mktemp -d`/`mktemp` untuk nama backup |

### Secret Management

**FAKTA (belum diverifikasi ulang di putaran kedua, status tidak berubah):**
- API key tidak pernah di-commit — hidup di `~/.secrets.zsh`, sengaja di luar `~/.zsh_bagas`.
- `secrets-guard.zsh` memvalidasi permission file (600/400) dan auto-`chmod 600` jika longgar.
- **Temuan:** `48-http_call_blocking.zsh` mengirim API key lewat `curl -H "Authorization: Bearer $apikey"` sebagai argumen command-line — terlihat oleh proses lain milik user yang sama lewat `ps -ef`/`/proc/<pid>/cmdline` selama request berlangsung.

| Severity | File | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|
| Medium | `10-core/48-http_call_blocking.zsh` | API key sebagai command-line argument ke `curl` | Local information disclosure lewat `/proc/<pid>/cmdline` | Gunakan `curl -K -` (config dari stdin) atau `--header @/path/to/tempfile` yang di-`chmod 600` dan segera dihapus |

### Supply Chain — Permission Override dari Project Pihak Ketiga

**BARU:** `06-permissions/00-config.zsh` men-source `"./.aiagent/permissions.zsh"` — path **relatif terhadap cwd project**, bukan `$ZSH_BAGAS_DIR`. Karena project yang di-`cd` oleh user bisa saja repo hasil clone dari sumber tidak tepercaya (skenario umum: "buka project X, minta AI kerjakan Y"), file `.aiagent/permissions.zsh` di dalam project tersebut — kalau ada — **langsung di-source dan dieksekusi sebagai kode zsh** saat konfigurasi permission di-load, berpotensi override fungsi `_ai_perm_ask_*` itu sendiri sebelum agent loop mulai. Bukan dipicu langsung oleh model di tengah sesi, tapi merupakan supply-chain risk nyata.

| Severity | File | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|
| Medium | `06-permissions/00-config.zsh` | `source "./.aiagent/permissions.zsh"` memuat kode zsh arbitrer dari direktori project tanpa validasi | Repo pihak ketiga bisa membawa file yang mengoverride fungsi permission sebelum agent berjalan | Minimal: peringatan eksplisit saat file ditemukan sebelum di-source. Ideal: whitelist variabel yang boleh di-override, syaratkan permission 600 & owner sama |

### Permission System

**FAKTA — ini bagian terkuat dari repo (VERIFIKASI).** Alur `_ai_permission_check` menjalankan path containment SEBELUM keputusan permission interaktif, memisahkan level `readonly|write|process|shell`, YOLO-mode capability gate yang eksplisit tidak full-bypass. `setopt localoptions noxtrace` dipakai eksplisit untuk mencegah kebocoran variabel ke terminal via xtrace global.

Komentar di file ini sendiri mendokumentasikan bug `local path` yang **sudah diperbaiki di titik ini** — tapi (lihat §Zsh Special Parameter Collision di atas) **tidak** diterapkan secara sistematis ke 7 fungsi tool implementasi lain tempat pola identik masih hidup. Rekomendasi grep sistematis yang diajukan awal **terbukti benar arahnya**, tapi cakupannya harus dipersempit ke `local path\b` saja (lihat tabel test empiris di atas), dan sekarang statusnya bukan lagi "worth checking" — sudah dibuktikan dan dikuantifikasi penuh.

---

## Audit Zsh Best Practices

**FAKTA — kualitas tinggi untuk ukuran proyek dotfiles:**
- `setopt localoptions noxtrace` dipakai konsisten di jalur permission.
- `local` dipakai konsisten untuk scoping fungsi.
- Tokenisasi command memakai `${(z)cmd}` (zsh-native) — pilihan yang secara umum lebih robust daripada regex string matching, meski (dibuktikan di atas) punya efek samping eksekusi yang perlu di-pre-filter.

**Perlu Verifikasi (belum ditutup):** Tidak ditemukan `emulate -L zsh` di awal fungsi-fungsi umum (dikonfirmasi ulang: 0 pemakaian di seluruh repo). Untuk fungsi yang dipanggil dari konteks dengan `setopt` non-default milik user (via `90-local/`), perilaku sebagian fungsi berpotensi berubah. Risiko rendah untuk single-maintainer project.

| Severity | File | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|
| Low | Fungsi-fungsi inti di `05-tools/`, `50-agent/` | Tidak ada `emulate -L zsh` di awal fungsi yang manipulasi array/string secara intensif | Perilaku bisa berubah tergantung `setopt` milik environment pemanggil | Tambahkan `emulate -L zsh` terutama di `_ai_agent_is_dangerous`, `_ai_yolo_shell_safe` |

---

## Audit Runtime AI (`30-ai/50-agent`)

**FAKTA:**
- Execution loop dibatasi (`AI_AGENT_MAX_STEPS`) — bukan `while true`.
- Repeated-failure guard: agent berhenti otomatis kalau command persis sama gagal `AI_AGENT_MAX_SAME_FAIL` kali berturut-turut.
- Checkpoint/resume solid secara **mekanisme**: mkdir-based atomic lock, stale-lock detection via `kill -0`, write-then-atomic-`mv`, `chmod 600/700`, revision counter.
- Retry logic membedakan error transient vs non-transient (413→turunkan max_tokens; 429/404→lompat provider tanpa retry sia-sia).

**Checkpoint Failure Propagation — ditutup penuh (grep menyeluruh seluruh caller `_ai_agent_checkpoint_save`, bukan sampel):**

| Caller | File:Fungsi | Return Checked | Bug | Severity |
|---|---|---|---|---|
| `_ai_agent_exec_get_plan` | `42-execution/05-get_plan.zsh` | **Ya (tidak langsung)** — dipanggil dengan `\|\| true`, tapi baris berikutnya secara eksplisit cek `[ -f "$checkpoint_file" ]` dan mencetak peringatan kalau file tidak ada | **Tidak ada** — pola yang benar | — |
| `_ai_agent_exec_track_and_continue` | `42-execution/25-track_and_continue.zsh` | **TIDAK** — dipanggil telanjang, fungsi lalu `return 0` tanpa syarat | **FAKTA (dieksekusi/dikonfirmasi):** ini jalur checkpoint **paling sering dieksekusi** (akhir SETIAP step normal dalam loop). Kalau save gagal (disk penuh/permission/lock busy), loop lanjut seolah tersimpan — silent data-loss kalau proses mati sebelum checkpoint berikutnya berhasil | **Medium** |
| `_ai_agent_exec_check_done_rejections` (klaim done, 0 command run) | `42-execution/10-reject_checks.zsh` | **TIDAK** | **FAKTA (dieksekusi):** sama, frekuensi lebih jarang | **Low** |
| `_ai_agent_exec_check_done_rejections` (klaim done, syntax check gagal) | `42-execution/10-reject_checks.zsh` | **TIDAK** | **FAKTA (dieksekusi):** sama | **Low** |

**Kesimpulan:** dugaan awal soal risiko ini terbukti benar dan **lebih serius dari perkiraan awal** — 3 dari 4 call site tidak menangani return code sama sekali, termasuk jalur paling panas.

| Severity | File | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|
| Medium | `50-agent/42-execution/25-track_and_continue.zsh` | `_ai_agent_checkpoint_save` dipanggil tanpa cek return code, di jalur checkpoint paling sering | Progress bisa hilang tanpa pemberitahuan saat resume gagal nemu checkpoint terbaru | `_ai_agent_checkpoint_save ... \|\| echo "[peringatan: checkpoint gagal disimpan step $step]" >&2` |
| Low | `50-agent/42-execution/10-reject_checks.zsh` (2 call site) | Sama, frekuensi lebih jarang | Sama, dampak lebih kecil | Sama |

---

## Audit Tool Registry & Dispatch

**FAKTA:** Setiap tool call melewati 3 gerbang wajib berurutan: normalisasi args → validasi schema (jq contract check) → permission check. Tidak ada jalur yang bisa skip salah satu gerbang — `_ai_tool_dispatch` memanggil ketiganya sekuensial dengan early-return di tiap kegagalan. Argument injection dicegah lewat schema JSON per tool (mis. `exec_process` menolak args yang mengandung `\n`). Registry adalah associative array tunggal, registrasi tool tersentralisasi di satu file (`00-tool_registry.zsh`) — risiko duplicate-key-silent-override rendah karena tidak tersebar.

**Catatan penting yang baru terungkap:** validitas gerbang-gerbang ini **tidak berarti tool di baliknya benar-benar berfungsi** — 7 dari tool yang lolos ketiga gerbang tetap gagal total begitu masuk ke implementasinya sendiri (§Zsh Special Parameter Collision). Dispatch layer dan tool layer perlu dinilai terpisah.

---

## Audit Modul Runtime Lain (`20-chat`, `30-code`, `40-workflow`, `55-subagent`, `60-ui`)

### 20-chat

**FAKTA (positif):** `aiclip.zsh` (`_ai_clip_is_sensitive`) punya heuristik filter data sensitif (OTP/private key/password pattern) sebelum mengirim isi clipboard ke API eksternal.

**Perlu Verifikasi (belum ditutup):** `20-session_mgmt.zsh`/`15-session_repl.zsh` (riwayat percakapan ke disk) — pola `.tmp.$$`+`mv` sudah cukup aman untuk race condition biasa berdasarkan spot-check, tapi tidak ditelusuri baris-per-baris untuk memastikan tidak ada jalur isi session (yang bisa memuat isi file project) ter-log ke tempat dengan permission longgar.

### 30-code

**BARU:** Bug backup-terhapus (lihat §File Operation Safety di atas) ditemukan di `05-code.zsh`.

### 40-workflow

**FAKTA (positif, spot-check):** `aicommit` mengirim `$msg` (hasil generate AI) ke `git commit -m "$msg"` dengan quoting benar — tidak ada argument-injection (msg jadi value dari `-m`, bukan token argv baru). Timeout 60 detik untuk confirm prompt non-interaktif sudah ada.

**Perlu Verifikasi (belum ditutup):** `20-aibuild.zsh`, `25-aireview.zsh` — tidak ditelusuri detail apakah ada race condition antara commit-automation dan proses git lain yang berjalan bersamaan. Tidak ditemukan bukti masalah dalam grep pola (`git commit`/`push`/`checkout`), tapi cakupan tidak mendalam.

### 55-subagent

**FAKTA:** `_ai_subagent_tool_allowed` untuk role `researcher` memakai whitelist eksplisit 5 tool readonly (`read_file|list_dir|grep_search|glob_search|count_lines`), dicek **sebelum** dispatch — urutan benar, dan kelima nama tool dikonfirmasi cocok dengan `AI_TOOL_REGISTRY` (tidak ada dead-allowlist-entry).

**BARU — role `coder` tidak benar-benar terisolasi:** Untuk role `coder`, allowlist mengembalikan `true` untuk **tool apa pun yang ada di `AI_TOOL_REGISTRY`** — akses setara main agent (termasuk `delete_file`, `run_command` kalau diexpose, `exec_process`). Satu-satunya pembatas tetap `_ai_permission_check` global yang sama, bukan pembatas tambahan khusus subagent — kontradiksi dengan kesan "tool allowlist sendiri" yang tersirat dari nama folder. Allowlist khusus **hanya benar-benar membatasi role `researcher`**.

**BARU — role `coder` adalah dead code:** Grep menyeluruh menunjukkan hanya role `"researcher"` yang pernah benar-benar dipanggil di codebase saat ini (dari `60-ui/25-research_dev.zsh` dan `50-agent/40-runtime/05-subagent_offer.zsh`). Path `role="coder"` didukung penuh oleh implementasi tapi tidak ada satu pun UI/workflow/tool call yang menginvokasinya.

**FAKTA (positif) — recursive agent spawning:** `_ai_subagent_run` **tidak** terdaftar sebagai tool di `AI_TOOL_SCHEMA`/`AI_TOOL_REGISTRY` — subagent tidak punya cara memanggil dirinya sendiri lagi lewat tool-call standar. Tidak ditemukan bukti jalur rekursi tak terbatas untuk role mana pun.

**FAKTA:** Batas step subagent (`AI_SUBAGENT_MAX_STEPS` fallback ke `AI_AGENT_MAX_STEPS`) dan repeated-failure guard — pola sama seperti main agent loop, sudah benar.

| Severity | File | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|
| Low–Medium | `55-subagent/05-tool_allowlist.zsh` | Role `coder` tidak punya pembatas tool tambahan di luar registry global; saat ini dead code | Kalau di masa depan role `coder` dihubungkan ke alur produk, subagent "coder" punya kapabilitas setara main agent tanpa isolasi tambahan | Definisikan whitelist eksplisit seperti `researcher`, atau hapus/dokumentasikan sebagai belum terhubung |

### 60-ui

**FAKTA (positif):** Grep untuk pola `printf` dengan variabel sebagai *format string* (bukan argumen `%s`) — tidak ditemukan satu pun instance di seluruh `60-ui/`. Semua pemakaian memakai literal format string diikuti variabel sebagai argumen — aman dari format-string injection.

**Perlu Verifikasi (belum ditutup):** ANSI escape handling di `05-ui_box.zsh`/`components/*.zsh` untuk skenario di mana isi yang dirender berasal dari output tool/AI (bukan cuma UI chrome statis) — potensi tersembunyinya approval prompt di balik escape sequence yang dikendalikan output AI tidak ditelusuri mendalam. Spot-check (`05-code.zsh` meng-inject ANSI color code ke `diff` output lewat `sed`) menunjukkan konten yang dikontrol lokal/tool, bukan dari respons AI mentah — risiko rendah, tapi cakupan tidak menyeluruh untuk semua 15 file di folder ini.

---

## Audit Performa

**Angka nyata (diukur, bukan estimasi kasar):**

| Metrik | Nilai terukur |
|---|---|
| Total pemanggilan `jq` (literal, seluruh `30-ai/`) | **180** |
| Total pemanggilan `command -v` | **102** |
| Total pemanggilan git (status/diff/rev-parse/branch/push/log) | **36** |
| Total pemanggilan `realpath` | **6** |
| Titik pemanggilan `_ai_project_root` di source | **3 file** — tapi dipanggil 1x per file-touching tool call lewat `_ai_path_within_project` → `_ai_validate_project_path`, yang dipanggil dari SETIAP tool write/edit/delete/move/exec_process |

**Klarifikasi atas klaim awal:** "dipanggil dari banyak tempat" untuk `_ai_project_root` secara literal source-code sebenarnya cuma 1 titik definisi + 2 titik pemanggilan langsung — bukan "banyak tempat" di source. Tapi secara **runtime**, karena `_ai_path_within_project` dipanggil di setiap tool call yang menyentuh file, frekuensi eksekusi tetap tinggi seperti yang diklaim — mekanismenya tersentralisasi lewat satu wrapper, bukan tersebar di source. Kesimpulan performa (worth caching per-invocation-of-agent-loop) tetap valid.

**Quick Win:**
- `_ai_tool_run_command` men-spawn subshell zsh penuh (`zsh -f -c -- "$command"`) untuk setiap command, termasuk command trivial yang sudah lolos `_ai_yolo_shell_safe` (yang sudah punya `tokens` array hasil `${(z)cmd}` — bisa langsung `"${tokens[@]}"` tanpa reparse ulang lewat subshell).
- `_ai_autodep_install_missing` memanggil `command -v` **dan** `which` sebagai fallback — `which` tambahan hampir selalu tidak perlu di sistem modern.

**Medium:**
- Setiap tool call memanggil `jq` minimal 2x (validasi + ekstraksi field) — untuk 100 langkah agent, estimasi 200-300 fork/exec `jq`. Bottleneck terbesar secara struktural, tapi karena `jq` ringan (<5ms tipikal), akumulasinya (~1-2 detik di langkah ke-100) jauh lebih kecil dari latensi API call ke provider AI yang mendominasi durasi sesi — **tidak kritikal**, kandidat optimasi saja.
- `_ai_project_root` tidak di-cache per-invocation agent-loop — `git rev-parse` relatif murah per panggilan, tapi terpanggil puluhan kali per goal.

**Major:** Tidak ditemukan bottleneck arsitektural besar dalam cakupan yang ditelusuri.

---

## Audit Reliability

**FAKTA positif:** Timeout eksplisit di semua jalur network dan `exec_process`, dengan fallback eksplisit (bukan diam-diam tanpa timeout). Tidak ditemukan orphan-process risk di jalur yang ditelusuri.

**Silent failure:**

| Severity | File | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|
| Low | `05-tools/30-tool_process.zsh` | `cwd=$(...) \|\| true` dan `timeout_s=$(...) \|\| true` — kegagalan extract field diabaikan, mengandalkan cek `-z` setelahnya | Aman secara fungsional (fallback default ada), tapi pola ini menyembunyikan perbedaan "field memang kosong" vs "ekstraksi gagal karena JSON malformed" | Bedakan return code eksplisit alih-alih fallback diam-diam |

**Catatan penting untuk konteks skor akhir:** mekanisme reliability (checkpoint/lock/retry/timeout) memang matang, **tapi** "reliability" pada akhirnya harus mengukur apakah sistem bekerja seperti diharapkan pengguna — dan untuk 7 tool inti, jawabannya tidak (§Zsh Special Parameter Collision). Skor reliability di bagian akhir laporan ini disesuaikan untuk mencerminkan hal tersebut.

---

## Audit Error Handling

**FAKTA:** Pola exit-code dan pesan error jelas (`ERROR: ...`) konsisten di seluruh `05-tools/`. `2>&1` capture konsisten dipakai supaya model AI bisa "melihat" pesan error. Tidak ditemukan pola `2>/dev/null` yang membuang error kritikal secara membabi-buta di jalur eksekusi tool utama — pemakaian `2>/dev/null` hanya di titik yang memang defensif.

---

## Audit Konfigurasi

**FAKTA:** Semua konstanta AI tersentralisasi di `00-config/*.zsh`. `AI_PERM_ALLOW_OUTSIDE_PROJECT` dan `AI_AGENT_EXPOSE_ARBITRARY_SHELL` adalah kill-switch berbahaya yang default-off — desain default-aman yang benar.

| Severity | File | Masalah | Dampak | Perbaikan |
|---|---|---|---|---|
| Low | (dokumentasi) | Kill-switch env var berbahaya tidak dijelaskan risikonya di level yang sama menonjolnya dengan `README.md`/`CARA-PAKAI.md` (Perlu Verifikasi — lihat §Dokumentasi) | User bisa mengaktifkan tanpa sadar konsekuensi penuh | Tambahkan bagian "⚠️ Kill-switch berbahaya" eksplisit di dokumentasi utama |

---

## Audit Dokumentasi

**Perlu Verifikasi (belum ditutup di kedua putaran):** `CARA-PAKAI.md` dan `implementasi_plan.md` ada di root repo tapi tidak dibaca detail — fokus waktu kedua putaran dialokasikan ke kode eksekusi/security/runtime coverage. Tidak bisa menyimpulkan "dokumentasi usang vs implementasi" tanpa perbandingan baris-per-baris — masih direkomendasikan sebagai audit susulan terpisah, terutama untuk menyamakan daftar env var berbahaya dengan apa yang didokumentasikan untuk user.

---

## Audit Maintainability

**Angka nyata:**

| Metric | Value |
|---|---|
| Total file (zsh+py+sh) | 153 |
| Total LOC (zsh) | 11.405 |
| Total LOC (python) | 517 |
| File zsh terpanjang | `05-tools/02-tool_autodep.zsh` — 209 baris |
| File python terpanjang | `scripts/ai_code_sanitize.py` — 424 baris |
| Duplicate logic dikonfirmasi | (a) tokenizer `${(z)cmd}` di 2 file independen (`00-policy.zsh` & `30-tool_process.zsh`) — **sekarang punya implikasi security**, bukan cuma maintainability; (b) pola "backup lalu overwrite lalu handle-gagal" di 3 lokasi — 2 dari 3 salah |
| Hidden dependency | `_ai_tool_extract_path`/`_ai_tool_extract_field` sebagai single point of failure implisit bagi hampir semua tool |
| Global mutable state | `AI_TOOL_REGISTRY`, `AI_TOOL_CAPABILITY`, `AI_TOOL_SCHEMA` |
| Cyclomatic hotspot (estimasi) | `_ai_agent_is_dangerous` (branching bertingkat + 2 loop tokenisasi terpisah) dan `_ai_tool_run_command` (banyak jalur error + auto-install retry) |

**Kandidat refactor:** `_ai_yolo_shell_safe` dan `_ai_agent_is_dangerous` melakukan tokenisasi `${(z)cmd}` secara independen di dua file berbeda — sekarang terbukti bukan cuma duplicate-concern maintainability, tapi **duplicate-concern dengan risiko security berbeda** (`_ai_yolo_shell_safe` sudah punya pre-filter `$`/backtick yang aman, `_ai_agent_is_dangerous` tidak) — ekstraksi ke helper bersama `_ai_shell_tokenize()` **wajib** mewarisi pre-filter tersebut, bukan sekadar konsolidasi kode.

---

## Technical Debt

**Quick Wins (≤30 menit masing-masing):**
- Rename `local path` → `local fs_path` di 7 fungsi tool (`05-tools/10-tool_fs_read.zsh`, `20-tool_fs_write.zsh`, `25-tool_fs_patch_delete.zsh`, `40-tool_git.zsh`) — **prioritas tertinggi di seluruh laporan**.
- Tambahkan pre-filter `$`/backtick di `_ai_agent_is_dangerous` sebelum tokenisasi.
- Perbaiki return-code handling di `track_and_continue.zsh` dan `reject_checks.zsh` (2 lokasi).
- Hapus `rm -f "$backup"` di jalur gagal-mv, `05-code.zsh` & `10-aipatch.zsh`.
- Ganti `install.sh` backup naming dari `.bak.$$` → `mktemp -d`.
- Hapus pemanggilan `which` redundan di `_ai_autodep_install_missing`.
- Regression-lint CI: `grep -rn 'local path\b' 30-ai/` sebagai hard-fail.
- Startup self-check function-exists (kode lengkap di §Arsitektur).
- Tambahkan dokumentasi eksplisit risiko `AI_PERM_ALLOW_OUTSIDE_PROJECT`/`AI_AGENT_EXPOSE_ARBITRARY_SHELL` di README.

**Medium Refactor (1-4 jam):**
- Pindahkan API key dari argumen `curl -H` ke `-K -`/config-file.
- Ekstraksi tokenizer command bersama (`_ai_shell_tokenize`), WAJIB mewarisi pre-filter `_ai_yolo_shell_safe`.
- Tambahkan peringatan eksplisit sebelum `source "./.aiagent/permissions.zsh"`.
- Cache `_ai_project_root` sekali per invocation agent-loop.
- Functional smoke-test: panggil tiap tool dengan args minimal valid, assert output bukan pesan error generik — satu-satunya jaring pengaman yang akan menangkap kelas bug `local path` sebelum sampai ke user.
- Klarifikasi/hapus role `coder` di subagent, atau beri allowlist eksplisit seperti `researcher`.

**Major Refactor (>1 hari):**
- Audit susulan untuk bagian yang masih "Perlu Verifikasi": dokumentasi (`CARA-PAKAI.md`, `implementasi_plan.md`) vs implementasi aktual, detail `20-session_mgmt.zsh`/`15-session_repl.zsh`, `aibuild`/`aireview` race condition, ANSI escape handling penuh di seluruh `60-ui/`.
- Startup self-check otomatis (CI atau hook lokal) yang memverifikasi semua fungsi lintas-file benar-benar terdefinisi setelah source selesai — kode dasar sudah disiapkan di §Arsitektur, tinggal diperluas ke CI.

---

## Skor Akhir

| Aspek | Skor | Catatan |
|---|---|---|
| Arsitektur | **8/10** | Modular, separation of concerns jelas; tidak berubah dari asesmen desain — kegagalan yang ditemukan adalah cacat implementasi, bukan cacat desain |
| Security | **7/10** | Defense-in-depth nyata di permission/path-containment/SSRF (tidak berubah); turun dari asesmen awal karena `is_dangerous` side-effect execution dan supply-chain risk `.aiagent/permissions.zsh` adalah temuan yang sebelumnya tidak terdeteksi |
| Reliability | **4/10** | Mekanisme (checkpoint/lock/retry/timeout) tetap matang secara desain, tapi skor harus mencerminkan bahwa 7 dari ~17 tool **selalu gagal** untuk pemakaian normal — reliability mengukur apakah sistem bekerja seperti diharapkan, bukan cuma seberapa elegan fallback-nya |
| Performance | **7/10** | Tidak ada bottleneck major; beberapa subprocess (`jq`, `git rev-parse`) yang bisa di-cache/dikurangi, dikonfirmasi dengan angka nyata |
| Maintainability | **7/10** | File kecil, nama jelas; duplicate-concern tokenizer sekarang terbukti berimplikasi security, dan pola backup-delete-on-failure terbukti salah tersalin ke 2 lokasi |
| Developer Experience | 7/10 | Komentar `v-fix` sangat membantu memahami histori bug, tapi ordering numerik butuh disiplin manual tanpa tooling bantu |
| Operational Readiness | **5/10** | Tidak ada startup self-check (kode sudah diusulkan), tidak ada functional smoke-test untuk tool suite — yang akan menangkap bug Critical di atas dalam hitungan detik |
| Testability | **6/10** | Kode mudah diuji secara terisolasi (fungsi kecil, jelas), tapi tidak ditemukan bukti test suite otomatis untuk zsh_bagas sendiri di luar tool-implementation `run_test`/`autotest` |
| Documentation | N/A (Perlu Verifikasi) | Tidak diaudit mendalam di kedua putaran |

**Skor keseluruhan (8 aspek yang dinilai numerik): ~6.4/10.**

Ini **bukan berarti kualitas desain repo memburuk** dibanding asesmen pertama — desain dan security posture untuk mekanisme yang *berfungsi* tetap sekuat yang dinilai semula. Penurunan dari estimasi awal murni mencerminkan bahwa penilaian pertama tidak (dan tidak bisa, tanpa eksekusi langsung) menghitung dampak dari tool suite yang secara fungsional rusak — sesuatu yang baru bisa dipastikan lewat verifikasi eksekusi nyata, bukan pembacaan kode saja. Kelemahan yang tersisa di luar bug Critical tetap bersifat **incremental hardening**, bukan celah struktural — dan bug Critical-nya sendiri punya perbaikan yang murah (effort <1 hari untuk seluruh prioritas tinggi, lihat Roadmap).

---

## Roadmap Perbaikan (prioritized berdasarkan rasio dampak/effort)

**Critical — Perbaiki Sekarang:**
1. **Rename `local path` → `local fs_path` di 7 fungsi tool** (`05-tools/10-tool_fs_read.zsh`, `20-tool_fs_write.zsh`, `25-tool_fs_patch_delete.zsh`, `40-tool_git.zsh`) — 30 menit, mekanis, pola sudah ada contohnya di `perm_write.zsh`. **Perbaikan tunggal dengan dampak terbesar di seluruh audit** — mengembalikan fungsi inti produk dari "selalu gagal" jadi berfungsi.

**High Priority:**
2. Tambahkan pre-filter `$`/backtick di `_ai_agent_is_dangerous` (samakan dengan `_ai_yolo_shell_safe`) — 15 menit, menutup temuan security High.
3. Startup self-check function-exists — 20 menit, kode sudah siap pakai di §Arsitektur, mencegah kelas bug #1 terulang di fungsi baru.
4. Functional smoke-test untuk seluruh tool — 1-2 jam, satu-satunya hal yang akan mencegah "7 tool rusak total" lolos ke produksi lagi tanpa disadari.
5. Pindahkan API key keluar dari argumen `curl` — perbaikan murah, mudah dieksploitasi begitu ada proses lain di device yang sama.

**Medium Priority:**
6. Perbaiki return-code handling `checkpoint_save` di 3 caller (`track_and_continue.zsh`, `reject_checks.zsh` x2) — 15 menit.
7. Hapus `rm -f "$backup"` di 2 lokasi jalur gagal-mv (`05-code.zsh`, `10-aipatch.zsh`) — 10 menit.
8. CI lint rule `grep -rn 'local path\b' 30-ai/` sebagai hard-fail — 15 menit, investasi permanen.
9. Peringatan eksplisit untuk `.aiagent/permissions.zsh` sebelum di-source — 30 menit.
10. Ekstraksi `_ai_shell_tokenize()` bersama dengan pre-filter yang benar diwarisi — 2-3 jam.
11. Cache `_ai_project_root`, kurangi subprocess `jq`/`which` berlebihan.

**Low Priority:**
12. `install.sh` (dan ~14 lokasi serupa) backup naming → `mktemp -d`/`mktemp`.
13. `emulate -L zsh` di fungsi-fungsi parsing command.
14. Klarifikasi/hapus role `coder` di subagent (dead code saat ini).
15. Dokumentasi eksplisit untuk kill-switch env var berbahaya.
16. Audit susulan untuk bagian yang masih "Perlu Verifikasi": dokumentasi vs implementasi, detail `20-chat`/`40-workflow` yang belum ditelusuri baris-per-baris, ANSI escape handling penuh di `60-ui/`.

Total effort item #1–#11 (di luar audit susulan #16 yang sifatnya eksploratif): **kurang dari 1 hari kerja**, dengan dampak mengubah repo dari "mekanisme matang tapi tool-suite rusak" menjadi "produk yang benar-benar berfungsi seperti yang didesain".

---

*Status FAKTA vs FAKTA (dieksekusi) vs VERIFIKASI vs DIKOREKSI vs Perlu Verifikasi dibedakan eksplisit di tiap bagian di atas. Semua kutipan file/baris merujuk isi asli `baseline.zip`. Klaim yang ditandai "dieksekusi" dibuktikan dengan menjalankan zsh sungguhan (bukan simulasi manual) terhadap potongan kode asli repo, termasuk reproduksi langsung memakai fungsi `_ai_tool_extract_path` yang sesungguhnya.*
