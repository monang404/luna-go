package chat

import (
	"context"
	"errors"
	"testing"
)

type fakeClipboard struct {
	content string
	getErr  error
	set     string
	setErr  error
}

func (f *fakeClipboard) Get(ctx context.Context) (string, error) { return f.content, f.getErr }
func (f *fakeClipboard) Set(ctx context.Context, content string) error {
	f.set = content
	return f.setErr
}

func TestIsClipSensitive(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{"otp digits", "123456", true},
		{"private key", "-----BEGIN RSA PRIVATE KEY-----\nabc", true},
		{"card number", "4111 1111 1111 1111", true},
		{"password kv", "password: hunter2x", true},
		{"normal text", "tolong ringkas paragraf ini soal kucing", false},
		{"code snippet", "func main() {\n  fmt.Println(\"hi\")\n}", false},
	}
	for _, c := range cases {
		if got := IsClipSensitive(c.content); got != c.want {
			t.Errorf("%s: IsClipSensitive(%q) = %v, want %v", c.name, c.content, got, c.want)
		}
	}
}

func TestClip_RefusesSensitiveWithoutForce(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "unused"})
	clip := &fakeClipboard{content: "123456"}
	_, err := svc.Clip(context.Background(), clip, false, "")
	if !errors.Is(err, ErrClipSensitive) {
		t.Errorf("expected ErrClipSensitive, got %v", err)
	}
}

func TestClip_ForceAllowsSensitive(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "@@JAWABAN@@\nringkasan bersih"})
	clip := &fakeClipboard{content: "123456"}
	res, err := svc.Clip(context.Background(), clip, true, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Answer != "ringkasan bersih" {
		t.Errorf("Answer = %q", res.Answer)
	}
}

func TestClip_EmptyClipboard(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "x"})
	clip := &fakeClipboard{content: "   "}
	_, err := svc.Clip(context.Background(), clip, false, "")
	if !errors.Is(err, ErrClipEmpty) {
		t.Errorf("expected ErrClipEmpty, got %v", err)
	}
}

func TestClip_CopiesCleanAnswerBack(t *testing.T) {
	withFakeKey(t)
	svc := NewService(&fakeCompleter{content: "Jawaban bersih saja.\n**Thought**\nreasoning leak"})
	clip := &fakeClipboard{content: "sesuatu yang biasa aja"}
	res, err := svc.Clip(context.Background(), clip, false, "jelaskan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.CopiedBack {
		t.Error("expected CopiedBack=true")
	}
	if clip.set != "Jawaban bersih saja." {
		t.Errorf("expected only the clean answer copied back, got %q", clip.set)
	}
}
