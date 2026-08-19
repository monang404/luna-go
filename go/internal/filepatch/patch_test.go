package filepatch

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/monang404/luna-go/internal/aiops"
	"github.com/monang404/luna-go/internal/config"
	"github.com/monang404/luna-go/internal/llmclient"
)

// fakeCompleter is a test double implementing aiops.Completer, so
// package tests never depend on live LUNA providers.
type fakeCompleter struct {
	content string
	err     error
}

func (f fakeCompleter) Complete(ctx context.Context, systemPrompt, userPrompt string, class config.TaskClass, order []string, maxTokens int) (aiops.Result, error) {
	if f.err != nil {
		return aiops.Result{}, f.err
	}
	return aiops.Result{Content: f.content, Provider: "fake", Model: "fake-model"}, nil
}

func (f fakeCompleter) CompleteMessages(ctx context.Context, messages []llmclient.Message, class config.TaskClass, order []string, maxTokens int) (aiops.Result, error) {
	return f.Complete(ctx, "", "", class, order, maxTokens)
}

func approveConfirm(ctx context.Context, prompt string) (aiops.Decision, error) {
	return aiops.Approved, nil
}
func declineConfirm(ctx context.Context, prompt string) (aiops.Decision, error) {
	return aiops.Declined, nil
}
func timeoutConfirm(ctx context.Context, prompt string) (aiops.Decision, error) {
	return aiops.TimedOut, nil
}

func TestPatch_Applied(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	os.WriteFile(file, []byte("print('old')\n"), 0o644)

	svc := NewService(fakeCompleter{content: "print('new')\n"}, approveConfirm)
	res, err := svc.Patch(context.Background(), file, "update the print statement", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Applied {
		t.Fatal("expected Applied=true")
	}
	if res.BackupPath == "" {
		t.Error("expected a backup path")
	}
	if _, err := os.Stat(res.BackupPath); err != nil {
		t.Errorf("expected backup file to exist: %v", err)
	}
	got, _ := os.ReadFile(file)
	if string(got) != "print('new')\n" {
		t.Errorf("file not updated, got %q", got)
	}
	backupContent, _ := os.ReadFile(res.BackupPath)
	if string(backupContent) != "print('old')\n" {
		t.Errorf("backup should hold the original content, got %q", backupContent)
	}
}

func TestPatch_NoChange(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	os.WriteFile(file, []byte("print('same')\n"), 0o644)

	svc := NewService(fakeCompleter{content: "print('same')\n"}, approveConfirm)
	res, err := svc.Patch(context.Background(), file, "no real change", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.NoChange {
		t.Error("expected NoChange=true")
	}
}

func TestPatch_Declined(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	original := "print('old')\n"
	os.WriteFile(file, []byte(original), 0o644)

	svc := NewService(fakeCompleter{content: "print('new')\n"}, declineConfirm)
	_, err := svc.Patch(context.Background(), file, "change it", false)
	if err == nil {
		t.Fatal("expected error on decline")
	}
	got, _ := os.ReadFile(file)
	if string(got) != original {
		t.Error("file must not change when the user declines")
	}
}

func TestPatch_Timeout(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	os.WriteFile(file, []byte("print('old')\n"), 0o644)

	svc := NewService(fakeCompleter{content: "print('new')\n"}, timeoutConfirm)
	_, err := svc.Patch(context.Background(), file, "change it", false)
	if err != ErrPatchTimedOut {
		t.Errorf("expected ErrPatchTimedOut, got %v", err)
	}
}

func TestPatch_SecretFileRejectedWithoutForce(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".env")
	os.WriteFile(file, []byte("SECRET=1\n"), 0o644)

	svc := NewService(fakeCompleter{content: "SECRET=2\n"}, approveConfirm)
	_, err := svc.Patch(context.Background(), file, "change it", false)
	if err == nil {
		t.Fatal("expected secret-file guard to reject the patch")
	}
	got, _ := os.ReadFile(file)
	if string(got) != "SECRET=1\n" {
		t.Error("secret file must not be modified")
	}
}

func TestPatch_SecretFileAllowedWithForce(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".env")
	os.WriteFile(file, []byte("SECRET=1\n"), 0o644)

	svc := NewService(fakeCompleter{content: "SECRET=2\n"}, approveConfirm)
	res, err := svc.Patch(context.Background(), file, "change it", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Applied {
		t.Error("expected force to allow the patch")
	}
}

func TestPatch_BinaryFileRejected(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "a.bin")
	os.WriteFile(file, []byte("a\x00b"), 0o644)

	svc := NewService(fakeCompleter{content: "whatever"}, approveConfirm)
	_, err := svc.Patch(context.Background(), file, "change it", true)
	if err == nil {
		t.Fatal("expected binary-file guard to reject even with force")
	}
}

func TestPatch_FileTooBigWithoutForce(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "big.txt")
	big := make([]byte, 100)
	for i := range big {
		big[i] = 'x'
	}
	os.WriteFile(file, big, 0o644)

	svc := NewService(fakeCompleter{content: "new"}, approveConfirm)
	svc.Limits.FileMaxChars = 50
	_, err := svc.Patch(context.Background(), file, "change it", false)
	if err == nil {
		t.Fatal("expected size guard to reject the patch")
	}
}

func TestPatch_MissingFile(t *testing.T) {
	svc := NewService(fakeCompleter{content: "x"}, approveConfirm)
	_, err := svc.Patch(context.Background(), "/nonexistent/file.txt", "change it", false)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestPatch_UsageErrors(t *testing.T) {
	svc := NewService(fakeCompleter{content: "x"}, approveConfirm)
	if _, err := svc.Patch(context.Background(), "", "instr", false); err != ErrPatchUsage {
		t.Errorf("expected ErrPatchUsage for empty file, got %v", err)
	}
	if _, err := svc.Patch(context.Background(), "somefile", "", false); err != ErrPatchUsage {
		t.Errorf("expected ErrPatchUsage for empty instruction, got %v", err)
	}
}

func TestPatch_ApplyFailure_RestoresBackupNotNeeded(t *testing.T) {
	// Sanity check: a request error must not touch the file at all.
	dir := t.TempDir()
	file := filepath.Join(dir, "main.py")
	original := "print('old')\n"
	os.WriteFile(file, []byte(original), 0o644)

	svc := NewService(fakeCompleter{err: context.DeadlineExceeded}, approveConfirm)
	_, err := svc.Patch(context.Background(), file, "change it", false)
	if err == nil {
		t.Fatal("expected error to propagate from the completer")
	}
	got, _ := os.ReadFile(file)
	if string(got) != original {
		t.Error("file must be untouched when the LLM request fails")
	}
}
