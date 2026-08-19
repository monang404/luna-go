// Ported from the checkpoint-persistence subset of
// 30-luna/50-agent/10-state.zsh (_ai_agent_checkpoint_save) and
// 30-luna/50-agent/40-runtime/10-load_checkpoint.zsh
// (_ai_agent_load_checkpoint).
//
// Checkpoint schema, field-for-field, is a literal port of the jq
// template in _ai_agent_checkpoint_save:
//
//	{schema_version:2, revision:$r, session_id:$sid, updated_at:$ts,
//	 goal:$g, step:$s, messages:$m[0]}
//
// Deliberately NOT ported: the flock-style mkdir lock directory
// (checkpoint_file.lock, owner-pid staleness detection) that guards
// concurrent writers in the zsh version. Per session rule 14 ("jangan
// memperkenalkan lock mechanism kompleks kecuali memang diperlukan"),
// this package instead relies on temp-file-then-rename atomicity alone,
// which is sufficient for the single-process Go agent this package is
// built for; SESSION-50 (or later) can add cross-process locking if a
// concrete concurrent-writer requirement shows up.
//
// Deliberately NOT included in the Checkpoint struct: a Phase/lifecycle
// field. The zsh checkpoint JSON never contains lifecycle_state -- that
// lives in a separate flat file ($state_dir/lifecycle_state), a
// different persistence mechanism entirely from the resume checkpoint.
// Inventing a Phase field on Checkpoint would be exactly the kind of
// YAML-driven fabrication rule 8/9 warns against: the actual source is
// authoritative here, and the actual source does not persist phase in
// the checkpoint.
package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/monang404/luna-go/internal/llmclient"
)

// CurrentCheckpointSchemaVersion is the only checkpoint schema version
// this package writes or accepts on Load. It mirrors the literal `2` in
// _ai_agent_checkpoint_save's jq template and the `== 2` check in
// _ai_agent_load_checkpoint. Kept as a single named constant per rule 17
// ("Jangan hardcode angka 2 di banyak tempat").
const CurrentCheckpointSchemaVersion = 2

// Checkpoint is the on-disk resume-checkpoint schema. Field order/names
// match the zsh jq template exactly (see package doc comment).
type Checkpoint struct {
	SchemaVersion int                 `json:"schema_version"`
	Revision      int                 `json:"revision"`
	SessionID     string              `json:"session_id"`
	UpdatedAt     string              `json:"updated_at"`
	Goal          string              `json:"goal"`
	Step          int                 `json:"step"`
	Messages      []llmclient.Message `json:"messages"`
}

// NewCheckpoint builds a Checkpoint ready to hand to Store.Save.
// SchemaVersion/Revision/UpdatedAt are filled in by Save (Revision is
// derived from any existing on-disk checkpoint, exactly like the zsh
// source reading `.revision // 0` before incrementing -- so the value
// passed here is intentionally not settable directly).
func NewCheckpoint(sessionID, goal string, step int, messages []llmclient.Message) *Checkpoint {
	if messages == nil {
		messages = []llmclient.Message{}
	}
	return &Checkpoint{
		SessionID: sessionID,
		Goal:      goal,
		Step:      step,
		Messages:  messages,
	}
}

// Validate checks the structural invariants _ai_agent_load_checkpoint
// enforces via its jq guard:
//
//	(.schema_version // 1) == 2 and (.goal | type == "string") and (.messages | type == "array")
//
// plus the step-is-a-non-negative-integer check the loader performs via
// `[[ "$step_offset" =~ ^[0-9]+$ ]] || step_offset=0` (the zsh loader
// silently coerces a bad step to 0 instead of rejecting; this Go port
// instead rejects explicitly, since silent fallback to a zero-value
// state is exactly what rule 16 says Load must not do).
func (c *Checkpoint) Validate() error {
	if c == nil {
		return errors.New("agent: nil checkpoint")
	}
	if c.SchemaVersion != CurrentCheckpointSchemaVersion {
		return fmt.Errorf("agent: unsupported checkpoint schema_version %d (want %d)", c.SchemaVersion, CurrentCheckpointSchemaVersion)
	}
	if c.Goal == "" {
		return errors.New("agent: checkpoint missing goal")
	}
	if c.Step < 0 {
		return fmt.Errorf("agent: checkpoint has invalid step %d", c.Step)
	}
	if c.Messages == nil {
		return errors.New("agent: checkpoint missing messages array")
	}
	if c.SessionID == "" {
		return errors.New("agent: checkpoint missing session_id")
	}
	return nil
}

