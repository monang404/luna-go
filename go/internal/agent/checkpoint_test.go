package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/monang404/luna-go/internal/llmclient"
)

func TestCheckpointSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	msgs := []llmclient.Message{
		{Role: "system", Content: "sysprompt"},
		{Role: "user", Content: "fix the bug"},
		{Role: "assistant", Content: `{"thought":"ok","tool":"run_command"}`},
	}
	cp := NewCheckpoint("fix_the_bug", "fix the bug", 3, msgs)
	if err := store.Save(cp); err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := store.Load("fix_the_bug")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if loaded.Goal != cp.Goal {
		t.Errorf("Goal = %q, want %q", loaded.Goal, cp.Goal)
	}
	if loaded.Step != cp.Step {
		t.Errorf("Step = %d, want %d", loaded.Step, cp.Step)
	}
	if loaded.SessionID != cp.SessionID {
		t.Errorf("SessionID = %q, want %q", loaded.SessionID, cp.SessionID)
	}
	if loaded.SchemaVersion != CurrentCheckpointSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", loaded.SchemaVersion, CurrentCheckpointSchemaVersion)
	}
	if loaded.Revision != 1 {
		t.Errorf("Revision = %d, want 1", loaded.Revision)
	}
	if loaded.UpdatedAt == "" {
		t.Errorf("UpdatedAt is empty")
	}
	if len(loaded.Messages) != len(msgs) {
		t.Fatalf("Messages len = %d, want %d", len(loaded.Messages), len(msgs))
	}
	for i := range msgs {
		if loaded.Messages[i] != msgs[i] {
			t.Errorf("Messages[%d] = %+v, want %+v", i, loaded.Messages[i], msgs[i])
		}
	}
}

func TestCheckpointRoundTrip_EmptyMessages(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cp := NewCheckpoint("empty_goal", "empty goal", 0, nil)
	if err := store.Save(cp); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	loaded, err := store.Load("empty_goal")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.Messages == nil {
		t.Fatalf("Messages should be an empty array, not nil, after round-trip")
	}
	if len(loaded.Messages) != 0 {
		t.Fatalf("Messages len = %d, want 0", len(loaded.Messages))
	}
}

func TestCheckpointRevisionIncrements(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	for i, want := range []int{1, 2, 3} {
		cp := NewCheckpoint("sess", "goal", i, nil)
		if err := store.Save(cp); err != nil {
			t.Fatalf("Save() #%d error: %v", i, err)
		}
		if cp.Revision != want {
			t.Errorf("save #%d: Revision = %d, want %d", i, cp.Revision, want)
		}
		loaded, err := store.Load("sess")
		if err != nil {
			t.Fatalf("Load() #%d error: %v", i, err)
		}
		if loaded.Revision != want {
			t.Errorf("load #%d: Revision = %d, want %d", i, loaded.Revision, want)
		}
	}
}

