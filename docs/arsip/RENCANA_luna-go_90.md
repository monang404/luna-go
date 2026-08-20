# Rencana Implementasi — `luna-go` menuju Skor Kesiapan 90/100

**Baseline saat ini:** 35/100 (build gagal total di HEAD)
**Target:** 90/100, siap dogfooding penuh
**Format:** Mengikuti metodologi SESSION Anda yang sudah ada (audit → YAML plan → eksekusi bertahap → verifikasi). Saya nomori lanjutan dari SESSION-75 (migrasi terakhir yang sudah selesai) sebagai **SESSION-76 s/d 80**.

Prinsip pengurutan: **P0 dulu** (apa pun yang membuat `go build` gagal — tidak ada gunanya membenahi apa pun lain kalau binary-nya sendiri tidak bisa dikompilasi), baru P1 (bug fungsional/keamanan nyata), baru P2 (hardening + cakupan test). Setiap sesi punya *acceptance criteria* yang bisa diverifikasi otomatis (`go build`, `go vet`, `go test`, atau test baru yang saya spesifikasikan) — bukan "kelihatannya sudah benar".

---

## SESSION-76 — P0: Perbaiki build yang gagal total

**Kenapa ini duluan:** tidak ada satu pun sesi lain yang bisa diverifikasi (`go build`/`go test`) selama repo tidak bisa dikompilasi. Ini blocker mutlak.

### Tugas 76.1 — Perbaiki `WebSearchTool` supaya implement `tools.Tool`
**File:** `internal/tools/websearch.go`

Masalah: `Execute` bertanda tangan `(string, error)`, interface `Tool` butuh `(Result, error)`; juga tidak ada `Capability()`.

```go
// internal/tools/websearch.go
func (t *WebSearchTool) Capability() permission.Capability {
	return permission.CapNetworkPublic // sama seperti web_fetch — akses jaringan readonly ke domain publik
}

func (t *WebSearchTool) Execute(ctx context.Context, args json.RawMessage) (Result, error) {
	// ... body sama seperti sekarang, tapi setiap `return "...", err`
	// diganti `return Result{}, err`, dan return sukses di akhir jadi:
	return Result{Output: fmt.Sprintf("=== Hasil Pencarian Web untuk '%s' ===\n%s", query, text)}, nil
}
```
Cek juga `Registry["web_search"]` di `registry.go` — pastikan `Capability` di sana **sama persis** dengan yang dikembalikan `WebSearchTool.Capability()` (kalau beda, `RegisterFromRegistry` bukan yang dipakai di `app.go` jadi tidak auto-tervalidasi — cek manual).

### Tugas 76.2 — Perbaiki wiring `deps` yang tidak terdefinisi di `buildDispatcher()`
**File:** `cmd/luna/commands/app.go:114`

`WebSearchTool` butuh `PermDeps` (untuk request HTTP dengan konteks permission), tapi `buildDispatcher()` tidak punya akses ke `App`'s `PermDeps` di titik itu — ini kemungkinan kenapa sesi sebelumnya menulis `deps` mentah tanpa mendefinisikannya (placeholder yang lupa diselesaikan).

Dua opsi, pilih salah satu (saya sarankan opsi B, konsisten dengan pola `ConfigureDelegateTask` yang sudah ada untuk `DelegateTaskTool`):

**Opsi A — construct langsung tanpa Deps** (kalau `WebSearchTool` sebenarnya tidak butuh `PermDeps` untuk apa pun — cek isi struct-nya, field `Deps PermDeps` di `websearch.go:15` sekarang tidak dipakai sama sekali di `Execute`):
```go
register("web_search", &tools.WebSearchTool{})
```
Kalau field `Deps` memang tidak pernah dibaca di `Execute`, **hapus field itu sekalian** — dead field yang menyesatkan pembaca kode berikutnya.

**Opsi B — inject setelah registrasi**, kalau `Deps` memang dipakai (mis. untuk baca `Cwd`/`AgentCtx` demi rate-limiting per-session ke depannya):
```go
func (d *Dispatcher) ConfigureWebSearch(deps PermDeps) {
	d.mu.Lock()
	defer d.mu.Unlock()
	for _, reg := range d.tools {
		if ws, ok := reg.tool.(*WebSearchTool); ok {
			ws.Deps = deps
		}
	}
}
```
lalu di `NewApp()` setelah `buildDispatcher()`, panggil `dispatcher.ConfigureWebSearch(PermDeps{...})` — sama persis pola `dispatcher.ConfigureDelegateTask(...)` yang sudah ada.

