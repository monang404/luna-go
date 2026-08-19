package filepatch

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// ErrBinaryFile is returned by Cat when the target looks like a binary
// file, matching aicat's own binary guard.
var ErrBinaryFile = errors.New("filepatch: file looks binary, refusing to display")

// ErrFileNotFound mirrors aicat's "Usage: ..." message for a missing/
// unreadable file.
var ErrFileNotFound = errors.New("filepatch: file not found")

// Cat mirrors aicat(file, start, end): read a text file with line
// numbers, optionally restricted to [start, end] (1-based, inclusive),
// matching `sed -n "${start},${end}p" | nl -ba -v"$start" -w4 -s'  '`
// (whole-file case: `nl -ba -w4 -s'  '`). start/end of 0 means "no
// range" (whole file), matching the zsh source treating empty
// start/end as "print everything".
func Cat(file string, start, end int) (string, error) {
	if file == "" {
		return "", fmt.Errorf("%w: %s", ErrFileNotFound, file)
	}
	info, err := os.Stat(file)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("%w: %s", ErrFileNotFound, file)
	}
	if IsBinaryFile(file) {
		return "", fmt.Errorf("%w: %s", ErrBinaryFile, file)
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return "", err
	}
	lines := splitLines(string(raw))

	from, to := 1, len(lines)
	if start > 0 && end > 0 {
		from, to = start, end
	}

	var b strings.Builder
	for i := from; i <= to && i <= len(lines); i++ {
		if i < 1 {
			continue
		}
		fmt.Fprintf(&b, "%4d  %s\n", i, lines[i-1])
	}
	return b.String(), nil
}

// splitLines splits on "\n" without producing a trailing empty element
// for a final newline, matching how `sed`/`nl` treat a file's trailing
// newline (no phantom extra numbered blank line at EOF).
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	trimmed := strings.TrimSuffix(s, "\n")
	return strings.Split(trimmed, "\n")
}
