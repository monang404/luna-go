package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// clearProviderKeys unsets every provider key var so each test starts from
// a clean slate regardless of what the host environment happens to have
// set (e.g. a real GROQ_API_KEY in a dev shell).
func clearProviderKeys(t *testing.T) {
	t.Helper()
	for _, p := range Providers() {
		t.Setenv(p.KeyVar, "")
		os.Unsetenv(p.KeyVar)
	}
}

// --- AC-01: provider without an API key in env is auto-skipped ---

func TestActiveProviders_SkipsMissingKey(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("GROQ_API_KEY", "test-groq-key")
	// GEMINI_API_KEY / CEREBRAS_API_KEY / DEEPSEEK_API_KEY left unset.

	got := ActiveProviders(ProviderOrder) // [groq gemini cerebras]
	want := []string{"groq"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ActiveProviders(ProviderOrder) = %v, want %v", got, want)
	}
}

func TestActiveProviders_IncludesSetKey(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("GROQ_API_KEY", "k1")
	t.Setenv("CEREBRAS_API_KEY", "k2")

	got := ActiveProviders(ProviderOrder) // order: groq gemini cerebras
	want := []string{"groq", "cerebras"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ActiveProviders(ProviderOrder) = %v, want %v", got, want)
	}
}

func TestActiveProviders_PreservesTaskOrder(t *testing.T) {
	clearProviderKeys(t)
	t.Setenv("GROQ_API_KEY", "k1")
	t.Setenv("DEEPSEEK_API_KEY", "k2")
	t.Setenv("CEREBRAS_API_KEY", "k3")

	// TaskProviderOrderSmart = [deepseek cerebras gemini groq]; gemini has
	// no key, so expect [deepseek cerebras groq] in that exact order.
	got := ActiveProviders(TaskProviderOrderSmart)
	want := []string{"deepseek", "cerebras", "groq"}

	if !reflect.DeepEqual(got, want) {
		t.Errorf("ActiveProviders(TaskProviderOrderSmart) = %v, want %v", got, want)
	}
}

func TestHasAnyKey(t *testing.T) {
	clearProviderKeys(t)
	if HasAnyKey(ProviderOrder) {
		t.Error("HasAnyKey() = true with no keys set, want false")
	}
	t.Setenv("GEMINI_API_KEY", "k")
	if !HasAnyKey(ProviderOrder) {
		t.Error("HasAnyKey() = false with GEMINI_API_KEY set, want true")
	}
}

func TestProviderOrder_ExcludesDeepseek(t *testing.T) {
	// Direct parity check against 35-providers.zsh:33 --
	// AI_PROVIDER_ORDER=(groq gemini cerebras), no deepseek.
	for _, name := range ProviderOrder {
		if name == "deepseek" {
			t.Errorf("ProviderOrder = %v must not contain deepseek (parity with 35-providers.zsh)", ProviderOrder)
		}
	}
	want := []string{"groq", "gemini", "cerebras"}
	if !reflect.DeepEqual(ProviderOrder, want) {
		t.Errorf("ProviderOrder = %v, want %v", ProviderOrder, want)
	}
}

// --- AC-02: default models match 35-providers.zsh exactly ---

func TestProviders_DefaultModelsAndEndpoints(t *testing.T) {
	clearProviderKeys(t)
	os.Unsetenv("GEMINI_MODEL")
	os.Unsetenv("CEREBRAS_MODEL")
	os.Unsetenv("DEEPSEEK_MODEL")

	want := map[string]struct {
		Endpoint string
		Model    string
	}{
		"groq":     {Endpoint: "https://api.groq.com/openai/v1/chat/completions", Model: "openai/gpt-oss-120b"},
		"gemini":   {Endpoint: "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions", Model: "gemini-flash-latest"},
		"cerebras": {Endpoint: "https://api.cerebras.ai/v1/chat/completions", Model: "gpt-oss-120b"},
		"deepseek": {Endpoint: "https://api.deepseek.com/chat/completions", Model: "deepseek-v4-flash"},
	}

	providers := Providers()
	for name, wantData := range want {
		p, ok := providers[name]
		if !ok {
			t.Errorf("Providers() missing %q", name)
			continue
		}
		if p.Model != wantData.Model {
			t.Errorf("Providers()[%q].Model = %q, want %q", name, p.Model, wantData.Model)
		}
		if p.Endpoint != wantData.Endpoint {
			t.Errorf("Providers()[%q].Endpoint = %q, want %q", name, p.Endpoint, wantData.Endpoint)
		}
	}
}

