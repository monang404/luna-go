package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/permission"
)

// This file ports 30-luna/05-tools/25-tool_fs_patch_delete.zsh
// (_ai_tool_patch_file, _ai_tool_delete_file).

// PatchFileTool implements _ai_tool_patch_file: apply a unified diff to
// an existing file via the external `patch -p0` binary (a literal
// wrapper, like the zsh source -- Go's standard library has no unified
// diff applier, and reimplementing `patch`'s hunk-matching/fuzz
// semantics from scratch would risk silently diverging from what the
// model was actually told to expect). Backs up the file first and
// restores that backup if the patch fails, exactly like the source.
type PatchFileTool struct{}

func (PatchFileTool) Name() string                      { return "patch_file" }
func (PatchFileTool) Capability() permission.Capability { return Registry["patch_file"].Capability }

func (PatchFileTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	fsPath := ExtractPath(args)
	diffContent := ExtractField(args, "diff_content")

	if fsPath == "" {
		return Result{}, fmt.Errorf("ERROR: patch_file membutuhkan args.path (string non-empty)")
	}
	if diffContent == "" {
		return Result{}, fmt.Errorf("ERROR: patch_file membutuhkan args.diff_content (string non-empty)")
	}
	if maxChars := config.LoadLimits().PatchMaxChars; len(diffContent) > maxChars {
		return Result{}, fmt.Errorf("ERROR: diff terlalu besar (maks %d karakter)", maxChars)
	}
	info, err := os.Stat(fsPath)
	if err != nil || !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("ERROR: file %s gak ketemu", fsPath)
	}
	if IsSecretFile(fsPath) {
		return Result{}, fmt.Errorf("ERROR: [%s] kelihatan kayak file secrets. Ditolak.", fsPath)
	}
	patchBin, err := exec.LookPath("patch")
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: command 'patch' gak ketemu. Install via: pkg install patch")
	}

	backup := backupPath(fsPath)
	if err := copyFile(fsPath, backup); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membuat backup %s", backup)
	}

	diffFile, err := os.CreateTemp("", "*.patch")
	if err != nil {
		os.Remove(backup)
		return Result{}, fmt.Errorf("ERROR: gagal membuat file diff sementara")
	}
	diffFileName := diffFile.Name()
	defer os.Remove(diffFileName)
	if _, err := diffFile.WriteString(diffContent + "\n"); err != nil {
		diffFile.Close()
		os.Remove(backup)
		return Result{}, fmt.Errorf("ERROR: gagal menulis file diff sementara")
	}
	diffFile.Close()

	cmd := exec.Command(patchBin, "-p0", fsPath)
	f, err := os.Open(diffFileName)
	if err != nil {
		os.Remove(backup)
		return Result{}, fmt.Errorf("ERROR: gagal membaca file diff sementara")
	}
	cmd.Stdin = f
	out, runErr := cmd.CombinedOutput()
	f.Close()

	if runErr == nil {
		os.Remove(diffFileName)
		return Result{Output: fmt.Sprintf("OK: patch berhasil diterapkan ke %s (backup: %s)", fsPath, backup)}, nil
	}

	// Restore backup on failure, matching the zsh source's `command cp
	// -f "$backup" "$fs_path"` (bypassing any `cp -i` alias).
	restoreErr := copyFile(backup, fsPath)
	os.Remove(backup)
	if restoreErr != nil {
		return Result{}, fmt.Errorf("ERROR: patch gagal diterapkan DAN restore backup juga gagal untuk %s: %s", fsPath, string(out))
	}
	return Result{}, fmt.Errorf("ERROR: patch gagal diterapkan (backup di-restore ke semula):\n%s", string(out))
}

// DeleteFileTool implements _ai_tool_delete_file: back up the file to
// path.bak.<timestamp> and then remove it. Restoring is a manual `luna
// undo <path>` step outside this tool's scope, matching the source's
// own comment.
type DeleteFileTool struct{}

func (DeleteFileTool) Name() string                      { return "delete_file" }
func (DeleteFileTool) Capability() permission.Capability { return Registry["delete_file"].Capability }

func (DeleteFileTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	fsPath := ExtractPath(args)
	if fsPath == "" {
		return Result{}, fmt.Errorf("ERROR: delete_file membutuhkan args.path (string non-empty)")
	}
	info, err := os.Stat(fsPath)
	if err != nil || !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("ERROR: file %s gak ketemu (atau bukan file biasa)", fsPath)
	}
	if IsSecretFile(fsPath) {
		return Result{}, fmt.Errorf("ERROR: [%s] kelihatan kayak file secrets. Ditolak.", fsPath)
	}

	backup := backupPath(fsPath)
	if err := copyFile(fsPath, backup); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membuat backup %s", backup)
	}
	if err := os.Remove(fsPath); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal menghapus %s", fsPath)
	}
	return Result{Output: fmt.Sprintf("OK: %s dihapus (backup: %s, restore lewat 'luna undo %s')", fsPath, backup, fsPath)}, nil
}
