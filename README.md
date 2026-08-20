# Luna-Go 🚀

Luna-Go adalah AI Assistant berbasis terminal (CLI) mutakhir yang dirancang sebagai pengganti toolkit agentic lawas (berbasis bash/zsh). Dibangun menggunakan Go, Luna dirancang untuk menghadirkan pengalaman AI-assisted programming yang setara dengan Claude Code CLI.

## Fitur Utama

- **Zero-Config Agentic REPL**: Siklus interaksi LLM lengkap secara lokal dengan integrasi _tools_ otomatis (Filesystem, Shell Execute, Git, dll).
- **Multi-Provider LLM**: Mendukung Anthropic, OpenRouter, Groq, Gemini, dan Cerebras secara dinamis.
- **Robust Permission System**: Eksekusi command dan akses file yang kritis secara otomatis dibekukan dan dilindungi oleh *permission gate* untuk persetujuan pengguna.
- **Subagent & Delegation**: Dapat melahirkan (spawn) sub-agent otonom (mis. role Researcher atau Coder) yang membaca dari definisi YAML.
- **Web Search Natively**: Terintegrasi penuh dengan *DuckDuckGo Lite Scraper* untuk melakukan pencarian *web* tanpa kunci API eksternal (zero-config).
- **Session Resume & Memory**: Menyimpan memori *chat* dan status sesi agar dapat di-_resume_ kapan saja. Dilengkapi command `/rewind` untuk manajemen memori.

## Cara Instalasi & Penggunaan

Pastikan Anda telah menginstal [Go](https://golang.org/dl/) versi terbaru.

```bash
# 1. Clone repository ini
git clone https://github.com/monang404/luna-go.git
cd luna-go/go

# 2. Unduh dependencies (vendored)
go mod tidy
go mod vendor

# 3. Jalankan Luna CLI
go run ./cmd/luna
```

### Slash Commands Utama
Di dalam interaktif REPL, ketikkan perintah berikut:
- `/clear`: Menghapus context chat saat ini.
- `/compact`: Merangkum isi percakapan untuk menghemat _token limit_.
- `/cost`: Menampilkan ringkasan tagihan token API saat ini.
- `/rewind`: Memutar ulang dan menghapus *N* percakapan ke belakang.
- `/resume <id>`: Melanjutkan sesi agen lama yang sebelumnya terputus.

## Struktur Project
- `go/cmd/luna/`: Entrypoint dan konfigurasi framework *Cobra CLI*.
- `go/internal/repl/`: Core mesin iterasi LLM (REPL).
- `go/internal/tools/`: Implementasi fungsionalitas tool lokal (fs, network, shell).
- `go/internal/llmclient/`: Adapter untuk semua provider LLM (Groq, Gemini, Anthropic, dll).
- `go/internal/permission/`: Mesin *gatekeeper* untuk keamanan eksekusi AI.
- `go/internal/subagent/`: Mesin pendelegasian multi-agen.
