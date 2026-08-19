# Audit UI/UX & Workflow — Seluruh Command Repository `zsh_bagas`

**Sumber:** `baseline.zip` (snapshot `zsh_bagas-main`, ~150 file `.zsh`, ~11.400 baris) + `audit.md` (brief audit).
**Metode:** Static code audit (bukan eksekusi live) — setiap temuan menyebut file sumbernya. Item yang butuh pengujian interaktif nyata di terminal ditandai **[Perlu Verifikasi]**.
**Legenda:** 🟢 Fakta (terverifikasi dari kode) · 🟡 Perlu Verifikasi (butuh uji manual terminal) · 🔵 Rekomendasi

**Revisi 4 — perubahan dari draf sebelumnya:** menambahkan §14.0 (rubrik severity Critical/High/Medium/Low eksplisit, dipakai mengaudit ulang seluruh Friction Matrix — `airun` naik jadi High yang pasti, diff-color naik jadi Medium karena kebocoran aksesibilitas nyata), §12b (narrow-terminal stress test 80/60/40/20 kolom), §17 (UX Validation Matrix — konsolidasi 16 item "Perlu Verifikasi" yang sebelumnya tersebar di berbagai bab jadi satu checklist dengan prioritas verifikasi), dan §18 (Design System Coverage ~20/100 + Command Lifecycle frekuensi×durasi untuk mengurutkan prioritas rollout berdasarkan pemakaian harian, bukan cuma jumlah temuan).

---

## 1. Executive Summary

Repo ini punya **dua kelas pengalaman yang sangat berbeda kualitasnya**, dan itu jadi tema utama audit ini:

1. **Jalur Agent + Permission** (`ai agent`, box approval, state renderer) — sudah melalui `fase1_ui_ux_overhaul` dan hasilnya *genuinely polished*: box adaptif unicode/ASCII (`60-ui/05-ui_box.zsh`), sistem warna semantik (`60-ui/02-ui_colors.zsh`), state machine visual (`60-ui/components/state.zsh`), dan tiga jenis approval box yang konsisten (`06-permissions/20-25-30-*.zsh`).
2. **Jalur command langsung** (`aiplan`, `aispec`, `aiprompt`, `aisummarize`, `aicommit`, `aireview`) — **tidak tersentuh overhaul sama sekali**. Semuanya masih `echo` polos, tanpa warna, tanpa box, tanpa icon. Ini command yang paling sering dipakai harian (planning, spec, commit) tapi terasa seperti CLI generasi lama dibanding `ai agent`.

Temuan paling kritis (detail di §6 & §14):
- **Command Palette punya dua fungsi `ui_palette()` yang saling menimpa** (`components/palette.zsh` vs `screens/palette.zsh`) — yang aktif hanya menang karena kebetulan urutan alfabetis loader, bukan karena desain. 🟢
- **Sistem verbosity (`/config verbosity 0-3`) di-dokumentasikan lengkap, dan variabelnya (`AI_VERBOSITY`) memang dibaca luas di ~6 file (logger, state machine, agent renderer) — tapi lewat pengecekan inline yang terduplikasi di setiap file, bukan lewat API getter resmi (`_ai_verbose`/`_ai_verbose_c`) yang didesain untuk itu, dan API resmi itu sendiri nol pemanggil.** 🟢 *(dikoreksi dari draf sebelumnya yang menyimpulkan verbosity sepenuhnya mati — lihat §6.2)*
- **Ada dua variabel mirip nama dengan nasib implementasi berbeda**: `AI_VERBOSE` (0/1, dipakai konsisten lewat titik-titik permission/diagnostics) vs `AI_VERBOSITY` (0-3, dipakai luas tapi lewat inline check terduplikasi di 6 file, bukan API resmi — lihat §6.2). 🟢
- **`aifix` vs `aipatch`/`aicode -o` punya standar review yang berbeda** untuk operasi yang secara konsep sama (AI mengubah kode existing) — satu wajib diff+confirm+backup, satu lagi cuma nulis `.fixed` dan "cek dulu sebelum overwrite" secara manual. 🟢
- **`ai deps`, `install.sh`, dan sistem UI utama (`60-ui/`) pakai tiga bahasa visual berbeda**: text label polos (`OK`/`MISSING`), emoji (`✔ ⚠ ⬇ 🔗 🎉`), dan icon-berwarna-semantik (`✓ ✗ → ◌`) — untuk pesan status yang secara fungsi setara. 🟢
- **Akar dari sebagian besar temuan visual di atas adalah satu hal: lima renderer berbeda tanpa kontrak bersama** (box/state system, `echo` polos, ANSI hardcode, emoji, text label) — dikonsolidasikan sebagai temuan kritis tersendiri di §13b, karena membaca gejalanya terpisah-pisah menyembunyikan bahwa akarnya satu. 🟢

Repository ini **bukan cuma `ai agent`** — ada ~35 command publik di 7 kategori (`20-shell`, `20-chat`, `30-code`, `35-files`, `40-workflow`, `45-project`/`46-index`, `50-agent`/`55-subagent`). Audit ini mencakup semuanya.

---

## 2. Fase 1 — Command Inventory

### 2.1 Command Publik (bisa dipanggil user langsung dari shell)

| Command | Kategori | File | Entry Point |
|---|---|---|---|
| `ai` | Dispatcher | `60-ui/40-dispatcher.zsh` | Publik, entry utama |
| `ai` (tanpa arg) | Workspace | `60-ui/20-menu.zsh` → `screens/home.zsh` | Publik |
| `aic` / `ai chat` | Chat | `20-chat/00-quick_chat.zsh` | Publik |
| `aicl` / `ai long` | Chat | `20-chat/00-quick_chat.zsh` | Publik |
| `aish` / `ai shell` | Chat | `20-chat/00-quick_chat.zsh` | Publik |
| `aiask` / `ai ask` | Chat | `20-chat/05-aiask.zsh` | Publik |
| `aiclip` / `ai clip` | Chat | `20-chat/25-aiclip.zsh` | Publik |
| `aicode` / `ai code` | Code | `30-code/05-code.zsh` | Publik |
| `aifix` / `ai fix` | Code | `30-code/45-fix.zsh` | Publik |
| `airun` / `ai run` | Code | `30-code/50-run.zsh` | Publik |
| `aiproject` / `ai project` | Code | `30-code/30-project.zsh` | Publik |
| `aiscrap` / `ai scrap` | Code | `30-code/00-scrap.zsh` | Publik |
| `aicat` / `ai view` | Files | `35-files/05-aicat.zsh` | Publik |
| `aipatch` / `ai edit` | Files | `35-files/10-aipatch.zsh` | Publik |
| `aiundo` / `ai undo` | Files | `35-files/15-aiundo.zsh` | Publik |
| `aibakclean` / `ai bakclean` | Files | `35-files/20-aibakclean.zsh` | Publik |
| `aishare` / `ai share` | Files | `35-files/25-aishare.zsh` | Publik |
| `aiplan` / `ai plan` | Workflow | `40-workflow/05-aiplan.zsh` | Publik |
| `aiprompt` / `ai prompt` | Workflow | `40-workflow/10-aiprompt.zsh` | Publik |
| `aispec` / `ai spec` | Workflow | `40-workflow/15-aispec.zsh` | Publik |
| `aibuild` / `ai build` | Workflow | `40-workflow/20-aibuild.zsh` | Publik |
| `aireview` / `ai review` | Workflow | `40-workflow/25-aireview.zsh` | Publik |
| `aisummarize` / `ai summarize` | Workflow | `40-workflow/30-aisummarize.zsh` | Publik |
| `aicommit` / `ai commit` | Workflow | `40-workflow/00-aicommit.zsh` | Publik |
| `aiscan` / `ai scan` | Project | `45-project.zsh` | Publik |
| `aiindex` / `ai index` | Project | `46-index.zsh` | Publik (**tidak ada di `CARA-PAKAI.md`**, lihat §9) |
| `aiagent` / `ai agent` | Agent | `50-agent/40-runtime/30-aiagent.zsh` | Publik |
| `aidebug` / `ai debug` | Subagent | `55-subagent/40-debug.zsh` | Publik |
| `airesearch` / `ai research` | Utility | `60-ui/25-research_dev.zsh` | Publik |
| `aidev` / `ai dev` | Utility | `60-ui/25-research_dev.zsh` | Publik |
| `aistats` / `ai stats` | Utility | `60-ui/10-help_stats.zsh` | Publik |
| `aih` / `ai log` | Utility | `60-ui/10-help_stats.zsh` | Publik — **nama tabrakan dengan `ai h`**, lihat §11 |
| `ai_check_deps` / `ai deps` | Utility | `60-ui/15-diagnostics.zsh` | Publik |
| `ai_testmodels` / `ai testmodels` | Utility | `60-ui/15-diagnostics.zsh` | Publik |
| `ai h` / `ai --help` | Help | `60-ui/10-help_stats.zsh` (`_ai_help`) | Publik |
| `_ai_session` / `ai session` | Session | `20-chat/20-session_mgmt.zsh` | Publik lewat dispatcher |
| `_ai_menu` / `ai menu` | Menu | `60-ui/20-menu.zsh` | Publik (alias `_ai_workspace`) |
| `_ai_update_confirm_pull` / `ai update` | Update | `60-ui/35-update_confirm.zsh` | Publik |

**Non-AI shell layer** (`20-shell/`): `mkcd`, `extract`, `bak`, `vf`, `gacp`, `ports`, `y`, `copy`, `proj`/`pj`, `tm`, plus alias standar (`ll`, `gs`, `ga`, `gc`, `gp`, `gl`, `update`, `sshkey`, dll — `20-shell/aliases.zsh`).

**Slash commands** (dari dalam AI Workspace, `60-ui/router.zsh`): `/chat`, `/code`, `/fix`, `/scan`, `/index`, `/agent`, `/commit`, `/review`, `/stats`, `/dev`, `/session`, `/details`, `/config verbosity N`, `/help`, `/` atau `/?` (Command Palette).

### 2.2 Command Internal (prefix `_ai_...` — tidak dipanggil langsung, tapi banyak)

>100 fungsi internal tersebar di `05-tools/`, `06-permissions/`, `10-core/`, `50-agent/*`, `55-subagent/*`, `60-ui/components/`. Ini bukan permukaan UX langsung tapi membentuk *behavior* command publik di atasnya — direferensikan di seksi relevan sepanjang laporan ini.

### 2.3 Command tidak terdokumentasi tapi bisa dipakai

- `ai index` — ada di `_AI_SUBCOMMANDS` (`60-ui/40-dispatcher.zsh`) dan di slash-router (`router.zsh` case `index`), tapi **tidak muncul di `CARA-PAKAI.md`** (baik di tabel subcommand top-level maupun tabel slash command). 🟢
- `aiscrap` — command scraping web ke Python, ada subcommand publiknya tapi tidak dijelaskan di `CARA-PAKAI.md`. 🟢
- `ai_testmodels` — dipetakan ke `ai testmodels`, tidak dijelaskan di dokumentasi user-facing manapun. 🟢
- `proj`/`pj`, `tm`, `vf`, `y` — utility shell non-AI yang cukup powerful (fzf project switcher, tmux session manager) tapi juga tidak ada di `CARA-PAKAI.md` (dokumen itu fokus penuh ke AI layer).

---

## 3. Fase 2 — UI Component Inventory

| Komponen | File | Dipakai oleh Command |
|---|---|---|
| Box (approval only) | `60-ui/05-ui_box.zsh` (`_ai_ui_box`) | Agent loop (`50-agent/42-execution/00-loop_main.zsh`, `44-finalize.zsh`), permission asks (`06-permissions/20-25-30-*.zsh`) — **hanya 5 pemanggil di luar file `.zsh` component-nya sendiri**. |
| Step rule / separator | `60-ui/05-ui_box.zsh` (`_ai_ui_step_rule`) | Agent loop antar-step |
| Line dengan icon (`→ ✓ ✗ ◌ •`) | `60-ui/05-ui_box.zsh` (`_ai_ui_line`) | Agent execution renderer |
| State indicators (thinking/sending/acting/waiting/done/error/tool/debug) | `60-ui/components/state.zsh` | `screens/agent.zsh`, `screens/report.zsh`, `50-agent/44-finalize.zsh` — **3 file saja** |
| Header (judul workspace) | `60-ui/components/header.zsh` | `screens/home.zsh`, `60-ui/20-menu.zsh`, request blocking/streaming (loading state) |
| Command Palette | `60-ui/components/palette.zsh` **dan** `60-ui/screens/palette.zsh` (**dua fungsi nama sama**, lihat §6.1) | `router.zsh` |
| Cards (`ui_card_summary`, `ui_card_stats`) | `60-ui/components/cards.zsh` | **Tidak dipakai di mana pun** — dead code |
| Progress | `60-ui/components/progress.zsh` (`ui_progress`) | Hanya `30-code/10-project_generate.zsh` |
| Timeline | `60-ui/components/timeline.zsh` (`ui_timeline`) | `20-chat/00-quick_chat.zsh` (`aicl`), `30-code/10-project_generate.zsh` — 2 pemanggil |
| Disclosure (progressive detail, `/details`) | `60-ui/components/disclosure.zsh` | `router.zsh`, dipanggil dari dalam agent execution untuk push detail |
| Verbosity system | `60-ui/components/verbosity.zsh` | Router (`/config verbosity`) — **tapi getter-nya (`_ai_verbose`/`_ai_verbose_c`) tidak dipanggil dari command manapun**, lihat §6.2 |
| Colors (`AI_C_*`) | `60-ui/02-ui_colors.zsh` | Dipakai luas — komponen paling konsisten di seluruh repo |
| Text wrap/width helper | `60-ui/00-ui_text.zsh` | Dipakai `_ai_ui_box` |
| Approval component (generic) | `60-ui/components/approval.zsh` (`ui_approve`) | **Tidak ditemukan pemanggil** — kemungkinan dead code, approval nyata pakai `_ai_perm_ask*` di `06-permissions/`, bukan ini |
| Spinner | `10-core/15-spinner.zsh` | `10-core/50-request_blocking.zsh` saja — chat/plan/spec/dll semuanya lewat jalur ini jadi spinner cukup luas terpakai secara *tidak langsung* |
| Diff colorizer (inline, ANSI manual) | Duplikat identik di `35-files/10-aipatch.zsh` dan `30-code/05-code.zsh` | Bukan komponen bersama — 2 salinan kode sama persis |

**Pengelompokan gaya berbeda:**
1. **Gaya "Agent-polished"**: box unicode/ASCII adaptif + warna semantik + state machine — dipakai `ai agent`, permission dialogs.
2. **Gaya "Plain echo"**: `echo "..."` polos tanpa warna — dipakai `aiplan`, `aispec`, `aiprompt`, `aisummarize`, `aicommit`, `aireview`, `aicat`, `aiundo`, `aibakclean`, `aishare`.
3. **Gaya "Diff manual"**: kode warna ANSI hardcode inline (`printf '\033[31m'` dst, tanpa lewat `AI_C_*`) — dipakai `aipatch` dan `aicode -o`, terpisah dari sistem warna terpusat.
4. **Gaya "Emoji installer"**: `✔ ⚠ ⬇ 🔗 🎉` — hanya di `install.sh`.
5. **Gaya "Text label"**: `OK` / `MISSING` / `WARNING` polos — hanya di `ai deps` (`60-ui/15-diagnostics.zsh`).

