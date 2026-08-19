package chat

import (
	"context"
	"fmt"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
)

// SessionAsk mirrors _ai_session_ask(name, msg): one multi-turn session
// turn. The session file is only rewritten AFTER a successful reply --
// a cancelled/failed turn never leaves an orphan user message in the
// persisted session (matching the zsh source's own request_file/
// tmp_session two-stage commit). An empty msg is a silent no-op success,
// matching `[ -n "$msg" ] || return 0`.
func (s *Service) SessionAsk(ctx context.Context, store *SessionStore, name, msg string) (ChatResult, error) {
	if err := needAnyKey(config.TaskProviderOrderFast); err != nil {
		return ChatResult{}, err
	}
	if msg == "" {
		return ChatResult{}, nil
	}

	history, err := store.Load(name)
	if err != nil {
		return ChatResult{}, fmt.Errorf("chat: failed to load session %q: %w", name, err)
	}

	request := append(append([]llmclient.Message{}, history...), llmclient.Message{Role: "user", Content: msg})

	res, err := s.Requester.CompleteMessages(ctx, request, config.TaskFast, config.TaskProviderOrderFast, 0)
	if err != nil {
		return ChatResult{}, fmt.Errorf("chat: session %q request failed: %w", name, err)
	}

	updated := append(history, llmclient.Message{Role: "user", Content: msg}, llmclient.Message{Role: "assistant", Content: res.Content})
	limits := config.LoadLimits()
	updated = llmclient.TrimSession(updated, limits.SessionMaxMsgs)
	if err := store.save(name, updated); err != nil {
		return ChatResult{}, fmt.Errorf("chat: failed to save session %q: %w", name, err)
	}

	if store.Log != nil {
		store.Log("session:"+name, msg, res.Content)
	}

	return ChatResult{Answer: res.Content, Raw: res.Content, Provider: res.Provider, Model: res.Model}, nil
}
