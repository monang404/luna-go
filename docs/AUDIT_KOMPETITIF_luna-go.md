# Audit Kompetitif Menyeluruh — `luna-go`
### vs Claude Code CLI, Gemini CLI, OpenCode, Hermes Agent, DeepSeek Harness (dsh)

**Tanggal audit:** 20 Agustus 2026
**Metodologi:** Bukan review dokumen semata. Repo di-*build* ulang dari nol di sandbox bersih (`apt install golang-go` 1.22.2, `go build ./...`, `go vet ./...`, `go test ./...`, lalu **binary hasil build benar-benar dijalankan** — `luna --help`, dsb). Temuan fitur/gap kompetitor diverifikasi lewat riset web per-Agustus 2026 (bukan dari memori model), karena lanskap CLI agentic ini bergerak sangat cepat dan sebagian nama yang Anda sebut baru muncul dalam beberapa minggu terakhir.

---

## 0. Catatan penting sebelum mulai

Repo ini sudah punya `docs/AUDIT_luna-go.md` dari sesi audit sebelumnya. Saya jalankan ulang semua langkah verifikasinya alih-alih mempercayainya begitu saja, dan hasilnya **campuran**: dua temuan kritis lama (§3.1 build gagal, §3.2 endpoint Cerebras salah) **sudah diperbaiki** di HEAD saat ini — tapi saya menemukan **bug baru yang levelnya sama fatalnya**, yang lolos dari audit sebelumnya. Detail di §1.

Audit kali ini punya sudut pandang berbeda dari audit sebelumnya: bukan cuma "apakah kodenya aman dan benar", tapi **"apakah produk ini punya kans bersaing sebagai alternatif Claude Code / Gemini CLI / OpenCode / Hermes / DeepSeek Harness"**. Itu pertanyaan produk, bukan cuma pertanyaan kode — jadi separuh dokumen ini soal *feature parity* dan *distribution*, bukan cuma bug.

---

## 1. TEMUAN BARU — CRITICAL: binary crash di setiap invocation

**File:** `go/cmd/luna/commands/app.go:122-126`

```go
if len(tools.Registry) != 21 {
    panic(fmt.Sprintf("commands: buildDispatcher: tools.Registry has %d entries, only 21 are wired -- add the missing register() call", len(tools.Registry)))
}
```

Ini adalah *self-check* yang dipasang sesi sebelumnya untuk mencegah entri `tools.Registry` baru lupa di-`register()`. Niatnya baik. Tapi angka `21` sudah **basi** — `internal/tools/registry.go` sekarang punya **22** entri, dan `app.go` memang sudah punya 22 baris `register(...)` yang benar (saya hitung manual, cocok satu-satu). Guard-nya sendiri yang salah hitung, bukan wiring-nya.

Akibatnya: `buildDispatcher()` dipanggil dari `NewApp()`, dipanggil dari `Execute()`, dipanggil dari `main()` — **setiap** eksekusi binary `luna`, termasuk `luna --help`, langsung `panic()` sebelum baris output pertama:

```
$ ./luna --help
panic: commands: buildDispatcher: tools.Registry has 22 entries, only 21 are wired -- add the missing register() call
```

Saya verifikasi ini dengan build binary asli (`go build -o luna ./cmd/luna && ./luna --help`) — bukan cuma baca kode. Saya juga verifikasi bahwa mengganti `21` → `22` (satu-satunya baris yang perlu diubah) langsung membuat binary jalan normal dan menampilkan help text lengkap.

**Kenapa ini lebih parah dari temuan §3.1 di audit sebelumnya:** dulu `go build` sendiri gagal (jelas terlihat di CI, tidak mungkin lolos review). Sekarang **`go build ./...` sukses bersih** — tapi binary hasilnya 100% tidak bisa dipakai untuk apa pun. Ini kelas bug yang justru lebih berbahaya untuk rilis publik: kontributor yang cuma menjalankan `go build` di CI (seperti workflow `.github/workflows/ci.yml` yang ada di repo ini) akan melihat centang hijau di step "Build", padahal binary-nya mati total. Hanya step **"Test"** yang akan menangkapnya (`TestRegistryParity` di `root_test.go` — dan benar, `go test ./...` saya jalankan dan ini yang gagal), jadi untungnya CI Anda *akan* merah kalau dijalankan hari ini — tapi hanya karena kebetulan ada test itu, bukan karena desainnya robust.