func TestCheckpointLoad_Rejections(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name+".json"), []byte(content), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
	}

	tests := []struct {
		name    string
		content string
	}{
		{"invalid_json", `{not valid json`},
		{"missing_schema_version", `{"goal":"g","step":0,"messages":[]}`},
		{"unsupported_schema_version", `{"schema_version":1,"goal":"g","step":0,"messages":[]}`},
		{"missing_goal", `{"schema_version":2,"step":0,"messages":[]}`},
		{"empty_goal", `{"schema_version":2,"goal":"","step":0,"messages":[]}`},
		{"wrong_goal_type", `{"schema_version":2,"goal":123,"step":0,"messages":[]}`},
		{"missing_messages", `{"schema_version":2,"goal":"g","step":0}`},
		{"wrong_messages_type", `{"schema_version":2,"goal":"g","step":0,"messages":"nope"}`},
		{"invalid_step_negative", `{"schema_version":2,"goal":"g","step":-1,"messages":[]}`},
		{"wrong_step_type", `{"schema_version":2,"goal":"g","step":"three","messages":[]}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			write(tt.name, tt.content)
			if _, err := store.Load(tt.name); err == nil {
				t.Errorf("Load(%s) expected error, got nil", tt.name)
			}
		})
	}
}

func TestCheckpointLoad_NotFound(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if _, err := store.Load("nope"); err == nil {
		t.Fatal("expected error loading nonexistent checkpoint")
	}
}

func TestCheckpointDirectoryCreatedIfMissing(t *testing.T) {
	base := t.TempDir()
	dir := filepath.Join(base, "nested", "checkpoints")
	store := NewStore(dir)
	cp := NewCheckpoint("s", "goal", 0, nil)
	if err := store.Save(cp); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("expected dir to exist: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("%s is not a directory", dir)
	}
}

func TestCheckpointPermissions(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cp := NewCheckpoint("s", "goal", 0, nil)
	if err := store.Save(cp); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("dir perm = %o, want 700", perm)
	}
	fileInfo, err := os.Stat(filepath.Join(dir, "s.json"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != 0o600 {
		t.Errorf("file perm = %o, want 600", perm)
	}
}

func TestCheckpointPathTraversalRejected(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	bad := []string{
		"../escape",
		"../../etc/passwd",
		"a/b",
		"a\\b",
		"",
		"has space",
		"UPPER",
	}
	for _, id := range bad {
		t.Run(id, func(t *testing.T) {
			cp := NewCheckpoint(id, "goal", 0, nil)
			if err := store.Save(cp); err == nil {
				t.Errorf("Save() with session id %q expected error, got nil", id)
			}
			if _, err := store.Load(id); err == nil {
				t.Errorf("Load() with session id %q expected error, got nil", id)
			}
		})
	}

	// Confirm no file escaped the checkpoint dir.
	entries, _ := os.ReadDir(filepath.Dir(dir))
	for _, e := range entries {
		if e.Name() == "escape.json" || e.Name() == "passwd.json" {
			t.Fatalf("path traversal escaped checkpoint dir: found %s", e.Name())
		}
	}
}

func TestCheckpointAtomicSave_NoPartialFileOnMarshalPath(t *testing.T) {
	// Save should never leave a stray .tmp-* file behind on success.
	dir := t.TempDir()
	store := NewStore(dir)
	cp := NewCheckpoint("s", "goal", 0, nil)
	if err := store.Save(cp); err != nil {
		t.Fatalf("Save() error: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 || entries[0].Name() != "s.json" {
		t.Fatalf("unexpected directory contents after Save: %v", entries)
	}
}

func TestCheckpointSave_MissingGoalRejected(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	cp := NewCheckpoint("s", "", 0, nil)
	if err := store.Save(cp); err == nil {
		t.Fatal("expected error saving checkpoint with empty goal")
	}
}

func TestCheckpointValidate(t *testing.T) {
	tests := []struct {
		name    string
		cp      *Checkpoint
		wantErr bool
	}{
		{"nil", nil, true},
		{
			"valid",
			&Checkpoint{SchemaVersion: 2, SessionID: "s", Goal: "g", Step: 0, Messages: []llmclient.Message{}},
			false,
		},
		{
			"wrong schema",
			&Checkpoint{SchemaVersion: 1, SessionID: "s", Goal: "g", Messages: []llmclient.Message{}},
			true,
		},
		{
			"missing goal",
			&Checkpoint{SchemaVersion: 2, SessionID: "s", Messages: []llmclient.Message{}},
			true,
		},
		{
			"negative step",
			&Checkpoint{SchemaVersion: 2, SessionID: "s", Goal: "g", Step: -1, Messages: []llmclient.Message{}},
			true,
		},
		{
			"nil messages",
			&Checkpoint{SchemaVersion: 2, SessionID: "s", Goal: "g"},
			true,
		},
		{
			"missing session id",
			&Checkpoint{SchemaVersion: 2, Goal: "g", Messages: []llmclient.Message{}},
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cp.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMigrateLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)

	legacyDir := t.TempDir()
	legacyPath := filepath.Join(legacyDir, "old.json")
	legacy := map[string]any{
		"goal": "legacy goal",
		"step": 2,
		"messages": []map[string]string{
			{"role": "user", "content": "hi"},
		},
	}
	data, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("marshal legacy fixture: %v", err)
	}
	if err := os.WriteFile(legacyPath, data, 0o600); err != nil {
		t.Fatalf("write legacy fixture: %v", err)
	}

	migrated, err := MigrateLegacyJSON(store, "migrated_session", legacyPath)
	if err != nil {
		t.Fatalf("MigrateLegacyJSON() error: %v", err)
	}
	if migrated.Goal != "legacy goal" {
		t.Errorf("migrated.Goal = %q, want %q", migrated.Goal, "legacy goal")
	}
	if migrated.SchemaVersion != CurrentCheckpointSchemaVersion {
		t.Errorf("migrated.SchemaVersion = %d, want %d", migrated.SchemaVersion, CurrentCheckpointSchemaVersion)
	}

	loaded, err := store.Load("migrated_session")
	if err != nil {
		t.Fatalf("Load() after migration error: %v", err)
	}
	if loaded.Goal != "legacy goal" || loaded.Step != 2 {
		t.Errorf("loaded after migration = %+v, want goal=legacy goal step=2", loaded)
	}
}

func TestMigrateLegacyJSON_AlreadyCurrentSchemaRejected(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	legacyDir := t.TempDir()
	path := filepath.Join(legacyDir, "current.json")
	os.WriteFile(path, []byte(`{"schema_version":2,"goal":"g","step":0,"messages":[]}`), 0o600)
	if _, err := MigrateLegacyJSON(store, "s", path); err == nil {
		t.Fatal("expected error migrating an already-current-schema checkpoint")
	}
}

func TestMigrateLegacyJSON_DoesNotCrashOnMissingFile(t *testing.T) {
	dir := t.TempDir()
	store := NewStore(dir)
	if _, err := MigrateLegacyJSON(store, "s", filepath.Join(dir, "does-not-exist.json")); err == nil {
		t.Fatal("expected error for missing legacy file")
	}
}
