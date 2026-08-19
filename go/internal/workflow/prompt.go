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

// promptSysPrompt mirrors aiprompt's inline sysprompt.
const promptSysPrompt = `Kamu expert prompt engineering. Diberikan deskripsi tugas dari user, buat SATU prompt terstruktur dan siap pakai untuk dikasih ke LLM lain. Wajib ada bagian dengan header berikut: [ROLE] peran spesifik yang harus dimainkan LUNA. [CONTEXT] konteks relevan yang perlu diketahui. [TASK] instruksi tugas yang spesifik, jelas, gak ambigu. [FORMAT] format output yang diinginkan (struktur, panjang, gaya). [CONSTRAINTS] batasan/aturan penting yang harus dipatuhi. Output HANYA prompt final yang siap copy-paste, tanpa penjelasan tambahan, tanpa markdown backtick.`

// ErrPromptUsage mirrors aiprompt's "Usage: luna prompt <deskripsi tugas>".
var ErrPromptUsage = errors.New("workflow: usage: task description is required")

// PromptResult is Prompt's return shape.
type PromptResult struct {
	Content    string
	Outfile    string
	CopiedBack bool
}

// Prompt mirrors aiprompt(task): generate a ready-to-use structured
// prompt, save it under AI_PROMPT_DIR/<slug>_<ts>.txt, and best-effort
// copy it to the clipboard (clipboard == nil mirrors `command -v
// termux-clipboard-set` failing -- silently skipped, matching the zsh
// source's own `if command -v ...` guard).
func (s *Service) Prompt(ctx context.Context, task string, clipboard aiops.Clipboard) (PromptResult, error) {
	if err := needAnyKey(config.TaskProviderOrder); err != nil {
		return PromptResult{}, err
	}
	if task == "" {
		return PromptResult{}, ErrPromptUsage
	}
	if err := os.MkdirAll(s.Paths.PromptDir, 0o755); err != nil {
		return PromptResult{}, err
	}

	res, err := s.Requester.Complete(ctx, promptSysPrompt, "Deskripsi tugas: "+task, config.TaskSmart, config.TaskProviderOrder, 0)
	if err != nil || res.Content == "" {
		return PromptResult{}, fmt.Errorf("workflow: prompt generation failed: %w", err)
	}

	outfile := filepath.Join(s.Paths.PromptDir, fmt.Sprintf("%s_%s.txt", aiops.Slugify(task, 40), aiops.Timestamp()))
	if err := os.WriteFile(outfile, []byte(res.Content+"\n"), 0o644); err != nil {
		return PromptResult{}, err
	}

	result := PromptResult{Content: res.Content, Outfile: outfile}
	if clipboard != nil {
		if err := clipboard.Set(ctx, res.Content); err == nil {
			result.CopiedBack = true
		}
	}
	return result, nil
}
