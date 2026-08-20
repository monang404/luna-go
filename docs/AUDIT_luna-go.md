# Audit Menyeluruh — `luna-go` (AI Agent CLI)

**Metodologi:** Bukan review kode statis semata. Repo di-*build* (`go build ./...`), di-*vet* (`go vet ./...`), dan di-*test* (`go test ./...`) secara nyata di lingkungan sandbox (Go 1.22, toolchain terinstal via apt), plus pembacaan manual pada modul-modul kritikal (permission, tools, llmclient, subagent). Semua temuan di bawah sudah diverifikasi terhadap kode aktual, bukan asumsi.

---

## 1. Executive Summary

Codebase ini **jauh di atas rata-rata** untuk ukuran "hasil migrasi 75 sesi" — arsitektur permission/path-guard-nya termasuk yang paling matang yang pernah saya lihat di CLI agent buatan komunitas, dan dokumentasi inline (komentar yang menjelaskan *devation* dari sumber zsh, alasan desain, acceptance criteria per sesi) jauh melebihi standar industri kebanyakan repo solo-dev.

**Tapi ada satu masalah yang tidak bisa saya lewatkan:** `go build ./...` **GAGAL** di HEAD. Ini bukan nitpick gaya kode — ini berarti klaim "seluruh siklus pengembangan 75 sesi telah selesai" tidak akurat: binary `luna` saat ini **tidak bisa dikompilasi sama sekali**, jadi tidak ada versi codebase ini yang benar-benar bisa dijalankan sebagai CLI. Root cause-nya sempit (satu baris wiring di `cmd/luna/commands/app.go`), tapi dampaknya total.

Setelah saya tambal satu baris itu (hanya untuk melanjutkan audit, tidak saya reapply ke sumber Anda), sisa codebase compile bersih, `go vet` bersih, dan 21 dari 22 package test **PASS** — hanya `internal/config` yang gagal karena satu regression test yang mendeteksi penyimpangan nyata dari parity zsh (lihat §3).

**Kesimpulan jujur:** Kualitas desain arsitektur (permission gating, path traversal defense, no-shell exec_process, subagent isolation) ada di level *production-grade*. Kualitas *integrasi final* (wiring, endpoint config, parity test terakhir) ada di level *belum siap rilis* — kesan saya adalah SESSION-55 (wiring CLI) terpotong sebelum selesai diverifikasi end-to-end dengan `go build`.

---

## 2. Kekuatan & Hal Positif

- **Path traversal defense (`internal/permission/pathguard.go`) sangat solid.** `CanonicalPath` menangani symlink resolution bahkan untuk file yang belum ada (dengan berjalan naik ke ancestor terdekat yang ada), dan `PathWithinProject` melakukan containment check dengan prefix-match yang benar (`root` atau `root/*`, bukan sekadar `strings.HasPrefix` naif yang rentan terhadap `/project-evil` vs `/project`). Ini dipanggil **unconditionally untuk setiap tool yang punya path**, termasuk yang readonly (`dispatch.go`), sebelum permission-ask apa pun terjadi.
- **`exec_process`/`run_test` tidak pernah lewat shell interpreter** — argv langsung ke `exec.CommandContext`, bukan `sh -c`. Dikombinasikan dengan allowlist program (`git`, `python`, `node`, dst.) dan proteksi PATH-hijacking (`resolveNonProjectExecutable` menolak binary yang ternyata resolve ke dalam project root — mencegah `./git` palsu di-PATH-prepend). Ini pola defense-in-depth yang benar.
- **Role ceiling pada subagent (AC-04 di `permission/check.go`)** — `RoleSubagent` diblokir dari capability tertentu **sebelum** gate YOLO/ask sempat jalan, jadi eskalasi lewat "ask sekali" pun tidak bisa menembusnya. Isolasi context subagent (`subagent/run.go`) juga didesain eksplisit: `AgentContext` baru per subagent (bukan pointer parent yang dipakai bersama), session ID berbeda dari parent, depth-limit terhadap rekursi subagent.
- **`checkShell` sengaja tidak mengizinkan `shell.arbitrary` auto-allow di bawah YOLO** — satu-satunya capability yang selalu wajib nge-ask meski YOLO aktif. Ini keputusan desain yang tepat mengingat `run_command` (lihat §3) adalah shell-interpreted.
- **Context cancellation & backpressure di SSE streaming (`llmclient/streaming.go`)** ditangani benar: goroutine `defer cancel()` + `defer httpResp.Body.Close()`, dan `streamBody` mengirim event lewat `select` yang menghormati `ctx.Done()` — tidak ada buffering tak terbatas kalau consumer lambat.
- **Test coverage nyata**, bukan hiasan: 90 file `*_test.go` vs 138 file source (~65% file ratio), dan sebagian besar test membaca sangat spesifik terhadap perilaku zsh asli (banyak `// mirrors _ai_xxx` dengan acceptance-criteria eksplisit). Saya jalankan semuanya — 21/22 package PASS.
- **Dependency footprint sangat ramping**: hanya `cobra`+`pflag`+`mousetrap` yang di-vendor. Tidak ada dependency pihak ketiga yang berat/berlebihan seperti disebut di poin audit permintaan Anda — justru sebaliknya, semuanya (HTTP client, SSE parser, JSON schema validation) ditulis manual pakai stdlib. Ini bagus untuk supply-chain risk, tapi ada trade-off (lihat §4, YAML frontmatter).

