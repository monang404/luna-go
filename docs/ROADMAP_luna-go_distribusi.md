# Roadmap Distribusi — `luna-go` sebagai Alternatif Claude Code CLI / Gemini CLI / OpenCode CLI

Dokumen sebelumnya (`RENCANA_luna-go_90.md`, SESSION-76–80) menutup temuan audit teknis dan membawa skor **kesehatan kode** ke 90/100. Dokumen ini berbeda cakupan: yang saya nilai sekarang adalah **kelayakan aplikasi ini didistribusikan ke orang lain** sebagai produk yang bersaing head-to-head dengan Claude Code CLI, Gemini CLI, dan OpenCode. Itu soal yang jauh lebih luas daripada "kode-nya benar" — mencakup packaging, onboarding orang asing yang tidak tahu konteks project Anda, kepercayaan (trust/security posture yang bisa diverifikasi orang lain), dan kesetaraan fitur dengan pesaing.

Saya cek langsung ke repo: **saat ini luna-go tidak punya README, tidak punya LICENSE, tidak punya `--version`, tidak punya CI, tidak punya mekanisme rilis/binary distribution sama sekali** selain `make build` lokal. Untuk audiens internal (Anda sendiri di Termux/dev machine) itu tidak masalah. Untuk didistribusikan ke orang lain, itu blocker keras — orang asing tidak akan bisa (atau tidak akan berani) `curl | install` sesuatu tanpa lisensi jelas dan tanpa cara verifikasi apa yang mereka jalankan.

Saya bagi jadi 4 pilar, tiap pilar berisi sesi lanjutan (SESSION-81 dst., melanjutkan nomor dari rencana sebelumnya):

- **Pilar A — Fondasi teknis** (RENCANA_luna-go_90.md, SESSION-76–80 — prasyarat mutlak, tidak diulang di sini)
- **Pilar B — Legal, trust, dan packaging** (SESSION-81–83)
- **Pilar C — Kesetaraan fitur dengan Claude Code/Gemini CLI/OpenCode** (SESSION-84–87)
- **Pilar D — Onboarding, dokumentasi, dan operasional pasca-rilis** (SESSION-88–90)

---

## Kenapa perbandingan ke 3 pesaing itu relevan (baseline yang saya pakai)

Ketiga CLI yang Anda sebut punya beberapa kesamaan struktural yang jadi ekspektasi baseline pengguna teknis saat ini:

| Kapabilitas | Claude Code CLI | Gemini CLI | OpenCode | **luna-go saat ini** |
|---|---|---|---|---|
| Instalasi 1-baris (`curl`/`npm`/`brew`) | Ya | Ya | Ya | **Tidak ada** — cuma `go build` manual |
| `--version` / self-update check | Ya | Ya | Ya | **Tidak ada sama sekali** |
| MCP (Model Context Protocol) client | Ya | Ya | Ya | **Tidak ada** |
| Multi-provider LLM | Terbatas (Anthropic-first) | Terbatas (Google-first) | **Ya, ini nilai jual utamanya** | **Ya** (5 provider) |
| Permission/approval system granular | Ya | Ya | Ya | **Ya, dan menurut audit saya termasuk yang paling matang** |
| Subagent/delegation | Ya | Terbatas | Ya | **Ya** |
| Hook system (pre/post tool use) | Ya | Terbatas | Ya | **Ya** |
| Session resume/rewind | Ya | Terbatas | Ya | **Ya** |
| Cross-platform binary release | Ya | Ya | Ya | **Tidak ada** (hanya build lokal + termux arm64) |
| Lisensi open-source jelas | Ya | Ya | Ya | **Tidak ada file LICENSE sama sekali** |
| Dokumentasi pengguna (bukan komentar kode) | Ya | Ya | Ya | **Tidak ada README** |

Kesimpulan strategis: **arsitektur inti Anda sudah setara atau lebih ketat** dari sisi permission/security dibanding tiga pesaing itu (ini nilai jual nyata — bisa jadi *positioning* utama: "agent CLI dengan permission model paling defensif"). Yang hilang murni di lapisan **distribusi dan kelengkapan ekosistem**, bukan di kualitas mesin agent-nya. Itu kabar baik — pekerjaan yang tersisa lebih banyak "packaging & polish" daripada "menulis ulang".

---