### Tugas 76.3 — Verifikasi
```bash
cd go
go build ./...      # HARUS nol output
go vet ./...         # HARUS nol output
```
**Acceptance criteria SESSION-76:** `go build ./...` sukses tanpa modifikasi apa pun selain dua tugas di atas. Ini gerbang wajib sebelum lanjut ke SESSION-77.

---

## SESSION-77 — P0: Perbaiki test suite yang merah + endpoint provider salah

Digabung jadi satu sesi karena akar masalahnya berdekatan (package `internal/config`) dan saling mempengaruhi keputusan desain.

### Tugas 77.1 — Putuskan `ProviderOrder` yang benar, lalu selaraskan kode+test
**File:** `internal/config/providers.go:76`, `internal/config/config_test.go:76-88`

Ini keputusan produk, bukan cuma bug — Anda perlu memutuskan salah satu:
- **(a)** `ProviderOrder` **memang seharusnya** `[groq, gemini, cerebras]` (parity murni dengan `35-providers.zsh`) → revert kode ke daftar itu, test tetap seperti sekarang.
- **(b)** `ProviderOrder` **sengaja** diperluas untuk memasukkan `anthropic`/`openrouter` sebagai fallback (mis. karena provider-provider itu lebih reliable untuk fallback umum, beda dari `TaskProviderOrder*` yang task-specific) → update test `TestProviderOrder_ExcludesDeepseek` supaya `want` sesuai daftar baru, dan **ubah nama test-nya** (nama sekarang menyiratkan parity zsh murni, jadi menyesatkan kalau daftarnya sudah sengaja beda) + tambahkan komentar yang menjelaskan alasan penyimpangan dari sumber, sama seperti pola dokumentasi deviation yang sudah konsisten dipakai di file-file lain (`subagent/run.go`, dsb).

Saya sarankan (b) kalau alasan aslinya memang disengaja — tapi **wajib didokumentasikan**, jangan biarkan test merah dibiarkan begitu saja seperti sekarang.

### Tugas 77.2 — Perbaiki endpoint Cerebras
**File:** `internal/config/providers.go:44`
```go
"cerebras": {
	Endpoint: "https://api.cerebras.ai/v1/chat/completions", // sebelumnya: api.cerebras.luna (domain tidak valid)
	Model:    envOr("CEREBRAS_MODEL", "gpt-oss-120b"),
	KeyVar:   "CEREBRAS_API_KEY",
},
```

### Tugas 77.3 — Tutup celah cakupan test: tambahkan assertion `Endpoint` per provider
**File:** `internal/config/config_test.go`

Tambahkan test baru (ini yang akan mencegah bug seperti 77.2 lolos lagi di masa depan):
```go
func TestProviders_EndpointsAreValidHTTPSHosts(t *testing.T) {
	for name, p := range Providers() {
		u, err := url.Parse(p.Endpoint)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			t.Errorf("provider %q: endpoint %q tidak valid", name, p.Endpoint)
		}
		// opsional tapi disarankan: DNS resolve check sebagai smoke test manual,
		// JANGAN dijadikan bagian dari `go test` reguler (network flaky) —
		// taruh di script terpisah `scripts/check_provider_endpoints.sh`
		// yang dijalankan manual sebelum rilis, bukan di CI otomatis.
	}
}
```

### Tugas 77.4 — Verifikasi
```bash
go test ./... 2>&1 | grep -v '^ok'   # HARUS kosong (semua PASS)
```
**Acceptance criteria SESSION-77:** `go test ./...` 100% PASS di semua package, tanpa skip.

---

## SESSION-78 — P1: Perbaiki risiko deadlock & kebocoran memori di background process

### Tugas 78.1 — Pisahkan pembacaan stdout/stderr jadi goroutine independen
**File:** `internal/tools/bash_output.go`