---

## 3b. Command Density per Kategori

~35 command publik tersebar tidak rata di 7 kategori. Menghitungnya secara eksplisit menunjukkan area yang paling padat (paling berisiko membingungkan) dan area yang lengang:

| Kategori | Jumlah command | Command | Kepadatan |
|---|---|---|---|
| Workflow (`40-workflow`) | 7 | `aiplan`, `aiprompt`, `aispec`, `aibuild`, `aireview`, `aisummarize`, `aicommit` | 🔴 Paling padat — dan 4 di antaranya (`aiplan`/`aispec`/`aiprompt`/`aibuild`) tumpang tindih secara konsep, lihat §11b |
| Utility (`60-ui` misc) | 8 | `airesearch`, `aidev`, `aistats`, `aih`, `ai_check_deps`, `ai_testmodels`, `ai h`, `ai update` | 🟠 Padat, tapi fungsinya cukup berbeda satu sama lain — risiko tumpang tindih rendah, risiko *naming clash* tinggi (`aih` vs `ai h`, lihat §11) |
| Code (`30-code`) | 5 | `aicode`, `aifix`, `airun`, `aiproject`, `aiscrap` | 🟡 Sedang |
| Files (`35-files`) | 5 | `aicat`, `aipatch`, `aiundo`, `aibakclean`, `aishare` | 🟡 Sedang |
| Chat (`20-chat`) | 5 | `aic`, `aicl`, `aish`, `aiask`, `aiclip` | 🟡 Sedang |
| Project (`45-project`/`46-index`) | 2 | `aiscan`, `aiindex` | 🟢 Lengang |
| Agent/Subagent | 2 | `aiagent`, `aidebug` | 🟢 Lengang (sesuai ekspektasi — ini "mode" berat, bukan utility ringan) |

**Bacaan:** kepadatan tinggi bukan masalah dengan sendirinya, tapi kategori **Workflow** padat *dan* empat commandnya saling mirip secara konsep (lihat §11b) — kombinasi itu yang membuatnya jadi titik kebingungan paling nyata di seluruh repo, lebih dari kategori Utility yang padat tapi fungsinya jelas berbeda-beda.

---

## 4. Fase 3 — Visual Consistency

| Aspek | Konsisten? | Bukti |
|---|---|---|
| Box | 🔴 Tidak | `_ai_ui_box` sengaja *hanya* memberi border untuk title mengandung kata "approval" (`05-ui_box.zsh` baris ~30-40) — command lain sama sekali tidak pakai box meski isinya setara pentingnya (hasil `aiplan`/`aispec` yang panjang, output review `aireview`). |
| Warna ANSI | 🟡 Sebagian | Sistem warna terpusat (`AI_C_*`) bagus dan dipakai luas di jalur agent, tapi `aipatch`/`aicode -o` bikin warna merah/hijau diff mereka sendiri lewat `printf '\033[31m'` — tidak lewat `AI_C_ERR`/`AI_C_OK`, jadi kalau tema warna diubah di `02-ui_colors.zsh` nanti, diff view ini **tidak ikut berubah** (drift risk). |
| Emoji vs icon unicode | 🔴 Tidak | `install.sh` pakai emoji penuh warna (✔⚠⬇🔗🎉📦🎉), sedangkan `60-ui/` sengaja membangun sistem icon monokrom-dengan-warna-semantik + ASCII fallback (`_ai_ui_supports_unicode`) yang eksplisit **menghindari emoji**. Dua bahasa visual berbeda untuk pengalaman yang seharusnya terasa satu produk. |
| Border/lebar box | 🟡 Sebagian | Lebar `_ai_ui_box` adaptif (`_ai_ui_width`), sudah baik untuk terminal sempit — tapi karena hanya dipakai approval, mayoritas command tidak merasakan manfaat ini. |
| Confirm prompt | 🔴 Tidak | 3 pola confirm berbeda untuk aksi yang sama-sama "AI mau ubah/hapus sesuatu, minta izin": (a) `_ai_perm_ask` terpusat dengan box (agent tools); (b) `gum confirm`/`read -t 60` duplikat identik di `aicommit`, `aipatch`, `aicode -o` (60 detik timeout); (c) `gum confirm`/`read -t 30` duplikat identik di `aiundo`, `aibakclean` (30 detik timeout, beda dari grup (b) tanpa alasan yang jelas dari kode). |
| Pesan sukses/gagal | 🟡 Sebagian | Sebagian pakai "GAGAL: ..." / "ERROR: ..." (aicode, aibuild), sebagian "Gak ada ..." (aiplan-style tanpa prefix status), sebagian pakai ❌ emoji (`aicl` di `20-chat/00-quick_chat.zsh`: `echo "❌ Tahap '$stage' gagal."`) — satu-satunya tempat emoji ❌ muncul di jalur AI (di luar installer). |
| Bahasa Indonesia vs Inggris | 🟡 Sebagian | Pesan status mayoritas Bahasa Indonesia santai, tapi beberapa pesan sistem Inggris murni (mis. `_ai_ui_box "Command requires approval"`, `"File change requires approval"` di `06-permissions/*`, dan hint di `screens/home.zsh`: *"Ketik prompt atau / untuk Command Palette"* dicampur istilah Inggris "Command Palette"). Bukan salah, tapi campuran tidak sistematis — detail per kategori pesan di §4b. |

### 4b. Peta Bahasa per Kategori Pesan

Campuran bahasa di atas bukan acak — kalau dikelompokkan per *jenis* pesan, polanya cukup konsisten dalam kategorinya sendiri, tapi tidak konsisten *antar* kategori:

| Jenis pesan | Bahasa dominan | Contoh | Konsisten di dalam kategorinya? |
|---|---|---|---|
| Error/gagal (runtime) | 🇮🇩 Indonesia | `"GAGAL: ..."`, `"Bukan git repo."`, `"File gak ketemu: ..."` | 🟢 Ya — hampir seluruh pesan error command pakai Indonesia santai |
| Approval/permission box | 🇬🇧 Inggris | `"Command requires approval"`, `"File change requires approval"` | 🟢 Ya — seluruh box approval di `06-permissions/` konsisten Inggris |
| Help / `ai h` | 🌐 Campuran | Label kategori & deskripsi subcommand campur Indonesia-Inggris dalam baris yang sama | 🔴 Tidak — satu-satunya kategori yang benar-benar campur di level kalimat |
| Command Palette / hint navigasi | 🌐 Campuran | *"Ketik prompt atau / untuk Command Palette"* — kalimat Indonesia, istilah fitur Inggris | 🟡 Sebagian — pola "kalimat ID + istilah teknis EN" konsisten dipakai, tapi tidak didokumentasikan sebagai keputusan sadar |
| Instalasi (`install.sh`) | 🇮🇩 Indonesia + emoji | Pesan ramah Indonesia, tapi ikon emoji bukan istilah bahasa | — (bukan soal bahasa, tapi ikut menambah "bahasa visual" ke-3, lihat §4 baris emoji) |

**Bacaan:** repo ini *bukan* tanpa aturan bahasa — errornya konsisten Indonesia, approval box konsisten Inggris. Yang belum ada adalah **keputusan produk eksplisit**: apakah target akhirnya bilingual by design (errors ID untuk keakraban, approval EN untuk ketegasan hukum/formalitas — pola yang sebenarnya masuk akal kalau didokumentasikan), atau salah satu harus dimenangkan sepenuhnya. Tanpa keputusan itu, kategori "Help" dan "Palette" akan terus drift karena tidak ada aturan yang jadi rujukan kontributor baru.

🔵 **Rekomendasi**: tulis satu baris keputusan di `CARA-PAKAI.md` atau `CONTRIBUTING.md`: *"Pesan error & UI harian: Indonesia. Pesan sistem/keamanan (approval, permission): Inggris. Istilah fitur (Command Palette, YOLO mode): tetap Inggris meski dalam kalimat Indonesia."* — lalu audit ulang `ai h` khusus terhadap aturan ini.

**Skor Visual Consistency: 4/10** — sistem desain yang mendasarinya (warna, box adaptif, icon set) berkualitas tinggi, tapi cakupan penerapannya sangat sempit (≈15% dari command publik memakainya secara langsung). Skor rendah murni karena *coverage*, bukan karena kualitas desain intrinsiknya (kualitas desain jalur agent sendiri kalau dinilai terpisah: 8/10).

---

## 5. Fase 4 — Information Hierarchy

Contoh yang representatif:

- **`aiplan`** (`40-workflow/05-aiplan.zsh`): `echo "Generating rencana..."` → dump seluruh Markdown hasil AI ke terminal → `echo ""` → `echo "Rencana tersimpan di: $outfile"`. Urutan informasi sudah logis (proses → hasil → lokasi file), tapi **tidak ada pemisah visual** antara "sedang generate" dan "hasil" dan "lokasi file" — tiga jenis informasi bercampur rata di terminal tanpa hierarki visual, berbeda dengan `aispec` yang setidaknya menambahkan hint langkah berikut (`"Lanjut: ai project <nama_folder> $outfile"`) — **`aiplan` tidak punya "next step" hint sama sekali**, padahal `aispec` dan `aiprompt` punya.
- **`aibuild`** (`40-workflow/20-aibuild.zsh`): pakai `[1/2]`/`[2/2]` — satu-satunya command non-agent yang punya progress numerik eksplisit. Bagus, tapi jadi inkonsisten dengan command workflow lain yang tidak punya progress indicator sama sekali (`aiplan`, `aispec`, `aiprompt`).
- **`aisummarize`** (`40-workflow/30-aisummarize.zsh`): untuk konten panjang, print `"chunk $i/${#parts[@]}..."` — progress ada, tapi levelnya beda gaya dari `[1/2]` ala `aibuild` (satu pakai bracket format, satu pakai kata "chunk").
- **`ui_agent_dashboard`** (`60-ui/screens/agent.zsh`): urutan sudah jelas — status aksi → timeline compact (✓ ● ○) → output — ini contoh terbaik hierarki informasi di seluruh repo.

**Noise/duplikasi**: `aicode`, `aifix`, `aiscrap` semuanya punya baris `grep -v '```'` yang mengasumsikan model kadang membalas dengan markdown fence meski system prompt eksplisit melarang — artinya **sistem sudah tahu modelnya tidak selalu patuh**, tapi user tidak diberi tahu kalau filtering ini terjadi (silent cleanup, tidak transparan bagi yang penasaran kenapa outputnya "hilang" satu baris).

---

## 6. Fase 5 — Workflow Mapping (command utama)

```
Command: ai agent <goal>
User → Input goal → [_ai_need_any_key] → [checkpoint? / goal baru]
     → Header box (model, project) → Loop: Thinking → Acting (approval jika perlu)
     → Verify (npm checks / touched files) → Auto-review (aireview, kecuali --no-review)
     → Done (files changed, runtime) / Error
Langkah: ~7-9 (tergantung jumlah tool call)   Friction: approval fatigue jika banyak file (§6)   Ideal: sama, sudah cukup ramping
```

```
Command: aiplan <goal>
User → Input goal → [need_key check] → "Generating rencana..." (tanpa progress %)
     → Dump Markdown mentah ke terminal → Path file tersimpan → (tidak ada next-step hint)
Langkah: 4   Friction: tidak ada next-step hint, tidak ada preview/pagination untuk output panjang   
Ideal: tambah "Lanjut: ..." seperti aispec, dan opsi --short untuk ringkas dulu
```

```
Command: aipatch <file> <instruksi>
User → Args → Guard (binary? secret? ukuran?) → "Minta AI menyusun perubahan..."
     → AI generate full-file → Diff berwarna (ANSI manual) → Confirm (gum/read -t 60)
     → Backup → Apply → "Diterapkan. Backup: ..."
Langkah: 8   Friction: tidak ada, ini SALAH SATU workflow paling matang di repo (guard rails lengkap)
Ideal: sama — jadikan referensi pola untuk aifix (lihat §6)
```

```
Command: aifix <file> "<error>"
User → Args → AI generate full-file fix → Tulis ke <file>.fixed →
     → "cek dulu sebelum overwrite (diff ...)" — SELESAI, user harus jalanin diff SENDIRI secara manual
Langkah: 3   Friction TINGGI: tidak ada diff otomatis, tidak ada confirm, tidak ada apply-step —
     user harus tahu untuk copy-paste command diff sendiri dan mv manual
Ideal: samakan dengan pola aipatch (diff otomatis + confirm + backup + apply)
```

```
Command: airun <file.py>
User → Run python3 → Error? → aifix (otomatis, TANPA diff/confirm user) → auto-apply fixed → retry (maks 2x)
Langkah: variabel (1-2 percobaan)
Friction: airun MEMANGGIL aifix TAPI meng-otomatiskan penerapan fix (mv -f) tanpa
     review sama sekali — padahal dipakai standalone, aifix eksplisit MINTA user review manual dulu.
     Sehingga pengalaman "seberapa hati-hati aifix" berbeda tergantung siapa yang manggil (user vs airun).
```

```
Command: /  atau  ai  (workspace kosong, ketik "/")
User → ketik "/" → ui_router("") → source screens/palette.zsh → gum filter 17 opsi hardcoded
     → pilih → ui_router(cmd_part) → eksekusi
Langkah: 4   Friction: BERGANTUNG pada urutan file mana yang menang saat override nama fungsi (§6.1) —
     kalau suatu saat urutan loading berubah (mis. refactor folder), Command Palette bisa
     mendadak kosong/rusak tanpa pesan error yang jelas ke user.
```

| Command | Langkah | Friction | Ideal |
|---|---|---|---|
| `ai agent` | 7-9 | Approval fatigue di project besar | Sudah dekat ideal |
| `aiplan` | 4 | Tidak ada next-step hint | Tambah hint, progress |
| `aispec` | 5 | Minimal, sudah baik | - |
| `aibuild` | 8-9 (gabung spec+project) | Bisa gagal di tengah tanpa rollback jelas jika step 2 gagal parsial | Tambah status recovery |
| `aipatch` | 8 | Nyaris tidak ada | Jadikan pola referensi |
| `aifix` | 3 (standalone) | **Tinggi** — tanpa diff/confirm otomatis | Samakan dgn aipatch |
| `airun` | 1-2 iterasi | Auto-apply fix tanpa confirm (beda dari aifix standalone) | Konsistenkan kontrak "review wajib" |
| `aicommit` | 3 | Rendah | - |
| `aireview` | 2 | Rendah, tapi output panjang tanpa box/pagination | Tambah struktur visual |
| Command Palette (`/`) | 4 | **Risiko struktural** (duplikasi fungsi) | Hapus salah satu `ui_palette` |

---

## 6b. Fase 5b — Workflow-as-Product: Analisis Lintas-Command

Fase 5 di atas memetakan command satu per satu. Tapi user harian tidak memakai repo ini per-command — mereka memakainya lewat **loop kerja** yang memanggil beberapa command berurutan. Bagian ini menghitung friksi di level loop, bukan command tunggal.

### Coding loop
`aiplan` (opsional) → `aispec`/`aiprompt` → `aicode`/`aibuild` → edit manual/`aipatch` → `aicommit`
- **Total command berbeda dilewati**: 4-5, masing-masing dengan gaya visual berbeda (plain-echo untuk plan/spec/prompt, diff-manual untuk aipatch, plain-echo lagi untuk commit).
- **Friction loop**: user berpindah dari command tanpa struktur visual (`aiplan`) ke command dengan diff berwarna (`aipatch`) lalu balik lagi ke plain-echo (`aicommit`) — pergantian gaya visual di tengah satu alur kerja terasa seperti berpindah aplikasi, bukan tetap di satu produk. Ini dampak konkret dari temuan "dua kelas UX" di §1, bukan cuma soal estetika command individual.