## PILAR B — Legal, Trust, dan Packaging (SESSION-81–83)

Tanpa pilar ini, tidak etis dan secara praktis tidak mungkin mendistribusikan ke orang asing — ini prasyarat sebelum SESSION-84 (fitur) dikerjakan, karena percuma menambah fitur kalau tidak ada yang bisa memasang aplikasinya secara sah.

### SESSION-81 — Legal & keamanan dasar untuk rilis publik

**81.1 — Pilih dan tambahkan LICENSE.**
Repo ini sekarang **tanpa lisensi apa pun** — secara default itu berarti *all rights reserved*, orang lain secara hukum tidak boleh menyalin/memodifikasi/mendistribusikan ulang meski repo publik. Untuk positioning "alternatif open" ke Claude Code/Gemini CLI (yang closed-source) atau setara OpenCode (open-source, MIT), sarankan **MIT** atau **Apache-2.0** — Apache-2.0 kalau Anda ingin patent grant eksplisit (relevan karena ini tooling AI agent yang mungkin menyentuh area sensitif paten di masa depan).
```
LICENSE          # teks lisensi lengkap
```
Tambahkan juga header SPDX singkat di file-file inti (opsional, tapi standar untuk project yang serius soal compliance):
```go
// SPDX-License-Identifier: MIT
```

**81.2 — Kebijakan keamanan & pelaporan kerentanan.**
Buat `SECURITY.md` — terutama penting untuk tool ini karena dia **mengeksekusi command shell dan menulis file atas nama user**. Minimal isi: cara melaporkan kerentanan secara privat (bukan lewat issue publik), cakupan yang dianggap kerentanan (mis. path traversal, privilege escalation lewat permission bypass — dua hal yang justru jadi kekuatan desain Anda, jadi ini kesempatan menunjukkannya), dan versi mana yang masih dapat security patch.

**81.3 — Kebijakan privasi/telemetri, dan nyatakan eksplisit "tidak ada telemetri" (atau: opt-in).**
Cek kode Anda: saya tidak menemukan pengiriman data ke server pihak ketiga selain provider LLM yang dipilih user sendiri — itu bagus, tapi **harus dinyatakan eksplisit** di dokumentasi. Pengguna CLI teknis (target audiens realistis untuk tool seperti ini) akan mengecek ini sebelum instal apa pun yang punya akses shell/filesystem. Tambahkan bagian "Privacy" di README (lihat SESSION-88) yang menyatakan: data apa yang dikirim ke mana, provider mana yang menyimpan log di sisi mereka (di luar kendali Anda — perlu link ke privacy policy masing-masing provider), dan bahwa tidak ada data yang dikirim ke server Anda sendiri.

**Acceptance criteria:** `LICENSE`, `SECURITY.md` ada di root repo; README (draft awal, detail penuh di SESSION-88) menyatakan sikap privasi secara eksplisit.

### SESSION-82 — Versioning, `--version`, dan mekanisme update

**82.1 — Wire `version` yang sudah ada di `main.go` tapi tidak pernah dipakai.**
Saat ini `var version = "dev"` di `cmd/luna/main.go` di-set lewat ldflags tapi **tidak pernah diteruskan ke `commands.Execute()`** — jadi `luna --version` tidak ada sama sekali. Perbaikan:
```go
// cmd/luna/main.go
func main() {
	commands.Execute(version)
}

// cmd/luna/commands/root.go
func Execute(version string) {
	loadSecretsAtStartup()
	app := NewApp()
	root := NewRootCmd(app)
	root.Version = version          // cobra otomatis menyediakan flag --version
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```
Ini satu perubahan kecil tapi wajib — package manager (Homebrew, Scoop, AUR) dan bug report dari pengguna asing **selalu** butuh `--version` untuk triage.

**82.2 — Semantic versioning + CHANGELOG.**
Mulai tag rilis dengan SemVer (`v0.1.0` untuk rilis publik pertama, bukan `v1.0.0` — jujur ke pengguna bahwa ini masih awal). Tambahkan `CHANGELOG.md` mengikuti format Keep a Changelog. Ini juga otomatis mengisi `VERSION` di `Makefile` Anda yang sudah benar memakai `git describe --tags`.