**Saran perbaikan (dua opsi):**
1. Tambal cepat: ganti `21` jadi `22` di dua tempat pada `app.go`. Ini yang saya verifikasi bekerja, tapi punya masalah yang sama persis akan terulang lagi begitu tool ke-23 ditambahkan.
2. **Perbaikan struktural (disarankan):** hilangkan magic number sama sekali. `register` closure sudah bisa menghitung berapa kali dirinya dipanggil (`registered := 0; register := func(...) { registered++; ... }`), lalu bandingkan `registered != len(tools.Registry)` di akhir — self-check yang sama, tapi tidak pernah basi lagi karena tidak ada angka hardcoded yang perlu diingat manusia untuk diupdate.

**Status build/test riil saat ini (diverifikasi ulang hari ini):**

| Langkah | Hasil |
|---|---|
| `go build ./...` | ✅ PASS |
| `go vet ./...` | ✅ PASS |
| `go test ./...` | ❌ **FAIL** — panic di `TestRegistryParity` (`cmd/luna/commands`), 21/22 package lain PASS |
| `./luna --help` (binary nyata) | ❌ **panic, exit sebelum output apa pun** |

Regresi dari audit sebelumnya yang **sudah fix** (saya verifikasi ulang, bukan cuma percaya klaim lama):
- §3.1 lama (WebSearchTool tidak implement interface `Tool`, `deps` undefined) — **fixed**, `web_search` sekarang `register("web_search", &tools.WebSearchTool{})` dan compile bersih.
- §3.2 lama (endpoint Cerebras `api.cerebras.luna` — domain tidak nyata) — **fixed**, sekarang `https://api.cerebras.ai/v1/chat/completions`.
- §3.3 lama (`ProviderOrder` menyimpang dari test parity zsh) — **fixed**, `go test ./internal/config/...` sekarang PASS.

Saya tidak sempat verifikasi ulang §3.4 (deadlock `io.MultiReader` di `bash_output.go`), §3.5 (leak `bgProcesses`), §3.6 (residual risk `run_command` denylist) secara runtime (butuh skenario proses background nyata) — dari pembacaan kode, ketiganya **masih ada** di HEAD saat ini (saya cek file-nya, pattern-nya belum berubah). Anggap masih valid sampai diverifikasi lebih lanjut.

---

## 2. Kerangka pembanding: siapa sebenarnya kompetitor Anda (per Agustus 2026)

Sebelum masuk ke tabel fitur, penting diluruskan dulu — lanskapnya berubah cepat dan beberapa nama yang Anda sebut bukan produk yang stabil/matang seperti Claude Code:

- **Claude Code CLI** — closed-source, produk Anthropic. Login via langganan Claude Pro/Max (bukan cuma API key), TUI interaktif matang, MCP (stdio + remote), sub-agent, hooks, `CLAUDE.md` project memory, plan mode.
- **Gemini CLI** — **sedang dalam masa pensiun**: Google mengumumkan Gemini CLI akan di-retire 18 Juni 2026, digantikan penerus closed-source (dilaporkan bernama Antigravity). Free tier publiknya juga sudah berakhir Juni 2026. Kalau Anda mem-benchmark terhadap "Gemini CLI" hari ini, pastikan sadar target sebenarnya sedang bergerak.
- **OpenCode** — open-source, **saat ini yang paling populer** di kategori ini (~165rb bintang GitHub), provider-agnostic (30+ provider termasuk model lokal via Ollama), TUI penuh ("Mission Control"), sesi berbasis git, multi-agent orchestration, integrasi Slack/Linear. Ini realistisnya **kompetitor langsung paling relevan** untuk `luna-go` — sama-sama filosofi "jangan lock-in ke satu vendor LLM".
- **Hermes Agent** (Nous Research) — bukan CLI harness mandiri seperti tiga di atas, melainkan lebih ke *model gateway* (Portal) dengan akses 200+ model, dioptimalkan untuk tool-calling/agentic workflow (chain-of-thought trace, structured output sebagai first-class concern). Kalau ini yang Anda maksud sebagai pembanding, perbandingannya lebih ke "kualitas orkestrasi provider" ketimbang "kelengkapan fitur CLI".
- **DeepSeek Harness (`dsh`)** — baru rilis **13 Agustus 2026** (developer preview), open-source (MIT), arsitektur "semua adalah plugin" (model adapter, tool registry, sandbox, bahkan UI — semuanya plugin yang bisa diganti). Viral cepat (~95rb bintang dalam 2 hari), tapi per tanggal audit ini **belum punya TUI terminal interaktif** — entry point utamanya web app lokal, bukan CLI murni, dan komunitasnya sendiri sedang aktif minta fitur TUI/standalone client yang belum ada. Jangan panik menyamakan diri dengan ini — ini masih *developer preview*, API-nya sendiri diakui pihak DeepSeek masih akan berubah.