### Debug loop
`airun` → (gagal) → `aifix` (otomatis, tanpa review) → retry → (gagal lagi) → `aidebug`/manual
- **Friction loop**: sudah dibahas per-command di §6 (`airun` vs `aifix` beda kontrak review) — di level loop, dampaknya adalah user yang terbiasa `airun` auto-apply lalu pindah ke `aifix` manual (atau sebaliknya) akan salah asumsi soal siapa yang mengubah file tanpa izin eksplisit mereka.

### Review loop
`aiagent` (auto-review internal) → `aireview` (manual, standalone) → baca output panjang tanpa box/pagination
- **Friction loop**: `aireview` menghasilkan output read-only tanpa struktur visual (§6 tabel), jadi user yang baru selesai lihat `aiagent` dengan state machine rapi lalu masuk `aireview` akan merasakan penurunan kualitas visual di tengah alur yang secara logis menyambung (agent selesai → review hasilnya).

### Commit loop
`aicommit` (generate pesan + confirm 60s) → (opsional) `aireview` sebelum/sesudah
- **Friction loop**: relatif rendah — ini loop paling matang, `aicommit` sendiri sudah 3 langkah ringkas (§6 tabel). Titik lemahnya cuma visual (plain-echo), bukan struktural.

### Resume loop
`ai agent --resume <slug>` → checkpoint dibaca → lanjut dari state terakhir
- **Friction loop**: fiturnya ada dan cukup matang (§6.3, §10 power-user score 8/10), tapi **tidak terekspos di empty-state** (`screens/home.zsh` tidak menampilkan sesi/resume terakhir, lihat §9d) — user harus sudah tahu flag `--resume` sebelum memakainya, tidak ada penemuan pasif.

**Kesimpulan lintas-loop**: friksi terbesar bukan di dalam satu command, tapi di **transisi antar command dalam satu loop yang sama** — pergantian gaya visual (coding loop, review loop) dan pergantian kontrak keamanan (debug loop) di tengah alur kerja yang secara mental terasa sebagai satu tugas berkelanjutan bagi user. Redesign Sprint 3 (§15) yang menerapkan design system ke seluruh `40-workflow/*` akan menyelesaikan sebagian besar friksi loop ini sekaligus, karena akar masalahnya sama: satu rendering layer belum menyentuh seluruh command yang membentuk loop harian.

---

## 6. Fase 6 — Permission UX

### 6.1 Command Palette: dua fungsi `ui_palette()` bertabrakan nama 🟢 Fakta — TEMUAN KRITIS

Ditemukan **dua definisi fungsi dengan nama identik `ui_palette()`**:

- `.zsh_bagas/30-ai/60-ui/components/palette.zsh` — versi generik, **menerima daftar item lewat `"$@"`** (parameter fungsi), tidak memiliki data bawaan.
- `.zsh_bagas/30-ai/60-ui/screens/palette.zsh` — versi lengkap, **daftar 17 command sudah di-hardcode di dalam fungsi**, tidak butuh argumen.

Satu-satunya pemanggil, `router.zsh`, memanggil `ui_palette` **tanpa argumen apa pun**:
```zsh
source ".../components/palette.zsh" ...
if type ui_palette >/dev/null 2>&1; then
    ui_palette
fi
```
Karena kedua file di-`source` lewat loader glob `.zshrc` (`for f in "$ZSH_BAGAS"/**/*.zsh(N.on); do source "$f"; done`), definisi fungsi yang **di-source belakangan menang** (menimpa yang pertama). Secara alfabetis path, `components/` < `screens/` (huruf 'c' < 's'), jadi `screens/palette.zsh` termuat setelah `components/palette.zsh` dan menang — **Command Palette kebetulan berfungsi hari ini murni karena urutan alfabet folder**, bukan karena desain eksplisit siapa yang harus menang. Kalau nanti folder `screens/` di-rename jadi sesuatu yang secara alfabet mendahului `components/` (mis. `00-screens/`), Command Palette akan diam-diam kosong (memanggil versi generik tanpa item) tanpa error jelas ke user.

🔵 **Rekomendasi**: rename salah satu fungsi (mis. `ui_palette_generic` untuk versi `components/`, atau hapus sama sekali kalau memang tidak dipakai — cek dulu apakah versi generik dimaksudkan untuk dipakai command lain di masa depan).

### 6.2 Verbosity system — sebagian rusak, bukan sepenuhnya mati 🟢 Fakta — **[Koreksi dari draf sebelumnya]**

`CARA-PAKAI.md` mendokumentasikan 4 level verbosity dengan janji perilaku spesifik:
```
/config verbosity 0   Output minimal (hanya hasil akhir)
/config verbosity 1   Output normal — default
/config verbosity 2   Output detail (nama tool, file)
/config verbosity 3   Output debug (semua log internal)
```
Router (`router.zsh`) memanggil `ai_verbosity_set` yang **benar-benar berfungsi** — mengeset `AI_VERBOSITY` dan mencetak konfirmasi.

**Koreksi**: draf audit sebelumnya menyimpulkan verbosity "tidak pernah dibaca" dan menandainya sebagai fitur hantu. Setelah ditelusuri lebih lanjut, klaim itu **terlalu keras**. Getter resmi (`_ai_verbose()`/`_ai_verbose_c()` di `60-ui/components/verbosity.zsh`) memang benar tidak dipanggil satu kali pun — itu tetap dead code. Tapi variabel `AI_VERBOSITY` itu sendiri **dibaca langsung** (bypass getter, `${AI_VERBOSITY:-0}` inline) di setidaknya 6 file lain: `60-ui/01-logger.zsh` (status WAIT/DONE/ERROR dari `_ai_chat_request`, dipakai semua command lewat jalur chat termasuk `aiplan`/`aispec`/`aicode`), `60-ui/components/state.zsh`, `20-chat/01-chat_display.zsh`, `50-agent/20-presentation/20-tool_step_render.zsh`, `50-agent/40-runtime/25-execute_and_finalize.zsh`, dan `50-agent/42-execution/00-loop_main.zsh`. Menaikkan `/config verbosity` **benar-benar mengubah** status line "Thinking...`/`Sending...`" yang muncul saat request AI berjalan, serta detail tool-step di jalur agent.

Temuan sebenarnya jadi lebih menarik daripada "fitur mati": **implementasi verbosity terduplikasi** — API resmi (`_ai_verbose`) tidak dipakai siapa pun, sementara ~6 file lain masing-masing menulis ulang pengecekan `${AI_VERBOSITY:-0}` sendiri-sendiri secara inline. Efeknya verbosity *berfungsi* tapi lewat pola yang tidak konsisten (kadang lewat getter yang tidak dipakai, kadang inline) — risikonya bukan "user dibohongi dokumentasi" (severity Kritis), tapi "kalau logika verbosity perlu diubah nanti, harus diedit di 6+ tempat berbeda karena tidak lewat satu API" (risiko maintenance, severity Sedang, bukan Kritis). §14 dan §16 direvisi mengikuti koreksi ini.

Sebagai tambahan, komentar di file yang sama menyatakan default level `0` ("← DEFAULT"), sementara `CARA-PAKAI.md` menyebut level `1` sebagai "default" — **dokumentasi dan kode saling kontradiksi soal apa defaultnya**. Ini bagian dari temuan yang tetap valid.

Terpisah dari itu, ada variabel **lain** `AI_VERBOSE` (0/1, bukan `AI_VERBOSITY`) yang benar-benar dipakai di `06-permissions/20-25-*.zsh`, `50-agent/42-execution/05-get_plan.zsh`, dan `10-core/40-circuit_breaker.zsh` untuk menampilkan detail policy/decision teknis. Dua sistem bernama mirip — satu (`AI_VERBOSITY`) dipakai luas tapi lewat getter resmi yang diabaikan, satu lagi (`AI_VERBOSE`) dipakai sempit tapi konsisten — berpotensi membingungkan siapa pun yang mengembangkan fitur baru di sini nanti.

🔵 **Rekomendasi**: (a) hapus/rename getter resmi `_ai_verbose`/`_ai_verbose_c` yang tidak dipakai, lalu refactor 6 file yang punya inline check `${AI_VERBOSITY:-0}` supaya benar-benar lewat satu API bersama (bisa jadi getter yang sudah ada, tinggal disambungkan) — ini menyelesaikan technical debt duplikasi tanpa perlu membangun ulang dari nol karena wiring dasarnya sudah ada; (b) sinkronkan klaim default (`0` vs `1`) antara kode dan `CARA-PAKAI.md`; (c) satukan `AI_VERBOSE` dan `AI_VERBOSITY` secara konseptual atau beri nama yang jelas beda kalau memang harus tetap dua sistem.

### 6.3 Permission agent-tools (`06-permissions/`) — SUDAH BAIK 🟢

- Tiga jenis approval box konsisten by design: *Command requires approval* (proses/shell), *File change requires approval* (write/edit/patch/move/delete), *Action requires approval* (web fetch, fallback generik) — semua lewat `_ai_ui_box` + `_ai_perm_ask`, semua ke `/dev/tty` eksplisit (aman meski dipanggil dari konteks stderr-redirect).
- Ada mode `ask_once_per_file` yang menghindari approval fatigue untuk file yang sama dalam satu sesi (`_AI_SESSION_APPROVED` keyed per-session-slug, `25-perm_write.zsh`) — desain matang.
- Ada `AI_VERBOSE=1` opsional yang menampilkan detail Policy/Decision sebelum prompt — baik untuk power user, tidak mengganggu default flow.
- `--yolo` (bypass approval) dijaga tetap tidak membolehkan command shell arbitrary lewat `_ai_yolo_shell_safe` (`30-perm_shell.zsh`) — bukan bypass total, ada whitelist keamanan di baliknya.

### 6.4 Confirm ad-hoc di luar sistem terpusat — inkonsisten 🟢

`aicommit`, `aipatch`, `aicode -o` pakai pola identik: `gum confirm` kalau ada, kalau tidak `read -t 60`. `aiundo`, `aibakclean` pakai pola sama tapi `read -t 30`. Empat command, dua timeout berbeda, kode duplikat 8-10 baris di masing-masing file (bukan fungsi bersama) — risiko drift kalau salah satu diperbaiki (mis. bug escaping) tapi yang lain lupa diupdate, persis pola bug yang disebutkan di komentar `aispec`/`aibuild` soal `AI_SPEC_SYSPROMPT` yang dulu diketik ulang manual di dua tempat.

🔵 **Rekomendasi**: ekstrak jadi satu fungsi `_ai_confirm(prompt, timeout)` dipakai keempatnya — plus putuskan timeout standar (30 atau 60) berdasarkan risiko aksi (delete/restore vs write/commit), bukan kebetulan siapa yang nulis duluan.

---

## 7. Fase 7 — Error UX

| Kategori | Contoh Sekarang | Seharusnya |
|---|---|---|
| Provider/API gagal total | `aiplan`/`aispec`: kalau `_ai_chat_request` gagal, `$reply` kosong tetap di-`echo` (baris kosong ke user) tanpa pesan error eksplisit — **beda** dari `aibuild`/`aicode`/`aifix` yang sudah eksplisit cek `rc`/`-z "$reply"` dan kasih pesan `"GAGAL: ..."` | Samakan: semua command yang manggil `_ai_chat_request`/`_ai_quick` wajib cek exit code, bukan cuma sebagian |
| File tidak ditemukan | `aicat`, `aipatch`, `aishare`: pesan konsisten `"File gak ketemu: $file"` / `"File tidak ditemukan: $1"` (beda kata tapi makna sama) | Standarkan satu frasa persis di semua tempat |
| Secret/binary file terdeteksi | `aipatch`, `aiclip`: sudah bagus — pesan jelas + tahu *kenapa* ditolak + tahu cara bypass (`--force`) | Sudah baik, jadikan pola referensi |
| Timeout konfirmasi | Pesan konsisten `"Timeout ... dianggap batal"` di semua tempat yang pakai `read -t` | Sudah konsisten — poin positif |
| Git bukan repo | `aicommit`, `aireview`: identik `"Bukan git repo."` | Sudah konsisten |
| Validation URL | `aiscrap`: `"URL harus diawali http:// atau https:// (dapet: ${url:0:40})"` — bagus, kasih tahu apa yang salah + potongan input | Jadikan pola referensi untuk validasi lain |
| Subcommand tidak dikenal | Dispatcher (`40-dispatcher.zsh`): fuzzy-match Levenshtein ≤2 → tawarkan koreksi interaktif, timeout 20 detik → fallback ke chat biasa | Salah satu UX error-recovery terbaik di repo — patut dicontoh |
| Bug internal (drift subcommand list) | Dispatcher: pesan eksplisit "ini bug internal, bukan salah kamu. Lapor ini." — jujur dan tidak menyalahkan user | Baik, pertahankan nada ini |

**Nilai bantuan-menyelesaikan-masalah**: mayoritas pesan error di repo ini **konkret dan actionable** (kasih tahu command follow-up yang harus dijalankan: `aiundo "$file"`, `aipatch --force ...`, `pkg install ...`). Ini kekuatan besar dibanding banyak CLI tool lain yang cuma bilang "Error occurred". Kelemahan utamanya bukan kualitas pesan individual, tapi **konsistensi cakupan** — sebagian command (terutama `aiplan`) tidak selalu mengecek kegagalan API secara eksplisit.

---

## 8. Fase 8 — Progress & Feedback

| Command | Feedback selama proses | Masalah |
|---|---|---|
| `aic`/`aiask` dll (chat biasa) | Spinner dari `10-core/15-spinner.zsh` (jalur blocking) | 🟡 Perlu Verifikasi visual real — animasi spinner terlihat baik di kode tapi belum diuji langsung di Termux/berbagai terminal emulator |
| `aiagent` | State machine visual (Thinking→Acting→Waiting→Done), step rule antar-task | Baik, salah satu yang terbaik |
| `aiplan`/`aispec`/`aiprompt` | Cuma `echo "Generating ..."` statis, **tidak ada spinner terlihat** kecuali lewat jalur blocking yang sama (perlu verifikasi apakah spinner ikut tampil untuk command2 ini juga karena semua lewat `_ai_chat_request`) | 🟡 Kalau spinner memang ikut jalan di sini juga, ini kabar baik yang tidak terlihat dari baca kode command-nya sendiri saja — worth mendokumentasikan eksplisit di komentar biar jelas |
| `aisummarize` (chunked) | `"chunk $i/${#parts[@]}..."` per iterasi — real progress counter | Baik, tapi gaya beda dari `[1/2]` `aibuild` |
| `aibuild` | `[1/2]` / `[2/2]` eksplisit | Baik |
| `airun` | `"Error terdeteksi, mencoba perbaikan otomatis ($((tries+1))/2)..."` | Baik, jelas |

**Jeda membingungkan**: pada command yang cuma `echo "Generating..."` lalu diam sampai API selesai (bisa puluhan detik untuk model "smart"/"big"), user tidak tahu apakah proses hang atau memang lagi jalan — kecuali spinner dari jalur blocking benar-benar tampil (perlu verifikasi manual, item 🟡 di atas). Kalau ternyata spinner **tidak** tampil untuk command non-chat (karena caranya manggil `_ai_chat_request` beda), ini jadi celah UX nyata untuk command yang paling sering dipakai (plan/spec/prompt).