**82.3 — Cek update sederhana (bukan auto-update, cukup notifikasi).**
Baik Claude Code maupun Gemini CLI menampilkan notifikasi "versi baru tersedia" saat start. Untuk luna-go, tambahkan pengecekan ringan dan **non-blocking**: sekali per hari (cache di `~/.luna/.update_check`), fetch GitHub Releases API (`https://api.github.com/repos/<org>/luna-go/releases/latest`) dengan timeout pendek, kalau versi lebih baru cetak satu baris ke stderr ("versi baru vX.Y.Z tersedia — lihat <url>") tanpa memblokir apa pun. **Wajib**: fail silent kalau offline/API gagal, dan wajib bisa dimatikan lewat `LUNA_NO_UPDATE_CHECK=1` (privasi — beberapa pengguna tidak mau CLI mereka menghubungi internet tanpa diminta sama sekali).

**Acceptance criteria:** `luna --version` mencetak versi git-describe yang benar; update-check tidak pernah menambah latensi terasa ke startup normal (jalankan async, jangan block `PersistentPreRunE`).

### SESSION-83 — Cross-platform build & distribusi binary

**83.1 — Perluas `Makefile`/tambahkan `goreleaser` untuk build matrix penuh.**
Makefile sekarang hanya build host + linux/arm64 (Termux). Untuk distribusi publik minimal butuh: `linux/amd64`, `linux/arm64`, `darwin/amd64`, `darwin/arm64` (Apple Silicon — signifikan, banyak developer target pakai Mac M-series), `windows/amd64`. Sarankan pakai `goreleaser` (`.goreleaser.yaml`) daripada menulis tangan setiap kombinasi — otomatis handle checksums, archive naming konsisten, dan integrasi GitHub Releases:
```yaml
# .goreleaser.yaml (ringkas)
builds:
  - main: ./cmd/luna
    env: [CGO_ENABLED=0]
    goos: [linux, darwin, windows]
    goarch: [amd64, arm64]
    ldflags: ["-s -w -X main.version={{.Version}}"]
checksum:
  name_template: "checksums.txt"
```

**83.2 — Checksum & (idealnya) signing.**
Publikasikan `checksums.txt` (SHA-256) untuk setiap rilis — `goreleaser` melakukan ini otomatis. Untuk kepercayaan lebih tinggi (dan ini yang membedakan tool "serius" dari "script random di GitHub" di mata pengguna teknis yang skeptis, wajar mengingat tool ini eksekusi shell command), pertimbangkan **cosign** (Sigstore) untuk sign binary — makin umum jadi ekspektasi untuk tooling developer.

**83.3 — Install script satu baris.**
```bash
curl -fsSL https://raw.githubusercontent.com/<org>/luna-go/main/install.sh | sh
```
Script ini mendeteksi OS/arch, download binary yang sesuai dari GitHub Releases, verifikasi checksum, taruh di `~/.local/bin` (atau `$LUNA_INSTALL_DIR`). **Tampilkan isi script ini di README sebelum link curl-nya** — norma di komunitas developer adalah orang harus bisa baca script itu dulu sebelum menjalankannya (`curl ... | less` dulu, baru `| sh`), jangan buat itu sulit dilakukan.

**83.4 — Package manager kedua: Homebrew tap (macOS/Linux), Scoop (Windows).**
Setelah 83.1–83.3 stabil di 1-2 rilis, tambahkan `homebrew-tap` repo terpisah dengan formula sederhana yang menunjuk ke binary rilis GitHub — ini menurunkan friksi instalasi drastis untuk audiens macOS yang jadi mayoritas pengguna developer-tool CLI semacam ini.

**Acceptance criteria:** satu tag git (`git tag v0.1.0 && git push --tags`) menghasilkan rilis GitHub lengkap dengan 6 binary (3 OS × 2 arch) + checksums, otomatis lewat CI (lihat SESSION-90.2), tanpa langkah manual.

---

## PILAR C — Kesetaraan Fitur dengan Claude Code / Gemini CLI / OpenCode (SESSION-84–87)

Ini pilar terbesar. Saya urutkan dari yang paling berdampak ke *user-perceived competitiveness* dulu.

### SESSION-84 — MCP (Model Context Protocol) client

