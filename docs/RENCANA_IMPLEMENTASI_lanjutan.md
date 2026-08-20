# Rencana Implementasi Lanjutan — `luna-go`
### Tindak lanjut dari `AUDIT_KOMPETITIF_luna-go.md`, dilebur dengan status riil `docs/RENCANA_luna-go_90.md` & `docs/ROADMAP_luna-go_distribusi.md`

**Tanggal:** 20 Agustus 2026
**Format:** melanjutkan konvensi SESSION-XX yang sudah Anda pakai. Saya **tidak menulis ulang** rencana yang sudah bagus di dua dokumen lama (`SESSION-76` s/d `SESSION-90`) — saya verifikasi dulu satu-per-satu mana yang **sudah benar-benar dikerjakan** di kode (bukan cuma diklaim), lalu lanjutkan penomoran dari sana. Beberapa yang sudah selesai malah lebih baik dari ekspektasi dokumen lama (mis. README sudah ada, padahal `SESSION-88` lama bilang "repo ini tidak punya README sama sekali" — itu sudah basi).

---

## Bagian 1 — Audit status: mana yang sudah, mana yang belum

Saya verifikasi ini lewat `grep`/pembacaan kode langsung hari ini, bukan asumsi dari dokumen lama.

| Sesi lama | Target | Status riil hari ini |
|---|---|---|
| SESSION-76 | Perbaiki `web_search` + `deps` undefined | ✅ **Selesai** — compile bersih |
| SESSION-77 | `ProviderOrder` selaras test, endpoint Cerebras benar | ✅ **Selesai** — `go test ./internal/config/...` PASS |
| SESSION-78 | Deadlock `io.MultiReader`, leak `bgProcesses` | ✅ **Selesai** — `MultiReader` sudah hilang dari `bash_output.go`, ada `delete(bgProcesses, id)` |
| SESSION-79 | Parser YAML frontmatter subagent jadi YAML asli | ❌ **Belum** — masih parser manual `strings.Split` baris-per-baris, tidak ada dependency yaml di `go.mod`. Sedikit lebih baik dari sebelumnya (pencarian delimiter `---` penutup sudah per-baris, bukan `SplitN` naif), tapi tetap bukan YAML nyata. |
| SESSION-80 | Verifikasi akhir + gerbang rilis | ❌ **Belum bisa lulus** — lihat SESSION-91 baru di bawah, ada bug baru yang jadi blocker |
| SESSION-81–83 | Legal, versioning, cross-platform build | ⚠️ **Sebagian** — `SECURITY.md` ada, `.goreleaser.yaml` sudah cover 3 OS, `--version` flag sudah ada di `root.go`. Belum diverifikasi apakah `main.version` benar-benar di-set lewat ldflags saat build asli (perlu dicek saat SESSION-92). |
| SESSION-84 | MCP client | ⚠️ **Sebagian** — stdio transport **sudah jalan nyata** (saya trace wiring-nya sampai `repl.go`, bukan dead code), tapi remote/HTTP/SSE **belum ada sama sekali**. |
| SESSION-85 | `LUNA.md` project memory | ✅ **Selesai** — `memory.LoadMemoryFiles` dipanggil dari `repl.go`, dengan `@import` + cycle detection. |
| SESSION-86 | TUI parity (interrupt ganda, multi-line input, image attach) | ❌ **Belum** — ini yang paling kritikal, lihat SESSION-93 di bawah. Tidak ada raw terminal mode sama sekali, `bufio.Scanner` baris-per-baris murni. |
| SESSION-87 | Sandboxing opsional (`bubblewrap`) | ❌ Belum — tetap sebagai item roadmap pasca-rilis, tidak berubah prioritasnya. |
| SESSION-88 | README yang menjual | ✅ **Selesai** — README sekarang punya positioning, instalasi, quickstart, config, privacy/security, lisensi. Dokumen lama ini sudah basi di poin ini. |
| SESSION-89 | `docs/user/` terpisah dari dokumentasi proses | ❌ Belum — `docs/user/` belum ada, README masih bilang "segera hadir". |
| SESSION-90 | CI/CD penuh (matrix OS, goreleaser-on-tag, issue template) | ❌ Belum — CI masih `ubuntu-latest` saja, tidak ada job release, tidak ada `.github/ISSUE_TEMPLATE/`. |