// sessionIDPattern mirrors _ai_agent_slug's output alphabet
// (`tr '[:upper:]' '[:lower:]' | tr -cs 'a-z0-9' '_'`): lowercase
// letters, digits, and underscores. Store enforces this on every
// caller-supplied session ID (not just slugs Go itself generated) so a
// caller can never smuggle a path-traversal segment ("../../foo") or a
// path separator into the checkpoint filename (rule 24).
var sessionIDPattern = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)

// SafeSessionID validates id against sessionIDPattern and returns it
// unchanged, or an error if id is empty, too long, or contains anything
// that could escape the checkpoint directory.
func SafeSessionID(id string) (string, error) {
	if !sessionIDPattern.MatchString(id) {
		return "", fmt.Errorf("agent: invalid session id %q", id)
	}
	return id, nil
}

// Store persists checkpoints as JSON files under Dir, one file per
// session ID (`<Dir>/<session_id>.json`), matching
// `$AI_AGENT_CHECKPOINT_DIR/<slug>.json` in the zsh source.
type Store struct {
	Dir string
}

// NewStore returns a Store rooted at dir. dir is not created until the
// first Save.
func NewStore(dir string) *Store {
	return &Store{Dir: dir}
}

func (s *Store) pathFor(sessionID string) (string, error) {
	safe, err := SafeSessionID(sessionID)
	if err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, safe+".json"), nil
}

// Save writes cp to disk, atomically, under Store.Dir. It mutates cp in
// place to reflect what was actually written: SchemaVersion is forced to
// CurrentCheckpointSchemaVersion, Revision is set to
// (existing on-disk revision, or 0 if none/unreadable) + 1 -- mirroring
// `revision=$(jq -r '.revision // 0' "$checkpoint_file" 2>/dev/null); revision=$((revision+1))`
// -- and UpdatedAt is set to the current time.
//
// The write itself goes: temp file in the same directory -> fsync ->
// rename over the final path, so a process death mid-write can never
// leave a torn/partial checkpoint at the canonical path (rule 14).
// Directory permissions are 0700 and the file is 0600, mirroring
// `mkdir -p ...; chmod 700 ...` / `chmod 600 -- "$tmp"` in
// _ai_agent_checkpoint_save.
func (s *Store) Save(cp *Checkpoint) error {
	if cp == nil {
		return errors.New("agent: nil checkpoint")
	}
	path, err := s.pathFor(cp.SessionID)
	if err != nil {
		return err
	}
	if cp.Goal == "" {
		return errors.New("agent: checkpoint missing goal")
	}
	if cp.Step < 0 {
		return fmt.Errorf("agent: checkpoint has invalid step %d", cp.Step)
	}

	if err := os.MkdirAll(s.Dir, 0o700); err != nil {
		return fmt.Errorf("agent: create checkpoint dir: %w", err)
	}
	_ = os.Chmod(s.Dir, 0o700)

	revision := 0
	if existing, err := os.ReadFile(path); err == nil {
		var prior Checkpoint
		if jsonErr := json.Unmarshal(existing, &prior); jsonErr == nil {
			revision = prior.Revision
		}
	}

	cp.SchemaVersion = CurrentCheckpointSchemaVersion
	cp.Revision = revision + 1
	cp.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if cp.Messages == nil {
		cp.Messages = []llmclient.Message{}
	}

	data, err := json.MarshalIndent(cp, "", "  ")
	if err != nil {
		return fmt.Errorf("agent: marshal checkpoint: %w", err)
	}

	tmp, err := os.CreateTemp(s.Dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("agent: create temp checkpoint file: %w", err)
	}
	tmpPath := tmp.Name()
	cleanupTmp := true
	defer func() {
		if cleanupTmp {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("agent: write temp checkpoint file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("agent: fsync temp checkpoint file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("agent: close temp checkpoint file: %w", err)
	}
	if err := os.Chmod(tmpPath, 0o600); err != nil {
		return fmt.Errorf("agent: chmod temp checkpoint file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("agent: rename checkpoint into place: %w", err)
	}
	cleanupTmp = false
	return nil
}

// Delete removes the on-disk checkpoint for sessionID, if any. It
// mirrors 44-finalize.zsh's `rm -f -- "$checkpoint_file"` on a verified
// COMPLETE run: a checkpoint only exists to resume an unfinished run, so
// once the lifecycle reaches COMPLETE it has no further purpose. Like
// `rm -f`, deleting a checkpoint that does not exist is not an error.
func (s *Store) Delete(sessionID string) error {
	path, err := s.pathFor(sessionID)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("agent: delete checkpoint: %w", err)
	}
	return nil
}

// Load reads and validates the checkpoint for sessionID. It rejects
// (rather than silently falling back to a zero-value state, rule 16):
// invalid JSON, an unsupported/missing schema_version, a missing or
// non-string goal, a missing or non-array messages field, and a negative
// step.
func (s *Store) Load(sessionID string) (*Checkpoint, error) {
	path, err := s.pathFor(sessionID)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("agent: checkpoint %q not found", sessionID)
		}
		return nil, fmt.Errorf("agent: read checkpoint: %w", err)
	}
	return decodeCheckpoint(data)
}