Ganti pola `io.MultiReader` sekuensial dengan dua goroutine paralel + `sync.WaitGroup`:
```go
func startBackgroundProcess(cmd *exec.Cmd, name string) (Result, error) {
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return Result{}, fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return Result{}, fmt.Errorf("failed to create stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("failed to start background process: %w", err)
	}

	bgMu.Lock()
	bgCounter++
	id := strconv.Itoa(bgCounter)
	bgp := &bgProcess{cmd: cmd}
	bgProcesses[id] = bgp
	bgMu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	pump := func(r io.Reader) {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := r.Read(buf)
			if n > 0 {
				bgp.mu.Lock()
				bgp.buf.Write(buf[:n])
				bgp.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}
	go pump(stdoutPipe)
	go pump(stderrPipe)

	go func() {
		wg.Wait() // tunggu KEDUA pipe EOF, baru Wait() proses
		err := cmd.Wait()
		bgp.mu.Lock()
		bgp.done = true
		bgp.err = err
		bgp.mu.Unlock()
	}()

	return Result{Output: fmt.Sprintf("Process started in background with ID: %s", id)}, nil
}
```
Catatan penting: `cmd.Wait()` **harus** dipanggil setelah kedua pipe selesai dibaca (dokumentasi `os/exec` eksplisit soal ini — memanggil `Wait()` sebelum pipe habis dibaca bisa menyebabkan data hilang/race). Pola `wg.Wait()` sebelum `cmd.Wait()` di atas menjamin urutan itu benar.

### Tugas 78.2 — Bersihkan `bgProcesses` setelah proses selesai & sudah dibaca habis
**File:** `internal/tools/bash_output.go`

Tambahkan TTL-based cleanup. Paling sederhana: hapus entry dari map begitu `BashOutputTool.Execute` melaporkan `done && output kosong` (artinya user/model sudah membaca hasil akhirnya, tidak ada gunanya menyimpan proses itu lagi):
```go
func (BashOutputTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	// ... kode sama sampai bagian ini ...
	if bgp.done {
		status := "selesai (exit 0)"
		if bgp.err != nil {
			status = fmt.Sprintf("gagal (%v)", bgp.err)
		}
		bgMu.Lock()
		delete(bgProcesses, id) // proses sudah selesai & dilaporkan — aman dihapus
		bgMu.Unlock()
		if out == "" {
			return Result{Output: fmt.Sprintf("Proses %s telah %s. Tidak ada output baru.", id, status)}, nil
		}
		return Result{Output: fmt.Sprintf("Proses %s telah %s.\nOutput akhir:\n%s", id, status, out)}, nil
	}
	// ...
}
```
Untuk proses yang **tidak pernah** di-poll ulang oleh model (di-abandon begitu saja), tambahkan juga *reaper* sederhana: goroutine background di level App yang setiap N menit membuang entry dengan `done == true` dan umur > 5 menit, sebagai jaring pengaman kedua di luar path `bash_output`.

### Tugas 78.3 — `KillShellTool`: tambahkan fallback SIGKILL
**File:** `internal/tools/process.go` (`KillShellTool.Execute`)
```go
err := bgp.cmd.Process.Signal(syscall.SIGTERM)
if err != nil {
	return Result{}, fmt.Errorf("ERROR: gagal mengirim SIGTERM ke proses %s: %w", id, err)
}

go func() {
	time.Sleep(3 * time.Second)
	bgp.mu.Lock()
	done := bgp.done
	bgp.mu.Unlock()
	if !done {
		_ = bgp.cmd.Process.Kill() // SIGKILL — proses tidak merespons SIGTERM dalam 3 detik
	}
}()
```

### Tugas 78.4 — Test baru untuk regresi §3.4/§3.5
Tambahkan `internal/tools/bash_output_test.go` (belum ada sebelumnya — ini file baru):
1. **Test anti-deadlock**: spawn proses yang sengaja menulis >64KB ke stderr dan **tidak menulis apa pun ke stdout**, assert `bash_output` tetap bisa membaca output dalam waktu wajar (pakai `context.WithTimeout` di test, `t.Fatal` kalau timeout — ini akan gagal dengan kode lama sebelum 78.1, dan pass setelahnya, jadi ini regression test yang valid).
2. **Test cleanup map**: spawn proses singkat, tunggu selesai, panggil `bash_output` sampai `done`, assert `len(bgProcesses) == 0` (butuh expose helper test-only atau taruh test di package yang sama untuk akses `bgProcesses` langsung).