**Kesimpulan Bagian 1:** progress nyata dan cukup cepat sejak dokumen-dokumen lama ditulis — P0 lama (76-78) tuntas, beberapa item P2 (85, 88) bahkan sudah lebih maju dari ekspektasi. Tapi **satu bug baru** (di luar cakupan dokumen manapun) sekarang jadi blocker rilis yang lebih fatal dari semua yang sudah diperbaiki, dan **gap terbesar untuk daya saing produk (TUI)** justru salah satu yang paling belum tersentuh.

---

## Bagian 2 — Rencana eksekusi baru, lanjut dari SESSION-91

Urutan: **P0 dulu** (blocker rilis mutlak) → **P1** (gap adopsi terbesar menurut audit kompetitif) → **P2** (kelengkapan rilis & operasional).

---

### SESSION-91 — P0: Perbaiki panic fatal di `buildDispatcher()`

**Kenapa ini duluan mutlak:** binary `luna` saat ini crash di **setiap** eksekusi (`luna --help` sekalipun). Tidak ada gunanya mengerjakan apa pun lain sebelum ini selesai — sama seperti alasan SESSION-76 dulu jadi prioritas pertama.

**File:** `go/cmd/luna/commands/app.go:100-126`

**Tugas 91.1 — Hilangkan magic number, hitung otomatis**

```go
func buildDispatcher() *tools.Dispatcher {
	d := tools.NewDispatcher()
	registered := 0
	register := func(name string, tool tools.Tool) {
		entry := tools.Registry[name]
		if err := d.Register(name, entry, tool); err != nil {
			panic(fmt.Sprintf("commands: buildDispatcher: %v", err))
		}
		registered++
	}
	register("read_file", tools.ReadFileTool{})
	// ... (semua 22 baris register() yang sudah ada, tidak berubah)
	register("delegate_task", &tools.DelegateTaskTool{})

	if registered != len(tools.Registry) {
		panic(fmt.Sprintf(
			"commands: buildDispatcher: tools.Registry has %d entries, only %d are wired -- add the missing register() call",
			len(tools.Registry), registered,
		))
	}
	return d
}
```

Ini menghilangkan angka `21`/`22` yang perlu diingat manusia untuk diupdate — self-check-nya sekarang **tidak bisa basi lagi**, karena `registered` dihitung dari closure itu sendiri, bukan diketik ulang manual. Ini juga langsung memperbaiki akar masalah supaya bug yang sama tidak terulang saat tool ke-23 ditambahkan nanti (dan pasti akan ditambahkan, mengingat MCP dynamic registration di SESSION-94 di bawah akan menambah entri terus-menerus).

**Tugas 91.2 — Tambahkan regression test eksplisit**

`cmd/luna/commands/root_test.go` sudah punya `TestRegistryParity` yang **berhasil menangkap** bug ini (itu sebabnya `go test ./...` merah, meski `go build` hijau) — bagus, tapi pastikan test ini juga dijalankan sebagai bagian smoke-test binary nyata, bukan cuma lewat `go test`. Tambahkan satu baris di CI:

```yaml
# .github/workflows/ci.yml, tambahkan job step baru setelah "Test"
- name: Smoke test binary
  run: |
    go build -o /tmp/luna-smoke ./cmd/luna
    /tmp/luna-smoke --help
```

Ini menutup celah yang bikin bug ini lolos dari review manual kemarin: `go build ./...` hijau tidak menjamin binary bisa *dijalankan*. Smoke test literal menjalankan binary adalah satu-satunya cara menutup celah itu — `go vet`/`go build` tidak akan pernah menangkap `panic()` runtime seperti ini.

**Acceptance criteria SESSION-91:** `go test ./...` 100% PASS, dan `go build -o luna ./cmd/luna && ./luna --help` menampilkan help text lengkap tanpa panic. **Ini gerbang wajib** sebelum sesi lain di bawah dikerjakan — sama seperti SESSION-76 dulu.

---

### SESSION-92 — P0: Tutup verifikasi rilis yang masih menggantung dari SESSION-81–83

