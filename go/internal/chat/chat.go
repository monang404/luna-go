package chat

import (
	"context"
	"errors"
	"fmt"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
)

// ErrNoPrompt mirrors aic/aicl's `[ -z "$usermsg" ] && return 1` guard
// (empty prompt is invalid).
var ErrNoPrompt = errors.New("chat: prompt is empty")

// ErrNoProvider mirrors _ai_need_any_key failing (no API key configured
// for any candidate provider) -- the guard every command in this
// package starts with.
var ErrNoProvider = errors.New("chat: no LUNA provider configured (no API key set)")

// ChatResult is QuickChat/Aish's return shape: the model's reasoning
// (if any, already split out) and clean answer, plus attribution.
type ChatResult struct {
	Thought  string
	Answer   string
	Raw      string
	Provider string
	Model    string
}

// Service bundles the shared dependency (an aiops.Completer) every
// function in this package needs.
type Service struct {
	Requester aiops.Completer
}

// NewService builds a Service around the given Completer.
func NewService(requester aiops.Completer) *Service {
	return &Service{Requester: requester}
}

func needAnyKey(order []string) error {
	if len(config.ActiveProviders(order)) == 0 {
		return ErrNoProvider
	}
	return nil
}

// QuickChat mirrors aic(prompt): a short, fast chat turn using the
// chat-short persona and the FAST task-class provider order.
// SplitReply is applied to the raw response before returning, matching
// _ai_chat_render's own split-then-display step.
func (s *Service) QuickChat(ctx context.Context, prompt string) (ChatResult, error) {
	if err := needAnyKey(config.TaskProviderOrderFast); err != nil {
		return ChatResult{}, err
	}
	if prompt == "" {
		return ChatResult{}, ErrNoPrompt
	}
	res, err := s.Requester.Complete(ctx, config.PersonaChatShort, prompt, config.TaskFast, config.TaskProviderOrderFast, 0)
	if err != nil {
		return ChatResult{}, err
	}
	thought, answer := SplitReply(res.Content)
	return ChatResult{Thought: thought, Answer: answer, Raw: res.Content, Provider: res.Provider, Model: res.Model}, nil
}

// Aish mirrors aish(prompt): a shell-command-focused chat turn (FAST
// order, a fixed Linux/Termux-expert system prompt, no @@JAWABAN@@
// marker contract -- the zsh source passes stream=1 with a distinct
// persona, not the chat-short/-long marker persona, so SplitReply is
// intentionally NOT applied here).
func (s *Service) Aish(ctx context.Context, prompt string) (ChatResult, error) {
	if err := needAnyKey(config.TaskProviderOrderFast); err != nil {
		return ChatResult{}, err
	}
	const sysprompt = `Kamu expert Linux dan Termux. Berikan perintah shell yang tepat, aman, dan langsung bisa dijalankan di Termux Android.`
	res, err := s.Requester.Complete(ctx, sysprompt, prompt, config.TaskFast, config.TaskProviderOrderFast, 0)
	if err != nil {
		return ChatResult{}, err
	}
	return ChatResult{Answer: res.Content, Raw: res.Content, Provider: res.Provider, Model: res.Model}, nil
}

// LongChatStageName is one of the five fixed aicl stages, in order.
type LongChatStageName string

const (
	StageOutline    LongChatStageName = "Outline"
	StageDraft      LongChatStageName = "Draft"
	StageRefinement LongChatStageName = "Refinement"
	StageReview     LongChatStageName = "Review"
	StageFinal      LongChatStageName = "Final"
)

// LongChatStages is the fixed stage order aicl always runs, in order --
// exported so callers (SESSION-55's UI timeline) can render progress
// without duplicating this list.
var LongChatStages = []LongChatStageName{StageOutline, StageDraft, StageRefinement, StageReview, StageFinal}

// StageResult is one completed stage of a LongChatResult.
type StageResult struct {
	Stage  LongChatStageName
	Output string
}

// LongChatResult is aicl's return shape: every stage's output (for
// logging/debugging, matching $combined_results) plus the final
// rendered answer (matching $current_context after the last stage).
type LongChatResult struct {
	Stages []StageResult
	Final  string
}

// stagePrompt mirrors aicl's `case "$stage" in ...` prompt-building
// switch exactly, one branch per stage.
func stagePrompt(stage LongChatStageName, usermsg, context string) string {
	switch stage {
	case StageOutline:
		return fmt.Sprintf("Buat outline ringkas (3-5 poin utama) untuk menjawab permintaan user berikut. Jangan tulis konten lengkapnya, cukup poin-poin strukturnya saja.\n\n%s", context)
	case StageDraft:
		return fmt.Sprintf("Berdasarkan outline berikut, buatlah draft awal konten yang detail dan menyeluruh. Fokus pada kelengkapan informasi.\n\nOutline:\n%s", context)
	case StageRefinement:
		return fmt.Sprintf("Perbaiki dan perluas draft berikut agar bahasanya lebih natural, mengalir, dan informasinya padat/akurat.\n\nDraft:\n%s", context)
	case StageReview:
		return fmt.Sprintf("Review hasil perbaikan berikut. Apakah sudah menjawab permintaan asli user ('%s')? Jika ada yang kurang, perbaiki. Jika sudah pas, pertajam hasilnya.\n\nKonten:\n%s", usermsg, context)
	case StageFinal:
		return fmt.Sprintf("Ini adalah tahap final. Format dan rangkum hasil akhir berikut dengan rapi agar siap dibaca user (gunakan heading/poin yang sesuai, bersihkan teks internal LUNA, pastikan profesional).\n\nKonten:\n%s", context)
	default:
		return context
	}
}

// LongChat mirrors aicl(prompt): the fixed five-stage
// Outline->Draft->Refinement->Review->Final pipeline, each stage fed the
// previous stage's full output as context, using the chat-long persona
// and the SMART task-class provider order (config.TaskProviderOrder,
// the "smart" alias). If any stage fails, LongChat stops immediately
// and returns the stages completed so far alongside the error --
// mirroring aicl's own `[ "$rc" -ne 0 ] && ... return "$rc"` early-out
// (no partial "Final" is synthesized from a failed stage).
func (s *Service) LongChat(ctx context.Context, usermsg string) (LongChatResult, error) {
	if err := needAnyKey(config.TaskProviderOrder); err != nil {
		return LongChatResult{}, err
	}
	if usermsg == "" {
		return LongChatResult{}, ErrNoPrompt
	}

	result := LongChatResult{}
	currentContext := "Permintaan Asli: " + usermsg

	for _, stage := range LongChatStages {
		prompt := stagePrompt(stage, usermsg, currentContext)
		res, err := s.Requester.Complete(ctx, config.PersonaChatLong, prompt, config.TaskSmart, config.TaskProviderOrder, 0)
		if err != nil {
			return result, fmt.Errorf("chat: stage %q failed: %w", stage, err)
		}
		currentContext = res.Content
		result.Stages = append(result.Stages, StageResult{Stage: stage, Output: res.Content})
	}

	result.Final = currentContext
	return result, nil
}