**Implikasi buat Anda:** target realistis untuk `luna-go` bukan "menyamai Claude Code" (itu produk lab besar dengan tim penuh) — target yang jauh lebih masuk akal adalah **menyamai OpenCode** dari sisi filosofi (multi-provider, no lock-in) sambil membedakan diri lewat **permission/path-guard model** yang menurut audit sebelumnya memang sudah level production-grade. Itu adalah *niche* yang sah: banyak tool kompetitor mengorbankan ketatnya sandbox demi kemudahan (`run_command` di banyak tool lain juga masih shell-arbitrary dengan mitigasi serupa).

---

## 3. Tabel perbandingan fitur

Legenda: ✅ ada & berfungsi (saya verifikasi di kode) · ⚠️ ada tapi terbatas/dead code/tidak lengkap · ❌ tidak ada

| Fitur | luna-go | Claude Code | Gemini CLI | OpenCode | Hermes/DeepSeek Harness |
|---|---|---|---|---|---|
| Multi-provider LLM | ✅ (5 provider: Anthropic, OpenRouter, Groq, Gemini, Cerebras, +DeepSeek di config tapi tak masuk `ProviderOrder` legacy) | ❌ (Anthropic only) | ❌ (Google only) | ✅ (30-75+ provider, termasuk model lokal Ollama) | ✅ (200+ model via gateway / plugin adapter) |
| Login tanpa API key (OAuth/subscription) | ❌ **tidak ada sama sekali** — saya grep seluruh repo, nol implementasi OAuth/browser-login | ✅ (login Pro/Max) | ✅ (login Google) | ✅ (opsional, bisa pakai key sendiri) | bervariasi per provider |
| TUI interaktif (readline/history/live-render) | ❌ **tidak ada** — REPL berbasis `bufio.Scanner` baris-per-baris, tanpa `golang.org/x/term`, tanpa library TUI apa pun di vendor (hanya cobra+pflag) | ✅ matang | ✅ matang | ✅ matang ("Mission Control") | ⚠️ dsh: web-app lokal, belum ada TUI publik |
| MCP support | ⚠️ **stdio-only**, tidak ada remote/HTTP/SSE MCP server. Tapi ini *berfungsi nyata* — saya trace wiring-nya sampai `repl.go`, bukan dead code | ✅ stdio + remote | ✅ | ✅ | bervariasi |
| Permission/path-guard per tool | ✅ **kuat** — path traversal defense, role ceiling subagent, wajib-ask untuk shell arbitrary meski YOLO mode (diverifikasi audit sebelumnya, saya tidak ulangi verifikasi detail ini) | ✅ (sandbox berbeda pendekatan) | ⚠️ | ⚠️ bervariasi per konfigurasi | ⚠️ sandbox = plugin, kualitas tergantung plugin |
| Subagent/delegation | ✅ dengan role ceiling & context isolation | ✅ | ⚠️ | ✅ | ✅ (dsh bahkan bisa panggil Claude Code/Codex sebagai sub-agent) |
| Hooks (Pre/PostToolUse) | ✅ ada (`internal/hooks`) | ✅ | ⚠️ | ✅ | ✅ (event waterfall di dsh) |
| Project memory file (`LUNA.md`/`CLAUDE.md`) | ✅ dengan `@import` syntax + cycle detection | ✅ (`CLAUDE.md`) | ✅ | ✅ (`AGENTS.md`) | bervariasi |
| Custom slash commands | ✅ (`.luna/commands/`) | ✅ | ✅ | ✅ | bervariasi |
| Session resume/checkpoint | ✅ (checkpoint schema v2, atomic write) | ✅ | ✅ | ✅ (git-backed) | ⚠️ dsh: session log sebagai plugin, baru preview |
| Plugin architecture (extend model/tool/sandbox sendiri) | ❌ — semua tool hardcoded Go struct, tidak ada mekanisme plugin eksternal | ❌ (tidak plugin-first) | ❌ | ⚠️ sebagian (MCP sebagai extension point) | ✅ **ini justru diferensiator utama dsh** — semuanya plugin |
| Windows support | ✅ (goreleaser build windows/amd64) tapi `install.sh` hanya untuk macOS/Linux — user Windows harus unduh manual | ✅ | ✅ | ✅ | ⚠️ |
| CI aktual hijau di HEAD | ❌ **merah** (lihat §1) | n/a (closed) | n/a | n/a | n/a |
| Web search bawaan | ✅ diklaim README ("DuckDuckGo Lite Scraper"), tool ter-register | ✅ | ✅ (grounding) | ⚠️ tergantung provider | ⚠️ |
| Dependency footprint | ✅ **sangat ramping** (cuma cobra+pflag+mousetrap, HTTP/SSE/JSON-schema semua manual stdlib) — bagus untuk supply-chain, tapi juga sebab TUI-nya tidak ada | — | — | lebih berat (banyak provider SDK) | lebih berat (plugin ecosystem) |

