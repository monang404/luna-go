// Package filepatch ports 30-luna/35-files/ (aicat, aipatch, aiundo,
// aibakclean, aishare, plus the shared secret/binary file guards) into
// Go, per SESSION-54 (see SESSION-54_EXECUTION_CONTEXT.md).
//
// Behavioral parity notes (see that file's §2 for the zsh source of
// truth):
//   - Guards (IsSecretFile/IsBinaryFile) are read-only classification
//     helpers with no side effects, safe to call from any package.
//   - Patch/Undo require an aiops.ConfirmFunc (interactive confirmation
//     is a SESSION-55/CLI concern, not this package's).
//   - Every destructive path (Patch apply, Undo restore) takes a backup
//     first and verifies the destination's post-write state before
//     declaring success, matching the zsh source's RC-013 restore-
//     before-delete discipline.
package filepatch