**Koreksi kecil setelah membaca `10-core/15-spinner.zsh`**: "spinner" di repo ini bukan animasi berputar dengan proses background — `_ai_spinner_start`/`_ai_spinner_update` cuma memanggil `_ai_log_wait` yang mencetak **satu baris status statis** (`● Thinking...`) ke `/dev/tty`, tanpa loop animasi maupun proses terpisah. Ini relevan untuk dua hal: (1) tidak ada risiko "cursor tersembunyi lupa dikembalikan" khas spinner animasi (lihat §8c di bawah), tapi (2) klaim "spinner" di draf sebelumnya sedikit menyesatkan — yang terjadi lebih tepat disebut *status line satu-kali*, bukan indikator progres berjalan. Untuk task yang makan waktu lama (model "smart"/"big"), status line statis tanpa perubahan visual apa pun masih berisiko terasa seperti hang meski secara teknis "berfungsi".

---

## 8c. Perceived Latency Audit — kapan feedback pertama benar-benar muncul?

Audit static tidak bisa mengukur waktu nyata dalam detik (butuh eksekusi live, ditandai 🟡 di seluruh bagian ini), tapi bisa memetakan **struktur** feedback berdasarkan urutan pemanggilan di kode — yaitu tiga metrik standar CLI: **Time to First Feedback (TTFF)** (jeda dari Enter sampai output pertama apa pun), **Time Between Feedback** (jeda antar-update status selama proses berjalan), dan **Silent Gap** (periode tanpa output sama sekali padahal proses masih berjalan).

| Command | TTFF terstruktur | Time Between Feedback | Silent Gap berisiko? |
|---|---|---|---|
| `aiplan`/`aispec`/`aiprompt` | 🟢 Baik secara struktur — `echo "Generating rencana..."` dicetak **sebelum** `_ai_chat_request` dipanggil (`40-workflow/05-aiplan.zsh`), jadi TTFF seharusnya instan (bukan menunggu network dulu) | 🟡 Setelah status line awal, tidak ada update lagi sampai reply selesai — kecuali status line dari `_ai_log_status` (§6.2 koreksi) tampil di `AI_VERBOSITY>=1` | 🔴 Ya, kalau `AI_VERBOSITY=0` (default) — antara `"Generating rencana..."` dan hasil akhir, tidak ada output apa pun selama request AI berlangsung (bisa puluhan detik untuk model besar) |
| `ai agent` | 🟢 Baik — state machine (`Thinking→Acting→Waiting→Done`) dirancang untuk update berkelanjutan (§8 tabel) | 🟢 Baik — step rule antar-task memberi checkpoint visual reguler | 🟢 Rendah — desain state machine secara eksplisit menghindari silent gap panjang |
| `aicode`/`aifix` | 🟡 Perlu verifikasi urutan persis — pola pemanggilan mirip `aiplan` (echo lalu `_ai_chat_request`) tapi belum ditelusuri baris-per-baris di audit ini | 🟡 Sama seperti `aiplan` — bergantung `AI_VERBOSITY` | 🔴 Kemungkinan sama seperti `aiplan` (default silent) |
| `aisummarize` (chunked) | 🟢 Baik — `"chunk $i/N..."` per iterasi memberi checkpoint reguler untuk konten panjang | 🟢 Baik | 🟢 Rendah — chunking secara alami memecah silent gap jadi beberapa gap lebih pendek |

**Bacaan**: struktur kode menunjukkan TTFF sebenarnya sudah cukup baik di hampir semua command (status/echo pertama dicetak sebelum network call, bukan sesudah) — ini bukan masalah. Masalah nyatanya ada di **Silent Gap** pada level `AI_VERBOSITY=0` (default pabrik): command single-shot seperti `aiplan`/`aispec`/`aicode` tidak punya checkpoint visual apa pun selama request berlangsung, beda dengan `aiagent` (state machine) dan `aisummarize` (chunk counter) yang secara desain sudah aman dari silent gap panjang.

🔵 **Rekomendasi**: (1) verifikasi manual dengan `time` + observasi visual untuk mengisi kolom TTFF/gap yang masih 🟡; (2) untuk command single-shot, tambahkan minimal satu status line periodik (mis. tiap 5-10 detik "...masih memproses") di level `AI_VERBOSITY=0`, bukan cuma di level 1+, karena silent gap paling berisiko justru terjadi di level default yang paling banyak dipakai user.

---

## 8d. Command Interruption (`Ctrl+C`) — apa yang terjadi saat user membatalkan?

Ini beda dari reliability (apakah state korup) — ini murni soal apa yang **terlihat dan terasa** oleh user saat menekan `Ctrl+C`.

| Command/Area | Ctrl+C behavior (dari kode) | UX |
|---|---|---|
| `_ai_chat_request` (dipakai `aiplan`/`aispec`/`aicode`/chat) | `TRAPINT`/`TRAPTERM` eksplisit (`10-core/50-request_blocking.zsh`) — kill proses `curl` aktif, set flag `_ai_cancelled`, panggil `_ai_spinner_stop`, return kode 130/143 | 🟢 Baik — ini cooperative cancellation yang dirancang sengaja (komentar kode: *"Ctrl-C harus membatalkan request aktif dan membersihkan spinner"*), bukan sekadar mengandalkan default shell |
| `ai agent` (loop eksekusi) | Cancellation kooperatif lewat state file (`50-agent/40-runtime/25-execute_and_finalize.zsh`: `trap ... INT TERM` menulis `$state_dir/cancelled`), dibaca ulang di `42-execution/15-run_tool.zsh` dan `00-loop_main.zsh` untuk menghentikan loop dengan pesan eksplisit *"Agent dibatalkan oleh SIGINT/SIGTERM (step N)"* | 🟢 Baik — user tahu persis di step mana agent berhenti, bukan terpotong tanpa penjelasan |
| Checkpoint saat interrupt | 🟡 Perlu Verifikasi — kode menandai state "cancelled" tapi audit ini belum menelusuri apakah checkpoint/resume state tetap konsisten dipakai setelah interrupt di tengah tool call (mis. file setengah ditulis) | Perlu pengujian manual: interrupt di tengah `aiagent` lalu `--resume`, apakah lanjut dengan bersih atau ada state ganjil |
| Cursor/terminal state setelah interrupt | 🟢 Risiko rendah secara struktural | Karena "spinner" repo ini adalah status line statis, bukan animasi yang biasanya menyembunyikan cursor (`\033[?25l`) — tidak ditemukan kode yang eksplisit sembunyikan lalu lupa kembalikan cursor. Risiko klasik "cursor hilang setelah Ctrl+C" yang umum di spinner animasi kemungkinan **tidak berlaku** di sini (lihat koreksi §8) |
| Command tanpa trap eksplisit (mis. `aipatch`, `aicommit` di luar bagian `_ai_chat_request`-nya) | 🟡 Perlu Verifikasi — trap yang ditemukan terpusat di `_ai_chat_request` dan agent loop; belum ditelusuri apakah command yang punya langkah non-request (mis. proses `diff`/`backup`/`apply` di `aipatch`) juga aman diinterupsi di tengah langkah tersebut | Risiko: interrupt tepat di antara "backup" dan "apply" berpotensi meninggalkan state di tengah tanpa pesan status yang jelas — perlu pengujian manual per command |

**Bacaan**: repo ini **jauh lebih baik** dari CLI kebanyakan soal interrupt — trap SIGINT/SIGTERM eksplisit dan disengaja ditemukan di dua titik paling kritis (request AI, agent loop), dengan cleanup spinner dan pesan status yang jujur ("dibatalkan", bukan terpotong diam-diam). Ini kekuatan yang sebelumnya tidak tercatat sama sekali di audit. Titik lemahnya bukan di jalur yang sudah punya trap, tapi di **command lain yang tidak melalui `_ai_chat_request` di titik rawannya** (mis. antara backup dan apply di `aipatch`) — cakupan trap belum diverifikasi selebar apa.

🔵 **Rekomendasi**: (1) audit manual titik-titik non-request di `aipatch`/`aicommit`/`aiundo` (antara backup dan apply, antara diff dan commit) untuk memastikan interrupt di sana juga aman; (2) dokumentasikan pola `TRAPINT`/`TRAPTERM` yang sudah ada sebagai referensi eksplisit untuk kontributor baru, karena ini pola yang bagus tapi implisit (tidak disebut di `CARA-PAKAI.md` sama sekali).

---

## 8e. Progressive Disclosure — apakah repo ini benar-benar bertahap?

Repo sudah punya tiga bahan baku untuk progressive disclosure: `/details` (router), komponen `disclosure.zsh`, dan sistem verbosity 0-3. Pertanyaannya bukan cuma "apakah verbosity berfungsi" (sudah dijawab 🔴 rusak di §6.2), tapi apakah *prinsip*-nya diterapkan di tempat lain juga:

| Lapisan disclosure | Ada? | Berfungsi? | Catatan |
|---|---|---|---|
| Default ringkas (level 0-1) | 🟡 Sebagian | Tidak lewat verbosity (rusak), tapi *secara struktural* beberapa command sudah ringkas by default (`aiplan` cuma output hasil, tidak ada log internal berlebih) | Ringkasnya kebetulan dari desain awal command, bukan hasil sistem disclosure yang sengaja dipasang |
| Detail opsional (`/details`) | 🟢 Ada, fungsional | Dipanggil dari dalam eksekusi agent untuk push detail tambahan (§3 tabel komponen) | **Hanya jalur agent** — command workflow (`aiplan`/`aispec`/dst) tidak punya jalan untuk "lihat detail lebih" karena mereka cuma satu blok output, tidak ada state untuk di-expand |
| Debug opsional (`AI_VERBOSE=1`) | 🟢 Ada, fungsional | Menampilkan detail Policy/Decision di permission flow (§6.3) | Berfungsi, tapi ini variabel **berbeda** dari `AI_VERBOSITY` — dua sistem senama-mirip, satu konsisten (getter tunggal), satu lagi berfungsi tapi lewat implementasi terduplikasi di 6 file (§6.2, dikoreksi dari klaim "mati" di draf sebelumnya) |

**Bacaan**: prinsip progressive disclosure repo ini sebenarnya *ada* dan cukup luas berfungsi (`/details` di jalur agent, `AI_VERBOSE` di permission, `AI_VERBOSITY` di logger/state machine/agent renderer) — tapi tersebar di sistem-sistem yang tidak saling terhubung dan sebagian tidak lewat API resmi yang dirancang untuk itu (§6.2). Efeknya bukan "user coba naikkan verbosity dan tidak terjadi apa-apa" seperti klaim draf sebelumnya — level 1-3 memang mengubah status line yang tampil — tapi user yang membaca `CARA-PAKAI.md` dan berharap **command workflow seperti `aiplan` sendiri** (bukan cuma status line request) berubah verbosity-nya akan tetap kecewa, karena `echo` di dalam `aiplan` sendiri tidak dikondisikan level manapun.

🔵 **Rekomendasi**: satukan implementasi `AI_VERBOSITY` (lihat §6.2) supaya satu API dipakai konsisten, lalu perluas cakupannya ke `echo` command-command workflow sendiri (bukan cuma status line request), dan perluas `/details` supaya bisa dipanggil setelah command workflow selesai (mis. `aiplan` selesai → hint "ketik `/details` untuk lihat prompt lengkap yang dikirim ke model"), bukan cuma di dalam loop agent.

---

## 9. Fase 9 — Discoverability

| Command | Mudah ditemukan? | Masalah |
|---|---|---|
| `ai <tab>` | Ya | `_ai_complete` (`45-completion.zsh`) pakai `_describe` zsh, nunjukin semua subcommand di `_AI_SUBCOMMANDS` — tapi **tanpa deskripsi per item** (`_describe` biasa mendukung format `"nama:deskripsi"`, di sini array-nya cuma nama polos) |
| Command Palette (`/`) | Ya, dengan syarat `gum` terinstall | Kalau `gum` tidak ada, `ui_palette` (`screens/palette.zsh`) langsung `return 1` dengan pesan error, tidak ada fallback listing biasa (fzf polos, atau `echo` list) |
| `ai h` (help) | Ya | Daftar 33 subcommand jadi **satu baris rata tanpa kategori** (`echo "  ${_AI_SUBCOMMANDS[*]}"`) — sulit di-scan meski isinya lengkap. Bandingkan dengan bagian "Agent modes" di bawahnya yang justru terkategori rapi dengan deskripsi per baris. |
| `aih` (history search via fzf) | 🔴 **Berpotensi tertukar dengan `ai h`** | Nama fungsi `aih` = cari riwayat; subcommand `ai h` (dengan spasi) = bantuan. Sangat mirip secara visual/pengucapan (`aih` vs `ai h`), komentar di kode sendiri mengakui ini ("CATATAN JUJUR: nama fungsi `aih()` ... SUDAH DIPAKAI ... BUKAN typo/bug") — jujur, tapi tidak mengubah fakta bahwa dari sisi user baru, ini ambigu. |
| `ai index` | 🔴 Tidak terdokumentasi | Ada di dispatcher & router, hilang dari `CARA-PAKAI.md` — user tidak akan tahu fitur ini ada kecuali baca kode atau ketik `ai h` (yang juga cuma nampilin nama tanpa deskripsi). |
| Onboarding `install.sh` | Sebagian | Langkah instalasi jelas & informatif (emoji + pesan Indonesia ramah), **tapi tidak mengarahkan user ke `ai deps` atau `ai h` di akhir proses** — hanya bilang "ketik `exec zsh`". User baru harus sudah baca `CARA-PAKAI.md` terpisah untuk tahu langkah berikutnya. |
| Empty state | 🟡 Perlu Verifikasi | `screens/home.zsh` selalu menampilkan hint "Ketik prompt atau / untuk Command Palette" — desain "AI-first" ini secara sengaja **tidak** menunjukkan daftar command di layar utama (`# Tidak ada menu list` — komentar eksplisit di file). Untuk user baru yang belum baca dokumentasi, satu-satunya jalan menemukan command adalah menekan `/`. |
| Command confidence saat tab-complete | 🟡 Rendah | Ketik `ai pla<Tab>` menyelesaikan ke `plan` (satu-satunya match), tapi karena `_ai_complete` tidak memakai format `"nama:deskripsi"` yang didukung `_describe` (baris di atas), user tidak melihat apa pun selain nama sebelum menekan Enter — beda dengan tool yang preview deskripsi command di completion list, di sini user harus **yakin dari ingatan** bahwa `plan` adalah command yang benar, bukan dari yang ditampilkan zsh saat itu. Untuk command yang mirip namanya (`aiplan`/`aispec`/`aiprompt`, §11b), ini menaikkan risiko salah pilih di completion tanpa sadar. |

**Skor Discoverability: 6/10.** Levenshtein-suggest di dispatcher dan Command Palette (kalau `gum` ada) adalah kekuatan besar. Titik lemah: dokumentasi tidak lengkap (index, scrap, testmodels hilang), help text tidak terkategori, nama `aih` vs `ai h` yang berpotensi salah paham, dan tab-complete tanpa deskripsi membuat user tidak yakin sebelum Enter (command confidence rendah).