func TestProviders_ModelOverrideViaEnv(t *testing.T) {
	t.Setenv("GEMINI_MODEL", "gemini-custom")
	p := Providers()["gemini"]
	if p.Model != "gemini-custom" {
		t.Errorf("Providers()[gemini].Model = %q, want gemini-custom", p.Model)
	}
}

func TestModelsFor(t *testing.T) {
	got := ModelsFor("groq", TaskSmart)
	want := []string{"openai/gpt-oss-120b", "openai/gpt-oss-20b", "llama-3.3-70b-versatile", "llama-3.1-8b-instant"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ModelsFor(groq, smart) = %v, want %v", got, want)
	}

	if got := ModelsFor("nonexistent", TaskFast); got != nil {
		t.Errorf("ModelsFor(nonexistent, fast) = %v, want nil", got)
	}
}

// --- AC-03: secrets loader parses >=1 export line and sets env ---

func TestLoadSecrets_ParsesExportLines(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod 600 not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets.zsh")
	content := "# comment, should be skipped\n" +
		"\n" +
		"export GROQ_API_KEY=abc123\n" +
		"export GEMINI_API_KEY=\"quoted value\"\n" +
		"CEREBRAS_API_KEY='single-quoted'\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GROQ_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("CEREBRAS_API_KEY", "")

	set, warnings, err := LoadSecrets(path)
	if err != nil {
		t.Fatalf("LoadSecrets() error = %v", err)
	}
	if set != 3 {
		t.Errorf("LoadSecrets() set = %d, want 3", set)
	}
	if len(warnings) != 0 {
		t.Errorf("LoadSecrets() warnings = %v, want none (file is 0600)", warnings)
	}

	if got := os.Getenv("GROQ_API_KEY"); got != "abc123" {
		t.Errorf("GROQ_API_KEY = %q, want abc123", got)
	}
	if got := os.Getenv("GEMINI_API_KEY"); got != "quoted value" {
		t.Errorf("GEMINI_API_KEY = %q, want %q", got, "quoted value")
	}
	if got := os.Getenv("CEREBRAS_API_KEY"); got != "single-quoted" {
		t.Errorf("CEREBRAS_API_KEY = %q, want single-quoted", got)
	}
}

func TestLoadSecrets_MissingFileIsNotError(t *testing.T) {
	set, warnings, err := LoadSecrets(filepath.Join(t.TempDir(), "does-not-exist.zsh"))
	if err != nil {
		t.Fatalf("LoadSecrets() on missing file error = %v, want nil", err)
	}
	if set != 0 || warnings != nil {
		t.Errorf("LoadSecrets() on missing file = (%d, %v), want (0, nil)", set, warnings)
	}
}

func TestLoadSecrets_WarnsOnLoosePermissions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".secrets.zsh")
	if err := os.WriteFile(path, []byte("export FOO=bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FOO", "")

	_, warnings, err := LoadSecrets(path)
	if err != nil {
		t.Fatalf("LoadSecrets() error = %v", err)
	}
	if len(warnings) == 0 {
		t.Error("LoadSecrets() on 0644 file returned no warnings, want a permission warning")
	}
}

// --- Limits / Paths sanity (regression guard vs 15-limits.zsh / 10-paths.zsh) ---