Sebelum masuk ke fitur besar (TUI, MCP remote), tuntaskan dulu item P0/P1 lama yang statusnya masih "sebagian" — ini kerjaan cepat (jam, bukan sesi penuh), dan tanpa ini `SESSION-80` (gerbang rilis) tidak bisa lulus jujur.

**Tugas 92.1** — Verifikasi `main.version` benar-benar ter-inject dari `.goreleaser.yaml`:
```bash
cd go
go build -ldflags "-X main.version=v0.1.0-test" -o /tmp/luna-vtest ./cmd/luna
/tmp/luna-vtest --version   # harus print "v0.1.0-test", bukan string kosong/default
```
Kalau `main.go` tidak punya var `version` yang di-passing ke `Execute(version)` dengan benar, ini P0 kecil yang harus ditutup — versi binary yang salah/kosong adalah hal pertama yang dicek user power-user saat lapor bug.

**Tugas 92.2** — Tambahkan `.github/ISSUE_TEMPLATE/bug_report.md`, `feature_request.md`, `.github/PULL_REQUEST_TEMPLATE.md` (item 90.1 lama, belum dikerjakan). Ini murni template teks, tidak butuh keputusan desain — kerjakan sekarang, jangan tunda ke sesi "dokumentasi" nanti supaya tidak lupa lagi.

**Tugas 92.3** — Perluas CI matrix lintas OS (item 90.2 lama):
```yaml
jobs:
  build-test:
    strategy:
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
    runs-on: ${{ matrix.os }}
    defaults:
      run:
        working-directory: ./go
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with: { go-version: '1.22' }
      - run: go build ./...
      - run: go vet ./...
      - run: go test ./...
```
Ini penting **khusus** untuk `luna-go` karena `install.sh` saat ini eksplisit Unix-only (audit kompetitif §4) — kalau Anda berniat serius mendukung Windows (binary Windows sudah di-build lewat goreleaser, jadi niatnya ada), matrix Windows di CI adalah satu-satunya cara murah menangkap regresi path-separator/line-ending sebelum user Windows yang menemukannya duluan.

**Acceptance criteria SESSION-92:** `--version` menampilkan versi asli hasil build, CI hijau di 3 OS, issue/PR template ada.

---

### SESSION-93 — P1: Fondasi TUI interaktif (gap kompetitif #1)

**Ini sesi paling besar di rencana ini, dan yang paling menentukan apakah `luna-go` terasa "setara" saat pertama kali dicoba orang.** Audit kompetitif menyimpulkan ini gap terbesar — bukan cuma soal kode kurang, tapi soal *first impression*: user yang baru pindah dari Claude Code/OpenCode akan langsung merasakan bedanya dalam 10 detik pertama.

Dokumen roadmap lama (`SESSION-86`) menyebut poin-poin yang tepat (interrupt ganda, multi-line, image attach) tapi **tidak** membahas fondasi paling dasar yang harus ada duluan: **raw terminal mode + line editing**. Tanpa itu, ketiganya tidak bisa dibangun di atasnya. Saya pecah jadi 3 tugas, urut dari fondasi ke permukaan:

**Tugas 93.1 — Ganti `bufio.Scanner` dengan line editor yang punya history**

Tidak perlu langsung membangun TUI penuh ala Bubble Tea (itu investasi besar, bisa jadi sesi terpisah nanti kalau mau full-screen UI). Langkah paling murah dengan ROI tertinggi: pakai library line-editing readline-style yang stdlib-compatible dan ringan (`github.com/chzyer/readline` atau `github.com/peterh/liner` — keduanya MIT/BSD, dependency tunggal, tidak menambah footprint berat seperti kekhawatiran Anda soal dependency bloat). Ini memberi:
- Panah atas/bawah untuk riwayat command dalam sesi (ekspektasi standar sejak shell apa pun).
- Ctrl+A/E/W untuk navigasi baris (readline standar, gratis dari library-nya).
- Fondasi untuk multi-line input (Tugas 93.3).

