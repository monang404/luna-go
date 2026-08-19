package filepatch

import (
	"context"
	"errors"
	"fmt"
	"os"
)

// ErrShareNoSuchFile mirrors aishare's "File gak ketemu" message.
var ErrShareNoSuchFile = errors.New("filepatch: file not found")

// ErrShareUnavailable mirrors aishare's `command -v termux-share`
// guard failing.
var ErrShareUnavailable = errors.New("filepatch: termux-share unavailable")

// Share mirrors aishare(file): hand the file off to share (the
// injected termux-share equivalent) after confirming it exists. share
// == nil means "not available on this platform", matching the zsh
// source's `command -v termux-share` guard.
func Share(ctx context.Context, file string, share ShareFuncOrNil) error {
	if file == "" {
		return fmt.Errorf("%w: (empty path)", ErrShareNoSuchFile)
	}
	if info, err := os.Stat(file); err != nil || info.IsDir() {
		return fmt.Errorf("%w: %s", ErrShareNoSuchFile, file)
	}
	if share == nil {
		return ErrShareUnavailable
	}
	return share(ctx, file)
}

// ShareFuncOrNil is aiops.ShareFunc, aliased locally so callers that
// only import internal/filepatch don't also need internal/aiops just to
// name the parameter type.
type ShareFuncOrNil = func(ctx context.Context, path string) error
