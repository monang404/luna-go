package codeproject

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_SuccessFirstTry(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "ok.py")
	os.WriteFile(file, []byte("print('ok')\n"), 0o644)

	runner := &fakeRunner{responses: map[string]runnerResponse{
		"python3 " + file: {stdout: "ok\n", exitCode: 0},
	}}
	svc := &Service{Runner: runner}
	res, err := svc.Run(context.Background(), file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Error("expected Success=true")
	}
	if res.FixAttempts != 0 {
		t.Errorf("expected 0 fix attempts, got %d", res.FixAttempts)
	}
}

func TestRun_FailsThenFixesSuccessfully(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "buggy.py")
	os.WriteFile(file, []byte("prin('x')\n"), 0o644)

	callCount := 0
	runner := &fakeRunnerSeq{
		results: []runnerResponse{
			{stdout: "", stderr: "NameError: prin", exitCode: 1},
			{stdout: "x\n", exitCode: 0},
		},
	}
	_ = callCount
	svc := &Service{
		Requester: &fakeCompleter{contents: []string{"print('x')\n"}},
		Confirm:   approveConfirm,
		Runner:    runner,
	}
	res, err := svc.Run(context.Background(), file)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Success {
		t.Error("expected eventual success after auto-fix")
	}
	got, _ := os.ReadFile(file)
	if string(got) != "print('x')\n" {
		t.Errorf("file not fixed: %q", got)
	}
}

func TestRun_UsageError(t *testing.T) {
	svc := &Service{Runner: &fakeRunner{}}
	_, err := svc.Run(context.Background(), "")
	if err != ErrRunUsage {
		t.Errorf("expected ErrRunUsage, got %v", err)
	}
}

func TestRun_StillFailingAfterTwoAttempts(t *testing.T) {
	withFakeKey(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "buggy.py")
	os.WriteFile(file, []byte("bad\n"), 0o644)

	runner := &fakeRunnerSeq{
		results: []runnerResponse{
			{stderr: "err1", exitCode: 1},
			{stderr: "err2", exitCode: 1},
		},
	}
	svc := &Service{
		Requester: &fakeCompleter{contents: []string{"still bad\n"}},
		Confirm:   approveConfirm,
		Runner:    runner,
	}
	res, err := svc.Run(context.Background(), file)
	if err == nil {
		t.Fatal("expected an error after exhausting auto-fix attempts")
	}
	if res.Success {
		t.Error("expected Success=false")
	}
}

// fakeRunnerSeq returns each entry in results once, in order, then
// repeats the last one -- used to simulate airun's "fails, gets fixed,
// then re-run succeeds" sequence across multiple Runner.Run calls for
// the SAME script path (fakeRunner's map-by-key can't express that).
type fakeRunnerSeq struct {
	results []runnerResponse
	calls   int
}

func (f *fakeRunnerSeq) Run(ctx context.Context, name string, args ...string) (string, string, int, error) {
	// python3 -m py_compile / other sanitize calls always succeed;
	// only bare `python3 <file>` invocations consume the sequence.
	if len(args) == 1 {
		idx := f.calls
		if idx >= len(f.results) {
			idx = len(f.results) - 1
		}
		f.calls++
		r := f.results[idx]
		return r.stdout, r.stderr, r.exitCode, r.err
	}
	return "", "", 0, nil
}
