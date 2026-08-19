# Rendering Contract — `zsh_bagas` UI Output

> Status: **binding** for any command that prints to the terminal, sejak
> SESSION-24 (UX-003, root cause RC-007). Ditulis karena uiux_audit.md §13b/§18
> menemukan 5 gaya renderer independen tanpa kontrak bersama, dan Design
> System Coverage cuma ~20/100 (§18.1) — command frekuensi-tinggi seperti
> `aiplan`/`aicode`/`aifix` masih `echo`/`printf` polos (§18.2).

## 1. Tiga properti wajib

Command apa pun yang cetak output ke terminal (bukan cuma jalur `aiagent`)
**harus** memenuhi 3 properti ini:

1. **Warna via `AI_C_*` token saja** — jangan pernah tulis kode ANSI mentah
   (`\033[...m`, `\e[...m`) langsung di command/component manapun kecuali di
   `30-ai/60-ui/02-ui_colors.zsh` sendiri (satu-satunya tempat token
   didefinisikan). Semua warna lain WAJIB pakai `$AI_C_OK`, `$AI_C_ERR`,
   `$AI_C_WARN`, `$AI_C_INFO`, `$AI_C_MUTED`, `$AI_C_ACCENT`, `$AI_C_BOLD`,
   `$AI_C_RESET`, dst — token-token ini otomatis jadi string kosong kalau
   warna nonaktif (lihat poin 3), jadi kode pemanggil gak perlu cek kondisi
   sendiri.
2. **Unicode + ASCII fallback** — karakter box-drawing/icon unicode
   (`─│┌┐└┘✓✗●→◌•↻`) WAJIB ada padanan ASCII (`-|+++...+xno*~`) yang dipilih
   lewat `_ai_ui_supports_unicode` (`60-ui/00-ui_text.zsh`), bukan
   diasumsikan selalu didukung terminal (banyak device Termux/locale non-
   UTF-8 gak bisa render dengan benar). Override manual via
   `AI_UI_ASCII_FALLBACK=1`.
3. **`NO_COLOR` / `AI_UI_NO_COLOR` compliance** — kepatuhan ini otomatis
   didapat GRATIS kalau poin 1 dipatuhi: `_ai_ui_colors_init`
   (`02-ui_colors.zsh`) sudah membaca `NO_COLOR`/`AI_UI_NO_COLOR` dan
   mengosongkan semua `AI_C_*` kalau salah satunya aktif (atau stdout bukan
   tty, atau `TERM=dumb`). Command yang menulis ANSI mentah sendiri
   (melanggar poin 1) otomatis JUGA melanggar poin ini — dua pelanggaran
   dari satu akar penyebab yang sama.

## 2. Helper resmi yang WAJIB dipakai (bukan re-implementasi sendiri)

| Kebutuhan | Helper | Lokasi |
|---|---|---|
| Satu baris status ber-icon (`→ ✓ ✗ ◌ •`) | `_ai_ui_line icon text` | `60-ui/05-ui_box.zsh` |
| Blok judul + isi (approval = box border, non-approval = judul+isi rata kiri) | `_ai_ui_box title line...` | `60-ui/05-ui_box.zsh` |
| Card ringkasan siap pakai di atas `_ai_ui_box` | `ui_card_summary title content` | `60-ui/components/cards.zsh` |
| Header/footer diff yang di-review user | `_ai_ui_diff_header path`, `_ai_ui_diff_footer` | `60-ui/06-ui_diff.zsh` (baru, SESSION-24) |
| State agent-loop (`Thinking→Acting→Waiting→Done`) | `_ai_state_thinking/_sending/_acting/_waiting/_done/_error` | `60-ui/components/state.zsh` |
| Deteksi unicode/lebar terminal/word-wrap | `_ai_ui_supports_unicode`, `_ai_ui_width`, `_ai_ui_wrap` | `60-ui/00-ui_text.zsh` |

Command baru **tidak boleh** bikin gaya box/warna/icon ketiga — pakai salah
satu dari tabel di atas atau perluas helper yang ada (bukan duplikasi baru).

