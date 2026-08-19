// Traceability: 30-luna/60-ui/00-command_registry.zsh -> registry.go.
//
// Single source of truth for command name/category/description, exactly
// mirroring _AI_COMMAND_REGISTRY. Subcommands() below is the derived
// equivalent of _AI_SUBCOMMANDS (RC-008/UX-007's fix: one array, every
// consumer derives from it — dispatcher handler table in router.go,
// palette options in screens/palette.go, "luna h" style category render
// here).
package ui

import (
	"fmt"
	"strings"
)

// CommandEntry is the Go equivalent of one "name|category|description"
// line in _AI_COMMAND_REGISTRY.
type CommandEntry struct {
	Name        string
	Category    string
	Description string
}

// CommandRegistry is the port of _AI_COMMAND_REGISTRY. Order matches the
// zsh source exactly (definition order = display order within a
// category).
var CommandRegistry = []CommandEntry{
	// --- Chat ---
	{"chat", "Chat", "Chat cepat, model kelas fast, jawaban streaming"},
	{"long", "Chat", "Chat model kelas smart (lebih pintar, lebih lambat)"},
	{"ask", "Chat", "Tanya-jawab tunggal, pakai cache (pertanyaan identik gak manggil API lagi)"},
	{"shell", "Chat", "Minta perintah shell/Termux yang aman & langsung bisa dijalankan"},
	{"clip", "Chat", "Kirim isi clipboard ke LUNA"},
	{"session", "Chat", "Sesi chat multi-turn yang tersimpan (start/end/list/resume/prune)"},
	// --- Code ---
	{"code", "Code", "Generate file kode baru dari nol"},
	{"edit", "Code", "Edit file yang sudah ada lewat diff/confirm terpandu"},
	{"view", "Code", "Lihat isi file per-baris"},
	{"fix", "Code", "Perbaiki file dari pesan error"},
	{"run", "Code", "Jalankan Python, auto-fix kalau error (sampai 2x)"},
	{"commit", "Code", "Generate pesan commit dari git diff staged"},
	{"review", "Code", "Review diff/perubahan terakhir (read-only, gak modifikasi file)"},
	{"scrap", "Code", "Scraping/riset cepat lalu rangkum"},
	// --- Files ---
	{"undo", "Files", "Restore dari backup .bak.* terbaru"},
	{"bakclean", "Files", "Bersihin backup lebih tua dari N hari (default 14)"},
	{"share", "Files", "Share file lewat share-sheet Android"},
	{"scan", "Files", "Scan ulang ringkasan project"},
	{"index", "Files", "Bikin/lihat index codebase (functions/classes)"},
	// --- Workflow ---
	{"plan", "Workflow", "Bikin rencana kerja langkah demi langkah"},
	{"prompt", "Workflow", "Generate prompt siap-pakai buat LUNA lain"},
	{"spec", "Workflow", "Generate spesifikasi teknis aplikasi"},
	{"summarize", "Workflow", "Ringkas isi file atau halaman web"},
	// --- Project ---
	{"project", "Project", "Generate project multi-file dari nol"},
	{"build", "Project", "Mirip project, alur lebih terpandu"},
	// --- Agent ---
	{"agent", "Agent", "Agent full akses: baca/tulis file, jalankan command, looping sendiri"},
	{"debug", "Agent", "Diagnosis + test/command; read-only, gak ada auto-fix"},
	{"research", "Agent", "Riset/inspeksi codebase standalone, read-only"},
	{"delegate", "Agent", "Standalone coder subagent; permission existing dapat menulis file"},
	// --- Utility ---
	{"stats", "Utility", "Statistik pemakaian token"},
	{"log", "Utility", "History chat/perintah lewat fzf (nama kanonik: aihist)"},
	{"menu", "Utility", "Buka LUNA Workspace (sama seperti luna tanpa argumen)"},
	{"deps", "Utility", "Cek semua dependency & konfigurasi"},
	{"dev", "Utility", "Tools development toolkit ini sendiri (workspace tmux)"},
	{"testmodels", "Utility", "Test konektivitas ke semua provider"},
	{"update", "Utility", "Update toolkit ini sendiri (git pull, minta konfirmasi)"},
	{"h", "Utility", "Bantuan ringkas semua subcommand (teks ini)"},
}

// CommandCategories is the port of _AI_COMMAND_CATEGORIES.
var CommandCategories = []string{"Chat", "Code", "Files", "Workflow", "Project", "Agent", "Utility"}

// RegistryNames is the port of _ai_registry_names().
func RegistryNames() []string {
	names := make([]string, len(CommandRegistry))
	for i, e := range CommandRegistry {
		names[i] = e.Name
	}
	return names
}

// RegistryDescription is the port of _ai_registry_description(name).
func RegistryDescription(name string) (string, bool) {
	for _, e := range CommandRegistry {
		if e.Name == name {
			return e.Description, true
		}
	}
	return "", false
}

// RegistryRenderCategorized is the port of
// _ai_registry_render_categorized(): grouped listing, used by `luna h`.
func RegistryRenderCategorized() string {
	var b strings.Builder
	for _, cat := range CommandCategories {
		b.WriteString(cat + ":\n")
		for _, e := range CommandRegistry {
			if e.Category == cat {
				fmt.Fprintf(&b, "  %-12s %s\n", e.Name, e.Description)
			}
		}
	}
	return b.String()
}

// RegistryFlatList is the port of _ai_registry_flat_list(): flat
// "name<pad>description" listing, one command per line, used by the
// Command Palette.
func RegistryFlatList() string {
	var b strings.Builder
	for _, e := range CommandRegistry {
		fmt.Fprintf(&b, "%-14s%s\n", e.Name, e.Description)
	}
	return b.String()
}

// Subcommands is the port of the derived _AI_SUBCOMMANDS array.
// Single source of truth: don't hand-maintain a separate list anywhere
// else — derive from CommandRegistry, exactly like the zsh version.
func Subcommands() []string {
	return RegistryNames()
}
