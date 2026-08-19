package tools

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/monang404/luna-go/internal/permission"
)

// Entry is registry metadata for one tool name, mirroring one
// AI_TOOL_REGISTRY + AI_TOOL_CAPABILITY pair from
// 05-tools/00-tool_registry.zsh: a human-readable description (shown to
// the model in the tool manifest) and the permission.Level/Capability the
// dispatcher checks before Execute is ever called. Deliberately a plain
// data struct, not part of the Tool interface -- the zsh source keeps
// metadata and implementation in separate associative arrays for the same
// reason ("Tool metadata is deliberately separate from implementation"),
// so a not-yet-implemented tool still has a real registry entry.
type Entry struct {
	Description string
	Level       permission.Level
	Capability  permission.Capability
}

// Registry mirrors AI_TOOL_REGISTRY + AI_TOOL_CAPABILITY: the 17 tool
// names known to luna today, with the exact same descriptions
// (kept in the original Indonesian, matching the source verbatim) and
// the exact same readonly/write/process/shell level per name. AC-01
// requires this list match 1:1, by name and category, with the zsh
// source -- see registry_test.go's direct comparison against a literal
// copy of 00-tool_registry.zsh's keys.
//
// run_command is included here like every other entry (the zsh source's
// AI_AGENT_EXPOSE_ARBITRARY_SHELL gate only hides it from the *model-facing
// manifest*, in _ai_tool_manifest -- see Manifest below -- it does not
// remove the registry entry or the tool itself, which stays dispatchable
// by name).
var Registry = map[string]Entry{
	"read_file": {
		Description: "baca isi file (opsional offset/limit baris)",
		Level:       permission.LevelReadonly,
		Capability:  permission.CapFilesystemRead,
	},
	"list_dir": {
		Description: "list isi direktori",
		Level:       permission.LevelReadonly,
		Capability:  permission.CapFilesystemRead,
	},
	"grep_search": {
		Description: "cari pattern regex di project (wrapper rg/grep -rn)",
		Level:       permission.LevelReadonly,
		Capability:  permission.CapFilesystemRead,
	},
	"glob_search": {
		Description: "cari file by nama pattern (wrapper fd)",
		Level:       permission.LevelReadonly,
		Capability:  permission.CapFilesystemRead,
	},
	"count_lines": {
		Description: "hitung jumlah baris file / hitung kemunculan pattern di file",
		Level:       permission.LevelReadonly,
		Capability:  permission.CapFilesystemRead,
	},
	"write_file": {
		Description: "buat file BARU (tolak kalau udah ada)",
		Level:       permission.LevelWrite,
		Capability:  permission.CapFilesystemWrite,
	},
	"edit_file": {
		Description: "ganti blok teks unik di file existing (search-replace)",
		Level:       permission.LevelWrite,
		Capability:  permission.CapFilesystemWrite,
	},
	"patch_file": {
		Description: "apply unified diff (patch -p0) ke file existing",
		Level:       permission.LevelWrite,
		Capability:  permission.CapFilesystemWrite,
	},
	"run_command": {
		Description: "jalanin command shell (legacy compatibility; hidden from model by default)",
		Level:       permission.LevelShell,
		Capability:  permission.CapShellArbitrary,
	},
	"exec_process": {
		Description: "jalankan executable terstruktur tanpa shell interpreter",
		Level:       permission.LevelProcess,
		Capability:  permission.CapProcessExecute,
	},
	"run_test": {
		Description: "jalankan test suite project (typed runner; tanpa shell command string)",
		Level:       permission.LevelProcess,
		Capability:  permission.CapProcessTest,
	},
	"move_file": {
		Description: "pindah/rename file existing ke path baru",
		Level:       permission.LevelWrite,
		Capability:  permission.CapFilesystemWrite,
	},
	"delete_file": {
		Description: "hapus file existing (backup dulu ke .bak sebelum dihapus)",
		Level:       permission.LevelShell,
		Capability:  permission.CapFilesystemDelete,
	},
	"git_status": {
		Description: "lihat status git singkat (branch + file berubah), readonly",
		Level:       permission.LevelReadonly,
		Capability:  permission.CapGitRead,
	},
	"git_diff": {
		Description: "lihat diff git (opsional path spesifik), readonly",
		Level:       permission.LevelReadonly,
		Capability:  permission.CapGitRead,
	},
	"web_fetch": {
		Description: "ambil isi halaman web via curl, HTML di-strip jadi teks",
		Level:       permission.LevelShell,
		Capability:  permission.CapNetworkPublic,
	},
	"todo_write": {
		Description: "simpan/update checklist rencana kerja sesi ini (bukan file project)",
		Level:       permission.LevelReadonly,
		Capability:  permission.CapSessionTodo,
	},
	"todo_read": {
		Description: "baca checklist rencana kerja sesi ini saat ini",
		Level:       permission.LevelReadonly,
		Capability:  permission.CapSessionTodo,
	},
}

// Names returns every registered tool name, sorted -- matching the
// `| sort` at the end of _ai_tool_manifest's pipeline (and providing a
// stable, testable ordering that Go's map iteration doesn't).
func Names() []string {
	names := make([]string, 0, len(Registry))
	for name := range Registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// exposeArbitraryShellEnv is the env var _ai_tool_manifest checks to
// decide whether to list run_command at all, mirroring
// "${AI_AGENT_EXPOSE_ARBITRARY_SHELL:-0}" != "1" in the zsh source.
const exposeArbitraryShellEnv = "AI_AGENT_EXPOSE_ARBITRARY_SHELL"

// Manifest renders the tool list the same way _ai_tool_manifest does:
// one "name | capability=... | approval=... | description" line per
// tool, sorted by name, with run_command hidden unless
// AI_AGENT_EXPOSE_ARBITRARY_SHELL=1 is set in the environment. This is
// display/prompt-building text for the not-yet-ported sysprompt
// assembly (internal/llmclient/internal/agent); it has no bearing on
// Dispatcher.Dispatch, which always accepts run_command by name
// regardless of this flag -- exactly like the zsh source, where the
// manifest gate is presentation-only and _ai_tool_dispatch's own `case`
// has no such check.
func Manifest() string {
	expose := os.Getenv(exposeArbitraryShellEnv) == "1"
	var lines []string
	for _, name := range Names() {
		if name == "run_command" && !expose {
			continue
		}
		entry := Registry[name]
		lines = append(lines, fmt.Sprintf("%s | capability=%s | approval=%s | %s",
			name, entry.Capability, entry.Level, entry.Description))
	}
	return strings.Join(lines, "\n")
}
