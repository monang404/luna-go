package aiops

import (
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"
)

// nonSlugRunRE mirrors the `tr -cs 'a-z0-9' '_'` pipeline used
// throughout 30-code/05-code.zsh and 40-workflow/*.zsh to build output
// filenames from free-text prompts: any run of one-or-more characters
// outside [a-z0-9] collapses to a single '_' (tr's -s squeeze flag).
var nonSlugRunRE = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify mirrors `echo "$s" | tr '[:upper:]' '[:lower:]' | tr -cs
// 'a-z0-9' '_' | cut -c1-40`: lowercase, collapse non-alphanumeric runs
// to single underscores, then truncate to maxLen runes. maxLen <= 0
// disables truncation.
func Slugify(s string, maxLen int) string {
	lower := strings.ToLower(s)
	slug := nonSlugRunRE.ReplaceAllString(lower, "_")
	if maxLen > 0 && len(slug) > maxLen {
		slug = slug[:maxLen]
	}
	return slug
}

// Timestamp mirrors _ai_ts() (10-core/25-quick_chat.zsh): `date
// +%Y%m%d_%H%M%S` plus a 4-hex-digit random suffix (zsh's $RANDOM is
// 0-32767, printed here with the same '%04x' width/format), used as the
// uniqueness suffix for generated filenames and backup names.
func Timestamp() string {
	return fmt.Sprintf("%s_%04x", time.Now().Format("20060102_150405"), rand.Intn(0x8000))
}

// BackupSuffix mirrors the "$file.bak.$(_ai_ts)" naming convention used
// by aipatch, aicode -o, aifix/_ai_fix_apply, and aiundo's own
// before_undo safety copy.
func BackupPath(file string) string {
	return file + ".bak." + Timestamp()
}
