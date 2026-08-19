// Ported from 30-luna/10-core/60-session_trim.zsh (_ai_trim_session).
package llmclient

// TrimSession ports _ai_trim_session's jq filter
// `[.[0]] + ((.[1:] | .[-($max-1):]) as $tail | if ($tail|length)>0 and
// $tail[0].role=="assistant" then $tail[1:] else $tail end)` field for
// field:
//
//   - No-op if len(messages) <= maxMsgs (the zsh source only rewrites the
//     file when `$len -gt $AI_SESSION_MAX_MSGS`).
//   - messages[0] (the system message) is always kept, untouched, and
//     never counted against the trailing slice below.
//   - The remaining messages[1:] are trimmed to their last (maxMsgs-1)
//     elements.
//   - RC-010/BUG-005 role-aware fixup (see the zsh source's own comment,
//     reproduced in full there): the two callers of this function feed
//     it histories with different [1:] parities --
//     _ai_session_ask always appends [user,assistant] pairs, so [1:] is
//     always even-length and a maxMsgs-1 (odd, default 29) slice from
//     the end can land on an "assistant" element first, corrupting the
//     role alternation once resent to a provider. The agent loop appends
//     [assistant,user] steps onto an initial [system,user], so its [1:]
//     is always odd-length and never hits this case. Rather than special
//     -case which caller is calling, the fix applies uniformly: if the
//     trimmed tail's first element has role "assistant", drop one more
//     from the front so it starts with "user" again -- a no-op for the
//     agent-loop shape, a real fix for the chat shape.
//
// maxMsgs<=0 is treated as "no trimming" (matches the zsh source never
// being invoked with AI_SESSION_MAX_MSGS<=0 in practice, but avoids a
// panic on a negative/zero slice bound here rather than silently
// mirroring undefined zsh arithmetic).
func TrimSession(messages []Message, maxMsgs int) []Message {
	if maxMsgs <= 0 || len(messages) <= maxMsgs {
		return messages
	}
	if len(messages) == 0 {
		return messages
	}

	rest := messages[1:]
	tailLen := maxMsgs - 1
	if tailLen < 0 {
		tailLen = 0
	}
	if tailLen > len(rest) {
		tailLen = len(rest)
	}
	tail := rest[len(rest)-tailLen:]

	if len(tail) > 0 && tail[0].Role == "assistant" {
		tail = tail[1:]
	}

	out := make([]Message, 0, 1+len(tail))
	out = append(out, messages[0])
	out = append(out, tail...)
	return out
}
