package workflow

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/monang404/luna-go/internal/config"
)

// summarizeChunkMaxChars mirrors the fixed 12000 threshold in
// aisummarize (both the "needs chunking at all" check and each chunk's
// own target size).
const summarizeChunkMaxChars = 12000

// summarizeOverlapChars mirrors aisummarize's overlap_chars=300.
const summarizeOverlapChars = 300

// ErrSummarizeUsage mirrors aisummarize's "Usage: luna summarize
// <file|url>".
var ErrSummarizeUsage = errors.New("workflow: usage: file or url is required")

// ErrSummarizeNoContent mirrors aisummarize's "Gak ada konten buat
// diringkes (cek file/url)." message.
var ErrSummarizeNoContent = errors.New("workflow: no content to summarize")

// SummarizeResult is Summarize's return shape.
type SummarizeResult struct {
	Summary    string
	WasChunked bool
	ChunkCount int
}

// Summarize mirrors aisummarize(content): given already-resolved plain-
// text content (fetching a URL and stripping HTML, or reading a local
// file, is left to the caller -- see the doc comment below), summarize
// it directly if short enough, or split it into overlapping
// paragraph-based chunks, summarize each, then merge the chunk
// summaries into one coherent final summary.
//
// Content-source resolution note: the zsh source's own content
// resolution step (curl+BeautifulSoup HTML-to-text for a URL, or `cat`
// for a local file) is a fetch/IO concern, not chunking/summarization
// logic -- this function takes already-resolved plain text so it can be
// unit tested without a live network fetch or python3/bs4 dependency;
// callers (SESSION-55's CLI layer) own resolving `src` into `content`
// before calling this (aiscrap's own fetchAndSniff in
// internal/codeproject shows the same net/http pattern for URL
// fetching, reusable here).
func (s *Service) Summarize(ctx context.Context, content string) (SummarizeResult, error) {
	if err := needAnyKey(config.TaskProviderOrder); err != nil {
		return SummarizeResult{}, err
	}
	if strings.TrimSpace(content) == "" {
		return SummarizeResult{}, ErrSummarizeNoContent
	}

	if len(content) <= summarizeChunkMaxChars {
		res, err := s.Requester.Complete(ctx, config.PersonaChatLong+" Ringkes konten berikut jadi poin-poin penting.", content, config.TaskSmart, config.TaskProviderOrder, 0)
		if err != nil {
			return SummarizeResult{}, err
		}
		return SummarizeResult{Summary: res.Content}, nil
	}

	chunks := chunkByParagraph(content, summarizeChunkMaxChars, summarizeOverlapChars)
	var combined strings.Builder
	for _, chunk := range chunks {
		res, err := s.Requester.Complete(ctx, "Ringkes teks ini jadi poin-poin penting, bahasa Indonesia, singkat, tanpa markdown.", chunk, config.TaskFast, config.TaskProviderOrderFast, 0)
		if err != nil {
			return SummarizeResult{WasChunked: true, ChunkCount: len(chunks)}, fmt.Errorf("workflow: chunk summarization failed: %w", err)
		}
		combined.WriteString("\n")
		combined.WriteString(res.Content)
	}

	final, err := s.Requester.Complete(ctx, "Gabungkan poin-poin ringkasan berikut jadi satu ringkasan koheren, bahasa Indonesia, singkat, tanpa markdown, dan gak redundan.", combined.String(), config.TaskSmart, config.TaskProviderOrder, 0)
	if err != nil {
		return SummarizeResult{WasChunked: true, ChunkCount: len(chunks)}, err
	}
	return SummarizeResult{Summary: final.Content, WasChunked: true, ChunkCount: len(chunks)}, nil
}

// chunkByParagraph mirrors aisummarize's paragraph-based chunker:
// split content on blank lines (paragraph boundaries), then greedily
// pack paragraphs into chunks up to maxChars; when a chunk is closed
// out, the new chunk starts with the last overlapChars characters of
// the PREVIOUS chunk (not the previous paragraph) prepended, so
// consecutive chunks share trailing/leading context for the model.
func chunkByParagraph(content string, maxChars, overlapChars int) []string {
	paragraphs := splitParagraphs(content)

	var parts []string
	cur := ""
	for _, para := range paragraphs {
		if para == "" {
			continue
		}
		if cur != "" && len(cur)+len(para) > maxChars {
			parts = append(parts, cur)
			tail := cur
			if len(tail) > overlapChars {
				tail = tail[len(tail)-overlapChars:]
			}
			cur = tail + "\n\n" + para
			continue
		}
		if cur == "" {
			cur = para
		} else {
			cur = cur + "\n\n" + para
		}
	}
	if cur != "" {
		parts = append(parts, cur)
	}
	return parts
}

// splitParagraphs splits on runs of blank lines, matching `awk
// 'BEGIN{RS="";ORS=...}'`'s paragraph mode (a paragraph is a maximal
// run of non-blank lines; blank-line runs are the separator and are
// discarded, not preserved as empty paragraphs).
func splitParagraphs(content string) []string {
	lines := strings.Split(content, "\n")
	var paragraphs []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			paragraphs = append(paragraphs, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		cur = append(cur, line)
	}
	flush()
	return paragraphs
}