**Acceptance criteria SESSION-78:** kedua test baru di atas PASS, dan `go test -race ./internal/tools/...` bersih (tidak ada data race terdeteksi pada `bgp.buf`/`bgp.done`).

---

## SESSION-79 — P1/P2: Perbaiki YAML frontmatter subagent + hardening kecil

### Tugas 79.1 — Ganti parser frontmatter manual dengan parser yang benar
**File:** `internal/subagent/loader.go`

Dua pilihan:
- **(a)** Vendor `gopkg.in/yaml.v3` sungguhan (sesuai asumsi awal desain), parse frontmatter sebagai YAML asli. Ini paling robust tapi menambah dependency pertama di luar cobra/pflag — pertimbangkan apakah sepadan untuk 2 field sederhana.
- **(b)** **Tetap tanpa dependency**, tapi perbaiki bug konkretnya: gunakan pemisah frontmatter yang benar (baris `---` **persis di awal baris**, bukan `SplitN` naif pada seluruh isi), supaya body markdown yang mengandung horizontal rule (`---`) tidak salah terpotong:

```go
func parseDefinition(role Role, content string) *Definition {
	def := &Definition{Role: role, Tools: []string{}}
	content = strings.TrimSpace(content)

	if !strings.HasPrefix(content, "---") {
		def.System = content
		return def
	}

	lines := strings.Split(content, "\n")
	// baris 0 harus persis "---"; cari baris "---" PENUTUP berikutnya
	// (bukan sekadar kemunculan substring "---" di mana pun)
	closeIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			closeIdx = i
			break
		}
	}
	if closeIdx == -1 {
		// tidak ada penutup frontmatter yang valid — treat seluruh isi
		// sebagai system prompt, jangan diam-diam membuang isi (fail-safe)
		def.System = content
		return def
	}

	for _, line := range lines[1:closeIdx] {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "description:") {
			def.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		} else if strings.HasPrefix(line, "tools:") {
			toolsStr := strings.Trim(strings.TrimSpace(strings.TrimPrefix(line, "tools:")), "[]")
			for _, t := range strings.Split(toolsStr, ",") {
				if t = strings.TrimSpace(t); t != "" {
					def.Tools = append(def.Tools, t)
				}
			}
		}
	}
	def.System = strings.TrimSpace(strings.Join(lines[closeIdx+1:], "\n"))
	return def
}
```
Saya sarankan **(b)** — konsisten dengan filosofi "dependency ramping" yang sudah jadi kekuatan project ini, dan bug-nya memang di logika pemisahan, bukan di ketiadaan YAML.

### Tugas 79.2 — Tambahkan test regresi untuk kasus `---` di body
```go
func TestParseDefinition_HorizontalRuleInBody(t *testing.T) {
	content := "---\ndescription: test\ntools: read_file\n---\nSystem prompt.\n\n---\n\nBagian setelah horizontal rule harus tetap ada."
	def := parseDefinition("coder", content)
	if !strings.Contains(def.System, "Bagian setelah horizontal rule") {
		t.Errorf("System prompt terpotong: %q", def.System)
	}
}
```

### Tugas 79.3 — Hilangkan state global `registry` di `subagent/loader.go`
Ubah `var registry = make(map[Role]*Definition)` (package-level) jadi field di dalam struct `Loader` yang di-construct per-App, supaya test paralel dan penggunaan multi-instance di masa depan tidak saling menimpa:
```go
type Loader struct {
	mu       sync.RWMutex
	registry map[Role]*Definition
}
func NewLoader() *Loader { return &Loader{registry: make(map[Role]*Definition)} }
func (l *Loader) LoadDefinitions(agentsDir string) error { /* ... */ }
```
Ini refactor kecil tapi menyentuh caller (`app.go`/`agent` package) — lakukan terakhir di sesi ini setelah 79.1/79.2 stabil, dan pastikan `go build ./...` tetap hijau setelahnya.

