package aiops

import (
	"fmt"
	"strings"
)

// UnifiedDiff produces a `diff -u`-equivalent unified diff between old
// and newText, with "--- path" / "+++ path" headers -- the same review
// artifact aipatch, aicode -o, and _ai_fix_apply all show the user before
// asking for confirmation. Context is fixed at 3 lines, matching GNU
// diff's default (the same default the zsh source's plain `diff -u`
// invocations relied on).
func UnifiedDiff(path, oldText, newText string) string {
	const context = 3
	oldLines := splitDiffLines(oldText)
	newLines := splitDiffLines(newText)
	ops := diffOps(oldLines, newLines)

	var b strings.Builder
	fmt.Fprintf(&b, "--- %s\n", path)
	fmt.Fprintf(&b, "+++ %s\n", path)

	hunks := groupHunks(ops, context)
	for _, h := range hunks {
		writeHunk(&b, h, oldLines, newLines)
	}
	return b.String()
}

func splitDiffLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

type diffOpKind int

const (
	opEqual diffOpKind = iota
	opDelete
	opInsert
)

type diffOp struct {
	kind   diffOpKind
	oldIdx int // index into oldLines, valid for equal/delete
	newIdx int // index into newLines, valid for equal/insert
}

// diffOps computes a line-level diff via a classic O(n*m) LCS table --
// adequate for the file sizes these commands operate on (patch/fix
// targets are guarded to AI_FILE_MAX_CHARS well before this runs).
func diffOps(a, b []string) []diffOp {
	n, m := len(a), len(b)
	lcs := make([][]int, n+1)
	for i := range lcs {
		lcs[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lcs[i][j] = lcs[i+1][j+1] + 1
			} else if lcs[i+1][j] >= lcs[i][j+1] {
				lcs[i][j] = lcs[i+1][j]
			} else {
				lcs[i][j] = lcs[i][j+1]
			}
		}
	}

	var ops []diffOp
	i, j := 0, 0
	for i < n && j < m {
		switch {
		case a[i] == b[j]:
			ops = append(ops, diffOp{opEqual, i, j})
			i++
			j++
		case lcs[i+1][j] >= lcs[i][j+1]:
			ops = append(ops, diffOp{kind: opDelete, oldIdx: i})
			i++
		default:
			ops = append(ops, diffOp{kind: opInsert, newIdx: j})
			j++
		}
	}
	for ; i < n; i++ {
		ops = append(ops, diffOp{kind: opDelete, oldIdx: i})
	}
	for ; j < m; j++ {
		ops = append(ops, diffOp{kind: opInsert, newIdx: j})
	}
	return ops
}

type hunk struct {
	ops                []diffOp
	oldStart, newStart int
}

// groupHunks clusters ops into hunks separated by runs of >2*context
// unchanged lines, each padded with up to context lines of leading/
// trailing equal context -- the standard unified-diff hunk-merging rule.
func groupHunks(ops []diffOp, context int) []hunk {
	var hunks []hunk
	i := 0
	for i < len(ops) {
		if ops[i].kind == opEqual {
			i++
			continue
		}
		// Found a change; walk backward up to `context` equal lines for
		// leading context.
		start := i
		for k := 0; k < context && start > 0 && ops[start-1].kind == opEqual; k++ {
			start--
		}
		// Walk forward through changes, merging in runs of equal lines
		// that are short enough (<= 2*context) to belong to the same
		// hunk rather than splitting it.
		end := i
		for end < len(ops) {
			if ops[end].kind != opEqual {
				end++
				continue
			}
			runStart := end
			for end < len(ops) && ops[end].kind == opEqual {
				end++
			}
			runLen := end - runStart
			if end >= len(ops) || runLen > 2*context {
				end = runStart + min(runLen, context)
				break
			}
		}
		h := hunk{ops: ops[start:end]}
		hunks = append(hunks, h)
		i = end
	}
	// Fill in starting line numbers for each hunk.
	for hi := range hunks {
		if len(hunks[hi].ops) == 0 {
			continue
		}
		first := hunks[hi].ops[0]
		switch first.kind {
		case opEqual, opDelete:
			hunks[hi].oldStart = first.oldIdx
		case opInsert:
			hunks[hi].oldStart = -1 // resolved below
		}
	}
	return hunks
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func writeHunk(b *strings.Builder, h hunk, oldLines, newLines []string) {
	if len(h.ops) == 0 {
		return
	}
	oldCount, newCount := 0, 0
	oldStart, newStart := -1, -1
	for _, op := range h.ops {
		switch op.kind {
		case opEqual:
			oldCount++
			newCount++
			if oldStart == -1 {
				oldStart = op.oldIdx
			}
			if newStart == -1 {
				newStart = op.newIdx
			}
		case opDelete:
			oldCount++
			if oldStart == -1 {
				oldStart = op.oldIdx
			}
		case opInsert:
			newCount++
			if newStart == -1 {
				newStart = op.newIdx
			}
		}
	}
	// diff -u uses 1-based line numbers; a hunk with zero lines on one
	// side reports its start as the line before the insertion/deletion
	// point (matching GNU diff's convention), approximated here by
	// falling back to the other side's start when one is unset.
	if oldStart == -1 {
		oldStart = 0
	}
	if newStart == -1 {
		newStart = 0
	}
	fmt.Fprintf(b, "@@ -%d,%d +%d,%d @@\n", oldStart+1, oldCount, newStart+1, newCount)
	for _, op := range h.ops {
		switch op.kind {
		case opEqual:
			fmt.Fprintf(b, " %s\n", oldLines[op.oldIdx])
		case opDelete:
			fmt.Fprintf(b, "-%s\n", oldLines[op.oldIdx])
		case opInsert:
			fmt.Fprintf(b, "+%s\n", newLines[op.newIdx])
		}
	}
}