func TestLoadLimits_Defaults(t *testing.T) {
	os.Unsetenv("AI_MAX_RETRIES")
	os.Unsetenv("AI_CURL_TIMEOUT")
	os.Unsetenv("AI_AGENT_MAX_TOKS")
	os.Unsetenv("AI_AGENT_AUTO_NPM_CHECK")

	l := LoadLimits()
	cases := map[string]int{
		"MaxRetries":              l.MaxRetries,
		"CurlTimeoutSec":          l.CurlTimeoutSec,
		"SessionMaxMsgs":          l.SessionMaxMsgs,
		"LogMaxLines":             l.LogMaxLines,
		"DiffMaxChars":            l.DiffMaxChars,
		"PatchMaxChars":           l.PatchMaxChars,
		"FileMaxChars":            l.FileMaxChars,
		"ProjectMaxToks":          l.ProjectMaxToks,
		"AgentMaxToks":            l.AgentMaxToks,
		"GrepMaxResults":          l.GrepMaxResults,
		"GitDiffMaxChars":         l.GitDiffMaxChars,
		"WebFetchMaxChars":        l.WebFetchMaxChars,
		"WebFetchTimeoutSec":      l.WebFetchTimeoutSec,
		"BatteryWarnPct":          l.BatteryWarnPct,
		"DailyTokenWarn":          l.DailyTokenWarn,
		"NotifyMinIntervalSec":    l.NotifyMinIntervalSec,
		"CircuitBreakerWindowSec": l.CircuitBreakerWindowSec,
		"AgentMaxSteps":           l.AgentMaxSteps,
		"AgentMaxSameFail":        l.AgentMaxSameFail,
	}
	want := map[string]int{
		"MaxRetries":              1,
		"CurlTimeoutSec":          45,
		"SessionMaxMsgs":          30,
		"LogMaxLines":             5000,
		"DiffMaxChars":            15000,
		"PatchMaxChars":           200000,
		"FileMaxChars":            40000,
		"ProjectMaxToks":          3500,
		"AgentMaxToks":            8000,
		"GrepMaxResults":          100,
		"GitDiffMaxChars":         6000,
		"WebFetchMaxChars":        8000,
		"WebFetchTimeoutSec":      15,
		"BatteryWarnPct":          15,
		"DailyTokenWarn":          150000,
		"NotifyMinIntervalSec":    3,
		"CircuitBreakerWindowSec": 30,
		"AgentMaxSteps":           15,
		"AgentMaxSameFail":        3,
	}
	for k, v := range want {
		if cases[k] != v {
			t.Errorf("LoadLimits().%s = %d, want %d", k, cases[k], v)
		}
	}
	if !l.DataSaverWarn {
		t.Error("LoadLimits().DataSaverWarn = false, want true (AI_DATA_SAVER_WARN=1)")
	}
	if l.AgentAutoNpmCheck {
		t.Error("LoadLimits().AgentAutoNpmCheck = true, want false (default off)")
	}
}

func TestLoadLimits_EnvOverride(t *testing.T) {
	t.Setenv("AI_MAX_RETRIES", "5")
	t.Setenv("AI_AGENT_MAX_TOKS", "4000")
	t.Setenv("AI_AGENT_AUTO_NPM_CHECK", "1")

	l := LoadLimits()
	if l.MaxRetries != 5 {
		t.Errorf("MaxRetries = %d, want 5", l.MaxRetries)
	}
	if l.AgentMaxToks != 4000 {
		t.Errorf("AgentMaxToks = %d, want 4000", l.AgentMaxToks)
	}
	if !l.AgentAutoNpmCheck {
		t.Error("AgentAutoNpmCheck = false, want true (AI_AGENT_AUTO_NPM_CHECK=1)")
	}
}

func TestLoadPaths_DerivedFromZshBagas(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("paths on windows use backslashes")
	}
	t.Setenv("LUNA", "/tmp/fake-zsh-bagas")
	os.Unsetenv("AI_AGENT_CHECKPOINT_DIR")
	os.Unsetenv("AI_TOOL_RUNS_DIR")
	os.Unsetenv("AI_INDEX_DIR")
	os.Unsetenv("AI_TODO_DIR")
	os.Unsetenv("AI_CIRCUIT_BREAKER_FILE")

	p := LoadPaths()
	if p.GenerateDir != "/tmp/fake-zsh-bagas/generate" {
		t.Errorf("GenerateDir = %q, want /tmp/fake-zsh-bagas/generate", p.GenerateDir)
	}
	if p.HistoryLog != "/tmp/fake-zsh-bagas/generate/logs/history.jsonl" {
		t.Errorf("HistoryLog = %q, want .../logs/history.jsonl", p.HistoryLog)
	}
	if p.CacheTTLSeconds != 3600 {
		t.Errorf("CacheTTLSeconds = %d, want 3600", p.CacheTTLSeconds)
	}
}

// --- Context engine data sanity (40-context_engine_docs.zsh parity) ---

func TestContextEngineLevels_SixLevelsInOrder(t *testing.T) {
	if len(ContextEngineLevels) != 6 {
		t.Fatalf("len(ContextEngineLevels) = %d, want 6", len(ContextEngineLevels))
	}
	for i, lvl := range ContextEngineLevels {
		if lvl.Level != i+1 {
			t.Errorf("ContextEngineLevels[%d].Level = %d, want %d", i, lvl.Level, i+1)
		}
	}
}
