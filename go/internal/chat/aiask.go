package chat

import (
	"context"
	"errors"
	"fmt"

	"github.com/monang404/luna-go/internal/config"
)

// ErrAskNoContent mirrors aiask's "Gak ada konten (file kosong/gak
// ketemu, atau lupa pipe)." message.
var ErrAskNoContent = errors.New("chat: no content (empty/missing file, or a pipe was expected)")

// AskResult is Ask's return shape.
type AskResult struct {
	Answer   string
	Provider string
	Model    string
}

// Ask mirrors aiask(content, query): answer a question about
// externally-supplied context (a file's content or piped stdin) using
// the chat-long persona plus a context-QA instruction, FAST task-class
// order.
//
// Cache layer note: the zsh source wraps this in a per-(provider,
// model, task_class, prompt, persona) response cache
// (_ai_cache_key/_ai_cache_read/_ai_cache_write, defined in
// 30-luna/10-core/, NOT part of SESSION-54's ported file set -- see
// SESSION-54_EXECUTION_CONTEXT.md §2's file list). Reimplementing an
// approximation of that cache here without its actual key/storage
// primitives existing yet would risk silently diverging from whatever
// exact cache semantics that future session ports, so this function
// intentionally always performs a live request (equivalent to aiask
// always being called with --no-cache). content/query splitting
// (stdin-pipe vs. file-arg vs. bare-args) is also a CLI/stdin concern
// left to SESSION-55; callers of this package already have `content`
// and `query` split.
func (s *Service) Ask(ctx context.Context, content, query string) (AskResult, error) {
	if err := needAnyKey(config.TaskProviderOrderFast); err != nil {
		return AskResult{}, err
	}
	if content == "" {
		return AskResult{}, ErrAskNoContent
	}

	persona := config.PersonaChatLong + " Kamu akan dikasih konteks (isi file/output command), jawab pertanyaan berdasarkan konteks itu."
	prompt := fmt.Sprintf("Konteks:\n%s\n\nPertanyaan: %s", content, query)

	res, err := s.Requester.Complete(ctx, persona, prompt, config.TaskFast, config.TaskProviderOrderFast, 0)
	if err != nil {
		return AskResult{}, err
	}
	return AskResult{Answer: res.Content, Provider: res.Provider, Model: res.Model}, nil
}
