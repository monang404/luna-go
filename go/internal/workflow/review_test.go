package workflow

import (
	"context"
	"testing"
)

func TestReview_UsesCachedDiffWhenStaged(t *testing.T) {
	withFakeKey(t)
	runner := &fakeRunner{responses: map[string]runnerResponse{
		"git rev-parse --is-inside-work-tree": {exitCode: 0},
		"git diff --cached":                   {stdout: "staged diff content\n"},
		"git diff --cached --stat":            {stdout: "stat\n"},
	}}
	svc := &Service{Requester: &fakeCompleter{contents: []string{"1) no bugs found"}}, Runner: runner}
	res, err := svc.Review(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.WasStaged {
		t.Error("expected WasStaged=true")
	}
	if res.Review != "1) no bugs found" {
		t.Errorf("Review = %q", res.Review)
	}
}

func TestReview_FallsBackToUnstaged(t *testing.T) {
	withFakeKey(t)
	runner := &fakeRunner{responses: map[string]runnerResponse{
		"git rev-parse --is-inside-work-tree": {exitCode: 0},
		"git diff --cached":                   {stdout: ""},
		"git diff":                            {stdout: "unstaged diff\n"},
		"git diff --stat":                     {stdout: "stat\n"},
	}}
	svc := &Service{Requester: &fakeCompleter{contents: []string{"review text"}}, Runner: runner}
	res, err := svc.Review(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.WasStaged {
		t.Error("expected WasStaged=false")
	}
}

func TestReview_NothingToReview(t *testing.T) {
	withFakeKey(t)
	runner := &fakeRunner{responses: map[string]runnerResponse{
		"git rev-parse --is-inside-work-tree": {exitCode: 0},
		"git diff --cached":                   {stdout: ""},
		"git diff":                            {stdout: ""},
	}}
	svc := &Service{Requester: &fakeCompleter{contents: []string{"x"}}, Runner: runner}
	_, err := svc.Review(context.Background())
	if err == nil {
		t.Fatal("expected an error when there's nothing to review")
	}
}

func TestReviewDiffCore_GuardsLargeDiff(t *testing.T) {
	withFakeKey(t)
	big := make([]byte, 20000)
	for i := range big {
		big[i] = 'y'
	}
	fc := &fakeCompleter{contents: []string{"review"}}
	svc := &Service{Requester: fc}
	_, err := svc.ReviewDiffCore(context.Background(), string(big), "stat summary")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(fc.lastUser) >= 20000 {
		t.Error("expected the diff to be guarded/truncated before sending to the model")
	}
}