```go
// internal/repl/input.go (baru)
package repl

import "github.com/chzyer/readline"

func newLineReader(historyFile string) (*readline.Instance, error) {
	return readline.NewEx(&readline.Config{
		Prompt:          "\033[36m❯\033[0m ",
		HistoryFile:     historyFile, // ~/.luna/history — persist antar sesi juga, bonus
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
	})
}
```
Ganti pemanggilan `bufio.NewScanner(r.in)` di `repl.go:308` dan `bufio.NewReader(r.in)` di `repl.go:531` dengan instance ini **hanya saat `r.in == nil` (mode interaktif nyata)** — pertahankan jalur `bufio.Scanner` yang ada sebagai fallback untuk mode test/non-tty (`r.in` di-inject manual di test, `readline` butuh tty asli) supaya `root_test.go`/`session_repl_test.go` yang sudah ada tidak perlu dibongkar.

**Tugas 93.2 — Interrupt ganda (Ctrl+C sekali = stop generation, dua kali cepat = exit)**

`llmclient/streaming.go` sudah menangani `ctx.Done()` dengan benar (dikonfirmasi audit sebelumnya) — yang belum ada adalah REPL layer membedakan dua jenis Ctrl+C. Pola standar:
```go
// di REPL, saat streaming aktif:
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, os.Interrupt)
var lastInterrupt time.Time
go func() {
	for range sigCh {
		if time.Since(lastInterrupt) < 2*time.Second {
			os.Exit(130) // Ctrl+C ganda cepat = exit beneran
		}
		lastInterrupt = time.Now()
		cancelGeneration() // fungsi cancel context streaming yang sedang jalan
	}
}()
```

**Tugas 93.3 — Multi-line input**