---

## 4. Analisis: di mana `luna-go` sudah kompetitif, di mana belum

### Sudah kompetitif / potensi keunggulan nyata
- **Permission model**-nya, kalau klaim audit sebelumnya akurat (path traversal defense + role ceiling + wajib-ask untuk shell arbitrary), **sungguh-sungguh lebih ketat** dari kebanyakan kompetitor open-source yang saya temukan di riset — banyak tool sejenis masih mengandalkan sandbox longgar atau bahkan tanpa isolasi role untuk subagent.
- **Dependency footprint minim** = supply-chain attack surface kecil, build reproducible, cocok untuk audit keamanan pihak ketiga (nilai jual ke pengguna enterprise/security-conscious yang justru *tidak* ingin dependency plugin-everything ala dsh, karena permukaan serangannya jadi besar).
- **Multi-provider tanpa lock-in** — filosofi yang sama dengan OpenCode, dan pasar untuk ini terbukti besar (OpenCode #1 di kategori open-source justru karena alasan ini, ditambah beberapa insiden industri seperti OpenCode yang harus drop login Claude Pro/Max karena sengketa dengan Anthropic — pengingat bahwa strategi "provider-agnostic" bisa kena masalah kontraktual juga, bukan cuma teknis).

### Belum kompetitif — gap yang menentukan adopsi
1. **Binary rusak di HEAD (§1)** — ini bukan gap fitur, ini blocker mutlak. Tidak ada user yang akan mencoba tool yang crash di `--help`.
2. **Tidak ada TUI interaktif** — ini kemungkinan gap terbesar untuk *daya saing produk* (bukan cuma kode). Semua lima nama yang Anda sebut (Claude Code, Gemini CLI, OpenCode, dan bahkan dsh yang baru rilis Agustus ini) berinvestasi besar di pengalaman terminal yang hidup — live-updating diff, autocomplete slash command, riwayat command yang bisa di-scroll dengan panah atas/bawah antar sesi, dsb. `luna-go` saat ini adalah REPL baris-demi-baris klasik. Ini akan terasa sangat mentah dibanding kompetitor pada percobaan pertama, terlepas dari seberapa bagus permission model di baliknya.
3. **Tidak ada login tanpa API key** — hambatan adopsi nyata untuk user non-developer atau yang tidak mau urus billing API key manual. Claude Code dan Gemini CLI berhasil menang pengguna awam justru lewat ini.
4. **MCP stdio-only** — cukup untuk sebagian besar use case lokal, tapi remote MCP (server yang berjalan sebagai layanan HTTP, bukan proses lokal) makin jadi standar untuk integrasi tim/enterprise (Slack, Linear, dsb. seperti yang OpenCode sudah punya).
5. **Tidak ada mekanisme plugin eksternal** — kalau `dsh` dan tren "harness sebagai plugin platform" ini menang di komunitas (indikasi awal: 95rb bintang dalam 2 hari), maka *extensibility* akan jadi kriteria kompetitif utama ke depan, bukan cuma jumlah tool bawaan.

---

## 5. Rekomendasi prioritas (bukan cuma "perbaiki bug", tapi arah produk)

### Prioritas 0 — sebelum siapa pun mencoba tool ini
- Perbaiki off-by-one di `app.go` (§1). Lima menit kerja, blocker total.
- Perbaiki `install.sh` supaya jelas soal Windows (atau tambahkan `winget`/scoop manifest — banyak user Windows tidak akan tahu harus cari halaman Releases secara manual).

### Prioritas 1 — supaya orang mau *mencoba lagi* setelah pertama kali
- TUI minimal viable: bukan harus langsung menyamai Ink/Bubble Tea penuh, tapi minimal `golang.org/x/term` untuk raw mode + riwayat command dalam sesi (panah atas/bawah) + rendering diff/tool-call yang live-update, bukan print linear. Ini investasi UX dengan ROI adopsi tertinggi dari semua gap yang ada.
- Pertimbangkan device-code OAuth flow minimal untuk minimal satu provider (paling gampang lewat OpenRouter yang sudah jadi provider bawaan) supaya ada jalur "coba tanpa harus generate API key dulu".

### Prioritas 2 — supaya bisa disebut setara fitur
- Remote MCP (HTTP/SSE), bukan cuma stdio.
- Perbaiki §3.4/§3.5 dari audit sebelumnya (deadlock `io.MultiReader`, leak `bgProcesses`) — kelas bug ini akan langsung ketahuan user power-user yang menjalankan sesi panjang, justru target audiens paling vokal untuk tool seperti ini.
- Update default model Anthropic (`claude-3-7-sonnet-20250219` sekarang cukup lama; per Agustus 2026 default yang masuk akal adalah generasi Sonnet/Opus terbaru — cek katalog model terkini sebelum hardcode, karena ini akan basi lagi kalau di-hardcode tanpa proses update berkala).

### Prioritas 3 — diferensiasi jangka panjang
- Jangan coba menandingi `dsh` di "semua-adalah-plugin" secara langsung — itu taruhan arsitektur besar dan `dsh` baru developer preview, terlalu dini untuk tahu apakah pola itu menang. Sebaliknya, **pertajam identitas "CLI agentic paling ketat sandbox-nya"** sebagai posisi pasar yang jelas — dokumentasikan model permission itu secara eksplisit dan mudah diverifikasi pihak ketiga (mis. threat model doc terpisah), karena itu satu-satunya klaim yang audit hari ini benar-benar bisa mendukung secara teknis.

---

## 6. Skor kesiapan (per audit hari ini, bukan mengulang skor audit sebelumnya)

| Aspek | Skor | Catatan |
|---|---|---|
| Build/compile | 100% | `go build ./...` bersih |
| **Binary bisa dijalankan sama sekali** | **0%** | crash di setiap invocation, lihat §1 |
| Test suite hijau | 95% (21/22 package) | 1 kegagalan, tapi kegagalan itu adalah kegagalan *paling penting* di seluruh suite |
| Permission/security | ~90% (mewarisi penilaian audit sebelumnya, tidak diverifikasi ulang detail) | tetap kekuatan utama |
| Feature parity vs kompetitor open-source (OpenCode) | ~45% | multi-provider ✅, tapi TUI/login/remote-MCP/plugin semuanya kalah |
| Distribusi/onboarding | ~40% | install.sh Unix-only, tanpa OAuth, dokumentasi user (`docs/user/`) diakui README sendiri "segera hadir" |

**Kesimpulan jujur untuk pertanyaan Anda ("bisa bersaing atau minimal jadi alternatif"):** belum, hari ini — tapi bukan karena arsitekturnya lemah (justru sebaliknya, fondasi permission-nya termasuk yang lebih baik dari yang saya temukan di riset kompetitor). Yang menghalangi adalah (a) satu bug fatal yang membuat binary tak terpakai sama sekali, dan (b) investasi UX/onboarding (TUI, login tanpa key) yang di industri ini ternyata jadi faktor adopsi jauh lebih besar daripada kelengkapan tool backend. Perbaiki §1 hari ini, lalu targetkan TUI minimal dalam beberapa sesi ke depan — baru pada titik itu "alternatif OpenCode yang lebih ketat sandbox-nya" jadi klaim yang realistis untuk dipasarkan.
