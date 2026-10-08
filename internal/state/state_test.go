package state

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	// Cover both cache-dir conventions (macOS uses HOME, Linux XDG).
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	if _, ok, err := Load(); err != nil || ok {
		t.Fatalf("expected no snapshot on a clean cache, got ok=%v err=%v", ok, err)
	}

	s := Snapshot{SavedAt: time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC), PRs: map[string]Entry{
		"acme/widgets#1": {Status: "conflicts", Owner: "you", Title: "Add widget", URL: "https://x/1"},
	}}
	if err := Save(s); err != nil {
		t.Fatal(err)
	}

	got, ok, err := Load()
	if err != nil || !ok {
		t.Fatalf("load failed: ok=%v err=%v", ok, err)
	}
	e, found := got.PRs["acme/widgets#1"]
	if len(got.PRs) != 1 || !found || e.Owner != "you" || e.Status != "conflicts" {
		t.Fatalf("unexpected round-trip: %+v", got)
	}
	if !got.SavedAt.Equal(s.SavedAt) {
		t.Fatalf("saved_at lost: %v", got.SavedAt)
	}

	p, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(p) != "state.json" {
		t.Fatalf("unexpected snapshot path %s", p)
	}
}
