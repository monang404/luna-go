package config

import (
	"os"
	"path/filepath"
	"strconv"
)

// envOrInt returns the parsed int value of os.Getenv(key) if set and
// parseable, else def -- equivalent to zsh's "${VAR:-default}" /
// ": ${VAR:=default}" patterns for the numeric guards below.
func envOrInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// envOrBool treats "0"/"" as false and anything else as true, matching how
// zsh guards like AI_DATA_SAVER_WARN/AI_AGENT_AUTO_NPM_CHECK are used in
// `[ "$VAR" = "1" ]`-style checks elsewhere in the codebase.
func envOrBool(key string, def bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return def
	}
	return v != "0"
}

// Paths mirrors 30-luna/00-config/10-paths.zsh: all *runtime output* paths
// (not config) rooted under $LUNA/generate/, plus the
// ": ${VAR:=...}"-style overridable directories that were split across
// 15-limits.zsh and 20-runtime_guards.zsh but derive from the same
// AI_GENERATE_DIR/AI_LOG_DIR/AI_SESSION_DIR roots defined here.
type Paths struct {
	Luna            string
	GenerateDir     string
	CodeDir         string
	SanitizeScript  string
	ExtractScript   string
	LogDir          string
	SessionDir      string
	HistoryLog      string
	UsageLog        string
	PlanDir         string
	PromptDir       string
	CacheDir        string
	CacheTTLSeconds int

	// Overridable via env (": ${VAR:=...}" in zsh); default derived from
	// the roots above.
	AgentCheckpointDir string // AI_AGENT_CHECKPOINT_DIR (from 15-limits.zsh)
	ToolRunsDir        string // AI_TOOL_RUNS_DIR (from 15-limits.zsh)
	IndexDir           string // AI_INDEX_DIR (from 15-limits.zsh)
	TodoDir            string // AI_TODO_DIR (from 15-limits.zsh)
	CircuitBreakerFile string // AI_CIRCUIT_BREAKER_FILE (from 20-runtime_guards.zsh)
}

// lunaDir returns $LUNA, falling back to $HOME/.luna -- the
// same default .zshrc sets via `export LUNA="$HOME/.luna"`.
func lunaDir() string {
	if v := os.Getenv("LUNA"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".luna")
}

// LoadPaths builds Paths from the current environment.
func LoadPaths() Paths {
	zb := lunaDir()
	generate := filepath.Join(zb, "generate")
	logDir := filepath.Join(generate, "logs")
	sessionDir := filepath.Join(generate, "sessions")

	return Paths{
		Luna:            zb,
		GenerateDir:     generate,
		CodeDir:         filepath.Join(generate, "aicode"),
		SanitizeScript:  filepath.Join(zb, "30-luna", "scripts", "ai_code_sanitize.py"),
		ExtractScript:   filepath.Join(zb, "30-luna", "scripts", "ai_extract.py"),
		LogDir:          logDir,
		SessionDir:      sessionDir,
		HistoryLog:      filepath.Join(logDir, "history.jsonl"),
		UsageLog:        filepath.Join(logDir, "usage.jsonl"),
		PlanDir:         filepath.Join(generate, "plans"),
		PromptDir:       filepath.Join(generate, "prompts"),
		CacheDir:        filepath.Join(generate, "cache"),
		CacheTTLSeconds: 3600,

		AgentCheckpointDir: envOr("AI_AGENT_CHECKPOINT_DIR", filepath.Join(sessionDir, "agent_checkpoints")),
		ToolRunsDir:        envOr("AI_TOOL_RUNS_DIR", filepath.Join(sessionDir, "agent_runs")),
		IndexDir:           envOr("AI_INDEX_DIR", filepath.Join(generate, "index")),
		TodoDir:            envOr("AI_TODO_DIR", filepath.Join(sessionDir, "todos")),
		CircuitBreakerFile: envOr("AI_CIRCUIT_BREAKER_FILE", filepath.Join(logDir, "circuit_breaker.txt")),
	}
}

// Limits mirrors 30-luna/00-config/15-limits.zsh (retry/timeout/session/diff/
// patch/file-size guards) plus the threshold constants from
// 20-runtime_guards.zsh (battery/data-saver/daily-token/notify-interval/
// circuit-breaker/agent-step/npm-check). Only the *thresholds* live here --
// the guard logic that actually touches the OS/process (battery status,
// network-metered detection, etc.) is out of scope for this package and
// deferred to internal/permission (SESSION-42).
//
// Fields documented "(env override)" respect the same env var at process
// start as the zsh "${VAR:-default}" / ": ${VAR:=default}" pattern they
// were ported from; the rest were plain "VAR=value" assignments in zsh
// (always overwritten, not env-overridable) and are hardcoded here too.
type Limits struct {
	// --- 15-limits.zsh ---
	MaxRetries         int // (env override: AI_MAX_RETRIES)
	CurlTimeoutSec     int // (env override: AI_CURL_TIMEOUT)
	RetryDelaySec      int
	SessionMaxMsgs     int
	LogMaxLines        int
	DiffMaxChars       int
	PatchMaxChars      int
	FileMaxChars       int
	ProjectMaxToks     int
	AgentMaxToks       int // (env override: AI_AGENT_MAX_TOKS)
	GrepMaxResults     int
	GitDiffMaxChars    int
	WebFetchMaxChars   int
	WebFetchTimeoutSec int

	// --- 20-runtime_guards.zsh ---
	BatteryWarnPct          int
	DataSaverWarn           bool
	DailyTokenWarn          int
	NotifyMinIntervalSec    int
	CircuitBreakerWindowSec int
	AgentMaxSteps           int
	AgentMaxSameFail        int
	AgentAutoNpmCheck       bool // (env override: AI_AGENT_AUTO_NPM_CHECK)
}

// LoadLimits builds Limits from the current environment.
func LoadLimits() Limits {
	return Limits{
		MaxRetries:         envOrInt("AI_MAX_RETRIES", 1),
		CurlTimeoutSec:     envOrInt("AI_CURL_TIMEOUT", 45),
		RetryDelaySec:      2,
		SessionMaxMsgs:     30,
		LogMaxLines:        5000,
		DiffMaxChars:       15000,
		PatchMaxChars:      200000,
		FileMaxChars:       40000,
		ProjectMaxToks:     3500,
		AgentMaxToks:       envOrInt("AI_AGENT_MAX_TOKS", 8000),
		GrepMaxResults:     100,
		GitDiffMaxChars:    6000,
		WebFetchMaxChars:   8000,
		WebFetchTimeoutSec: 15,

		BatteryWarnPct:          15,
		DataSaverWarn:           true, // AI_DATA_SAVER_WARN=1
		DailyTokenWarn:          150000,
		NotifyMinIntervalSec:    3,
		CircuitBreakerWindowSec: 30,
		AgentMaxSteps:           50,
		AgentMaxSameFail:        3,
		AgentAutoNpmCheck:       envOrBool("AI_AGENT_AUTO_NPM_CHECK", false), // ": ${VAR:=0}"
	}
}
