package permission

import "sync"

// AskFunc is the interactive ask-confirmation hook the session's scope
// calls out as an interface with "implementasi nyata di UI layer nanti"
// (SESSION-52/53): given a human-readable prompt (the equivalent of
// _ai_perm_ask's $msg, shown *after* the caller's own approval box), it
// returns whether the user approved, or an error (e.g. the 60-second
// read timeout _ai_perm_ask enforces against /dev/tty). CheckPermission
// never assumes a particular terminal/UI implementation -- callers inject
// one, and tests inject a fake.
type AskFunc func(prompt string) (bool, error)

// ApprovalTracker holds the "ask_once_per_file" dedup state
// 25-perm_write.zsh keeps in the _AI_SESSION_APPROVED assoc array, keyed
// by "${_AI_AGENT_SESSION_SLUG}|${file_path}". That file's own comment
// (FIX BUG-7) is explicit that this must NOT be global/shared across
// parallel sessions; here that's expressed by requiring a sessionID on
// every call instead of a package-level map, so two AgentContexts (or two
// ApprovalTracker instances) never see each other's approvals even if a
// caller mistakenly reuses one tracker across sessions.
type ApprovalTracker struct {
	mu       sync.Mutex
	approved map[string]bool
}

// NewApprovalTracker returns an empty tracker, one per agent-loop run
// (mirrors _AI_SESSION_APPROVED starting empty each time aiagent() begins
// a new session).
func NewApprovalTracker() *ApprovalTracker {
	return &ApprovalTracker{approved: make(map[string]bool)}
}

func approvalKey(sessionID, path string) string {
	return sessionID + "|" + path
}

// IsApproved reports whether path was already approved for write in this
// session.
func (t *ApprovalTracker) IsApproved(sessionID, path string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.approved[approvalKey(sessionID, path)]
}

// Approve records path as approved for write for the rest of this
// session.
func (t *ApprovalTracker) Approve(sessionID, path string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.approved[approvalKey(sessionID, path)] = true
}