Dengan `readline` sudah terpasang (Tugas 93.1), ini jadi murah: deteksi baris yang diakhiri `\` (lanjut baris eksplisit, familiar dari shell) atau heuristik "kurung/backtick belum ditutup" untuk paste blok kode otomatis. Mulai dari yang eksplisit (`\` di akhir baris) — lebih sederhana dan tidak ambigu, cukup untuk v0.1.

**Tugas 93.4 — Image/file attachment (opsional, boleh sesi terpisah)**

Ini butuh perubahan di `llmclient` untuk membangun content block multimodal (bukan cuma REPL layer) — cek dulu apakah `internal/llmclient/payload.go` sudah punya struktur yang mendukung `image` content type untuk provider Anthropic/Gemini. Kalau belum, **jangan gabung ke sesi ini** — pisahkan jadi `SESSION-93b` supaya SESSION-93 tetap fokus dan bisa diverifikasi cepat.

**Acceptance criteria SESSION-93:** REPL interaktif nyata (bukan `go test`) punya riwayat command panah atas/bawah, Ctrl+C tunggal menghentikan generation tanpa exit app, dan `go build ./... && go test ./...` tetap hijau (test lama tidak rusak karena fallback non-tty dipertahankan).

---

### SESSION-94 — P1: MCP remote transport (HTTP/SSE)

Melanjutkan `SESSION-84` lama yang secara sadar cuma cakup stdio. Sekarang stdio sudah terbukti jalan nyata di kode, jadi lebih aman menambah transport kedua tanpa mengubah fondasi.

**Tugas 94.1** — Tambahkan `internal/mcp/http_client.go` dengan `Client` yang implement interface transport yang sama dengan `client.go` yang ada sekarang (butuh sedikit refactor: ekstrak interface `Transport` dari `Client` stdio yang sekarang, supaya `Manager` bisa memilih stdio vs HTTP berdasarkan config tanpa tahu detail transport).

```go
// internal/mcp/transport.go (baru)
type Transport interface {
	Send(ctx context.Context, msg *JSONRPCMessage) error
	Receive(ctx context.Context) (*JSONRPCMessage, error)
	Close() error
}
```
`Client` stdio yang sekarang di-refactor supaya `stdin`/`stdout` pipe-nya diakses lewat `Transport` ini, lalu `HTTPTransport` baru pakai `net/http` dengan `Content-Type: text/event-stream` untuk server-sent events (pola standar MCP remote per spesifikasi resmi — cek `modelcontextprotocol.io` untuk detail handshake terbaru sebelum implementasi, karena spec remote MCP masih berkembang aktif per pertengahan 2026).

**Tugas 94.2** — Perluas `.luna/mcp.json` untuk mendukung dua bentuk:
```json
{
  "mcpServers": {
    "filesystem-extra": { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/path"] },
    "team-tools": { "url": "https://mcp.example.com/sse", "headers": { "Authorization": "Bearer ${TEAM_MCP_TOKEN}" } }
  }
}
```
Field `command` → stdio, field `url` → HTTP/SSE. Deteksi otomatis dari config, tanpa flag terpisah.

**Acceptance criteria SESSION-94:** connect ke minimal satu MCP server remote referensi (bisa pakai server MCP publik sederhana untuk test, atau spin up server lokal dengan `net/http` sebagai bagian dari test integrasi), tool-nya muncul di `Manifest()`, permission gate tetap wajib (sama seperti kriteria stdio di dokumen lama — **jangan** buat jalur pintas untuk MCP remote).

---

### SESSION-95 — P2: Onboarding tanpa API key manual (satu provider dulu)

Gap ini nyata (audit §3/§4) tapi implementasinya paling kompleks dari semua rekomendasi — OAuth device-code flow butuh koordinasi dengan provider (client ID terdaftar, redirect handling, dst.), jadi taruh di P2 bukan P1, dan **jangan coba semua provider sekaligus**.

**Tugas 95.1** — Mulai dari **OpenRouter** (sudah jadi provider bawaan, dan OpenRouter API mendukung OAuth PKCE flow tanpa perlu backend server Anda sendiri — cek dokumentasi OpenRouter OAuth terbaru sebelum implementasi, ini area yang API-nya bisa berubah):
```
luna auth login openrouter
# → buka browser ke halaman otorisasi OpenRouter
# → user approve
# → callback ke localhost:PORT sementara, tangkap code, tukar jadi API key
# → simpan ke ~/.luna/credentials.json (permission 0600, path yang sudah dipakai secrets.go)
```
Ini satu sub-command baru (`cmd/luna/commands/auth.go`), tidak menyentuh arsitektur inti — aman dikerjakan tanpa mengganggu sesi lain.

**Acceptance criteria SESSION-95:** `luna auth login openrouter` berhasil dapat token nyata dan `luna` bisa langsung dipakai setelahnya tanpa `export OPENROUTER_API_KEY` manual. Provider lain (Anthropic, Gemini) jadi item roadmap lanjutan **setelah** pola ini terbukti jalan untuk satu provider — jangan bangun infrastruktur generik untuk semua provider sekaligus sebelum tahu polanya benar.

---

### SESSION-96 — P2: Sisa item dokumentasi & operasional dari roadmap lama

Konsolidasi item yang statusnya "belum" di tabel Bagian 1 tapi tidak butuh keputusan desain baru — murni eksekusi dari spesifikasi yang **sudah** ditulis lengkap di `docs/ROADMAP_luna-go_distribusi.md`:
- `SESSION-79` lama (YAML frontmatter asli, pakai `gopkg.in/yaml.v3` — satu-satunya dependency baru yang layak ditambah, dampaknya besar untuk maintainability parser subagent).
- `SESSION-89` lama (`docs/user/` terpisah).
- `SESSION-90.2/90.3` sisanya (goreleaser-on-tag job di CI).

Saya tidak menulis ulang detail ketiganya — dokumen lama sudah cukup lengkap untuk langsung dieksekusi tanpa keputusan tambahan.

---

## Ringkasan urutan eksekusi

```
SESSION-91  P0  Fix panic fatal buildDispatcher()          ← WAJIB PERTAMA, blocker mutlak
SESSION-92  P0  Tutup verifikasi rilis menggantung (81-83, 90.1-90.2)
SESSION-93  P1  Fondasi TUI (readline + interrupt ganda + multi-line)  ← gap kompetitif terbesar
SESSION-94  P1  MCP remote transport (HTTP/SSE)
SESSION-95  P2  OAuth login (OpenRouter dulu)
SESSION-96  P2  Sisa dokumentasi/operasional (79, 89, 90.3 lama)
```

**Kalau harus pilih satu untuk dikerjakan hari ini juga:** SESSION-91. Semua yang lain — termasuk seluruh rencana kompetitif yang bagus di dokumen lama — tidak relevan selama binary-nya sendiri crash di `--help`.