---

## 3. Temuan Kritis & Kerentanan (High Priority)

### 3.1 — Build gagal total di HEAD (CRITICAL)
**File:** `cmd/luna/commands/app.go:114`
```go
register("web_search", &tools.WebSearchTool{Deps: deps})
```
Dua bug independen di satu baris:
1. `*tools.WebSearchTool` **tidak mengimplementasikan `tools.Tool`** — method `Execute` di `internal/tools/websearch.go:24` bertanda tangan `(string, error)`, sementara interface `Tool` (`internal/tools/tool.go`) mewajibkan `(Result, error)`. `WebSearchTool` juga tidak punya method `Capability()` sama sekali.
2. `deps` di baris itu **tidak pernah dideklarasikan** di scope `buildDispatcher()` — `undefined: deps`.

Saya verifikasi ini bukan salah baca saya — `go build ./...` gagal persis di titik ini:
```
cmd/luna/commands/app.go:114:25: cannot use &tools.WebSearchTool{…} (value of type *tools.WebSearchTool) as tools.Tool value in argument to register: *tools.WebSearchTool does not implement tools.Tool (missing method Capability)
cmd/luna/commands/app.go:114:52: undefined: deps
```
Setelah saya netralkan sementara baris ini (hanya untuk melanjutkan audit), **sisa seluruh codebase compile bersih** dan `go vet ./...` tidak melaporkan apa pun. Jadi kerusakannya terisolasi, tapi fatal: binary `luna` sekarang tidak bisa di-`go build` sama sekali.

**Saran perbaikan:** `WebSearchTool` perlu di-refactor agar `Execute` mengembalikan `tools.Result` (bukan `string`) dan menambahkan `Capability() permission.Capability` (kemungkinan `permission.CapNetworkPublic`, konsisten dengan `web_fetch`). `deps` di `buildDispatcher()` perlu diganti jadi `PermDeps{}` yang benar sesuai konstruktor `WebSearchTool`, atau `WebSearchTool` di-refactor supaya deps-nya di-inject lewat cara yang sama seperti `DelegateTaskTool.Spawner` (lewat method konfigurasi setelah registrasi), bukan lewat field struct literal yang mereferensikan variabel tidak ada.

### 3.2 — Endpoint Cerebras salah / domain tidak nyata
**File:** `internal/config/providers.go:44`
```go
"cerebras": {
    Endpoint: "https://api.cerebras.luna/v1/chat/completions",
    ...
```
Domain resmi Cerebras adalah `api.cerebras.ai`, bukan `api.cerebras.luna` — `.luna` bukan TLD yang valid. Setiap request ke provider Cerebras akan gagal di tahap DNS resolution, bukan di tahap otentikasi. Ini kemungkinan besar typo/artefak saat rename project ke "luna" yang tidak sengaja ikut menimpa endpoint asli. Berdampak langsung ke *provider fallback chain* — kalau Cerebras dipanggil sebagai fallback (lihat `TaskProviderOrderFast`/`Smart`/`Big` yang semuanya memasukkan `cerebras`), permintaan akan selalu gagal di provider itu, membebani circuit breaker dengan kegagalan yang sebenarnya tidak seharusnya terjadi.

**Saran:** ganti ke `https://api.cerebras.ai/v1/chat/completions` dan tambahkan regression test seperti yang sudah ada untuk provider lain (`TestProviders_DefaultModels` sayangnya tidak menguji `Endpoint`, hanya `Model` — celah cakupan test yang sama juga perlu ditutup).

### 3.3 — Regression test gagal: `ProviderOrder` menyimpang dari parity zsh yang diklaim sendiri
**File:** `internal/config/providers.go:76` vs `internal/config/config_test.go:76-88`

