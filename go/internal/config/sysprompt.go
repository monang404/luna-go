package config

// Ported from 30-luna/00-config/30-sysprompt_spec.zsh.

// SpecSysprompt is the single source of truth for aispec/aibuild's system
// prompt (previously duplicated verbatim between the two, which risked
// drift -- see 30-sysprompt_spec.zsh's comment).
const SpecSysprompt = `Kamu software architect Python expert. Diberikan deskripsi aplikasi dari user, rancang struktur file aplikasi tsb (boleh 1 file kalau memang sesederhana itu, boleh banyak file kalau perlu dipisah per tanggung jawab). Output HANYA teks di bawah, tanpa markdown/backtick, tanpa penjelasan lain, ikuti header persis:

[APLIKASI] nama aplikasi & ringkasan 2 kalimat.
[FILES] daftar SETIAP file .py yang dibutuhkan, satu baris per file, format persis:
- nama_file.py: tugas spesifiknya apa, fungsi/class utama apa saja (nama + parameter singkat + apa yang dikembalikan), dan file lain di project ini yang dia import (kalau ada).
Kalau lebih dari 1 file, WAJIB ada main.py sebagai entry point yang cuma orkestrasi (manggil fungsi dari file lain), bukan taruh semua logic di situ.
[ALUR] urutan eksekusi program dari user run main.py sampai selesai, singkat per langkah bernomor.
[DEPENDENSI] library eksternal yang dipakai (nama pip install-nya), atau tulis 'tidak ada, cuma stdlib' kalau memang gak butuh.
[CONTOH_INPUT] 3-5 baris contoh nilai input yang bakal dimasukkan user kalau program ini dijalankan dan minta input() secara berurutan (satu nilai per baris, plain, tanpa penjelasan) — dipakai buat smoke-test otomatis. Kalau programnya gak butuh input sama sekali, tulis 'tidak perlu input'.
[EDGE_CASE] input/kondisi tidak wajar yang wajib ditangani (mis. input non-angka, angka negatif, divide by zero, file gak ada, dll).`

// TermuxContext is injected into every aiagent sysprompt (not an optional
// skill) so the agent never hallucinates server-Linux-only commands
// (sudo, systemctl, apt-get) that don't exist on Termux.
const TermuxContext = `KONTEKS WAJIB — kamu jalan di TERMUX (Android), BUKAN server Linux biasa: (1) TIDAK ADA sudo/root sama sekali, JANGAN PERNAH usulin command berawalan 'sudo'. (2) TIDAK ADA systemd/systemctl/service — Termux gak punya init system; proses background pakai nohup/tmux, atau termux-services kalau memang ke-install. (3) Package manager itu 'pkg' (wrapper apt Termux sendiri), JANGAN pakai 'apt-get'/'apt' langsung. (4) HOME sebenarnya /data/data/com.termux/files/home. (5) Storage HP (Download/DCIM/Pictures dst) baru bisa diakses lewat ~/storage/ SETELAH user jalanin 'termux-setup-storage' — jangan asumsikan /sdcard langsung bisa dibaca/ditulis. (6) Gak ada cron bawaan; buat task terjadwal pakai termux-job-scheduler (kalau termux-api ke-install) atau tmux+sleep loop. (7) Baterai & jaringan mobile gampang putus/throttle kalau layar mati — command yang makan waktu lama sebaiknya disaranin jalan di dalam tmux session (aidev/tm), bukan foreground shell biasa yang mati kalau app di-background.`

// ---------------------------------------------------------------------
// Ported from 30-luna/00-config/40-context_engine_docs.zsh.
//
// That file is pure documentation/mapping in zsh -- no variables, no
// functions -- describing 6 levels of a "progressive context engine": start
// from the cheapest context source, escalate a level only when the current
// one isn't enough. It's kept here (rather than a separate file, since this
// session's target_go_files doesn't list one) because it travels with the
// same sysprompt-building step as SpecSysprompt/TermuxContext -- the
// summarized version of this mapping is injected into the agent sysprompt
// by _ai_agent_build_sysprompt (30-luna/50-agent/40-runtime/00-sysprompt.zsh),
// ported later in SESSION-49/50. This Go slice is the fuller
// level-to-data-source reference the zsh comment describes, kept as real
// (if still unused) data now so that later session's sysprompt builder has
// a single source instead of re-copying the mapping into a string.
// ---------------------------------------------------------------------

// ContextEngineLevel documents one escalation level of the progressive
// context engine: which existing tool/data source backs it, and when to
// use it instead of jumping straight to a more expensive level.
type ContextEngineLevel struct {
	Level  int
	Name   string
	Source string
	Desc   string
}

// ContextEngineLevels is the full Level 1-6 mapping from
// 40-context_engine_docs.zsh, in escalation order. None of these levels
// require a tool that doesn't already exist elsewhere in the tool registry
// (internal/tools, SESSION-43/47/48) or project index (SESSION-49+).
var ContextEngineLevels = []ContextEngineLevel{
	{
		Level:  1,
		Name:   "Project metadata",
		Source: "_ai_project_context()",
		Desc:   "Ringkasan/manifest project (bahasa, struktur besar, dependency) — paling murah, paling general, dikirim di awal sesi aiagent.",
	},
	{
		Level:  2,
		Name:   "Directory structure / file discovery",
		Source: "list_dir / glob_search",
		Desc:   "Daftar file/folder, cari file berdasarkan nama/pola. Dipakai kalau Level 1 belum menunjukkan di mana file yang relevan berada.",
	},
	{
		Level:  3,
		Name:   "Relevant file content",
		Source: "read_file",
		Desc:   "Baca isi file penuh. Dipakai kalau sudah tahu file mana yang relevan (dari Level 2) tapi belum tahu bagian mana di dalamnya yang perlu diubah/dibaca.",
	},
	{
		Level:  4,
		Name:   "Relevant symbols",
		Source: "grep_search / index symbol data",
		Desc:   "Cari fungsi/class/simbol spesifik tanpa baca seluruh file. Lebih presisi & lebih murah dari Level 3 kalau yang dicari cuma lokasi satu simbol di file besar.",
	},
	{
		Level:  5,
		Name:   "Exact code region",
		Source: "read_file (offset + limit)",
		Desc:   "Baca cuma range baris tertentu (hasil dari Level 4), bukan file dari baris 1 sampai habis. Sama tool dengan Level 3, dipanggil lebih presisi.",
	},
	{
		Level:  6,
		Name:   "Execution evidence",
		Source: "run_test / run_command",
		Desc:   "Bukti runtime — output test/command aktual. Dipakai paling terakhir, kalau butuh verifikasi nyata (bukan cuma baca kode statis) bahwa perubahan/asumsi benar.",
	},
}