### 9d. Empty-State — first-time vs returning user

`screens/home.zsh` secara sengaja tidak menampilkan menu (komentar eksplisit `# Tidak ada menu list`, §9 tabel), hanya hint *"Ketik prompt atau / untuk Command Palette"*. Ini keputusan desain "AI-first" yang koheren, tapi layar kosong saat ini adalah **satu state yang sama** untuk dua persona yang kebutuhannya berbeda — user hari pertama belum tahu apa-apa, user yang sudah punya 10 sesi tersimpan tahu semuanya tapi tidak dibantu melanjutkan kerjanya. Memisahkan keduanya membuat rekomendasi lebih tajam daripada satu daftar elemen campur:

**First-time user** (belum ada history/session tersimpan — bisa dideteksi dari `_AI_SESSION`/log kosong):
- Hint navigasi dasar: `/` untuk Command Palette, `ai h` untuk daftar command — ini sudah ada.
- Satu baris contoh prompt konkret (mis. *"Contoh: ketik 'buatkan rencana redesign homepage' lalu Enter"*) — **belum ada**, dan ini yang paling menolong first-time user karena AI-first interface tanpa contoh sama sekali mengasumsikan user sudah tahu cara "bicara" ke sistemnya.
- Tidak perlu menampilkan resume/session — kosong, tidak relevan untuk persona ini.

**Returning user** (ada history/session tersimpan):
- Sesi/resume terakhir (`--resume <slug>`) — fitur resume ada tapi tidak ditemukan pasif (§6b, §10b), paling bernilai untuk persona ini karena mereka *sudah* punya konteks yang tertunda.
- Project terakhir dibuka — konteks langsung tanpa `cd`/`proj` manual (`proj`/`pj` sudah punya data project switcher).
- Command/subcommand terakhir dipakai — mempercepat pengulangan tugas yang sama (coding loop biasanya berulang beberapa command per hari), bisa diturunkan dari log/history yang sudah dipakai `aih`.
- Hint navigasi dasar (`/`, `ai h`) tetap relevan sebagai baris kedua/ketiga, bukan baris utama — returning user sudah tahu ini ada, tidak perlu jadi fokus layar.

**Catatan**: ini **bukan** rekomendasi untuk membalik keputusan "AI-first, tanpa menu list" — itu tetap keputusan desain yang valid dan disengaja untuk kedua persona. Yang diusulkan adalah membuat layar kosong **bercabang berdasarkan state yang sudah dimiliki repo** (ada/tidaknya session tersimpan) alih-alih satu tampilan statis untuk semua orang — first-time butuh *contoh cara mulai*, returning butuh *jalan melanjutkan*, dan keduanya adalah kebutuhan yang berlawanan kalau digabung jadi satu layar.

---

## 10. Fase 10 — Cognitive Load

| Tipe User | Skor | Alasan |
|---|---|---|
| Beginner | 5/10 | Entry point AI-first (`ai` tanpa argumen) bagus untuk yang tidak mau hafal command — tapi begitu keluar dari situ (mis. baca `CARA-PAKAI.md`, atau `ai h`), langsung dihadapkan ~35 subcommand dengan awalan `ai` yang mirip-mirip (`aiplan` vs `aiprompt` vs `aispec` — tiga command dengan output yang mirip secara konsep tapi beda scope, lihat §11). |
| Intermediate | 7/10 | Setelah paham pola dasar (`ai <verb> <goal>`, approval box, backup otomatis), sistem jadi cukup prediktif. Naming `ai<verb>` konsisten membantu di level ini. |
| Power User | 8/10 | `--yolo`, `--no-review`, `--resume`, `AI_VERBOSE=1`, `/config verbosity` (meski §6.2 rusak), session management — cukup kaya untuk dikuasai. |

Jumlah istilah teknis yang harus dipahami cukup tinggi untuk pemula: *provider*, *fallback*, *checkpoint*, *slug*, *YOLO mode*, *ask_once_per_file*, *smart/fast/big task class* — semuanya nyata dan berguna, tapi tidak ada satu pun "glossary" atau `ai h` yang menjelaskan istilah ini untuk pemula (help text langsung asumsikan familiar).

### 10b. Jalur Power-User — berapa langkah yang benar-benar dihemat?

Repo punya empat mekanisme power-user (`--yolo`, `--resume`, session management, checkpoint). §10 sudah memberi skor 8/10 untuk kekayaan fiturnya, tapi belum menghitung **penghematan langkah nyata** dibanding jalur default:

| Mekanisme | Jalur default (tanpa fitur) | Jalur power-user | Langkah dihemat |
|---|---|---|---|
| `--yolo` | Setiap file/proses yang disentuh agent → 1 approval box (baca + `y`/Enter) | Semua approval di-skip, kecuali tetap dijaga whitelist shell (§6.3) | Sebanding jumlah tool-call di task — untuk task 7-9 langkah (§6 tabel `ai agent`), bisa menghemat hingga ~5-7 interaksi Enter/`y` per run |
| `ask_once_per_file` | Approval berulang tiap file yang sama disentuh lagi dalam sesi | Approval sekali per file per sesi | Menghemat approval berulang khususnya di task yang bolak-balik ke file sama (refactor lintas fungsi) — jumlah persis tergantung pola edit, tidak terukur dari kode saja 🟡 |
| `--resume <slug>` | Ulang dari awal: input goal baru, AI baca ulang konteks project dari nol | Lanjut langsung dari checkpoint terakhir, skip fase re-orientasi | Menghemat 1 siklus penuh "AI membaca ulang konteks" — signifikan untuk task besar, tapi baru bisa dipakai kalau user **sudah tahu** flag ini ada (lihat §9d, tidak terekspos di empty-state) |
| `AI_VERBOSE=1` | — (fitur tambahan info, bukan penghemat langkah) | Menampilkan detail Policy/Decision tanpa harus baca kode untuk paham kenapa approval muncul | Bukan penghematan langkah, tapi penghematan waktu debugging/pemahaman |

**Bacaan**: penghematan terbesar (`--yolo`, `ask_once_per_file`) justru untuk task berisiko tinggi (approval di-skip), sementara penghematan yang paling aman dan paling sering relevan sehari-hari (`--resume`) adalah yang **paling tersembunyi** dari discovery pasif. Ini kebalikan dari urutan ideal — fitur paling aman & paling sering berguna seharusnya paling mudah ditemukan, bukan paling butuh baca dokumentasi dulu. Ini memperkuat rekomendasi §9d: tampilkan sesi/resume terakhir di empty-state.

---

## 11. Fase 11 — Command Naming Audit

**Pola positif**: prefix `ai` konsisten untuk *hampir* semua command AI (`aic`, `aicl`, `aiask`, `aicode`, `aifix`, `airun`, `aicat`, `aipatch`, `aiundo`, `aiplan`, `aispec`, `aibuild`, dst) — mental model "kalau mulai dengan `ai`, itu fitur AI" cukup kuat terbentuk.

**Masalah konkret**:
- `aih` (history search, fzf) vs `ai h` (help text) — nyaris identik secara visual/pelafalan, fungsi sangat berbeda. 🟢 (lihat §9)
- `aic` (chat pendek) vs `aicl` (chat panjang, multi-stage) vs `aicat` (baca file!) — tiga huruf pertama sama (`aic`), fungsi sama sekali tidak berhubungan. `aicat` khususnya berisiko diketik sebagai singkatan alami dari "AI cat file" tapi user yang baru kenal `aic`/`aicl` bisa salah tebak ini varian chat lain.
- `aiplan` vs `aispec` vs `aiprompt` vs `aibuild` — keempatnya menghasilkan dokumen terstruktur dari deskripsi/goal, tersimpan ke file, dengan alur sangat mirip (generate → simpan → tampilkan path). Perbedaan konseptual (rencana produktivitas vs spec aplikasi vs prompt siap-pakai vs build end-to-end) **tidak tercermin di nama** — user baru harus baca deskripsi masing-masing untuk tahu bedanya, namanya sendiri tidak cukup membedakan.
- `aiscan` (project scan) vs `aiscrap` (web scraper generator) — huruf awal sama (`ais`), fungsi jauh berbeda (satu untuk analisis project lokal, satu untuk generate script scraping web).
- Konflik dengan command shell umum: tidak ditemukan konflik nyata dengan builtin/command Unix umum (bagus) — semua prefix `ai`/ai-derivatives cukup unik. Alias non-AI (`ll`, `gs`, `gc`, `gp`, `c`, `v`) sudah pola umum dotfiles, wajar.

🔵 **Rekomendasi**: dokumentasikan perbedaan `aiplan`/`aispec`/`aiprompt`/`aibuild` secara eksplisit dalam satu tabel perbandingan di `CARA-PAKAI.md` (belum ada), dan pertimbangkan alias yang lebih deskriptif untuk `aih` (mis. `aihist`) untuk memisahkan dari `ai h`.

### 11b. Decision Tree — kapan pakai `aiplan` vs `aispec` vs `aiprompt` vs `aibuild`?

Keempat command ini berada di ruang keputusan yang sama secara mental model user ("saya punya ide, saya mau AI bantu strukturkan sebelum coding") tapi berbeda scope. Tabel perbandingan (rekomendasi di atas) membantu, tapi bentuk yang lebih langsung dipakai saat user bingung *di momen itu juga* adalah decision tree:

```
Saya punya ide/goal, saya butuh AI bantu strukturkan dulu sebelum coding...

├─ Saya cuma butuh dokumen rencana buat DIRI SAYA BACA
│  (bukan buat dikasih ke AI lain / tidak akan langsung dieksekusi)
│  → aiplan
│
├─ Saya butuh spesifikasi TEKNIS APLIKASI
│  (arsitektur, requirement, siap jadi acuan development)
│  → aispec
│
├─ Saya butuh teks PROMPT SIAP PAKAI
│  (untuk dikirim ke AI lain / dipakai ulang di tempat lain)
│  → aiprompt
│
└─ Saya mau langsung dari goal → PROJECT JADI
   (spec + scaffold project sekaligus, end-to-end)
   → aibuild   (setara aispec + aiproject digabung, lihat §6 tabel)
```

Kalau decision tree ini terasa terlalu kaku untuk sebagian kasus abu-abu (mis. rencana yang belakangan mau dijadikan spec juga), minimal versi ringkasnya sebagai tabel "gunakan ini ketika..." di `ai h` dan `CARA-PAKAI.md` sudah akan mengurangi banyak kebingungan — saat ini user harus baca deskripsi keempatnya satu-satu dan menyimpulkan sendiri perbedaannya (§11 temuan awal).

🔵 **Rekomendasi**: tempelkan versi ringkas tree ini (atau tabel "gunakan ketika...") langsung di output `ai h` bagian Workflow, bukan hanya di `CARA-PAKAI.md` — supaya muncul di titik keputusan (saat user mengetik `ai h` karena bingung), bukan cuma di dokumen terpisah yang harus dicari duluan.

---

## 12. Fase 12 — Accessibility Terminal

| Aspek | Status | Bukti |
|---|---|---|
| Lebar 80 kolom / terminal sempit | 🟢 Baik | `_ai_ui_width` (`00-ui_text.zsh`) dipakai box & step-rule untuk adaptif lebar — tapi ini cuma menolong command yang pakai `_ai_ui_box`; command `echo`-polos (mayoritas) tidak melakukan wrap sama sekali dan berpotensi baris panjang terpotong aneh di terminal sempit Termux. |
| ASCII fallback (non-unicode) | 🟢 Baik | `_ai_ui_supports_unicode` (`00-ui_text.zsh`) dicek sebelum render box/icon — `┌─┐│└┘` → `+-+|++`, `✓✗→◌•` → `+x>~*`. Desain eksplisit dan hati-hati. |
| Warna / buta warna | 🟡 Sebagian | Warna dipakai semantically (merah=error, hijau=sukses, kuning=approval, biru=info) — untuk buta-warna merah-hijau (deuteranopia, paling umum), skema **merah vs hijau berdampingan** (diff `-`/`+` di `aipatch`) berisiko sulit dibedakan tanpa penanda non-warna (`-`/`+` karakternya sendiri untungnya sudah membantu di sini karena bukan cuma warna, ada simbol). Box approval (kuning) vs blocked (merah) vs sukses (hijau) di `_ai_ui_box_accent` **mengandalkan warna sebagai pembeda utama** antara approval dan blocked tanpa simbol tambahan yang eksplisit dibedakan untuk kasus itu — 🟡 perlu verifikasi dengan color-blindness simulator. |
| Tanpa emoji (mode aksesibel/terminal minim font) | 🟡 Sebagian | Sistem `60-ui/` sengaja unicode-icon (bukan emoji) dengan fallback ASCII — desain baik. Tapi `install.sh` dan satu baris di `aicl` (`❌`) pakai emoji sungguhan yang **tidak** melalui pengecekan dukungan font, berisiko tampil sebagai kotak/`?` di beberapa terminal Termux lama. |
| Termux khusus | 🟢 Baik | Banyak guard spesifik Termux: `termux-clipboard-get/set`, `termux-share`, `termux-wake-lock`, `termux-battery-status`, `termux-notification` — semuanya dicek keberadaannya dulu (`command -v`) sebelum dipakai, dengan pesan install yang jelas kalau hilang (`ai deps`). |
| Linux/macOS | 🟡 Sebagian | `CARA-PAKAI.md` eksplisit menyebut dukungan macOS "parsial" — beberapa fitur Termux-only (share, wake-lock, battery) otomatis tidak aktif di platform lain (graceful degradation lewat `command -v`), tapi tidak ada pesan eksplisit ke user macOS/Linux soal fitur mana yang tidak akan pernah tersedia untuk mereka. |
| Mode `NO_COLOR` / monochrome | 🟡 Sebagian — **konsisten di satu sistem, bocor di sistem lain** | `_ai_ui_supports_color` (`60-ui/02-ui_colors.zsh`) sudah menghormati **konvensi standar `NO_COLOR`** (juga `AI_UI_NO_COLOR=1`) dan otomatis mati kalau `TERM=dumb` atau output di-pipe — desain yang benar dan eksplisit dikomentari sebagai "konvensi standar". Tapi ini **hanya berlaku untuk komponen yang lewat `AI_C_*`**; diff colorizer di `aipatch` (`35-files/10-aipatch.zsh`) dan `aicode -o` (`30-code/05-code.zsh`) pakai `printf '\033[31m'`/`'\033[32m'` hardcode langsung (§4 baris "Warna ANSI"), sama sekali tidak mengecek `_ai_ui_supports_color` atau `$NO_COLOR` — jadi user yang set `NO_COLOR=1` untuk terminal monochrome/aksesibilitas tetap akan menerima kode ANSI mentah di output diff. |
| Terminal sempit (narrow-width stress) | 🟡 Sebagian | `_ai_ui_width` (`00-ui_text.zsh`) membaca `$COLUMNS`/`tput cols`, fallback ke 40 kalau tidak terdeteksi, dan **di-clamp minimum 20** kolom — komentar kode eksplisit menyebut ini disengaja untuk "layar HP sempit". Desainnya cukup matang secara struktur, tapi seperti helper lain, coverage-nya terbatas pada command yang memanggil `_ai_ui_box`. Detail per lebar di tabel stress test di bawah. |

### 12b. Narrow-Terminal Stress Test

