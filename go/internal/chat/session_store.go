package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
)

// SessionStore owns reading/writing session JSON files under
// AI_SESSION_DIR (config.Limits.SessionDir), matching 20-chat/10-
// session_ask.zsh + 20-session_mgmt.zsh's direct file manipulation.
type SessionStore struct {
	Dir string
	// Log, if set, is called after every completed turn (session:<name>,
	// user message, assistant reply) -- the injectable stand-in for
	// _ai_log (10-core, not part of SESSION-54's scope; logging is a
	// no-op unless a caller wires this in).
	Log func(kind, request, response string)
}

// NewSessionStore builds a SessionStore rooted at the current
// environment's AI_SESSION_DIR.
func NewSessionStore() *SessionStore {
	paths := config.LoadPaths()
	return &SessionStore{Dir: paths.SessionDir}
}

func (st *SessionStore) path(name string) string {
	return filepath.Join(st.Dir, name+".json")
}

// labelPrefixRE mirrors _ai_session_sanitize_file's jq `sub(...)`
// pattern: one-or-more repeats of "<model-label> > " at the very start
// of an assistant message, left behind by pre-fix versions that let
// terminal presentation labels leak into the persisted content.
var labelPrefixRE = regexp.MustCompile(`^((llama|gpt-oss|gemini|qwen|deepseek|glm)\s*>\s*)+`)

// Load reads a session file, creating it (with just the chat-long
// persona as the system message) if it doesn't exist yet -- mirroring
// every call site's own `[ -f "$file" ] || jq -n ... > "$file"` guard.
// The returned messages are already sanitized (labelPrefixRE stripped
// from any assistant message), matching
// _ai_session_sanitize_file being run unconditionally before use.
func (st *SessionStore) Load(name string) ([]llmclient.Message, error) {
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		return nil, err
	}
	p := st.path(name)
	data, err := os.ReadFile(p)
	if os.IsNotExist(err) {
		msgs := []llmclient.Message{{Role: "system", Content: config.PersonaChatLong}}
		if err := st.save(name, msgs); err != nil {
			return nil, err
		}
		return msgs, nil
	}
	if err != nil {
		return nil, err
	}
	var msgs []llmclient.Message
	if err := json.Unmarshal(data, &msgs); err != nil {
		return nil, err
	}
	changed := false
	for i := range msgs {
		if msgs[i].Role != "assistant" {
			continue
		}
		cleaned := labelPrefixRE.ReplaceAllString(msgs[i].Content, "")
		if cleaned != msgs[i].Content {
			msgs[i].Content = cleaned
			changed = true
		}
	}
	if changed {
		if err := st.save(name, msgs); err != nil {
			return nil, err
		}
	}
	return msgs, nil
}

// save writes messages to name's session file as pretty JSON (matching
// jq's default 2-space indent output).
func (st *SessionStore) save(name string, msgs []llmclient.Message) error {
	if err := os.MkdirAll(st.Dir, 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(msgs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(st.path(name), data, 0o644)
}

// Exists reports whether name has a session file.
func (st *SessionStore) Exists(name string) bool {
	_, err := os.Stat(st.path(name))
	return err == nil
}

// List mirrors `luna session list`: every "*.json" basename (without
// extension) directly under the session dir, sorted -- matching zsh
// glob order via `ls`. An empty (nil, nil) result matches the zsh
// source's silent `return 0` when there are no session files.
func (st *SessionStore) List() ([]string, error) {
	entries, err := os.ReadDir(st.Dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		names = append(names, strings.TrimSuffix(e.Name(), ".json"))
	}
	return names, nil
}

// Clear resets name's session back to just the chat-long persona system
// message, mirroring the REPL's /clear command.
func (st *SessionStore) Clear(name string) error {
	return st.save(name, []llmclient.Message{{Role: "system", Content: config.PersonaChatLong}})
}
