package chat

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/monang404/luna-go/internal/config"
)

var (
	privateKeyRE  = regexp.MustCompile(`BEGIN (RSA |EC |OPENSSH |DSA |ENCRYPTED )?PRIVATE KEY`)
	cardNumberRE  = regexp.MustCompile(`([0-9][ -]?){13,19}`)
	secretValueRE = regexp.MustCompile(`(?i)(password|passwd|secret|api[_-]?key|token|otp|kode verifikasi|verification code)[\s:=]+[A-Za-z0-9!@#$%^&*_-]{4,}`)
	allWhitespace = regexp.MustCompile(`\s`)
	otpDigitsRE   = regexp.MustCompile(`^[0-9]{4,8}$`)
)

// IsClipSensitive mirrors _ai_clip_is_sensitive: a best-effort heuristic
// (NOT perfect detection, matching the zsh source's own comment) for
// clipboard content that looks like an OTP, password/token/secret,
// card number, or PEM private key -- so aiclip can refuse to send it to
// an external API without --force.
func IsClipSensitive(content string) bool {
	if privateKeyRE.MatchString(content) {
		return true
	}
	if cardNumberRE.MatchString(content) {
		return true
	}
	if secretValueRE.MatchString(content) {
		return true
	}
	trimmed := allWhitespace.ReplaceAllString(content, "")
	if otpDigitsRE.MatchString(trimmed) {
		return true
	}
	return false
}

// ErrClipEmpty mirrors aiclip's "Clipboard kosong." message.
var ErrClipEmpty = fmt.Errorf("chat: clipboard is empty")

// ErrClipSensitive mirrors aiclip's sensitive-content refusal (without
// --force).
var ErrClipSensitive = fmt.Errorf("chat: clipboard content looks sensitive, refusing without force")

// ClipResult is Clip's return shape: everything QuickChat-shaped
// (already-split reasoning/answer) plus whether the clean answer was
// copied back to the clipboard.
type ClipResult struct {
	ChatResult
	CopiedBack bool
}

// Clip mirrors aiclip(query) / aiclip --force: read the clipboard,
// refuse sensitive-looking content unless force is set, ask the model
// about it (chat-long persona + SMART order, matching `_ai_quick ...
// smart`), and copy only the CLEAN split answer back to the clipboard
// (never the raw reply, which might still carry "**Thought**" leakage
// -- see the zsh source's own "bug sama akar sama aic/aicl" comment).
func (s *Service) Clip(ctx context.Context, clipboard interface {
	Get(ctx context.Context) (string, error)
	Set(ctx context.Context, content string) error
}, force bool, query string) (ClipResult, error) {
	if err := needAnyKey(config.TaskProviderOrder); err != nil {
		return ClipResult{}, err
	}
	content, err := clipboard.Get(ctx)
	if err != nil {
		return ClipResult{}, fmt.Errorf("chat: clipboard unavailable: %w", err)
	}
	if strings.TrimSpace(content) == "" {
		return ClipResult{}, ErrClipEmpty
	}
	if !force && IsClipSensitive(content) {
		return ClipResult{}, ErrClipSensitive
	}
	if query == "" {
		query = "ringkes/jelaskan isi clipboard ini"
	}

	sysprompt := config.PersonaChatLong + " Konteks dari clipboard user, jawab sesuai permintaan."
	userMsg := fmt.Sprintf("Clipboard:\n%s\n\nPermintaan: %s", content, query)
	res, err := s.Requester.Complete(ctx, sysprompt, userMsg, config.TaskSmart, config.TaskProviderOrder, 0)
	if err != nil {
		return ClipResult{}, err
	}
	thought, answer := SplitReply(res.Content)
	result := ClipResult{ChatResult: ChatResult{Thought: thought, Answer: answer, Raw: res.Content, Provider: res.Provider, Model: res.Model}}

	if answer != "" {
		if err := clipboard.Set(ctx, answer); err == nil {
			result.CopiedBack = true
		}
	}
	return result, nil
}