// decodeCheckpoint performs a two-pass decode: first into a generic map
// so field *presence and type* can be checked explicitly (Go's struct
// unmarshal alone can't distinguish "goal absent" from "goal absent and
// defaulted to \"\"", which is exactly the silent-fallback rule 16
// forbids), then into the typed Checkpoint struct for the return value.
func decodeCheckpoint(data []byte) (*Checkpoint, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("agent: invalid checkpoint JSON: %w", err)
	}

	svRaw, ok := raw["schema_version"]
	if !ok {
		return nil, errors.New("agent: checkpoint missing schema_version")
	}
	var sv int
	if err := json.Unmarshal(svRaw, &sv); err != nil {
		return nil, fmt.Errorf("agent: checkpoint schema_version is not a number: %w", err)
	}
	if sv != CurrentCheckpointSchemaVersion {
		return nil, fmt.Errorf("agent: unsupported checkpoint schema_version %d (want %d)", sv, CurrentCheckpointSchemaVersion)
	}

	goalRaw, ok := raw["goal"]
	if !ok {
		return nil, errors.New("agent: checkpoint missing goal")
	}
	var goal string
	if err := json.Unmarshal(goalRaw, &goal); err != nil {
		return nil, fmt.Errorf("agent: checkpoint goal is not a string: %w", err)
	}
	if goal == "" {
		return nil, errors.New("agent: checkpoint has empty goal")
	}

	msgsRaw, ok := raw["messages"]
	if !ok {
		return nil, errors.New("agent: checkpoint missing messages")
	}
	var msgs []llmclient.Message
	if err := json.Unmarshal(msgsRaw, &msgs); err != nil {
		return nil, fmt.Errorf("agent: checkpoint messages is not an array: %w", err)
	}

	step := 0
	if stepRaw, ok := raw["step"]; ok {
		if err := json.Unmarshal(stepRaw, &step); err != nil {
			return nil, fmt.Errorf("agent: checkpoint step is not a number: %w", err)
		}
		if step < 0 {
			return nil, fmt.Errorf("agent: checkpoint has invalid step %d", step)
		}
	}

	cp := &Checkpoint{
		SchemaVersion: sv,
		Goal:          goal,
		Step:          step,
		Messages:      msgs,
	}
	if revRaw, ok := raw["revision"]; ok {
		_ = json.Unmarshal(revRaw, &cp.Revision)
	}
	if sidRaw, ok := raw["session_id"]; ok {
		_ = json.Unmarshal(sidRaw, &cp.SessionID)
	}
	if tsRaw, ok := raw["updated_at"]; ok {
		_ = json.Unmarshal(tsRaw, &cp.UpdatedAt)
	}
	return cp, nil
}