**Ini gap fitur paling besar.** Ketiga pesaing yang Anda sebut mendukung MCP — ekosistem *tools* pihak ketiga (filesystem lain, database, API eksternal, browser automation, dst.) yang makin jadi standar de-facto sejak dipopulerkan Claude Code. Tanpa MCP, luna-go terkunci hanya pada 21 tool internal yang sudah ada (bagus, tapi tertutup) — pengguna yang sudah punya MCP server terpasang (banyak orang di ekosistem ini sudah punya) tidak akan pindah ke CLI yang tidak bisa memakainya.

**Cakupan minimal yang realistis** (bukan implementasi MCP penuh sekaligus — pecah jadi sub-tahap):
1. **MCP client stdio transport** (paling umum dipakai server-server MCP komunitas): spawn proses server MCP sebagai child process, komunikasi lewat JSON-RPC 2.0 di atas stdio. Ini paling mirip pola yang sudah Anda punya di `internal/tools/process.go` (spawn + pipe management) — reuse desain `bgProcess`/pipe-handling yang sudah diperbaiki di SESSION-78.
2. **Tool discovery**: saat startup (atau lazy, saat pertama dipanggil), kirim `tools/list` MCP request ke setiap server yang dikonfigurasi, daftar tool yang dikembalikan otomatis masuk ke `Dispatcher` sebagai registrasi dinamis — desain `Dispatcher.Register` yang sudah ada mendukung ini tanpa perubahan struktural besar, karena Register menerima `Entry`+`Tool` sebagai data, bukan hardcoded.
3. **Config**: `.luna/mcp.json` (format mirip `.claude/mcp.json` Claude Code atau `~/.config/gemini/settings.json`-nya Gemini CLI — makin dekat ke format yang sudah familiar pengguna, makin rendah friksi migrasi):
```json
{
  "mcpServers": {
    "filesystem-extra": { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"] }
  }
}
```
4. **Permission**: setiap MCP tool yang di-discover **wajib** lewat `permission.CheckPermission` yang sama seperti tool internal — jangan buat jalur pintas. Default level untuk tool MCP yang tidak dikenal: `LevelProcess` (selalu tanya), bukan auto-trust, karena Anda tidak mengontrol kode server MCP pihak ketiga.

**Ini pekerjaan besar** (realistis 2-3 sesi tersendiri, bukan satu baris) — saya sengaja tidak menuliskan kode lengkap di sini karena butuh keputusan desain (SSE transport untuk MCP remote? sekarang atau nanti?) yang sebaiknya jadi sesi perencanaan tersendiri sebelum eksekusi, mengikuti pola kerja Anda yang biasa (PRD dulu, baru YAML session breakdown).

**Acceptance criteria minimal untuk rilis v0.1:** stdio MCP client bisa connect ke minimal satu server MCP referensi resmi (`@modelcontextprotocol/server-filesystem`), tool-nya muncul di `Manifest()`, dan permission gate berfungsi.

### SESSION-85 — Konfigurasi project-level yang setara `CLAUDE.md`/`GEMINI.md`

Cek kode Anda: sistem hook (`internal/hooks`) dan settings (`internal/settings`) sudah ada dan cukup matang. Yang perlu dipastikan setara dengan ekspektasi pengguna Claude Code/Gemini CLI:
- **File instruksi project-level** (`LUNA.md` di root project, dibaca otomatis sebagai bagian system prompt) — cek apakah sudah ada; kalau belum, ini fitur dengan ROI sangat tinggi karena murah diimplementasikan (baca file teks, suntik ke system prompt) tapi sangat terasa dampaknya ke UX (setiap project bisa punya "memori" konteks sendiri, persis pola `CLAUDE.md`).
- **Precedence config yang jelas**: env var → `.luna/settings.local.json` (per-user, gitignored) → `.luna/settings.json` (project, di-commit) → `~/.luna/config.yaml` (global). Dokumentasikan urutan ini eksplisit — kebingungan soal "kenapa setting saya tidak kepakai" adalah sumber keluhan #1 di tool sejenis.

### SESSION-86 — Paritas UX terminal: streaming render, interrupt, dan multi-line input

