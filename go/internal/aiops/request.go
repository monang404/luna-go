package aiops

import (
	"context"
	"errors"
	"fmt"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
)

// DefaultMaxTokens mirrors the plain (non-project, non-agent) request
// path's implicit max_tokens: 30-luna/10-core/50-request_blocking.zsh
// defaults max_toks to a moderate value when no override is passed
// (project generation and agent steps pass their own larger explicit
// overrides -- AI_PROJECT_MAX_TOKS / AI_AGENT_MAX_TOKS -- everything
// else, like aic/aiask/aiplan/aiprompt/aispec/aireview/aisummarize, does
// not). 2000 matches that "moderate default" for a single chat-style
// completion.
const DefaultMaxTokens = 2000

// DefaultTemperature mirrors the fixed temperature the zsh payload
// builder has always used for these commands (no per-call override
// existed anywhere in 20-chat/30-code/35-files/40-workflow).
const DefaultTemperature = 0.7

// ErrEmptyReply is kept for API compatibility with earlier callers that
// matched on it directly. CompleteMessages no longer returns this value
// itself (see llmclient.ExhaustionError) -- it now returns a wrapped
// error carrying which providers were actually tried and the last
// failure detail, since collapsing every kind of total failure into
// this one generic sentinel was the audit brief's headline "error
// message tidak membantu" finding. Match on errors.Is(err,
// llmclient.ErrNoProviderAvailable) instead if you need to detect "zero
// providers were ever configured" specifically.
var ErrEmptyReply = errors.New("aiops: no provider/model produced a usable reply")

// Result is what a successful Requester.Complete call returns: the
// model's answer plus which provider/model actually produced it (some
// callers -- aiask's cache key, chat display's meta line -- need that
// attribution, matching $provider_used/$model_used / AI_CURRENT_PROVIDER
// /AI_CURRENT_MODEL in the zsh source).
type Result struct {
	Content  string
	Provider string
	Model    string
}

// Completer is the interface Requester implements -- command packages
// depend on this instead of the concrete *Requester type so tests can
// substitute a fake completer and never need a live LUNA provider (per
// SESSION-54 §48 "do not use live LUNA providers in unit tests").
type Completer interface {
	Complete(ctx context.Context, systemPrompt, userPrompt string, class config.TaskClass, order []string, maxTokens int) (Result, error)
	CompleteMessages(ctx context.Context, messages []llmclient.Message, class config.TaskClass, order []string, maxTokens int) (Result, error)
}

// Requester is the Go equivalent of the shared "_ai_quick /
// _ai_chat_request" call tree (30-luna/10-core/25-quick_chat.zsh +
// 50-request_blocking.zsh): build a chat payload, walk the
// provider-order fallback list (skipping providers with no API key
// configured, exactly like config.ActiveProviders), and for each
// provider walk its task-class model fallback list
// (config.ModelsFor), returning the first response that comes back
// with usable Content.
//
// It does NOT implement retry/circuit-breaker/backoff (that's
// internal/llmclient's own resilience layer, SESSION-46, invoked
// per-call here via llmclient.CallBlocking -- this type only owns the
// provider/model *iteration order*, same boundary the zsh source draws
// between 41-provider_candidate.zsh's lookahead and
// 44-retry_decision.zsh's actual retry state machine, which this
// package does not reimplement).
type Requester struct {
	Limits config.Limits
}

// NewRequester builds a Requester from the current environment's Limits.
func NewRequester() *Requester {
	return &Requester{Limits: config.LoadLimits()}
}

var _ Completer = (*Requester)(nil)

// Complete sends one system+user message pair, trying every
// provider/model combination in order until one returns usable
// content. maxTokens <= 0 uses DefaultMaxTokens.
func (r *Requester) Complete(ctx context.Context, systemPrompt, userPrompt string, class config.TaskClass, order []string, maxTokens int) (Result, error) {
	return r.CompleteMessages(ctx, []llmclient.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}, class, order, maxTokens)
}

// CompleteMessages is Complete's variant for callers that already have a
// full message list to send (session turns, multi-stage chat context),
// equivalent to _ai_chat_request taking a $msgfile of arbitrary length
// rather than _ai_quick's fixed system+user pair.
func (r *Requester) CompleteMessages(ctx context.Context, messages []llmclient.Message, class config.TaskClass, order []string, maxTokens int) (Result, error) {
	if maxTokens <= 0 {
		maxTokens = DefaultMaxTokens
	}
	if len(config.ActiveProviders(order)) == 0 {
		return Result{}, fmt.Errorf("aiops: %w", llmclient.ErrNoProviderAvailable)
	}

	failed := map[string]bool{}
	var lastResp llmclient.Response
	for {
		candidate, err := llmclient.SelectProviderCandidate(order, failed)
		if err != nil {
			return Result{}, fmt.Errorf("aiops: %w", llmclient.ExhaustionError(order, failed, lastResp))
		}

		models := config.ModelsFor(candidate.Name, class)
		if len(models) == 0 {
			models = []string{candidate.Provider.Model}
		}

		succeededProvider := false
		for _, model := range models {
			payload, err := llmclient.BuildPayload(messages, llmclient.PayloadOptions{
				Provider:    candidate.Name,
				Model:       model,
				MaxTokens:   maxTokens,
				Temperature: DefaultTemperature,
			})
			if err != nil {
				continue
			}
			resp, err := llmclient.CallBlocking(ctx, candidate, payload, r.Limits)
			llmclient.Debugf("provider=%s model=%s http_status=%d content_len=%d err=%v", candidate.Name, model, resp.HTTPStatus, len(resp.Content), err)
			lastResp = resp
			if err != nil {
				if errors.Is(err, llmclient.ErrCancelled) {
					return Result{}, err
				}
				continue
			}
			if resp.HTTPStatus < 200 || resp.HTTPStatus >= 300 {
				continue
			}
			if resp.Content == "" {
				continue
			}
			succeededProvider = true
			return Result{Content: resp.Content, Provider: candidate.Name, Model: model}, nil
		}
		if !succeededProvider {
			failed[candidate.Name] = true
		}
	}
}