// -----------------------------------------------------------------------
// Legacy checkpoint migration
// -----------------------------------------------------------------------
//
// Evidence audit (rule 11/18): the repository's *only* trace of a
// checkpoint format other than the current schema_version:2 JSON shape
// is defensive code inside the current loader itself --
// `(.schema_version // 1) == 2` in
// 30-luna/50-agent/40-runtime/10-load_checkpoint.zsh -- which treats an
// on-disk checkpoint with schema_version absent as if it were
// schema_version 1, and then rejects it (since 1 != 2). No fixture, no
// CHANGELOG entry, and no other source file anywhere in this repository
// documents a shell var-dump (`typeset -p` / eval-based) checkpoint
// format ever existing; grepping the repo for "var-dump", "typeset -p",
// and "vardump" turns up nothing. Fabricating a var-dump parser under
// those conditions would violate rule 8/9 (no invented behavior) and
// rule 11 (no shell-assignment parser that could execute arbitrary
// code).
//
// What IS real and evidenced: a JSON checkpoint missing schema_version
// (or with schema_version 1) that otherwise has the same goal/step/
// messages shape. MigrateLegacyJSON handles exactly that best-effort
// case -- reading a schema_version-1-or-absent JSON object with a
// string goal and an array messages field, and re-writing it as a
// current schema_version:2 checkpoint via Store.Save (which assigns a
// fresh revision/updated_at). It never executes or evaluates shell
// syntax of any kind.
type LegacyCheckpoint struct {
	Goal     string              `json:"goal"`
	Step     int                 `json:"step"`
	Messages []llmclient.Message `json:"messages"`
}

// MigrateLegacyJSON reads a legacy (schema_version absent or == 1) JSON
// checkpoint file at legacyPath and saves it into store under sessionID
// as a current schema_version:2 checkpoint. It returns the migrated
// Checkpoint on success.
//
// This is intentionally the only migration path provided: there is no
// evidence in this repository of any non-JSON legacy checkpoint format,
// so none is implemented (see package doc comment above).
func MigrateLegacyJSON(store *Store, sessionID, legacyPath string) (*Checkpoint, error) {
	data, err := os.ReadFile(legacyPath)
	if err != nil {
		return nil, fmt.Errorf("agent: read legacy checkpoint: %w", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("agent: invalid legacy checkpoint JSON: %w", err)
	}
	if svRaw, ok := raw["schema_version"]; ok {
		var sv int
		if err := json.Unmarshal(svRaw, &sv); err == nil && sv == CurrentCheckpointSchemaVersion {
			return nil, errors.New("agent: checkpoint is already schema_version 2, nothing to migrate")
		}
	}

	var legacy LegacyCheckpoint
	if err := json.Unmarshal(data, &legacy); err != nil {
		return nil, fmt.Errorf("agent: legacy checkpoint does not match expected shape: %w", err)
	}
	if legacy.Goal == "" {
		return nil, errors.New("agent: legacy checkpoint missing goal")
	}
	if legacy.Messages == nil {
		legacy.Messages = []llmclient.Message{}
	}

	cp := NewCheckpoint(sessionID, legacy.Goal, legacy.Step, legacy.Messages)
	if err := store.Save(cp); err != nil {
		return nil, fmt.Errorf("agent: save migrated checkpoint: %w", err)
	}
	return cp, nil
}