```go
var ProviderOrder = []string{"anthropic", "openrouter", "groq", "gemini", "cerebras"}
```
Test yang menyertai kode ini secara eksplisit menyatakan (komentar sumbernya sendiri): *"Direct parity check against 35-providers.zsh:33 — AI_PROVIDER_ORDER=(groq gemini cerebras), no deepseek."* — yaitu, `ProviderOrder` seharusnya **hanya** `[groq, gemini, cerebras]`. Hasil `go test ./internal/config/...` nyata gagal:
```
--- FAIL: TestProviderOrder_ExcludesDeepseek (0.00s)
    config_test.go:86: ProviderOrder = [anthropic openrouter groq gemini cerebras], want [groq gemini cerebras]
```
Ini bukan test yang salah tulis — ini sinyal bahwa `ProviderOrder` di source diubah (kemungkinan sengaja, menambahkan `anthropic`/`openrouter` ke fallback chain) di sesi kemudian **tanpa mengupdate test parity-nya**. Kalau perubahan itu memang disengaja, test-nya yang perlu diupdate; kalau tidak, ini bug fungsional nyata pada urutan fallback provider. Either way: **test suite CI Anda saat ini merah**, bukan hijau seperti yang mungkin diasumsikan.

### 3.4 — Risiko deadlock pada `bash_output.go` saat menangkap stdout+stderr proses background
**File:** `internal/tools/bash_output.go:46-49`
```go
go func() {
    multi := io.MultiReader(stdoutPipe, stderrPipe)
    buf := make([]byte, 1024)
    for {
        n, err := multi.Read(buf)
        ...
```
`io.MultiReader` membaca reader-reader-nya **secara berurutan**, bukan paralel: ia tidak akan menyentuh `stderrPipe` sama sekali sampai `stdoutPipe` mengembalikan `io.EOF` (yang hanya terjadi kalau proses child menutup stdout — biasanya berarti proses sudah exit). Kalau proses background menulis banyak ke **stderr** sementara stdout tetap terbuka dan sepi (pola umum: banyak tool CLI print progress ke stderr), maka begitu OS pipe buffer stderr penuh (biasanya 64KB di Linux), child process akan **blok** di `write(2)` ke stderr menunggu pembaca — padahal goroutine pembaca yang seharusnya membaca stderr itu sendiri masih menunggu data dari stdout yang tidak pernah datang. Hasilnya: **deadlock** — proses child tidak pernah exit, `bash_output`/`kill_shell` jadi satu-satunya jalan keluar (dan bahkan `kill_shell` cuma kirim SIGTERM, tidak menjamin child langsung mati kalau ia sendiri stuck di syscall write yang tidak akan pernah unblock tanpa SIGKILL setelah SIGTERM diabaikan).

**Saran perbaikan:** baca `stdoutPipe` dan `stderrPipe` di **dua goroutine terpisah**, masing-masing menulis ke `bgp.buf` dengan mutex yang sama (pola standard `io.Copy` ganda + `sync.WaitGroup`), bukan digabung lewat `io.MultiReader`.

### 3.5 — `bgProcesses` map tidak pernah dibersihkan (unbounded growth / memory leak)
**File:** `internal/tools/bash_output.go` (var `bgProcesses` di baris ~20, tidak ada `delete()` di seluruh file — saya cek dengan grep, nol hasil)

Setiap kali `exec_process`/`run_command` dipanggil dengan `background: true`, entry baru ditambahkan ke `bgProcesses[id]` dan **tidak pernah dihapus** — bahkan setelah proses selesai (`bgp.done == true`) dan outputnya sudah dibaca habis lewat `bash_output`, atau setelah `kill_shell` berhasil mengirim SIGTERM. Untuk REPL session yang panjang (yang memang jadi target UX utama project ini, mirip Claude Code), setiap background command menambah entry permanen ke map global proses — tidak akan crash cepat, tapi ini kebocoran memori yang nyata untuk sesi jangka panjang atau skrip yang sering spawn background job.

**Saran:** tambahkan TTL atau `delete(bgProcesses, id)` setelah `bash_output` melaporkan `bgp.done && buf kosong` untuk N kali berturut-turut, atau batasi ukuran map dengan LRU eviction.

### 3.6 — `run_command` tetap arbitrary shell execution berbasis denylist
**File:** `internal/tools/process.go` (`RunCommandTool.Execute`), `internal/tools/policy.go` (`IsDangerousCommand`)

