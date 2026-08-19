package codeproject

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSalvageIfEmpty_DirHasContent_Noop(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "main.py"), []byte("x = 1\n"), 0o644)

	svc := &Service{Runner: &fakeRunner{}}
	res, err := svc.SalvageIfEmpty(context.Background(), dir, "raw", "log.txt", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.DirWasEmpty {
		t.Error("expected DirWasEmpty=false")
	}
	if res.SalvagedTo != "" {
		t.Error("should not salvage when dir already has content")
	}
}

func TestSalvageIfEmpty_EmptyWithMarkers_HardFailure(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{Runner: &fakeRunner{}}
	res, err := svc.SalvageIfEmpty(context.Background(), dir, "raw with ### FILE: markers", "log.txt", true)
	if err == nil {
		t.Fatal("expected a hard-failure error")
	}
	if !res.HardFailure {
		t.Error("expected HardFailure=true")
	}
}

func TestSalvageIfEmpty_EmptyNoMarkers_SalvagesSingleFile(t *testing.T) {
	dir := t.TempDir()
	svc := &Service{Runner: &fakeRunner{}}
	res, err := svc.SalvageIfEmpty(context.Background(), dir, "print('salvaged')\n", "log.txt", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.SalvagedTo == "" {
		t.Fatal("expected SalvagedTo to be set")
	}
	got, _ := os.ReadFile(res.SalvagedTo)
	if string(got) != "print('salvaged')\n" {
		t.Errorf("unexpected salvaged content: %q", got)
	}
}