### Tugas 79.4 — Verifikasi `run_command` exposure default
Konfirmasi eksplisit (sudah saya cek di kode: **sudah benar**) bahwa `AI_AGENT_EXPOSE_ARBITRARY_SHELL` default OFF sehingga `run_command` tidak muncul di `Manifest()` untuk model. Tidak perlu perubahan kode — tambahkan saja **satu test eksplisit** yang mengunci perilaku ini supaya tidak diam-diam berubah di masa depan:
```go
func TestManifest_RunCommandHiddenByDefault(t *testing.T) {
	os.Unsetenv("AI_AGENT_EXPOSE_ARBITRARY_SHELL")
	if strings.Contains(Manifest(), "run_command") {
		t.Error("run_command tidak boleh muncul di manifest tanpa AI_AGENT_EXPOSE_ARBITRARY_SHELL=1")
	}
}
```

**Acceptance criteria SESSION-79:** test baru di 79.2/79.4 PASS, `go build ./...` tetap hijau setelah refactor 79.3.

---

## SESSION-80 — P2: Verifikasi akhir + gerbang rilis

Sesi terakhir murni verifikasi menyeluruh, tidak ada kode baru kecuali yang ketinggalan dari sesi 76-79.

### Checklist gerbang rilis (jalankan berurutan, semua harus hijau)
```bash
cd go

# 1. Build bersih dari nol (simulasikan clone fresh)
go clean -cache
go build ./...

# 2. Vet bersih
go vet ./...

# 3. Seluruh test PASS, termasuk race detector di package yang menyentuh
#    goroutine/shared state (tools, llmclient, subagent, permission)
go test ./...
go test -race ./internal/tools/... ./internal/llmclient/... ./internal/subagent/... ./internal/permission/...

# 4. Smoke test endpoint provider (manual, sebelum rilis — bukan bagian CI)
#    Pastikan kelima domain resolve DNS-nya:
for h in api.groq.com generativelanguage.googleapis.com api.cerebras.ai api.deepseek.com api.anthropic.com openrouter.ai; do
  getent hosts "$h" >/dev/null && echo "OK  $h" || echo "FAIL $h"
done

# 5. Binary benar-benar jalan end-to-end minimal
go build -o /tmp/luna ./cmd/luna
/tmp/luna --help
echo "test prompt" | /tmp/luna -p "echo hello"   # headless mode, verifikasi manual output masuk akal
```

### Tugas 80.1 — Tambahkan CI check yang menolak regresi di masa depan
Kalau belum ada, tambahkan `.github/workflows/ci.yml` (atau setara di CI yang Anda pakai) yang menjalankan persis langkah 1-3 di atas pada setiap push — supaya bug seperti SESSION-76 (build gagal masuk ke HEAD tanpa terdeteksi) tidak terulang. Ini investasi kecil dengan ROI besar mengingat pola kerja Anda (banyak sesi berturut-turut, gampang satu commit lolos tanpa full-build check manual).

**Acceptance criteria SESSION-80 = acceptance criteria rilis:** kelima langkah checklist di atas hijau semua, plus CI workflow baru ada dan lolos di commit terakhir.

---

## Ringkasan dampak skor per sesi

| Sesi | Menutup temuan | Estimasi skor setelah sesi |
|---|---|---|
| Baseline | — | 35/100 |
| SESSION-76 | §3.1 (build gagal) | 55/100 — binary akhirnya bisa dikompilasi & dijalankan |
| SESSION-77 | §3.2, §3.3 (endpoint salah, test merah) | 70/100 — CI hijau, provider fallback benar |
| SESSION-78 | §3.4, §3.5 (deadlock, memory leak) | 80/100 — shell/process tool aman untuk sesi panjang |
| SESSION-79 | §4 (frontmatter parser, global state) | 87/100 — hardening menengah selesai |
| SESSION-80 | verifikasi + CI gate | **90/100** — siap dogfooding, dengan jaring pengaman supaya tidak regresi lagi |

Total pekerjaan realistis: **3-5 sesi kerja fokus** (bukan 75 sesi lagi) — karena fondasi arsitekturnya sudah kuat, ini murni soal menutup celah integrasi terakhir yang saya temukan di audit, bukan menulis ulang apa pun dari nol.