`_ai_ui_width` punya perilaku terdefinisi jelas dari kode (baca `$COLUMNS` → fallback `tput cols` → fallback 40 → clamp minimum 20), jadi bisa diproyeksikan perilakunya di beberapa lebar umum tanpa perlu eksekusi live untuk *strukturnya* — meski hasil visual aktual tetap 🟡 perlu diverifikasi mata langsung, terutama untuk command yang tidak lewat `_ai_ui_box` sama sekali:

| Lebar terminal | Skenario nyata | Prediksi `_ai_ui_box`/`_ai_ui_wrap` (dari kode) | Prediksi command `echo`-polos (mayoritas, §13b) |
|---|---|---|---|
| 80 kolom | Terminal desktop standar | 🟢 Aman — lebar penuh dipakai, wrap tidak perlu aktif untuk box pendek | 🟢 Aman — baris pendek/normal tidak masalah |
| 60 kolom | Tmux split 2 panel di layar medium | 🟢 Aman secara struktur — `_ai_ui_wrap` aktif memotong kata per lebar box | 🟡 Berisiko — baris panjang dari `echo` (mis. path file panjang di pesan error) tidak di-wrap, bisa terpotong ganjil oleh terminal itu sendiri |
| 40 kolom | HP portrait sempit / tmux split 3+ panel | 🟢 Aman — ini persis kondisi yang disebut komentar kode sebagai alasan desain (fallback default 40) | 🔴 Berisiko tinggi — diff `aipatch`/`aicode -o` (ANSI hardcode, §12 tabel) dan pesan panjang lain kemungkinan terpotong tanpa wrap, memecah baris di tengah kode ANSI (berpotensi meninggalkan warna "menyala" ke baris berikutnya) |
| 20 kolom | Kasus ekstrem (clamp minimum) | 🟡 Perlu Verifikasi — box mungkin secara teknis tetap "valid" (tidak crash) di clamp 20, tapi belum diverifikasi apakah kontennya masih *berguna* dibaca pada lebar seekstrem itu, atau cuma tidak error | 🔴 Sangat berisiko — hampir semua `echo` panjang akan terpotong terminal, termasuk pesan approval/error penting |

**Bacaan**: `_ai_ui_width`/`_ai_ui_wrap` sendiri terbukti dirancang dengan sengaja untuk skenario sempit (komentar kode eksplisit menyebut "layar HP" — konteks Termux jelas dipikirkan). Tapi persis seperti pola berulang di seluruh audit ini (§13b Rendering Layer Fragmentation): perlindungan ini **hanya melindungi command yang memakai `_ai_ui_box`**. Mayoritas command (`echo`-polos, §3 tabel) tidak mendapat manfaat apa pun dari helper ini — risikonya naik justru di command yang paling sering dipakai harian (`aiplan`/`aispec`/`aicommit`, bukan yang jarang dipakai seperti agent).

🔵 **Rekomendasi**: masukkan verifikasi visual langsung di 40 dan 60 kolom sebagai bagian dari checklist Sprint 3 (§15) saat command workflow dimigrasi ke `_ai_ui_box` — migrasi itu otomatis menutup celah narrow-terminal untuk command yang dipindah, jadi tidak perlu kerja terpisah.

**Uji per komponen terhadap `NO_COLOR=1`** (diturunkan dari kode, belum dijalankan langsung — 🟡):

| Komponen | Lewat `AI_C_*`? | Masih terbaca (kode ANSI mentah tidak bocor)? |
|---|---|---|
| Approval box (`06-permissions/`) | 🟢 Ya | 🟢 Ya — otomatis matikan warna, box/border ASCII tetap tampil |
| State machine (`aiagent`) | 🟢 Ya | 🟢 Ya |
| Diff `aipatch`/`aicode -o` | 🔴 Tidak (hardcode) | 🔴 Tidak — kode `\033[31m` dst tetap tercetak literal, berpotensi mengotori output atau merusak keterbacaan di terminal yang benar-benar monochrome |
| Progress/status line (`aiplan` dst) | 🟢 Ya (lewat logger, §6.2) | 🟢 Ya |

**Skor Accessibility: 7/10** — desain dasarnya (unicode+fallback, adaptif width, `NO_COLOR` awareness) di atas rata-rata CLI tool pada umumnya, tapi coverage-nya sama seperti masalah visual consistency: hanya menyentuh jalur agent/permission/logger — jalur yang sama persis dengan yang belum memakai `AI_C_*` di §4 (diff colorizer) juga jadi satu-satunya kebocoran `NO_COLOR`, bukan kebetulan.

