package filepatch

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/monang404/luna-go/internal/aiops"
)

// Errors returned by BakClean.
var (
	ErrBakCleanDeclined = errors.New("filepatch: cleanup declined by user")
	ErrBakCleanTimedOut = errors.New("filepatch: cleanup confirmation timed out")
)

// BakCleanResult reports what BakClean found/removed.
type BakCleanResult struct {
	OldBackups []string
	OldCache   []string
	Removed    bool
}

// bakSuffixRE-equivalent: a path segment containing ".bak." anywhere
// (matching the shell glob "**/*.bak.*").
func looksLikeBackup(name string) bool {
	return strings.Contains(name, ".bak.")
}

// findOld walks root recursively (mirroring zsh's `**/*.bak.*(N.mh+H)`
// glob: regular files only, older than the given age) and returns every
// backup-named file older than cutoff.
func findOld(root string, match func(name string) bool, cutoff time.Time) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort walk, matching glob's silent skip of unreadable entries
		}
		if d.IsDir() {
			return nil
		}
		if !match(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().Before(cutoff) {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// BakClean mirrors aibakclean(days): find `.bak.*` files (recursively
// from root) and `.json` cache files (in cacheDir, non-recursive --
// matching the zsh source's single-level `"$cache_dir"/*.json` glob)
// older than days, show them, and delete only after Confirm approves.
// A dry-run with nothing found returns a zero-value, non-error Result
// (Removed=false), matching aibakclean's own "Gak ada file backup/cache
// lebih tua dari N hari" no-op success.
func (s *Service) BakClean(ctx context.Context, root string, cacheDir string, days int, remove func(path string) error) (BakCleanResult, error) {
	if days <= 0 {
		days = 14
	}
	cutoff := time.Now().AddDate(0, 0, -days)

	oldBackups, err := findOld(root, looksLikeBackup, cutoff)
	if err != nil {
		return BakCleanResult{}, err
	}

	var oldCache []string
	if cacheDir != "" {
		entries, err := os.ReadDir(cacheDir)
		if err == nil {
			for _, e := range entries {
				if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
					continue
				}
				info, err := e.Info()
				if err != nil {
					continue
				}
				if info.ModTime().Before(cutoff) {
					oldCache = append(oldCache, filepath.Join(cacheDir, e.Name()))
				}
			}
		}
	}

	if len(oldBackups) == 0 && len(oldCache) == 0 {
		return BakCleanResult{}, nil
	}

	decision, err := s.Confirm(ctx, "Hapus semua di atas?")
	if err != nil {
		return BakCleanResult{OldBackups: oldBackups, OldCache: oldCache}, err
	}
	switch decision {
	case aiops.Approved:
	case aiops.TimedOut:
		return BakCleanResult{OldBackups: oldBackups, OldCache: oldCache}, ErrBakCleanTimedOut
	default:
		return BakCleanResult{OldBackups: oldBackups, OldCache: oldCache}, ErrBakCleanDeclined
	}

	if remove == nil {
		remove = os.Remove
	}
	for _, f := range oldBackups {
		_ = remove(f)
	}
	for _, f := range oldCache {
		_ = remove(f)
	}

	return BakCleanResult{OldBackups: oldBackups, OldCache: oldCache, Removed: true}, nil
}
