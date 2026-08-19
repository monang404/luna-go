package chat

import (
	"context"
	"testing"
)

func TestAsk_Success(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "jawaban berdasarkan konteks"})
	res, err := svc.Ask(context.Background(), "isi file di sini", "apa isinya?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Answer != "jawaban berdasarkan konteks" {
		t.Errorf("Answer = %q", res.Answer)
	}
}

func TestAsk_NoContent(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "x"})
	_, err := svc.Ask(context.Background(), "", "apa isinya?")
	if err != ErrAskNoContent {
		t.Errorf("expected ErrAskNoContent, got %v", err)
	}
}

func TestAsk_NoProvider(t *testing.T) {
	svc := NewService(&fakeCompleter{content: "x"})
	_, err := svc.Ask(context.Background(), "konten", "pertanyaan")
	if err != ErrNoProvider {
		t.Errorf("expected ErrNoProvider, got %v", err)
	}
}
