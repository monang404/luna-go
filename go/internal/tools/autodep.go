package tools

import (
	"os"
	"os/exec"
	"regexp"
)

// This file ports the pure, install-free half of 02-tool_autodep.zsh:
// package-manager detection and the command -> package-name mapping table.
// _ai_autodep_run_install/_ai_autodep_install_missing (the parts that
// actually shell out to `pkg install`/`apt-get install`/`pip3 install`)
// are deliberately NOT ported in this session -- they only make sense as
// a retry hook wired into a *running* shell/process tool's exit-127
// handling, and that tool doesn't exist in Go yet (run_command/
// exec_process land in SESSION-47/48). Porting the install-trigger now
// would mean either leaving it uncalled dead code or guessing at the
// retry-wiring shape SESSION-47/48 will actually need; the detection and
// mapping tables below have no such dependency and are exactly what
// those sessions will want off the shelf when they do wire up autodep.

// PackageManager identifies which package manager is available on the
// current host, mirroring _ai_autodep_pkg_manager: Termux's "pkg" (
// detected the same way, via the PREFIX env var Termux always sets, not
// just command presence -- a plain Linux box can have a stray "pkg"
// binary from an unrelated tool) takes priority over Debian/Ubuntu's
// "apt-get", and "" means neither was found.
type PackageManager string

const (
	PkgManagerTermux  PackageManager = "pkg"
	PkgManagerAPT     PackageManager = "apt"
	PkgManagerUnknown PackageManager = ""
)

// DetectPackageManager mirrors _ai_autodep_pkg_manager's detection order
// exactly: Termux (PREFIX set + `pkg` on PATH) first, then `apt-get`,
// else PkgManagerUnknown.
func DetectPackageManager() PackageManager {
	if os.Getenv("PREFIX") != "" {
		if _, err := exec.LookPath("pkg"); err == nil {
			return PkgManagerTermux
		}
	}
	if _, err := exec.LookPath("apt-get"); err == nil {
		return PkgManagerAPT
	}
	return PkgManagerUnknown
}

// cmdToPkg mirrors the literal `case "$cmd" in ... esac` table in
// _ai_autodep_cmd_to_pkg. Entries whose package name differs between
// Termux and Debian/Ubuntu are functions of PackageManager (python3/pip3/
// npm/gcc/fd -- the same five cases the zsh source branches on with
// `[ "$pkg_mgr" = "pkg" ] && echo ... || echo ...`); every other entry is
// a single constant package name shared by both.
var cmdToPkg = map[string]func(PackageManager) string{
	"head": constPkg("coreutils"), "tail": constPkg("coreutils"), "cut": constPkg("coreutils"),
	"sort": constPkg("coreutils"), "uniq": constPkg("coreutils"), "wc": constPkg("coreutils"),
	"tee": constPkg("coreutils"), "tr": constPkg("coreutils"), "nl": constPkg("coreutils"),
	"cat": constPkg("coreutils"), "cp": constPkg("coreutils"), "mv": constPkg("coreutils"),
	"rm": constPkg("coreutils"), "mkdir": constPkg("coreutils"), "touch": constPkg("coreutils"),
	"chmod": constPkg("coreutils"), "chown": constPkg("coreutils"), "stat": constPkg("coreutils"),
	"du": constPkg("coreutils"), "df": constPkg("coreutils"), "ln": constPkg("coreutils"),
	"readlink": constPkg("coreutils"), "realpath": constPkg("coreutils"), "basename": constPkg("coreutils"),
	"dirname": constPkg("coreutils"), "mktemp": constPkg("coreutils"), "date": constPkg("coreutils"),
	"od": constPkg("coreutils"), "xxd": constPkg("coreutils"),

	"awk": constPkg("gawk"), "gawk": constPkg("gawk"),
	"sed":        constPkg("sed"),
	"grep":       constPkg("grep"),
	"egrep":      constPkg("grep"),
	"fgrep":      constPkg("grep"),
	"python3":    termuxOr("python", "python3"),
	"python":     termuxOr("python", "python3"),
	"pip3":       termuxOr("python", "python3-pip"),
	"pip":        termuxOr("python", "python3-pip"),
	"git":        constPkg("git"),
	"curl":       constPkg("curl"),
	"wget":       constPkg("wget"),
	"jq":         constPkg("jq"),
	"node":       constPkg("nodejs"),
	"nodejs":     constPkg("nodejs"),
	"npm":        termuxOr("nodejs", "npm"),
	"zip":        constPkg("zip"),
	"unzip":      constPkg("unzip"),
	"make":       constPkg("make"),
	"cmake":      constPkg("cmake"),
	"gcc":        termuxOr("clang", "gcc"),
	"cc":         termuxOr("clang", "gcc"),
	"clang":      constPkg("clang"),
	"find":       constPkg("findutils"),
	"xargs":      constPkg("findutils"),
	"fzf":        constPkg("fzf"),
	"bat":        constPkg("bat"),
	"fd":         termuxOr("fd", "fd-find"),
	"rg":         constPkg("ripgrep"),
	"htop":       constPkg("htop"),
	"tmux":       constPkg("tmux"),
	"ssh":        termuxOr("openssh", "openssh-client"),
	"scp":        termuxOr("openssh", "openssh-client"),
	"ssh-keygen": termuxOr("openssh", "openssh-client"),
	"rsync":      constPkg("rsync"),
	"diff":       constPkg("diffutils"),
	"patch":      constPkg("diffutils"),
	"psutil":     constPkg("pip:psutil"),
	"requests":   constPkg("pip:requests"),
}

func constPkg(name string) func(PackageManager) string {
	return func(PackageManager) string { return name }
}

func termuxOr(termuxPkg, otherPkg string) func(PackageManager) string {
	return func(mgr PackageManager) string {
		if mgr == PkgManagerTermux {
			return termuxPkg
		}
		return otherPkg
	}
}

// CmdToPackage mirrors _ai_autodep_cmd_to_pkg: returns the package name
// to install for a missing command under the given package manager, or
// "" if the command isn't in the mapping table.
func CmdToPackage(cmd string, mgr PackageManager) string {
	f, ok := cmdToPkg[cmd]
	if !ok {
		return ""
	}
	return f(mgr)
}

// missingCmdPattern mirrors the `grep -o 'command not found: [^ ]*'`
// step of _ai_autodep_extract_missing_cmd.
var missingCmdPattern = regexp.MustCompile(`command not found: (\S+)`)

// ExtractMissingCmd mirrors _ai_autodep_extract_missing_cmd: pulls the
// command name out of a "command not found: <name>" line in a tool's
// captured output (the shape a failed exit-127 shell/process tool
// produces), taking the first match only (`awk 'NR==1{...}'`). Returns
// "" if no such line is present.
func ExtractMissingCmd(output string) string {
	m := missingCmdPattern.FindStringSubmatch(output)
	if m == nil {
		return ""
	}
	return m[1]
}
