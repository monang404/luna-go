package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/monang404/luna-go/internal/permission"
)

// This file ports 30-luna/05-tools/20-tool_fs_write.zsh
// (_ai_tool_write_file, _ai_tool_edit_file, _ai_tool_move_file).
//
// None of the three call permission.CheckPermission themselves --
// exactly like every other Tool in this package, that happens in
// Dispatcher.Dispatch (dispatch.go, SESSION-43) before Execute is ever
// reached (this session's own AC-05 regression test asserts that no
// write-capable tool can run without it).

// WriteFileTool implements _ai_tool_write_file: create a brand-new
// file, refusing to overwrite one that already exists (edit_file is
// what existing-file changes go through instead).
type WriteFileTool struct{}

func (WriteFileTool) Name() string                      { return "write_file" }
func (WriteFileTool) Capability() permission.Capability { return Registry["write_file"].Capability }

func (WriteFileTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	fsPath := ExtractPath(args)
	content := ExtractField(args, "content")

	if fsPath == "" {
		return Result{}, fmt.Errorf("ERROR: write_file membutuhkan args.path (string non-empty)")
	}
	// Mirrors the zsh source's `[ -z "$content" ]` check exactly: an
	// empty-string content is rejected the same as a missing one (a
	// quirk of the source, not fixed here -- writing a genuinely empty
	// file isn't reachable through this tool, by design of the port).
	if content == "" {
		return Result{}, fmt.Errorf("ERROR: write_file membutuhkan args.content (string non-empty)")
	}
	if info, err := os.Stat(fsPath); err == nil && info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("ERROR: file %s sudah ada. Gunakan edit_file untuk file existing.", fsPath)
	}
	if IsSecretFile(fsPath) {
		return Result{}, fmt.Errorf("ERROR: nama file %s menyerupai file rahasia.", fsPath)
	}

	dir := filepath.Dir(fsPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membuat direktori %s", dir)
	}
	// `printf '%s\n' "$content" > "$fs_path"` -- always appends a
	// trailing newline, regardless of whether content already had one.
	if err := os.WriteFile(fsPath, []byte(content+"\n"), 0o644); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal menulis %s", fsPath)
	}
	return Result{Output: fmt.Sprintf("OK: file %s berhasil dibuat.", fsPath)}, nil
}

// EditFileTool implements _ai_tool_edit_file: replace exactly one
// occurrence of old_str with new_str in an existing file, backing up
// the pre-edit content to path.bak.<timestamp> first.
type EditFileTool struct{}

func (EditFileTool) Name() string                      { return "edit_file" }
func (EditFileTool) Capability() permission.Capability { return Registry["edit_file"].Capability }

func (EditFileTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	fsPath := ExtractPath(args)
	oldStr := ExtractField(args, "old_str")
	newStr := ExtractField(args, "new_str")

	if fsPath == "" {
		return Result{}, fmt.Errorf("ERROR: edit_file membutuhkan args.path (string non-empty)")
	}
	if oldStr == "" {
		return Result{}, fmt.Errorf("ERROR: edit_file membutuhkan args.old_str (string non-empty)")
	}
	info, err := os.Stat(fsPath)
	if err != nil || !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("ERROR: file %s gak ketemu", fsPath)
	}
	if IsSecretFile(fsPath) {
		return Result{}, fmt.Errorf("ERROR: [%s] kelihatan kayak file secrets. Ditolak.", fsPath)
	}

	data, err := os.ReadFile(fsPath)
	if err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membaca %s: %w", fsPath, err)
	}
	content := string(data)
	count := strings.Count(content, oldStr)
	if count == 0 {
		return Result{}, fmt.Errorf("ERROR: old_str gak ketemu di %s", fsPath)
	}
	if count > 1 {
		return Result{}, fmt.Errorf("ERROR: old_str ketemu %d kali di %s. Harus match persis 1 kali.", count, fsPath)
	}
	newContent := strings.Replace(content, oldStr, newStr, 1)

	backup := backupPath(fsPath)
	if err := copyFile(fsPath, backup); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membuat backup %s", backup)
	}
	if err := writeAtomic(fsPath, []byte(newContent), info.Mode()); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal menerapkan perubahan ke %s: %w", fsPath, err)
	}
	return Result{Output: fmt.Sprintf("OK: diff diterapkan ke %s (backup: %s)", fsPath, backup)}, nil
}

// MoveFileTool implements _ai_tool_move_file: rename/move an existing
// file to a new path, refusing to overwrite an existing destination.
type MoveFileTool struct{}

func (MoveFileTool) Name() string                      { return "move_file" }
func (MoveFileTool) Capability() permission.Capability { return Registry["move_file"].Capability }

func (MoveFileTool) Execute(_ context.Context, args json.RawMessage) (Result, error) {
	src := ExtractPath(args)
	dest := ExtractField(args, "dest", "destination")

	if src == "" {
		return Result{}, fmt.Errorf("ERROR: move_file membutuhkan args.path (sumber, string non-empty)")
	}
	if dest == "" {
		return Result{}, fmt.Errorf("ERROR: move_file membutuhkan args.dest (tujuan, string non-empty)")
	}
	if info, err := os.Stat(src); err != nil || !info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("ERROR: file sumber %s gak ketemu", src)
	}
	if IsSecretFile(src) || IsSecretFile(dest) {
		return Result{}, fmt.Errorf("ERROR: salah satu path (%s / %s) kelihatan kayak file secrets. Ditolak.", src, dest)
	}
	if info, err := os.Stat(dest); err == nil && info.Mode().IsRegular() {
		return Result{}, fmt.Errorf("ERROR: file tujuan %s sudah ada. Hapus/pindahkan dulu manual kalau memang mau ditimpa.", dest)
	}

	destDir := filepath.Dir(dest)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("ERROR: gagal membuat direktori tujuan %s", destDir)
	}
	if err := os.Rename(src, dest); err != nil {
		// os.Rename fails across filesystem/device boundaries on some
		// platforms (unlike `mv`, which falls back to copy+remove
		// itself) -- reproduce that fallback here so a cross-device
		// move behaves the same as the zsh source's plain `mv`.
		if copyErr := copyFile(src, dest); copyErr != nil {
			return Result{}, fmt.Errorf("ERROR: gagal memindahkan %s ke %s", src, dest)
		}
		if rmErr := os.Remove(src); rmErr != nil {
			return Result{}, fmt.Errorf("ERROR: gagal memindahkan %s ke %s", src, dest)
		}
	}
	return Result{Output: fmt.Sprintf("OK: %s dipindah ke %s", src, dest)}, nil
}

// --- shared file helpers (used by edit_file/move_file/patch_file/delete_file) ---

// copyFile mirrors `cp -f -- src dest`.
func copyFile(src, dest string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	info, err := os.Stat(src)
	mode := os.FileMode(0o644)
	if err == nil {
		mode = info.Mode()
	}
	return os.WriteFile(dest, data, mode)
}

// writeAtomic mirrors edit_file's tmp-file-then-rename pattern
// (`mktemp` into the same directory, then `mv -f` over the original) --
// a reader never observes a partially-written file.
func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp.*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, mode); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return err
	}
	return nil
}