## 3. Command yang sudah migrasi (per sesi)

| Command | Sesi migrasi | Catatan |
|---|---|---|
| `aiagent` (loop utama) | pra-SESSION-24 (`fase1_ui_ux_overhaul`) | Referensi pola box/state — lihat `50-agent/20-presentation/`, `50-agent/44-finalize.zsh` |
| `aiplan` | SESSION-24 | Status line via `_ai_ui_line`, hasil rencana via `_ai_ui_box` |
| `aicode` / `aifix` | SESSION-24 | Status line via `_ai_ui_line`, header/footer diff via `_ai_ui_diff_header`/`_ai_ui_diff_footer` |
| `aicode -o` diff body | SESSION-25 | Body diff +/- via `AI_C_ERR`/`AI_C_OK`/`AI_C_RESET` (`30-code/05-code.zsh`) |
| `aipatch` diff body | SESSION-25 | Body diff +/- via `AI_C_ERR`/`AI_C_OK`/`AI_C_RESET` (`35-files/10-aipatch.zsh`) |

## 4. Known debt (didokumentasikan, bukan pelanggaran diam-diam)

Body diff berwarna (`diff -u ... | sed -e "s/^-/$(printf '\033[31m')-/" ...`)
di `30-code/45-fix.zsh` (`_ai_fix_apply`, dipakai `aifix`+`airun`) **masih**
pakai kode ANSI mentah, BUKAN `AI_C_*`. Ini adalah sisa UX-004: SESSION-25
("Route diff colorizer through AI_C_* theme and NO_COLOR compliance") migrasi
`30-code/05-code.zsh` (`aicode -o`) dan `35-files/10-aipatch.zsh` (`aipatch`)
ke `AI_C_ERR`/`AI_C_OK`/`AI_C_RESET`, tapi `30-code/45-fix.zsh` secara
eksplisit di-scope keluar (lihat `boundary_rationale` di
`25_route_diff_colorizer_through_ai_c.yaml` — affected_files sesi itu cuma 2
file di atas) dan perlu session lanjutan tersendiri. Lint rule (§5) tahu 1
file ini lewat allowlist eksplisit — bukan celah yang gak kedeteksi, tapi
utang yang dilacak dan dijadwalkan.

## 5. Consolidation note (SESSION-24)

`60-ui/theme.zsh` defined a **second, competing** color-token system
(`UI_C_*` / `ui_color()`) with raw `\e[...]` escapes and **no** `NO_COLOR`/
`AI_UI_NO_COLOR`/tty detection at all — unlike `02-ui_colors.zsh`'s
`_ai_ui_colors_init`. A repo-wide grep confirmed **zero callers** of
`ui_color()` or any `UI_C_*`/`UI_BG_*`/`UI_FG_*` variable anywhere else in
the codebase (dead code, never wired to any command). Keeping a second,
non-compliant, un-vetted token system sitting in `60-ui/` would undermine
the "one minimal rendering contract" this document defines, so the file was
deleted in this session rather than left as silent debt — same precedent as
SESSION-04's removal of the dead duplicate `ui_palette()` (RC-019).

## 6. Lint rule

`source/tests/lint_hardcoded_ansi.zsh` men-scan seluruh `.zsh_bagas/**/*.zsh`
cari pola `\033[`, `\e[`, atau escape byte `0x1b[` mentah. Satu-satunya file
yang boleh berisi pola ini tanpa flag adalah `02-ui_colors.zsh` sendiri, plus
1 file "known debt" di §4 (allowlist eksplisit, dikomentari alasannya). File
LAIN manapun yang lolos deteksi (baru atau existing) yang
BUKAN di allowlist membuat lint gagal (exit 1) — ini yang memenuhi AC-03
("no newly-merged command uses hardcoded ANSI ... without going through the
contract").

Jalankan: `zsh source/tests/lint_hardcoded_ansi.zsh`
Self-test (memverifikasi lint benar-benar mendeteksi pelanggaran baru):
`zsh source/tests/test_lint_hardcoded_ansi.zsh`