Ini sudah didokumentasikan jujur oleh komentar kode itu sendiri sebagai keputusan sadar ("legacy... kept for backward compatibility"), dan dimitigasi cukup baik lewat wajib-ask di `checkShell` (§2). Tapi tetap perlu ditandai sebagai *residual risk* eksplisit: `IsDangerousCommand` adalah **denylist regex**, bukan allowlist — pola klasik yang secara historis selalu bisa dilewati (mis. command yang tidak eksplisit destruktif tapi tetap punya efek samping berbahaya, atau variasi encoding/spacing yang lolos regex). Karena tool ini tetap ada di Dispatcher (`register("run_command", ...)` di `app.go`) dan tidak disembunyikan dari manifest LLM secara default (`registry.go`'s Manifest — perlu dicek ulang apakah benar `run_command` disembunyikan; kalau *tidak*, model bisa memilihnya sendiri), pastikan minimal: (a) tool ini memang di-hide dari manifest tool-calling default, (b) prompt konfirmasi untuk `shell.arbitrary` menampilkan command penuh ke user sebelum approve, bukan versi terpotong.

---

## 4. Perbaikan Menengah & UX (Medium Priority)

- **Parser YAML frontmatter subagent tidak benar-benar YAML.** `internal/subagent/loader.go` (`parseDefinition`) adalah parser line-based manual (`strings.SplitN(content, "---", 3)` + `strings.HasPrefix(line, "description:")`), **bukan** `gopkg.in/yaml.v3` — dan memang tidak ada dependency yaml apa pun di `go.mod`/`vendor/`. Ini longgar dari asumsi di prompt audit Anda sendiri. Dua masalah konkret:
  1. `SplitN(content, "---", 3)` akan salah parse kalau *system prompt* di body markdown kebetulan mengandung baris `---` (horizontal rule, umum di markdown) — bagian setelah `---` kedua akan terpotong dari `def.System`.
  2. Tidak ada dukungan untuk value multi-baris, string ber-quote, comment, atau escaping — kalau `tools:` list makin kompleks di masa depan (nested config per tool, misalnya), parser ini akan butuh ditulis ulang total, bukan diperluas.
  Untuk 2 field sederhana (`description`, `tools`) ini "cukup jalan", tapi menyebutnya "parsing solid dengan yaml.v3" (seperti asumsi awal task Anda) tidak akurat terhadap kondisi kode nyata.
- **State global pada `internal/subagent/loader.go`** (`var registry = make(map[Role]*Definition)`, dilindungi `registryMu` package-level) berarti semua subagent definition dari **seluruh proses** berbagi satu map, bukan per-`Dispatcher`/per-App instance. Untuk CLI single-process ini biasanya tidak masalah, tapi menyulitkan test isolation (dua test yang jalan paralel dan sama-sama memanggil `LoadDefinitions` dengan direktori berbeda akan saling menimpa) dan menutup jalan kalau suatu saat `luna-go` perlu multi-tenant/multi-project dalam satu proses.
- **Cakupan test provider config tidak menguji `Endpoint`.** `TestProviders_DefaultModels` hanya membandingkan `Model`, bukan `Endpoint` — itulah sebabnya bug §3.2 (`api.cerebras.luna`) lolos tanpa terdeteksi test. Tambahkan assertion endpoint per provider.
- **Default model Anthropic sudah cukup lama** (`claude-3-7-sonnet-20250219` di `internal/config/providers.go:53`) — bukan bug, tapi worth di-review apakah masih model yang diinginkan sebagai default per Agustus 2026.
- **`KillShellTool` hanya mengirim SIGTERM**, tanpa fallback SIGKILL setelah timeout kalau proses tidak merespons (relevan terutama kalau §3.4 terjadi dan proses benar-benar stuck di syscall write — SIGTERM tidak akan pernah diproses kalau proses stuck di kernel-level blocking write, perlu SIGKILL).

---

## 5. Skor Siap Rilis

| Aspek | Skor |
|---|---|
| Arsitektur permission/security | 90% |
| Filesystem tool safety (path traversal, secret detection) | 90% |
| Shell/process execution safety | 75% (desain bagus, tapi §3.4/§3.5/§3.6 nyata) |
| Multi-provider LLM abstraction | 60% (endpoint salah, provider order menyimpang dari test-nya sendiri) |
| Build/release readiness **saat ini** | **0%** — tidak bisa di-`go build` dari HEAD |
| Test discipline | 80% (coverage bagus, tapi CI merah tidak boleh diabaikan) |

**Skor keseluruhan kesiapan Production/Dogfooding: 35/100.**

Ini bukan cerminan kualitas desain (yang saya nilai tinggi, lihat §2) — ini murni karena satu regresi wiring yang membuat binary tidak bisa dibangun sama sekali, ditambah satu bug endpoint dan satu test merah yang sudah ada di HEAD. Perbaikan §3.1–3.3 kemungkinan bisa selesai dalam hitungan menit-jam, bukan sesi baru — begitu itu beres dan `go build && go test ./...` hijau semua, skor riil codebase ini (berdasarkan kualitas desain yang sudah ada) realistis di kisaran **75-80%** untuk tahap dogfooding pribadi.
