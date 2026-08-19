package chat

import "strings"

// SplitReply mirrors _ai_chat_split_reply (20-chat/01-chat_display.zsh):
// split a raw model reply into reasoning (thought) and the final
// answer.
//
// Primary contract: the model puts reasoning before the literal marker
// "@@JAWABAN@@" (config.ChatAnswerMarker) and the clean answer after it.
// Fallback (older/smaller models that don't follow the marker
// instruction): a "**Thought**" heading with the answer BEFORE it,
// reasoning after -- the reverse order of the marker contract, kept for
// parity with the zsh source's own documented production-observed
// fallback. If neither pattern is found, the entire raw text is treated
// as the answer -- the answer is never dropped just because parsing
// didn't recognize a marker.
func SplitReply(raw string) (thought, answer string) {
	const marker = "@@JAWABAN@@"
	const thoughtHeading = "**Thought**"

	answer = raw
	switch {
	case strings.Contains(raw, marker):
		idx := strings.Index(raw, marker)
		thought = raw[:idx]
		answer = raw[idx+len(marker):]
	case strings.Contains(raw, thoughtHeading):
		idx := strings.Index(raw, thoughtHeading)
		answer = raw[:idx]
		thought = raw[idx+len(thoughtHeading):]
	}

	thought = strings.TrimSpace(thought)
	answer = strings.TrimSpace(answer)
	if answer == "" {
		answer = raw
	}
	return thought, answer
}
