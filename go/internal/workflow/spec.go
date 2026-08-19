package workflow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
)

// SpecSysPrompt mirrors AI_SPEC_SYSPROMPT (00-config/30-sysprompt_spec.zsh)
// -- the single source of truth the zsh source itself factored out so
// aispec and aibuild never drift apart. Exported so
// internal/codeproject-adjacent callers/tests can reference the exact
// same string if needed.
const SpecSysPrompt = `Kamu software architect Python expert. Diberikan deskripsi aplikasi dari user, rancang struktur file aplikasi tsb (boleh 1 file kalau memang sesederhana itu, boleh banyak file kalau perlu dipisah per tanggung jawab). Output HANYA teks di bawah, tanpa markdown/backtick, tanpa penjelasan lain, ikuti header persis:

[APLIKASI] nama aplikasi & ringkasan 2 kalimat.
[FILES] daftar SETIAP file .py yang dibutuhkan, satu baris per file, format persis:
- nama_file.py: tugas spesifiknya apa, fungsi/class utama apa saja (nama + parameter singkat + apa yang dikembalikan), dan file lain di project ini yang dia import (kalau ada).
Kalau lebih dari 1 file, WAJIB ada main.py sebagai entry point yang cuma orkestrasi (manggil fungsi dari file lain), bukan taruh semua logic di situ.
[ALUR] urutan eksekusi program dari user run main.py sampai selesai, singkat per langkah bernomor.
[DEPENDENSI] library eksternal yang dipakai (nama pip install-nya), atau tulis 'tidak ada, cuma stdlib' kalau memang gak butuh.
[CONTOH_INPUT] 3-5 baris contoh nilai input yang bakal dimasukkan user kalau program ini dijalankan dan minta input() secara berurutan (satu nilai per baris, plain, tanpa penjelasan) -- dipakai buat smoke-test otomatis. Kalau programnya gak butuh input sama sekali, tulis 'tidak perlu input'.
[EDGE_CASE] input/kondisi tidak wajar yang wajib ditangani (mis. input non-angka, angka negatif, divide by zero, file gak ada, dll).`

// ErrSpecUsage mirrors aispec's "Usage: luna spec <deskripsi aplikasi>".
var ErrSpecUsage = errors.New("workflow: usage: app description is required")

// SpecResult is Spec's return shape.
type SpecResult struct {
	Content    string
	Outfile    string
	CopiedBack bool
}

// Spec mirrors aispec(task): generate a structured `[APLIKASI]/[FILES]/
// [ALUR]/...` app spec (consumable by internal/codeproject.Project) and
// save it under AI_PROMPT_DIR/<slug>_spec_<ts>.txt.
func (s *Service) Spec(ctx context.Context, task string, clipboard aiops.Clipboard) (SpecResult, error) {
	if err := needAnyKey(config.TaskProviderOrderBig); err != nil {
		return SpecResult{}, err
	}
	if task == "" {
		return SpecResult{}, ErrSpecUsage
	}
	if err := os.MkdirAll(s.Paths.PromptDir, 0o755); err != nil {
		return SpecResult{}, err
	}

	res, err := s.Requester.Complete(ctx, SpecSysPrompt, "Deskripsi aplikasi: "+task, config.TaskSmart, config.TaskProviderOrderBig, 0)
	if err != nil || res.Content == "" {
		return SpecResult{}, fmt.Errorf("workflow: spec generation failed: %w", err)
	}

	outfile := filepath.Join(s.Paths.PromptDir, fmt.Sprintf("%s_spec_%s.txt", aiops.Slugify(task, 40), aiops.Timestamp()))
	if err := os.WriteFile(outfile, []byte(res.Content+"\n"), 0o644); err != nil {
		return SpecResult{}, err
	}

	result := SpecResult{Content: res.Content, Outfile: outfile}
	if clipboard != nil {
		if err := clipboard.Set(ctx, res.Content); err == nil {
			result.CopiedBack = true
		}
	}
	return result, nil
}
