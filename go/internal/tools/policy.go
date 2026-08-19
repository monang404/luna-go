package tools

import (
	"regexp"
	"strings"
)

// This file ports the classification half of 30-luna/50-agent/00-policy.zsh's
// _ai_agent_is_dangerous (the AI_AGENT_DANGEROUS_PATTERNS regex list, the
// tokenized `rm -rf`-family scope check, and the tokenized `git push
// --force`-family scope check) for run_command's use in process.go.
// _ai_yolo_shell_safe's fast-path allowlist tokenizer (05-tools/30-tool_process.zsh)
// is deliberately NOT ported: it exists purely as a YOLO-mode
// *performance* optimization inside the zsh permission layer itself (skip
// the interactive ask for a handful of known-safe readonly commands), and
// in this Go port the permission decision for run_command is made
// entirely by permission.CheckPermission before Dispatcher.Dispatch ever
// calls RunCommandTool.Execute (see permission/check.go's checkShell,
// whose own doc comment already flags that CapShellArbitrary always asks
// rather than replicating that heuristic) -- Execute has no way to skip
// an ask that has already happened. IsDangerousCommand below is the part
// that still matters at the tool layer: a hard deny that applies
// regardless of how permission was granted, exactly like the zsh
// source's own call to _ai_agent_is_dangerous as the very first thing
// _ai_tool_run_command does, before any YOLO/ask branching.

// dangerousPatterns is a literal transcription of
// AI_AGENT_DANGEROUS_PATTERNS. zsh's `[[ $cmd =~ $pat ]]` evaluates each
// pattern as POSIX extended regex; Go's regexp (RE2) accepts the same
// syntax for every pattern in this list (POSIX character classes,
// alternation, anchors, backslash-escaped literal parens/braces/pipes),
// so each string below is used unmodified from the zsh source.
var dangerousPatterns = compileDangerousPatterns([]string{
	`:[[:space:]]*\([[:space:]]*\)[[:space:]]*\{[[:space:]]*:[[:space:]]*\|[[:space:]]*:[[:space:]]*&[[:space:]]*\}[[:space:]]*;[[:space:]]*:`,
	`mkfs\.`,
	`(^|[;&|]) *dd .*of=/dev/`,
	`> */dev/sd[a-z]`,
	`chmod +-R +000`,
	`(^|[;&|]) *(curl|wget) .*\| *(sh|bash|zsh)([ ;|&]|$)`,
	`(^|[;&|]) *(shutdown|reboot|poweroff)([ ;|&]|$)`,
	`(^|[;&|]) *find .*-delete`,
	`> */etc/`,
	`> */boot/`,
	`> *~?/?\.secrets\.zsh`,
	`> *~?/?\.zshrc`,
	`(^|[;&|]) *(pip3?|npm|pkg|apt|apt-get) +(uninstall|remove|purge)[^;&|]*(-y|--yes)`,
})

func compileDangerousPatterns(patterns []string) []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(patterns))
	for _, p := range patterns {
		out = append(out, regexp.MustCompile(p))
	}
	return out
}

// shellMetacharacters mirrors the pre-filter both _ai_agent_is_dangerous
// and _ai_yolo_shell_safe run before ever tokenizing: any of these
// characters (or a raw newline) anywhere in the command is treated as
// dangerous outright, deny-by-default, because zsh evaluates command
// substitution inside the string *before* tokenization finishes -- so
// merely classifying a `$(...)`-bearing command would already have run
// it as a side effect. Go never shells out to interpret this string at
// all, so that specific substitution hazard doesn't apply here, but the
// same conservative pre-filter is kept anyway: it's the single cheapest,
// most conservative signal in the whole function, and dropping it would
// be a deliberate loosening of the original safety margin with no
// corresponding upside in a Go rewrite.
const shellMetacharacters = ";|&<>`$"

// IsDangerousCommand mirrors _ai_agent_is_dangerous end-to-end, in the
// same order: pattern list, then metacharacter pre-filter, then the two
// tokenized scope scans (rm -rf-family, git push --force-family).
func IsDangerousCommand(cmd string) bool {
	for _, pat := range dangerousPatterns {
		if pat.MatchString(cmd) {
			return true
		}
	}
	if strings.ContainsAny(cmd, shellMetacharacters) || strings.Contains(cmd, "\n") {
		return true
	}

	tokens := tokenizeShellLike(cmd)
	if isDangerousRmScope(tokens) {
		return true
	}
	if isDangerousGitPushScope(tokens) {
		return true
	}
	return false
}