Cek `internal/ui` — sudah ada rendering contract dan design token system (bagus, dari histori Anda ini sudah jadi fokus SESSION-24-46). Yang perlu diverifikasi eksplisit untuk paritas kompetitif:
- **Ctrl+C mid-generation** menghentikan generation tanpa keluar dari REPL (bukan exit total) — cek `internal/llmclient/streaming.go` sudah menangani `ctx.Done()` dengan benar (sudah saya cek di audit sebelumnya, ini bagus), tapi pastikan **REPL layer**-nya juga membedakan "Ctrl+C sekali = stop generation" vs "Ctrl+C dua kali cepat = exit app" — ini pola yang jadi ekspektasi standar sekarang.
- **Multi-line input** (paste blok kode, atau `\` untuk lanjut baris) — cek apakah REPL sudah mendukung ini; kalau belum, prioritaskan, karena workflow "paste error log lalu tanya" sangat umum untuk tool jenis ini.
- **Image/file attachment lewat drag atau path**: baik Claude Code maupun Gemini CLI mendukung menyisipkan gambar (screenshot error, mockup UI) ke prompt. Cek apakah `internal/llmclient` sudah mendukung multimodal content block untuk provider yang bisa (Anthropic, Gemini keduanya support vision) — kalau belum, ini gap fitur nyata untuk use-case "tunjukkan screenshot bug ini".

### SESSION-87 — Sandboxing opsional untuk `run_command`/`exec_process`

Diferensiasi kompetitif yang realistis untuk luna-go, mengingat kekuatan desain permission Anda: pesaing besar (Claude Code) mulai menawarkan mode sandbox (container/seccomp-based) untuk eksekusi command yang lebih terisolasi dari filesystem host penuh. Anda **tidak perlu** membangun sandbox penuh untuk v0.1, tapi pertimbangkan sebagai roadmap item pasca-rilis:
- Opsi `--sandbox` yang menjalankan `exec_process`/`run_command` di dalam container ringan (`bubblewrap` di Linux, cukup mature dan ringan) yang membatasi filesystem write hanya ke project root — lapisan pertahanan **tambahan** di atas permission model yang sudah ada, bukan pengganti.
- Ini eksplisit saya taruh sebagai **item roadmap opsional pasca-v0.1**, bukan blocker rilis — tapi cantumkan di README sebagai "Roadmap" supaya calon pengguna teknis yang skeptis soal keamanan tahu ini sedang dipikirkan.

---

## PILAR D — Onboarding, Dokumentasi, Operasional (SESSION-88–90)

### SESSION-88 — README yang benar-benar menjual dan menjelaskan

Repo ini **tidak punya README sama sekali** — untuk repo yang isinya 29 ribu baris Go dengan arsitektur permission yang sangat matang, ini adalah pemborosan kerja keras Anda yang paling gampang diperbaiki. Struktur minimal yang saya sarankan (urutan penting — orang membaca README top-to-bottom lalu berhenti di kalimat pertama yang membuat mereka ragu):

1. **Satu paragraf positioning** — apa bedanya dari Claude Code/Gemini CLI/OpenCode, dalam satu kalimat tajam. Berdasarkan audit saya, klaim yang **jujur dan bisa dibuktikan**: *"multi-provider agentic CLI dengan permission/path-traversal model paling ketat di kelasnya, tanpa lock-in ke satu vendor LLM."*
2. **GIF/asciinema demo** 15-30 detik — REPL beraksi, tool call, permission prompt. Ini yang paling menentukan orang lanjut baca atau tutup tab.
3. **Instalasi** (satu blok kode, hasil SESSION-83.3).
4. **Quickstart** — set satu API key, `luna`, tanya sesuatu.
5. **Fitur** — daftar singkat dengan sub-bullet, bukan paragraf panjang: multi-provider, subagent, hooks, session resume, permission modes.
6. **Perbandingan tabel** (opsional tapi efektif) — versi ringkas dari tabel di dokumen ini.
7. **Konfigurasi** — link ke dokumen konfigurasi lengkap terpisah (`docs/CONFIGURATION.md`), jangan taruh semua di README.
8. **Privacy** (dari SESSION-81.3), **Security** (link ke `SECURITY.md`), **Contributing**, **License**.

### SESSION-89 — Dokumentasi pengguna terpisah dari dokumentasi sesi migrasi

Anda punya banyak dokumentasi (`docs/execution_sessions/*.yaml`, `RENCANA_MIGRASI_GO_RUST.md`, dst.) — itu **dokumentasi proses pengembangan**, bukan dokumentasi pengguna. Orang asing yang install luna-go tidak peduli soal SESSION-48 atau migrasi zsh-ke-Go; mereka peduli "bagaimana cara pakai `/rewind`" atau "bagaimana cara nulis subagent custom". Pisahkan:
- `docs/user/` — panduan pengguna murni: slash commands, konfigurasi, cara menulis `.luna/agents/*.md`, troubleshooting.
- `docs/dev/` (atau biarkan `docs/execution_sessions/` seperti sekarang) — dokumentasi internal proses migrasi Anda, boleh tetap ada untuk transparansi tapi jangan jadi entry point utama.

Idealnya `docs/user/` cukup lengkap untuk jadi dasar situs dokumentasi statis sederhana (mkdocs/Docusaurus) di kemudian hari, tapi untuk v0.1 markdown biasa di repo sudah cukup.

### SESSION-90 — CI/CD penuh + operasional pasca-rilis

Menyambung `SESSION-80` dari rencana sebelumnya (yang sudah mencakup CI dasar untuk build/test), lengkapi untuk kebutuhan rilis publik:

**90.1 — Issue/PR template.**
`.github/ISSUE_TEMPLATE/bug_report.md`, `feature_request.md`, `.github/PULL_REQUEST_TEMPLATE.md` — hal kecil, tapi tanpa ini repo terlihat "belum siap menerima kontribusi asing" sekalipun secara teknis publik.

**90.2 — CI penuh: test matrix lintas OS + goreleaser otomatis saat tag.**
```yaml
# .github/workflows/ci.yml (tambahan dari SESSION-80)
jobs:
  test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: go build ./...
      - run: go test ./...
  release:
    if: startsWith(github.ref, 'refs/tags/v')
    needs: test
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: goreleaser/goreleaser-action@v6
        with: { args: release --clean }
```
Test matrix lintas OS ini **penting secara khusus** untuk Anda — audit sebelumnya menandai potensi hardcoded Unix path separator di beberapa test; matrix Windows di CI adalah cara paling murah untuk menangkap regresi cross-platform otomatis, bukan manual review.

**90.3 — Analytics adopsi minimal (opsional, hormati privasi).**
Kalau Anda ingin tahu apakah rilis publik ini benar-benar dipakai orang: cukup pantau GitHub star/clone/release-download count bawaan GitHub Insights — **jangan** tambahkan telemetry custom yang mengirim data dari binary pengguna, itu bertentangan langsung dengan positioning privasi di SESSION-81.3 dan akan langsung ketahuan siapa pun yang baca source (yang justru jadi salah satu nilai jual: source terbuka, bisa diverifikasi).

---

## Ringkasan prioritas & urutan eksekusi realistis

```
Pilar A (RENCANA_luna-go_90.md, SESSION-76–80)  ─── wajib duluan, sudah dirinci sebelumnya
        │
        ▼
Pilar B (SESSION-81–83) ─── legal + versioning + binary release
        │                    tanpa ini, TIDAK ADA yang bisa didistribusikan sama sekali
        ▼
Pilar D.88 (README)     ─── satu sesi, dampak persepsi terbesar per waktu yang dihabiskan
        │
        ▼
   ── RILIS v0.1.0 di sini ──   ← titik realistis "layak didistribusikan sebagai
        │                          alternatif yang bisa dicoba orang", BUKAN
        │                          "setara penuh fitur Claude Code/Gemini CLI"
        ▼
Pilar C (SESSION-84–87) ─── kesetaraan fitur, terutama MCP (84) — kerjakan
        │                    pasca-rilis awal, iteratif per minor version
        ▼
Pilar D.89–90 (docs lanjutan, CI matrix, ops)
```

**Jangan tunda rilis v0.1.0 sampai Pilar C (MCP dkk.) selesai.** MCP client adalah pekerjaan besar (realistis 2-3 sesi tersendiri) dan menunda rilis publik demi itu berarti Anda kehilangan feedback loop dari pengguna nyata selama berminggu-minggu tanpa alasan kuat — rilis v0.1.0 setelah Pilar A+B+README selesai, beri label jelas "early release, MCP belum didukung" di README, lalu kerjakan Pilar C sebagai v0.2.0/v0.3.0. Ini juga persis pola kerja Anda yang sudah terbukti (audit-heavy, iteratif, PRD dulu baru eksekusi) — tidak perlu diubah, cuma diarahkan ke target baru: rilis nyata, bukan cuma commit ke repo pribadi.
