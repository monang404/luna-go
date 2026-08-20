# Luna-Go 🚀

Luna-Go adalah **multi-provider agentic CLI dengan permission/path-traversal model paling ketat di kelasnya, tanpa lock-in ke satu vendor LLM.** 

Dirancang sebagai pesaing langsung Claude Code dan Gemini CLI, Luna-Go memadukan keamanan sandbox dengan fleksibilitas untuk berganti provider AI on-the-fly, memastikan Anda memegang kendali penuh atas file mana yang bisa disentuh oleh agen AI.

![Luna-Go Demo](docs/placeholder_demo.gif)

## Kenapa Luna-Go?
- **Multi-Provider LLM**: Mendukung Anthropic, OpenRouter, Groq, Gemini, dan Cerebras.
- **Robust Permission System**: Setiap *tool* (filesystem, eksekusi shell) wajib melalui _permission gate_. Tidak ada `rm -rf /` di luar batas project Anda tanpa approval.
- **Subagent & Delegation**: Dapat membuat sub-agent otonom sesuai role yang didefinisikan.
- **Session Resume & Memory**: Bisa *rewind* sesi dan _resume_ kapanpun.
- **Zero-config Web Search**: Terintegrasi *DuckDuckGo Lite Scraper* secara native.

## Instalasi

Gunakan script instalasi ini (MacOS/Linux):

```bash
curl -fsSL https://raw.githubusercontent.com/monang404/luna-go/main/install.sh | sh
```

Atau unduh langsung file biner untuk Windows dari halaman **[Releases](https://github.com/monang404/luna-go/releases)**.

## Quickstart

Setelah diinstal, atur API Key untuk provider yang ingin Anda gunakan (mis. Anthropic):

```bash
export ANTHROPIC_API_KEY="sk-ant-..."
```

Lalu, jalankan Luna:

```bash
luna "Buatkan saya fungsi Golang untuk menghitung deret Fibonacci, lalu buat unit testnya."
```

## Konfigurasi

Semua konfigurasi bisa diatur dari file `.luna/settings.json` di *root* project atau `~/.luna/config.yaml` untuk level *global*. Luna juga dapat membaca *system prompt* kustom dari file `LUNA.md`.

Informasi selengkapnya mengenai konfigurasi dapat Anda baca di dokumentasi terpisah di direktori `docs/user/` (segera hadir).

## Security & Privacy

**Tidak Ada Telemetri**: 
Luna-Go **tidak mengumpulkan atau mengirimkan data telemetry apapun** ke server kami atau pihak ketiga selain data yang dikirimkan ke provider LLM yang Anda pilih. 

Pahami kebijakan privasi dari provider LLM Anda masing-masing (mis. Anthropic atau Google). 

Silakan merujuk ke **[SECURITY.md](SECURITY.md)** untuk kebijakan pelaporan *vulnerability* dan model keamanannya.

## Kontribusi
Laporan *bug* atau *feature requests* sangat diterima. Harap jangan ragu membuka Issue!

## Lisensi
Aplikasi ini dirilis dengan **[MIT License](LICENSE)**.
