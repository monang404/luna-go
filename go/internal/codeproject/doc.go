// Package codeproject ports 30-luna/30-code/ (aicode, aiproject + its 5
// generate/split/salvage/report/autotest/completeness helpers, aifix,
// airun, aiscrap) into Go, per SESSION-54.
//
// Scope notes:
//   - Progress tickers (_ai_progress_tick_start/_stop) are a terminal-
//     only cosmetic (background job printing a periodic "still
//     waiting" line to a real tty) -- out of scope per SESSION-54 §3
//     ("internal/ui: NOT part of this session"). Callers that want a
//     progress indicator wrap these functions themselves.
//   - Runtime execution of generated code (airun, `python3 file.py`)
//     and syntax checks (`python3 -m py_compile`) shell out via an
//     injected aiops.CommandRunner rather than os/exec directly, so
//     package tests never actually execute untrusted generated code.
//   - aiscrap's HTML structure sniffing used Python's BeautifulSoup in
//     the zsh source; this package reimplements just enough of that
//     (anchor tag + class + text extraction) with Go's stdlib
//     html/template-adjacent tokenizing via a small regexp-based
//     scanner, since no third-party HTML parser module is available in
//     this build environment. This is a best-effort approximation of
//     the original's structure-sniffing signal, not a byte-identical
//     port -- documented explicitly here per SESSION-54 §0's "deviation
//     must be flagged" contract.
package codeproject
