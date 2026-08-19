package chat

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/monang404/luna-go/internal/aiops"
)

// defaultSessionArchiveDays mirrors ${AI_SESSION_ARCHIVE_DAYS:-30}.
const defaultSessionArchiveDays = 30

// SessionPruneResult reports how many archived session files were
// removed.
type SessionPruneResult struct {
	Removed int
}

// SessionPrune mirrors _ai_session_prune(days): delete archived session
// files (store.Dir/archive/*.json) older than days. days<=0 uses
// defaultSessionArchiveDays. A missing archive dir is a silent no-op
// success, matching `[ -d "$AI_SESSION_DIR/archive" ] || return 0`.
func (st *SessionStore) SessionPrune(days int) (SessionPruneResult, error) {
	if days <= 0 {
		days = defaultSessionArchiveDays
	}
	archiveDir := filepath.Join(st.Dir, "archive")
	entries, err := os.ReadDir(archiveDir)
	if os.IsNotExist(err) {
		return SessionPruneResult{}, nil
	}
	if err != nil {
		return SessionPruneResult{}, err
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	removed := 0
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(filepath.Join(archiveDir, e.Name())); err == nil {
				removed++
			}
		}
	}
	return SessionPruneResult{Removed: removed}, nil
}

// End mirrors `luna session end [name]`: archive name's session file
// (mkdir archive, move file to archive/<name>_<ts>.json) and auto-prune
// the archive at the default retention window, matching the zsh
// source's bug #33 fix comment. name=="" mirrors
// `${1:-$AI_CURRENT_SESSION}` being empty -- callers own tracking
// "current session" (there is no process-wide env var equivalent in
// this package); an empty name is simply rejected here.
func (st *SessionStore) End(name string) error {
	if name == "" {
		return fmt.Errorf("chat: no active session to end")
	}
	archiveDir := filepath.Join(st.Dir, "archive")
	if err := os.MkdirAll(archiveDir, 0o755); err != nil {
		return err
	}
	src := st.path(name)
	dst := filepath.Join(archiveDir, name+"_"+aiops.Timestamp()+".json")
	if err := os.Rename(src, dst); err != nil {
		if os.IsNotExist(err) {
			return nil // matches the zsh source's `2>/dev/null` swallow
		}
		return err
	}
	_, _ = st.SessionPrune(defaultSessionArchiveDays)
	return nil
}

// Start mirrors `luna session start [name]`: (re)create name's session
// file from scratch (persona-only), ready for a REPL/Ask caller to use.
func (st *SessionStore) Start(name string) error {
	if name == "" {
		name = "main"
	}
	return st.Clear(name)
}
