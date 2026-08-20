package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- helpers ---------------------------------------------------------------

// writeTempJSON writes a settings JSON file into dir/name and returns its path.
func writeTempJSON(t *testing.T, dir, name string, s *Settings) string {
	t.Helper()
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if sub := filepath.Dir(path); sub != dir {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- loadFile tests --------------------------------------------------------

func TestLoadFile_Missing(t *testing.T) {
	s, err := loadFile("/nonexistent/path/settings.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if s != nil {
		t.Fatalf("expected nil settings for missing file, got: %+v", s)
	}
}

func TestLoadFile_Valid(t *testing.T) {
	dir := t.TempDir()
	path := writeTempJSON(t, dir, "settings.json", &Settings{
		Model:       "sonnet",
		DefaultMode: ModeDefault,
	})

	s, err := loadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil settings")
	}
	if s.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", s.Model, "sonnet")
	}
}

func TestLoadFile_Malformed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{not json`), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := loadFile(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if s != nil {
		t.Fatal("expected nil settings for malformed JSON")
	}
}

// --- Merge tests -----------------------------------------------------------

func TestMerge_ScalarOverride(t *testing.T) {
	base := &Settings{
		Model:       "haiku",
		DefaultMode: ModeDefault,
	}
	override := &Settings{
		Model: "sonnet",
	}
	result := Merge(base, override)
	if result.Model != "sonnet" {
		t.Errorf("Model = %q, want %q", result.Model, "sonnet")
	}
	// DefaultMode not set in override, should keep base
	if result.DefaultMode != ModeDefault {
		t.Errorf("DefaultMode = %q, want %q", result.DefaultMode, ModeDefault)
	}
}

func TestMerge_ScalarBasePreservedWhenOverrideEmpty(t *testing.T) {
	base := &Settings{
		Model:       "opus",
		DefaultMode: ModePlan,
	}
	override := &Settings{}
	result := Merge(base, override)
	if result.Model != "opus" {
		t.Errorf("Model = %q, want %q", result.Model, "opus")
	}
	if result.DefaultMode != ModePlan {
		t.Errorf("DefaultMode = %q, want %q", result.DefaultMode, ModePlan)
	}
}

func TestMerge_ArrayUnion(t *testing.T) {
	base := &Settings{
		Permissions: Permissions{
			Allow: []string{"Read", "Glob"},
			Deny:  []string{"Bash(rm -rf *)"},
		},
	}
	override := &Settings{
		Permissions: Permissions{
			Allow: []string{"Grep", "Read"}, // "Read" duplicate should be deduped
			Ask:   []string{"Edit"},
		},
	}
	result := Merge(base, override)

	wantAllow := []string{"Read", "Glob", "Grep"}
	if !sliceEqual(result.Permissions.Allow, wantAllow) {
		t.Errorf("Allow = %v, want %v", result.Permissions.Allow, wantAllow)
	}

	wantDeny := []string{"Bash(rm -rf *)"}
	if !sliceEqual(result.Permissions.Deny, wantDeny) {
		t.Errorf("Deny = %v, want %v", result.Permissions.Deny, wantDeny)
	}

	wantAsk := []string{"Edit"}
	if !sliceEqual(result.Permissions.Ask, wantAsk) {
		t.Errorf("Ask = %v, want %v", result.Permissions.Ask, wantAsk)
	}
}

func TestMerge_EnvMapOverrideWins(t *testing.T) {
	base := &Settings{
		Env: map[string]string{
			"FOO": "base_val",
			"BAR": "base_bar",
		},
	}
	override := &Settings{
		Env: map[string]string{
			"FOO": "override_val",
			"BAZ": "new_val",
		},
	}
	result := Merge(base, override)
	if result.Env["FOO"] != "override_val" {
		t.Errorf("Env[FOO] = %q, want %q", result.Env["FOO"], "override_val")
	}
	if result.Env["BAR"] != "base_bar" {
		t.Errorf("Env[BAR] = %q, want %q", result.Env["BAR"], "base_bar")
	}
	if result.Env["BAZ"] != "new_val" {
		t.Errorf("Env[BAZ] = %q, want %q", result.Env["BAZ"], "new_val")
	}
}

func TestMerge_HooksAppend(t *testing.T) {
	base := &Settings{
		Hooks: HookSet{
			PreToolUse: []HookDef{
				{Matcher: "Bash", Command: "echo base"},
			},
		},
	}
	override := &Settings{
		Hooks: HookSet{
			PreToolUse: []HookDef{
				{Matcher: "*", Command: "echo override"},
			},
			PostToolUse: []HookDef{
				{Matcher: "Edit", Command: "lint-check"},
			},
		},
	}
	result := Merge(base, override)
	if len(result.Hooks.PreToolUse) != 2 {
		t.Errorf("PreToolUse count = %d, want 2", len(result.Hooks.PreToolUse))
	}
	if len(result.Hooks.PostToolUse) != 1 {
		t.Errorf("PostToolUse count = %d, want 1", len(result.Hooks.PostToolUse))
	}
}

func TestMerge_NilInputs(t *testing.T) {
	// Merge(nil, nil) should not panic
	result := Merge(nil, nil)
	if result == nil {
		t.Fatal("expected non-nil result from Merge(nil, nil)")
	}

	// Merge(nil, override) should use override
	override := &Settings{Model: "test"}
	result = Merge(nil, override)
	if result.Model != "test" {
		t.Errorf("Model = %q, want %q", result.Model, "test")
	}

	// Merge(base, nil) should return base copy
	base := &Settings{Model: "base"}
	result = Merge(base, nil)
	if result.Model != "base" {
		t.Errorf("Model = %q, want %q", result.Model, "base")
	}
}

func TestMerge_AdditionalDirectories(t *testing.T) {
	base := &Settings{
		AdditionalDirectories: []string{"/home/user/shared"},
	}
	override := &Settings{
		AdditionalDirectories: []string{"/tmp/extra", "/home/user/shared"}, // dup
	}
	result := Merge(base, override)
	want := []string{"/home/user/shared", "/tmp/extra"}
	if !sliceEqual(result.AdditionalDirectories, want) {
		t.Errorf("AdditionalDirectories = %v, want %v", result.AdditionalDirectories, want)
	}
}

// --- Three-level load test -------------------------------------------------

func TestLoad_ThreeLevelMerge(t *testing.T) {
	// Set up a fake home dir
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	fakeHome := t.TempDir()
	os.Setenv("HOME", fakeHome)
	os.Setenv("USERPROFILE", fakeHome)
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("USERPROFILE", origUserProfile)
	}()

	projectRoot := t.TempDir()

	// Level 1: user settings
	userDir := filepath.Join(fakeHome, ".luna")
	os.MkdirAll(userDir, 0o755)
	writeTempJSON(t, userDir, "settings.json", &Settings{
		Model:       "haiku",
		DefaultMode: ModeDefault,
		Permissions: Permissions{
			Allow: []string{"Read", "Glob"},
		},
	})

	// Level 2: project settings
	projDir := filepath.Join(projectRoot, ".luna")
	os.MkdirAll(projDir, 0o755)
	writeTempJSON(t, projDir, "settings.json", &Settings{
		Model: "sonnet", // override user
		Permissions: Permissions{
			Allow: []string{"Grep"}, // adds to user's list
			Deny:  []string{"Bash(rm -rf *)"},
		},
	})

	// Level 3: local settings
	writeTempJSON(t, projDir, "settings.local.json", &Settings{
		DefaultMode: ModeAcceptEdits, // override user's default mode
		Permissions: Permissions{
			Allow: []string{"Edit"}, // adds to merged list
		},
	})

	result, err := Load(projectRoot)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	// Check scalar overrides
	if result.Model != "sonnet" {
		t.Errorf("Model = %q, want %q (project overrides user)", result.Model, "sonnet")
	}
	if result.DefaultMode != ModeAcceptEdits {
		t.Errorf("DefaultMode = %q, want %q (local overrides user)", result.DefaultMode, ModeAcceptEdits)
	}

	// Check array merges
	wantAllow := []string{"Read", "Glob", "Grep", "Edit"}
	if !sliceEqual(result.Permissions.Allow, wantAllow) {
		t.Errorf("Allow = %v, want %v", result.Permissions.Allow, wantAllow)
	}

	wantDeny := []string{"Bash(rm -rf *)"}
	if !sliceEqual(result.Permissions.Deny, wantDeny) {
		t.Errorf("Deny = %v, want %v", result.Permissions.Deny, wantDeny)
	}
}

func TestLoad_NoProjectRoot(t *testing.T) {
	// With empty projectRoot, should only load user-level
	origHome := os.Getenv("HOME")
	origUserProfile := os.Getenv("USERPROFILE")
	fakeHome := t.TempDir()
	os.Setenv("HOME", fakeHome)
	os.Setenv("USERPROFILE", fakeHome)
	defer func() {
		os.Setenv("HOME", origHome)
		os.Setenv("USERPROFILE", origUserProfile)
	}()

	result, err := Load("")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil settings")
	}
	// All zero values since no files exist
	if result.Model != "" {
		t.Errorf("Model = %q, want empty", result.Model)
	}
}

// --- ValidMode tests -------------------------------------------------------

func TestValidMode(t *testing.T) {
	tests := []struct {
		mode PermissionMode
		want bool
	}{
		{ModeDefault, true},
		{ModeAcceptEdits, true},
		{ModePlan, true},
		{ModeBypass, true},
		{"", true},
		{"invalid_mode", false},
		{"YOLO", false},
	}
	for _, tt := range tests {
		if got := ValidMode(tt.mode); got != tt.want {
			t.Errorf("ValidMode(%q) = %v, want %v", tt.mode, got, tt.want)
		}
	}
}

// --- mergeStringSlices tests -----------------------------------------------

func TestMergeStringSlices_Empty(t *testing.T) {
	result := mergeStringSlices(nil, nil)
	if result != nil {
		t.Errorf("expected nil for two nil inputs, got %v", result)
	}
}

func TestMergeStringSlices_Dedup(t *testing.T) {
	result := mergeStringSlices([]string{"a", "b"}, []string{"b", "c", "a"})
	want := []string{"a", "b", "c"}
	if !sliceEqual(result, want) {
		t.Errorf("result = %v, want %v", result, want)
	}
}

// --- helper ----------------------------------------------------------------

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