🔵 **Rekomendasi**: alirkan diff colorizer lewat `AI_C_ERR`/`AI_C_OK` (ini juga rekomendasi §4 dan §16 #12) — memperbaikinya sekaligus menutup dua temuan: drift tema warna DAN kebocoran `NO_COLOR`.

---

## 13. Fase 13 — Comparative UX Benchmark

*(Catatan: perbandingan ini berbasis pemahaman umum pola desain CLI-agent sejenis (Claude Code, Aider, GitHub Copilot CLI, Gemini CLI, Codex CLI) per awal 2026, bukan hasil pengujian langsung berdampingan — tandai sebagai referensi kualitatif, bukan benchmark terukur.)*

| Aspek | zsh_bagas (`ai agent`) | Tool CLI-agent sejenis pada umumnya |
|---|---|---|
| Onboarding | Instalasi manual (clone+symlink+chmod+install pkg) lewat `install.sh` — cukup banyak langkah manual dibanding tool modern yang biasanya satu binary/npm install | Umumnya satu perintah install (npm/curl script) + auto-detect API key |
| Discoverability | Slash command + palette + tab-complete + fuzzy-typo-correct — setara secara konsep, unggul di typo-correction (jarang ditemukan di tool lain) | Biasanya slash command tanpa fuzzy-correct otomatis |
| Planning | `aiplan`/`aispec`/`aibuild` terpisah dari `aiagent` — lebih granular (bisa generate spec tanpa langsung eksekusi) dibanding banyak tool lain yang plan-nya menyatu di dalam loop agent | Umumnya planning adalah bagian implisit dari satu loop agent, kurang exposed sebagai command terpisah |
| Editing | `aipatch` full-file rewrite + diff + confirm — desain sadar-keterbatasan-model-kecil (dijelaskan eksplisit di komentar kode: model kecil sering gagal generate unified-diff yang valid), pragmatis untuk konteks fallback multi-provider murah/gratis | Tool premium (Claude Code dkk) umumnya pakai diff/patch granular karena modelnya cukup kuat menghasilkan diff valid |
| Review | Auto-review setelah agent selesai (`--no-review` untuk skip) — fitur bagus, tapi read-only/informational saja | Sejumlah tool serupa punya siklus review-lalu-lanjut-edit otomatis; di sini eksplisit tidak (dijelaskan di `ai h`: "gak auto-lanjut edit lagi") — desain yang disengaja, bukan kekurangan |
| Commit | `aicommit` generate pesan + confirm — setara fitur umum di banyak tool | Setara |
| Resume | `--resume <slug>` + checkpoint — setara fitur `--continue`/session resume di tool lain | Setara |
| Permission | Box approval granular per-tool-type (process/file/action), `ask_once_per_file` — desain matang, sebanding dengan permission model tool kelas atas | Sebanding, kadang lebih |
| Completion | Report akhir dengan files-changed + runtime + review — cukup lengkap | Setara |

**Kesimpulan komparatif**: untuk *jalur agent*, `zsh_bagas` sudah mendekati kualitas UX tool CLI-agent modern kelas atas, dengan kekuatan unik di typo-correction dan desain permission granular. Kelemahan utamanya bukan di jalur agent, tapi di **konsistensi lintas seluruh permukaan produk** — banyak tool pembanding punya satu "rendering layer" tunggal yang dipakai semua command; di sini rendering layer bagus itu baru dipakai sebagian kecil command.

---

## 13b. Rendering Layer Fragmentation — Temuan Kritis Terkonsolidasi

Beberapa bagian audit ini (§3 komponen, §4 visual consistency, §4b bahasa, §12 aksesibilitas/`NO_COLOR`) masing-masing menyentuh gejala yang sama dari sudut berbeda. Dikumpulkan jadi satu tempat, akarnya jelas: **repo ini punya lima renderer berbeda yang tidak berbagi kontrak apa pun satu sama lain.**

| # | Renderer | Dipakai oleh | Sadar `AI_C_*`? | Sadar `NO_COLOR`? | Sadar unicode-fallback? |
|---|---|---|---|---|---|
| 1 | `_ai_ui_box`/`_ai_ui_line` (box/state system) | Agent loop, permission dialogs (§3) | 🟢 Ya | 🟢 Ya (lewat `AI_C_*`) | 🟢 Ya (`_ai_ui_supports_unicode`) |
| 2 | `echo`/`printf` polos | `aiplan`, `aispec`, `aiprompt`, `aisummarize`, `aicommit`, `aireview`, `aicat`, `aiundo`, `aibakclean`, `aishare` (§3) | 🔴 Tidak | — (tidak ada warna untuk dimatikan) | — (tidak ada icon untuk fallback) |
| 3 | ANSI hardcode inline (diff colorizer) | `aipatch`, `aicode -o` (§4, §12) | 🔴 Tidak | 🔴 **Tidak — bocor** (§12) | — |
| 4 | Emoji langsung | `install.sh`, satu baris `aicl` (§4) | 🔴 Tidak | — (emoji bukan ANSI, tidak terpengaruh `NO_COLOR`, tapi tidak dicek dukungan font) | 🔴 Tidak (§12 baris emoji) |
| 5 | Text label polos (`OK`/`MISSING`) | `ai deps` (§3) | 🔴 Tidak | — | — |

**Kenapa ini layak jadi satu temuan kritis tersendiri, bukan disebar**: dibaca terpisah di §3/§4/§12, masing-masing terlihat seperti masalah kecil berbeda-beda (satu command lupa pakai warna, satu lagi lupa cek `NO_COLOR`, dst). Dibaca sebagai satu tabel, polanya jelas — **hanya Renderer #1 yang punya kontrak lengkap** (warna + no-color + unicode-fallback secara bersamaan), dan seluruh renderer #2-5 masing-masing kehilangan sebagian atau seluruh kontrak itu **secara independen**, bukan karena keputusan desain sadar untuk berbeda. Setiap kali seseorang menambah command baru ke repo ini, mereka harus menebak sendiri renderer mana yang mereka ikuti — dan defaultnya (renderer #2, `echo` polos) adalah yang paling primitif, bukan yang paling lengkap.

Ini juga menjelaskan kenapa perbaikan tambal-sulam (menambahkan `NO_COLOR` check ke satu diff colorizer, menambahkan `AI_C_*` ke satu command) akan terus muncul lagi di command berikutnya yang belum disentuh — akar masalahnya bukan di command manapun secara spesifik, tapi di **tidak adanya satu kontrak rendering yang wajib diikuti semua command baru**.

🔵 **Rekomendasi**: sebelum (atau bersamaan dengan) Sprint 3 di §15 yang menerapkan box/warna ke `40-workflow/*`, definisikan kontrak renderer minimal yang wajib dipenuhi *command apa pun* di repo ini — warna lewat `AI_C_*` (bukan hardcode), unicode dengan fallback ASCII, dan tunduk pada `NO_COLOR`/`AI_UI_NO_COLOR`. Begitu kontrak itu ada (bisa berupa fungsi wrapper wajib, atau sekadar checklist review PR), migrasi command satu-per-satu di Sprint 3 sekaligus menutup celah `NO_COLOR` di §12 tanpa kerja tambahan.

---

## 13c. Interaction Consistency Score

Audit ini sudah punya skor per-dimensi (Discoverability 6/10, Accessibility 7/10, dst) tapi belum ada skor yang mengukur konsistensi *pola interaksi* — bagaimana user berinteraksi (menekan Enter, membatalkan, mengulang) diperlakukan sama atau berbeda di seluruh command. Ini melengkapi skor Visual Consistency (§4, murni soal tampilan) dengan sisi perilakunya:

| Area interaksi | Skor | Dasar |
|---|---|---|
| Confirm (Enter/`y` untuk lanjut) | 4/10 | Tiga pola berbeda untuk aksi setara: `_ai_perm_ask` terpusat (agent tools), `gum confirm`/`read -t 60` duplikat (aicommit/aipatch/aicode), `read -t 30` duplikat terpisah (aiundo/aibakclean) — §6.4 |
| Cancel/Escape (`Ctrl+C`) | 8/10 | Trap SIGINT/SIGTERM eksplisit dan konsisten di dua titik kritis (request AI, agent loop) dengan cleanup dan pesan status jujur — §8d. Diturunkan dari sempurna karena cakupan trap di command non-request (mis. antara backup/apply di `aipatch`) belum diverifikasi |
| Retry (setelah gagal) | 6/10 | `airun` retry eksplisit dengan counter jelas (§8), tapi command lain tidak punya pola retry yang konsisten — sebagian gagal total tanpa opsi ulang otomatis |
| Resume (lanjut sesi) | 7/10 | `--resume`/checkpoint matang secara implementasi (§6.3, §10b) tapi tidak ditemukan pasif dari UI (§9d) — skor teknis tinggi, skor UX-discoverability rendah, dirata-rata jadi sedang |
| Back/undo (batalkan hasil) | 6/10 | `aiundo` ada dan berfungsi untuk file changes, tapi tidak ada pola "back" universal untuk aksi lain (mis. tidak bisa "undo" sebuah plan/spec yang sudah dibuat selain hapus file manual) |

**Skor Interaction Consistency keseluruhan: ~6/10** (rata-rata tertimbang, confirm dan retry paling menarik turun). Dibaca bersama skor lain:

| Area | Skor |
|---|---|
| Visual | 4/10 |
| Workflow (loop coding/debug/review/commit/resume, §6b) | 8/10 |
| Permission | 9/10 |
| **Interaction** | **6/10** |

**Bacaan**: pola yang sama terus muncul di seluruh audit ini — jalur *agent* (permission 9/10, cancel 8/10 dalam skor interaction) jauh lebih matang daripada jalur *command langsung* (confirm 4/10, visual 4/10). Interaction Consistency Score mengkonfirmasi dari sudut pandang lain bahwa akar masalah repo ini bukan kekurangan fitur, tapi **fitur bagus yang belum menyebar merata ke seluruh permukaan produk** — persis kesimpulan §1 dan §13b, kali ini terukur lewat bagaimana user berinteraksi, bukan lewat apa yang mereka lihat.

---

## 14. Fase 14 — Friction Matrix

### 14.0 Rubrik Severity

Draf sebelumnya memberi label severity secara ad-hoc per temuan tanpa kriteria eksplisit tertulis di satu tempat, sehingga beberapa isu naming terasa nyaris setara isu arsitektur (mis. `aih` vs `ai h` vs konflik `ui_palette()`) meski dampaknya jauh berbeda. Rubrik berikut dipakai untuk mengaudit ulang seluruh severity di tabel bawah:

| Level | Kriteria |
|---|---|
| 🔴 Critical | Merusak workflow utama — data hilang/korup, fitur inti berhenti berfungsi, atau kegagalan silent tanpa error yang bisa merusak pengalaman produk secara fundamental |
| 🟠 High | Menghambat penggunaan harian — friksi nyata di loop yang sering dipakai (§6b), atau jaminan keamanan/kontrak yang tidak konsisten antar command serupa |
| 🟡 Medium | Menambah kebingungan — ambiguitas, inkonsistensi kecil, atau dokumentasi hilang yang bisa diatasi dengan usaha ekstra dari user tapi tidak menghentikan pekerjaan |
| 🟢 Low | Polish — estetika, technical debt tanpa dampak user langsung, atau optimisasi yang baik-untuk-ada tapi tidak mendesak |

**Hasil audit ulang terhadap rubrik ini**: hampir seluruh severity di draf sebelumnya sudah selaras begitu diperiksa ulang satu-satu — `ui_palette()` conflict memang Critical (bisa merusak Command Palette secara silent, kriteria tepat), `aifix` vs `aipatch` memang High (kontrak keamanan berbeda untuk operasi setara). Yang **direvisi turun** mengikuti kriteria di atas: `aih` vs `ai h` sudah berstatus Medium di tabel bawah (bukan mendekati isu struktural seperti dikira) — ini dikonfirmasi ulang sebagai Medium yang tepat (ambiguitas nama, bukan kerusakan fungsi, karena keduanya read-only). Perubahan paling signifikan: `airun` auto-apply sebelumnya ditulis "Sedang-Tinggi" (ambigu) — mengikuti rubrik ini dipastikan sebagai **High**, karena kriterianya jelas terpenuhi (jaminan keamanan tidak konsisten antar command serupa), bukan sekadar polish.

| Command/Area | Friction | Severity | Impact |
|---|---|---|---|
| Command Palette (dua `ui_palette`) | Fungsi silent-override, bergantung urutan alfabetis folder | 🔴 **Critical** | Bisa rusak total tanpa error jelas di refactor masa depan |
| **Rendering Layer Fragmentation** (5 renderer tanpa kontrak bersama, §13b) | Setiap command baru harus menebak sendiri renderer mana yang diikuti; default paling primitif | 🟠 **High** | Akar penyebab visual inconsistency (§4), kebocoran `NO_COLOR` (§12), dan drift bahasa (§4b) — memperbaiki gejalanya satu-satu tanpa kontrak akan terus berulang di command berikutnya |
| `aifix` vs `aipatch` inkonsistensi review | Command serupa, jaminan keamanan berbeda | 🟠 **High** | Risiko data loss/corrupt kalau user berasumsi `aifix` seaman `aipatch` |
| `airun` auto-apply fix tanpa confirm | Bypass niat `aifix` yang minta review manual | 🟠 **High** *(dipastikan, bukan lagi "Sedang-Tinggi" ambigu — sesuai rubrik 14.0: jaminan keamanan tidak konsisten antar command serupa)* | Kode berubah otomatis 2x percobaan tanpa persetujuan eksplisit per-perubahan |
| `/config verbosity` | Getter resmi tidak dipakai, 6 file lain re-implementasi inline sendiri-sendiri | 🟡 **Medium** *(diturunkan dari Tinggi — lihat koreksi §6.2: fiturnya berfungsi, masalahnya duplikasi implementasi bukan fitur mati)* | Technical debt maintenance, bukan lagi masalah kepercayaan user pada dokumentasi |
| Visual inconsistency (box hanya di agent) | Command sehari-hari (plan/spec/commit) terasa "kurang jadi" dibanding agent | 🟡 **Medium** | Persepsi kualitas produk tidak merata |
| `aih` vs `ai h` | Ambiguitas nama | 🟡 **Medium** *(dikonfirmasi tepat sesuai rubrik 14.0 — bukan isu struktural)* | Salah ketik berpotensi memicu operasi berbeda dari yang diniatkan (meski keduanya read-only, jadi risiko rendah secara data) |
| Dokumentasi tidak lengkap (`ai index`, `aiscrap`, `testmodels`) | Command tersembunyi | 🟡 **Medium** | Fitur berguna tidak ditemukan user |
| Diff warna hardcode terpisah dari `AI_C_*` (aipatch/aicode) | Drift risk kalau tema warna berubah + kebocoran `NO_COLOR` (§12) | 🟡 **Medium** *(dipastikan dari "Rendah-Sedang" — kebocoran aksesibilitas nyata, bukan cuma drift estetika)* | Konsistensi visual jangka panjang, dan aksesibilitas `NO_COLOR` |
| Confirm timeout beda (30 vs 60 detik) tanpa alasan jelas | Kode duplikat | 🟢 **Low** | Tidak berbahaya, tapi menandakan tidak ada single-source-of-truth |
| Emoji installer vs icon system utama | Estetika tidak seragam | 🟢 **Low** | Kesan pertama (saat instalasi) tidak konsisten dengan produk final |
| `ai_home`/menu re-source banyak file setiap invocation | Potensi latensi kecil tiap kali `ai` tanpa argumen dipanggil | 🟢 **Low** *(arsitektur, disebut karena berdampak ke UX responsiveness — bukan audit keamanan/arsitektur)* | 🟡 Perlu Verifikasi — belum diukur waktunya nyata di device |
| `ui_card_summary`/`ui_card_stats`/`ui_approve` dead code | Tidak dipakai di mana pun | 🟢 **Low** | Tidak berdampak user langsung, tapi menambah beban kognitif maintainer |

---

## 15. Fase 15 — Redesign Proposal

### Quick Wins (≤30 menit)

1. **Perbaiki dokumentasi**: tambahkan `ai index`, `aiscrap`, `ai testmodels` ke `CARA-PAKAI.md`. *(File: `CARA-PAKAI.md`)*
2. **Selesaikan konflik `ui_palette`**: rename fungsi di `components/palette.zsh` menjadi `ui_palette_generic` (atau hapus jika memang tidak dipakai) supaya tidak ada override implisit. *(File: `60-ui/components/palette.zsh`)*
3. **Perbaiki teks default verbosity**: samakan klaim default (`0` di kode vs `1` di dokumen) — pilih satu dan sinkronkan keduanya. *(File: `CARA-PAKAI.md`, `60-ui/components/verbosity.zsh`)*
4. **Tambah next-step hint ke `aiplan`**, menyamakan pola `aispec`/`aiprompt` yang sudah punya baris `"Lanjut: ..."`. *(File: `40-workflow/05-aiplan.zsh`)*
5. **Ganti emoji `❌` di `aicl`** dengan `_ai_ui_line "✗" "..."` supaya konsisten dengan sistem icon yang sudah ada. *(File: `20-chat/00-quick_chat.zsh`)*

**Mockup ASCII — `aiplan` sebelum vs sesudah**:
```
SEBELUM:                          SESUDAH:
Generating rencana...             → Menyusun rencana...
<dump markdown>                   <dump markdown>
                                   ✓ Rencana tersimpan di: plan/xxx.md
Rencana tersimpan di: ...           Lanjut: aicode / aiproject dari rencana ini
```

### Medium Improvements (1-4 jam)

1. **Samakan kontrak `aifix`**: tambahkan diff otomatis + confirm + backup persis pola `aipatch`, supaya semua "AI mengubah file existing" konsisten kontraknya baik dipanggil standalone maupun dari `airun`. *(File: `30-code/45-fix.zsh`, `30-code/50-run.zsh`)*
2. **Ekstrak fungsi `_ai_confirm(msg, timeout)` bersama** dari 4 duplikasi (`aicommit`, `aipatch`, `aicode`, `aiundo`, `aibakclean`) — satu titik implementasi, satu timeout policy yang disengaja. *(File baru: `06-permissions/` atau `10-core/`)*
3. **Alirkan warna diff lewat `AI_C_ERR`/`AI_C_OK`** alih-alih ANSI hardcode, di `aipatch` dan `aicode -o`. *(File: `35-files/10-aipatch.zsh`, `30-code/05-code.zsh`)*
4. **Kategorikan `ai h`** menjadi grup (Chat / Code / Files / Workflow / Project / Agent / Utility) alih-alih satu baris rata 33 kata. *(File: `60-ui/10-help_stats.zsh`)*

**Mockup ASCII — `ai h` sebelum vs sesudah**:
```
SEBELUM:
Subcommand yang tersedia (ai <subcommand> ...):
  chat long code edit view scan fix run build project scrap ask shell commit review debug research plan prompt spec summarize clip session agent stats log menu deps dev testmodels undo bakclean share index update h

SESUDAH:
Chat     : chat, long, ask, clip, shell
Code     : code, fix, run, scrap
Files    : view, edit, undo, bakclean, share
Project  : scan, index, project, build
Workflow : plan, spec, prompt, summarize, commit, review
Agent    : agent, debug, research
Utility  : stats, log, deps, dev, testmodels, session, menu, update
```

### Major Redesign (>1 hari)

1. **Terapkan sistem box/state/color ke seluruh command workflow** (`aiplan`, `aispec`, `aiprompt`, `aisummarize`, `aicommit`, `aireview`) — bukan cuma agent. Ini pekerjaan paling berdampak untuk menaikkan skor Visual Consistency dari 4/10 ke setara jalur agent (~8/10). *(File: seluruh `40-workflow/*.zsh`, memanfaatkan `60-ui/05-ui_box.zsh` yang sudah ada)*
2. **Bangun ulang verbosity system agar benar-benar berfungsi**, dengan `_ai_verbose` dipanggil dari titik-titik output di command utama, atau — kalau prioritasnya rendah — hapus fitur dari dokumentasi & UI sampai siap. *(File: `60-ui/components/verbosity.zsh` + seluruh command)*
3. **Satu sumber kebenaran untuk daftar command + deskripsi** (dipakai bareng oleh `ai h`, tab-complete `_ai_complete`, dan Command Palette `screens/palette.zsh`) — saat ini ketiganya independen dan berpotensi drift (`_AI_SUBCOMMANDS` untuk dispatcher, array hardcoded terpisah untuk palette, tanpa deskripsi untuk tab-complete). *(File: baru, mis. `60-ui/00-command_registry.zsh`)* — **item ini dipromosikan ke prioritas Tinggi (§16 #5)** karena satu perubahan menyelesaikan tiga masalah sekaligus (help tidak terkategori §9, tab-complete tanpa deskripsi §9, drift 3-arah), jauh lebih tinggi ROI-nya dibanding dikerjakan sebagai tiga perbaikan terpisah.

---

## 16. Top 23 Prioritas Perbaikan

**Catatan revisi**: dibanding draf sebelumnya, *Unified Command Registry* dinaikkan dari #13 ke #5 — satu perubahan ini sekaligus memperbaiki help (#8 lama), tab-complete description (#14 lama), dan mengurangi risiko drift 3-arah, jadi ROI-nya lebih tinggi daripada dikerjakan terpisah-pisah. *Rendering Layer Contract* (§13b) ditambahkan sebagai item baru prioritas Tinggi — mengerjakannya duluan membuat migrasi visual di #6 dan perbaikan `NO_COLOR` di #12 lebih murah karena tidak perlu menebak pola sendiri-sendiri. Sebaliknya, *emoji installer* diturunkan dari #12 ke #21 — dampaknya nyata tapi kecil (kesan pertama satu kali saat instalasi, bukan dipakai berulang), kalah prioritas dari perbaikan yang dirasakan tiap hari. Daftar diperluas dari Top 20 menjadi Top 22 untuk menampung temuan baru (rendering contract, interrupt coverage) tanpa membuang item lama.

| # | Prioritas | Item | File | Dampak |
|---|---|---|---|---|
| 1 | 🔴 Kritis | Selesaikan konflik `ui_palette()` ganda | `60-ui/components/palette.zsh`, `screens/palette.zsh` | Mencegah Command Palette rusak diam-diam |
| 2 | 🔴 Kritis | Samakan kontrak review `aifix` dengan `aipatch` | `30-code/45-fix.zsh` | Mencegah kehilangan/kerusakan kode tanpa review |
| 3 | 🟠 Tinggi | Definisikan **kontrak rendering minimal** (warna lewat `AI_C_*`, unicode+fallback, tunduk `NO_COLOR`) wajib untuk command baru | Baru — checklist/wrapper, lihat §13b | Akar penyebab visual inconsistency, kebocoran `NO_COLOR`, drift bahasa — kerjakan sebelum migrasi #7 supaya tidak perlu diulang |
| 4 | 🟠 Tinggi | Satukan implementasi verbosity: sambungkan 6 file yang punya inline check `${AI_VERBOSITY:-0}` ke satu getter bersama, hapus/pakai `_ai_verbose` resmi | `60-ui/components/verbosity.zsh` + 6 file lain (§6.2) | Kurangi technical debt duplikasi *(diturunkan dari Kritis — lihat koreksi §6.2, fiturnya berfungsi jadi bukan lagi masalah integritas dokumentasi)* |
| 5 | 🟠 Tinggi | `airun`: minta confirm sebelum auto-apply fix, atau beri opsi `--auto` eksplisit | `30-code/50-run.zsh` | Konsistensi jaminan "review wajib" |
| 6 | 🟠 Tinggi | **Unified Command Registry** — satu sumber kebenaran command+deskripsi dipakai bareng `ai h`, tab-complete, dan Command Palette | Baru, mis. `60-ui/00-command_registry.zsh` | ROI tertinggi: sekaligus benahi help, completion (termasuk command confidence, §9), palette, discoverability, dan cegah drift 3-arah — lihat §15 |
| 7 | 🟠 Tinggi | Terapkan box/warna terpusat ke `aiplan`/`aispec`/`aiprompt`/`aisummarize`, ikuti kontrak #3 | `40-workflow/*.zsh` | Konsistensi visual lintas coding/review loop (§6b), kesan produk lebih matang |
| 8 | 🟠 Tinggi | Ekstrak fungsi confirm bersama (`_ai_confirm`) | `06-permissions/` (baru) | Kurangi drift, standarkan timeout — juga naikkan skor Interaction Consistency §13c |
| 9 | 🟠 Tinggi | Dokumentasikan `ai index`, `aiscrap`, `ai testmodels` | `CARA-PAKAI.md` | Discoverability (sebagian otomatis terselesaikan begitu registry #6 jadi) |
| 10 | 🟡 Sedang | Tambahkan status line periodik untuk command single-shot di level `AI_VERBOSITY=0` (bukan cuma level 1+) | `40-workflow/*.zsh`, `30-code/05-code.zsh` | Tutup Silent Gap di perceived latency (§8c) untuk command paling sering dipakai |
| 11 | 🟡 Sedang | Audit manual titik non-request (`aipatch`/`aicommit`/`aiundo`, antara backup-apply/diff-commit) untuk cakupan interrupt | `35-files/*.zsh`, `40-workflow/00-aicommit.zsh` | Pastikan `Ctrl+C` aman di seluruh command, bukan cuma jalur `_ai_chat_request`/agent (§8d) |
| 12 | 🟡 Sedang | Tambahkan next-step hint ke `aiplan` | `40-workflow/05-aiplan.zsh` | Information hierarchy |
| 13 | 🟡 Sedang | Tambahkan tabel/decision-tree `aiplan` vs `aispec` vs `aiprompt` vs `aibuild` ke `ai h` & dokumentasi | `CARA-PAKAI.md`, `60-ui/10-help_stats.zsh` | Mengurangi kebingungan mental model (§11b) |
| 14 | 🟡 Sedang | Rename/bedakan `aih` vs `ai h` lebih jelas | `60-ui/10-help_stats.zsh` | Kejelasan naming |
| 15 | 🟡 Sedang | Alirkan warna diff `aipatch`/`aicode` lewat `AI_C_*` | `35-files/10-aipatch.zsh`, `30-code/05-code.zsh` | Cegah drift tema warna DAN tutup kebocoran `NO_COLOR` (§12) sekaligus |
| 16 | 🟡 Sedang | Tuliskan aturan bahasa Indonesia vs Inggris per kategori pesan (error/approval/help) | `CARA-PAKAI.md`/`CONTRIBUTING.md` | Cegah drift bahasa lebih lanjut (§4b) |
| 17 | 🟡 Sedang | Empty-state bercabang first-time vs returning (resume/session/project terakhir untuk returning, contoh prompt untuk first-time) | `60-ui/screens/home.zsh` | Kurangi friksi resume loop (§6b, §9d, §10b) tanpa membalik filosofi AI-first |
| 18 | 🟢 Rendah | Hapus atau dokumentasikan status dead code: `ui_card_*`, `ui_approve`, `components/palette.zsh` generik | `60-ui/components/cards.zsh`, `approval.zsh` | Kejelasan maintainer, tidak langsung ke user |
| 19 | 🟢 Rendah | Standarkan frasa "file tidak ditemukan" di semua command | Berbagai | Konsistensi pesan |
| 20 | 🟢 Rendah | Verifikasi manual TTFF/Silent Gap nyata (bukan cuma struktur kode) untuk `aiplan`/`aispec`/`aicode` | 🟡 Perlu Verifikasi | Perceived latency (§8c) |
| 21 | 🟢 Rendah | Verifikasi manual: kontras warna approval (kuning) vs blocked (merah) untuk buta-warna | 🟡 Perlu Verifikasi | Aksesibilitas |
| 22 | 🟢 Rendah | Ganti emoji instalasi jadi selaras ikonografi `60-ui/` (atau dokumentasikan sebagai pengecualian sadar) | `install.sh` | Konsistensi kesan pertama — dampak nyata tapi kecil, one-time saat instalasi (diturunkan lebih jauh dari draf sebelumnya) |
| 23 | 🟢 Rendah | Ukur latency `_ai_workspace` (re-source banyak file tiap panggil `ai` tanpa argumen) | 🟡 Perlu Verifikasi | Responsiveness |

---

## 17. UX Validation Matrix

Sepanjang audit ini, setiap klaim yang tidak bisa dipastikan lewat pembacaan kode saja (perilaku visual nyata, timing, rendering di terminal spesifik) ditandai 🟡 Perlu Verifikasi dan tersebar di banyak bab. Tabel ini mengkonsolidasikan seluruhnya jadi satu daftar kerja — siap dipakai sebagai checklist pengujian manual sebelum item terkait ditutup di sprint manapun.

| # | Skenario | Expected UX | Status | Sumber (§) | Prioritas verifikasi |
|---|---|---|---|---|---|
| 1 | `Ctrl+C` saat `aipatch` di antara backup dan apply | Rollback jelas, tidak ada state setengah jalan | 🟡 Perlu Verifikasi | §8d | 🟠 Tinggi — titik rawan data |
| 2 | `Ctrl+C` saat `aicommit`/`aiundo` di langkah non-request | Berhenti bersih dengan pesan status jujur | 🟡 Perlu Verifikasi | §8d | 🟠 Tinggi |
| 3 | Checkpoint `aiagent` setelah interrupt di tengah tool call, lalu `--resume` | Lanjut bersih, tidak ada file setengah tertulis | 🟡 Perlu Verifikasi | §8d | 🟠 Tinggi |
| 4 | Terminal 80 kolom | Layout box/wrap utuh, tidak terpotong | 🟡 Perlu Verifikasi (prediksi struktural: aman) | §12b | 🟢 Rendah |
| 5 | Terminal 60 kolom | Box tetap wrap rapi; command `echo`-polos berisiko baris panjang terpotong | 🟡 Perlu Verifikasi | §12b | 🟡 Sedang |
| 6 | Terminal 40 kolom (tmux split sempit / HP portrait) | Box tetap terbaca (desain eksplisit untuk ini); diff ANSI hardcode berisiko pecah warna antar-baris | 🟡 Perlu Verifikasi | §12b | 🟠 Tinggi — ini kondisi yang secara eksplisit disebut jadi target desain, layak dibuktikan benar-benar berfungsi |
| 7 | Terminal 20 kolom (clamp minimum `_ai_ui_width`) | Tidak crash; konten masih berguna dibaca, bukan cuma "tidak error" | 🟡 Perlu Verifikasi | §12b | 🟢 Rendah — kasus ekstrem |
| 8 | `NO_COLOR=1` pada approval box | Tetap terbaca, warna mati bersih | 🟡 Perlu Verifikasi (prediksi struktural: aman, lewat `AI_C_*`) | §12 | 🟢 Rendah |
| 9 | `NO_COLOR=1` pada diff `aipatch`/`aicode -o` | Tetap terbaca | 🟡 Perlu Verifikasi (prediksi struktural: **gagal** — ANSI hardcode bocor) | §12 | 🟠 Tinggi — prediksi kode sudah cukup pasti, verifikasi ini tinggal konfirmasi visual |
| 10 | `gum` tidak terpasang (fallback `read -t`) | Fallback tetap berfungsi dan jelas instruksinya | 🟡 Perlu Verifikasi | §6.4 (confirm timeout) | 🟡 Sedang |
| 11 | Spinner/status line saat `aiplan`/`aispec`/`aiprompt` berjalan | Ada indikasi visual proses berjalan, bukan diam total | 🟡 Perlu Verifikasi | §8c | 🟠 Tinggi — mempengaruhi command paling sering dipakai |
| 12 | TTFF & Silent Gap aktual (diukur dengan `time`) untuk `aiplan`/`aispec`/`aicode` | TTFF instan; Silent Gap idealnya < 5-10 detik tanpa update | 🟡 Perlu Verifikasi | §8c | 🟡 Sedang |
| 13 | Warna approval (kuning) vs blocked (merah) di simulator buta-warna deuteranopia | Tetap bisa dibedakan tanpa mengandalkan warna semata | 🟡 Perlu Verifikasi | §12 | 🟡 Sedang |
| 14 | Emoji (`❌` di `aicl`, ikon `install.sh`) di terminal Termux lama/minim font | Tidak tampil sebagai kotak/`?` | 🟡 Perlu Verifikasi | §12 | 🟢 Rendah |
| 15 | Latency `_ai_workspace` saat `ai` dipanggil tanpa argumen (re-source banyak file) | Tidak terasa lag pada penggunaan normal | 🟡 Perlu Verifikasi | §14 | 🟢 Rendah |
| 16 | `ask_once_per_file` — penghematan approval nyata pada task refactor lintas fungsi | Approval berulang benar-benar hilang setelah file pertama disentuh | 🟡 Perlu Verifikasi | §10b | 🟢 Rendah |

**Cara pakai**: 16 item ini adalah satu-satunya kesenjangan antara audit ini dan pengujian langsung di terminal nyata. Prioritas verifikasi mengikuti dampak (Tinggi = berpotensi mengubah severity temuan terkait di §14 kalau ternyata gagal; Rendah = sudah cukup yakin dari struktur kode, verifikasi tinggal formalitas konfirmasi). Item Tinggi (#1, #2, #3, #6, #9, #11) sebaiknya diverifikasi sebelum Sprint 1-2 dimulai karena bisa menaikkan severity kalau hasilnya buruk; sisanya bisa menyusul paralel dengan development.

---

## 18. Design System Coverage

### 18.1 Coverage per Komponen

Dari ~35 command publik dan ~150 file, seberapa besar bagian yang benar-benar memakai design system terpusat (`_ai_ui_box`, state machine, warna `AI_C_*`, spinner/logger) dibanding yang masih `echo`/`printf` polos atau implementasi ad-hoc sendiri:

| Komponen design system | Dipakai oleh (jumlah file/command) | Total command publik | Coverage |
|---|---|---|---|
| `_ai_ui_box`/`_ai_ui_box_accent` (box dengan border adaptif) | Permission dialogs, agent state header — **~2 area**, bukan per-command individual | ~35 command | 🔴 **~10-15%** — pada praktiknya hanya jalur agent+permission yang merasakan |
| State machine (`Thinking→Acting→Waiting→Done`) | `aiagent` saja (dan turunannya `aidebug`) — **3 file** disebut draf sebelumnya | ~35 command | 🔴 **~6-9%** (2 dari ~35 command) |
| Warna terpusat `AI_C_*` | Agent, permission, logger — tapi diff colorizer (`aipatch`, `aicode -o`) hardcode sendiri (§4, §12) | ~35 command | 🟡 **~40-50%** (lebih luas dari box/state karena logger dipakai banyak command via `_ai_chat_request`, tapi masih bocor di 2 command penting) |
| Progress/status feedback terstruktur (bukan `echo` sekali cetak) | State machine `aiagent`, chunk counter `aisummarize` — **~2 command** dengan feedback berkelanjutan sungguhan | ~35 command | 🔴 **~6%** — mayoritas command "single echo lalu diam" (§8c Silent Gap) |
| `NO_COLOR` awareness | Sama seperti `AI_C_*` — ikut coverage-nya (§12) | ~35 command | 🟡 **~40-50%**, sama seperti warna |
| Unicode+ASCII fallback | Sama seperti box — hanya jalur yang pakai `_ai_ui_box`/`00-ui_text.zsh` | ~35 command | 🔴 **~10-15%** |

**Skor Design System Coverage keseluruhan: ~20/100** (rata-rata tertimbang lintas komponen, dibobot ke arah komponen yang paling terasa user seperti box dan progress feedback, bukan cuma warna).

**Kesimpulan**: design system yang ada berkualitas tinggi — adaptif lebar (§12b), fallback unicode matang, warna semantik konsisten, `NO_COLOR` aware secara desain (§12) — tapi baru mencakup sebagian kecil (~1/5) permukaan produk yang sebenarnya. Ini angka tunggal yang merangkum seluruh temuan §4, §12, §13b (Rendering Layer Fragmentation): bukan design system-nya yang kurang bagus, tapi jangkauannya yang sempit dibanding jumlah command yang ada.

### 18.2 Command Lifecycle (Frekuensi × Durasi)

Prioritas redesign idealnya mengikuti seberapa sering dan seberapa lama command dipakai, bukan cuma jumlah temuan per command. Frekuensi diestimasi dari peran command dalam loop harian (§6b) — **bukan data telemetri nyata**, jadi seluruh kolom Frekuensi ditandai 🟡 estimasi struktural, sama seperti Durasi yang memakai jumlah langkah (§6 tabel) sebagai proksi, bukan pengukuran waktu nyata.

| Command | Frekuensi (estimasi, dari peran di §6b) | Durasi (proksi: jumlah langkah/kompleksitas, §6) | Coverage design system (§18.1) | Prioritas rollout |
|---|---|---|---|---|
| `aiplan` | 🟡 Tinggi — entry point umum coding loop | Sedang (4 langkah, tapi Silent Gap panjang saat request, §8c) | 🔴 Tidak ada (echo polos) | 🔴 **Tertinggi** — sering dipakai, coverage nol |
| `aicommit` | 🟡 Tinggi — akhir hampir setiap coding loop | Pendek (3 langkah, sudah matang, §6 tabel) | 🔴 Tidak ada (echo polos) | 🟠 Tinggi — sering dipakai, tapi frictionnya sudah rendah secara struktur (cuma soal visual) |
| `aicode`/`aifix` | 🟡 Tinggi — inti debug loop (§6b) | Sedang-panjang (tergantung ukuran fix) | 🔴 Tidak ada (echo polos), plus `aifix` friction tinggi (§6) | 🔴 **Tertinggi** — frekuensi tinggi + friction struktural + coverage nol |
| `aispec`/`aiprompt` | 🟡 Sedang — dipakai saat mulai fitur baru, tidak sesering `aiplan`/`aicommit` | Sedang (5 langkah) | 🔴 Tidak ada (echo polos) | 🟠 Tinggi |
| `aiagent` | 🟡 Sedang — dipakai untuk task besar, bukan setiap iterasi kecil | Panjang (7-9 langkah, task berjalan lama) | 🟢 Penuh (state machine, box, progress) | 🟢 Rendah — sudah paling matang di repo |
| `aipatch` | 🟡 Sedang — dipakai untuk perubahan file bertarget presisi | Sedang (8 langkah, tapi sudah matang, §6) | 🟡 Sebagian (diff berwarna tapi hardcode ANSI, §12) | 🟡 Sedang — friction rendah, tapi coverage `NO_COLOR` bocor (§12) layak diperbaiki karena tetap sering dipakai |
| `aireview` | 🟡 Sedang — bagian dari review loop (§6b) | Pendek (2 langkah) tapi output panjang tanpa struktur | 🔴 Tidak ada (echo polos) | 🟡 Sedang |
| Command Palette (`/`) | 🟡 Tinggi — jalur navigasi utama AI-first (§9) | Pendek (4 langkah) | 🟡 Sebagian (`gum filter`, tapi rentan konflik `ui_palette`, §13b) | 🔴 **Tertinggi** — dipakai sangat sering, risiko struktural (Critical di §14) |
| `airun` | 🟡 Rendah-Sedang — dipakai spesifik untuk eksekusi+auto-fix, bukan tiap iterasi | Variabel (1-2 percobaan) | 🔴 Tidak ada | 🟠 Tinggi — friction keamanan (§14) lebih penting daripada frekuensinya |
| `aiundo`/`aibakclean` | 🟡 Rendah — dipakai cuma saat butuh rollback | Pendek | 🔴 Tidak ada | 🟢 Rendah — frekuensi rendah, dampak kalau salah kecil (operasi file lokal) |

**Bacaan**: menyilangkan frekuensi × coverage mengonfirmasi §16 dari sudut berbeda — command dengan frekuensi tinggi *dan* coverage nol (`aiplan`, `aicode`/`aifix`, Command Palette) adalah kandidat migrasi/perbaikan paling bernilai, bukan sekadar "command yang paling banyak temuan". `aiagent` sengaja diabaikan dari daftar prioritas migrasi karena sudah 🟢 penuh — bukan berarti tidak penting, tapi karena sudah selesai duluan.

🔵 **Rekomendasi**: pakai kolom "Prioritas rollout" di atas untuk mengurutkan Sprint 3 (§15) — migrasikan `aiplan`, `aicode`/`aifix` dulu (frekuensi tertinggi + coverage nol), baru `aispec`/`aiprompt`/`aireview`, dan `aipatch`/`aicommit` terakhir karena friction strukturalnya sudah rendah meski masih echo-polos secara visual.

---

## Catatan Metodologis

- Audit ini adalah **static code review**, bukan pengujian interaktif nyata di terminal Termux/Linux/macOS. Semua klaim tentang bagaimana sesuatu *terlihat* di layar (warna, animasi spinner, lebar box aktual) diturunkan dari logika kode, ditandai 🟡 jika belum diverifikasi secara visual langsung.
- Fokus sesuai instruksi brief: **UI/UX & workflow**, bukan audit keamanan/arsitektur — kecuali di titik-titik yang berdampak langsung ke pengalaman pengguna (mis. `local path` shadowing yang disebutkan di komentar `25-perm_write.zsh` disinggung sekilas karena efeknya adalah approval box tidak pernah menampilkan path yang benar — tapi tidak dibahas dari sisi keamanannya).
- Beberapa file kecil (`00-core/`, `10-plugins/zinit.zsh`, seluruh `skills/*.md`) tidak dibahas detail karena tidak menghasilkan output UI langsung ke user — cakupan tetap 100% command publik seperti diminta.
