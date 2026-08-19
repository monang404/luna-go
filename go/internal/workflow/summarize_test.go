package workflow

import (
	"context"
	"strings"
	"testing"
)

func TestSummarize_ShortContent_NoChunking(t *testing.T) {
	withFakeKey(t)
	svc := &Service{Requester: &fakeCompleter{contents: []string{"ringkasan singkat"}}}
	res, err := svc.Summarize(context.Background(), "konten pendek aja")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.WasChunked {
		t.Error("expected WasChunked=false for short content")
	}
	if res.Summary != "ringkasan singkat" {
		t.Errorf("Summary = %q", res.Summary)
	}
}

func TestSummarize_LongContent_Chunks(t *testing.T) {
	withFakeKey(t)
	var b strings.Builder
	for i := 0; i < 20; i++ {
		b.WriteString(strings.Repeat("kalimat panjang dalam paragraf ini berulang-ulang. ", 30))
		b.WriteString("\n\n")
	}
	content := b.String()
	if len(content) <= summarizeChunkMaxChars {
		t.Fatalf("test setup error: content not long enough (%d chars)", len(content))
	}

	fc := &fakeCompleter{contents: []string{"chunk summary", "final combined summary"}}
	svc := &Service{Requester: fc}
	res, err := svc.Summarize(context.Background(), content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.WasChunked {
		t.Error("expected WasChunked=true for long content")
	}
	if res.ChunkCount < 2 {
		t.Errorf("expected multiple chunks, got %d", res.ChunkCount)
	}
	if res.Summary != "final combined summary" {
		t.Errorf("Summary = %q", res.Summary)
	}
}

func TestSummarize_NoContent(t *testing.T) {
	withFakeKey(t)
	svc := &Service{Requester: &fakeCompleter{}}
	_, err := svc.Summarize(context.Background(), "   ")
	if err != ErrSummarizeNoContent {
		t.Errorf("expected ErrSummarizeNoContent, got %v", err)
	}
}

func TestChunkByParagraph_PreservesMultilineParagraphs(t *testing.T) {
	content := "Paragraf satu\nmasih baris pertama\nbaris kedua paragraf satu.\n\nParagraf dua."
	chunks := chunkByParagraph(content, 12000, 300)
	if len(chunks) != 1 {
		t.Fatalf("expected everything in 1 chunk (under maxChars), got %d", len(chunks))
	}
	if !strings.Contains(chunks[0], "baris kedua paragraf satu.") {
		t.Error("multi-line paragraph should stay intact within a chunk")
	}
}

func TestChunkByParagraph_SplitsAtMaxCharsWithOverlap(t *testing.T) {
	// Build paragraphs that force at least 2 chunks.
	var paras []string
	for i := 0; i < 5; i++ {
		paras = append(paras, strings.Repeat("x", 3000))
	}
	content := strings.Join(paras, "\n\n")

	chunks := chunkByParagraph(content, 8000, 300)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// Second chunk should start with overlap from the first (a run of
	// 'x' shared with chunk 1's tail).
	if !strings.HasPrefix(chunks[1], "xxx") {
		t.Errorf("expected chunk 2 to start with overlap content, got prefix: %q", chunks[1][:20])
	}
}

func TestSplitParagraphs_DiscardsBlankRuns(t *testing.T) {
	content := "para one\n\n\n\npara two\npara two line 2\n\npara three"
	paras := splitParagraphs(content)
	if len(paras) != 3 {
		t.Fatalf("expected 3 paragraphs, got %d: %v", len(paras), paras)
	}
	if paras[1] != "para two\npara two line 2" {
		t.Errorf("paragraph 2 = %q", paras[1])
	}
}