// isOperatorToken mirrors the `';'|'&&'|'||'|'|'` case arm both scope
// scans use to reset their running state at a command-separator boundary
// (tokenizeShellLike already emits these as their own tokens).
func isOperatorToken(tok string) bool {
	switch tok {
	case ";", "&&", "||", "|":
		return true
	default:
		return false
	}
}

// isDangerousRmScope mirrors the `rm`-scope scan: within one
// operator-delimited segment, `rm` (or a path ending in "/rm") followed
// by both a recursive flag and a force flag anywhere later in the same
// segment is unconditionally dangerous, checked token-by-token so a
// filename that merely *contains* "r" or "f" (e.g. "refactor.py") can
// never be mistaken for a flag.
func isDangerousRmScope(tokens []string) bool {
	inRmScope := false
	hasRecursive, hasForce := false, false
	for _, tok := range tokens {
		if isOperatorToken(tok) {
			inRmScope, hasRecursive, hasForce = false, false, false
			continue
		}
		if tok == "rm" || strings.HasSuffix(tok, "/rm") {
			inRmScope, hasRecursive, hasForce = true, false, false
			continue
		}
		if !inRmScope {
			continue
		}
		switch {
		case tok == "--recursive":
			hasRecursive = true
		case tok == "--force" || tok == "--no-preserve-root":
			hasForce = true
		case strings.HasPrefix(tok, "--"):
			// Any other long flag -- no-op, matches the zsh `--*) : ;;` arm.
		case strings.HasPrefix(tok, "-"):
			if strings.ContainsAny(tok, "rR") {
				hasRecursive = true
			}
			if strings.Contains(tok, "f") {
				hasForce = true
			}
		}
		if hasRecursive && hasForce {
			return true
		}
	}
	return false
}

// isDangerousGitPushScope mirrors the `git push --force`-family scan:
// within one operator-delimited segment starting with a `git` token, a
// later `push` token plus a later force-flag token (checked the same
// token-precise way -- never a raw substring match against "-f", so a
// branch name like "main-final" can never trigger it) is dangerous.
func isDangerousGitPushScope(tokens []string) bool {
	inGitScope := false
	hasPush, hasForce := false, false
	for _, tok := range tokens {
		if isOperatorToken(tok) {
			inGitScope, hasPush, hasForce = false, false, false
			continue
		}
		if tok == "git" {
			inGitScope, hasPush, hasForce = true, false, false
			continue
		}
		if !inGitScope {
			continue
		}
		switch {
		case tok == "push":
			hasPush = true
		case tok == "--force" || tok == "--force-with-lease" || strings.HasPrefix(tok, "--force-with-lease="):
			hasForce = true
		case tok == "-f":
			hasForce = true
		case strings.HasPrefix(tok, "-") && !strings.HasPrefix(tok, "--") && strings.Contains(tok, "f"):
			hasForce = true
		}
		if hasPush && hasForce {
			return true
		}
	}
	return false
}

// tokenizeShellLike is a small, deliberately conservative approximation
// of zsh's `${(z)cmd}` word-splitter: it only needs to be precise enough
// for the two flag-scope scans above to classify a command correctly,
// not to actually execute it (Go never shells out to run the string this
// classifier receives -- see IsDangerousCommand's own doc comment). It
// splits on whitespace, keeps single/double-quoted spans together as one
// token (quotes stripped, no backslash-escape handling inside quotes --
// good enough for flag/command-name classification, not a general shell
// parser), and always emits each of ; & | (and the two-character && /
// ||) as its own separate token even when directly adjacent to another
// token with no surrounding whitespace.
func tokenizeShellLike(cmd string) []string {
	var tokens []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			tokens = append(tokens, cur.String())
			cur.Reset()
		}
	}
	runes := []rune(cmd)
	for i := 0; i < len(runes); i++ {
		r := runes[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			flush()
		case r == '\'' || r == '"':
			quote := r
			i++
			for i < len(runes) && runes[i] != quote {
				cur.WriteRune(runes[i])
				i++
			}
			// i now indexes the closing quote (or ran off the end for an
			// unterminated quote); the outer loop's i++ advances past it.
		case r == '&' && i+1 < len(runes) && runes[i+1] == '&':
			flush()
			tokens = append(tokens, "&&")
			i++
		case r == '|' && i+1 < len(runes) && runes[i+1] == '|':
			flush()
			tokens = append(tokens, "||")
			i++
		case r == ';' || r == '&' || r == '|':
			flush()
			tokens = append(tokens, string(r))
		default:
			cur.WriteRune(r)
		}
	}
	flush()
	return tokens
}
