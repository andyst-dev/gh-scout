package prs

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/andyst-dev/gh-scout/internal/report"
	"github.com/andyst-dev/gh-scout/internal/state"
)

func deltaReport() *Report {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	return &Report{
		GeneratedAt: now,
		User:        "andy",
		Repos: []Repo{
			{Name: "acme/widgets", PRs: []PR{
				{Number: 1, Title: "Add widget", URL: "https://x/1", Status: "conflicts",
					Action: Action{Owner: OwnerYou, Reasons: []string{"rebase"}}},
				{Number: 2, Title: "Fix widget", URL: "https://x/2", Status: "blocked",
					Action:   Action{Owner: OwnerThem, Waiting: "review and required checks"},
					Response: &Response{Author: "sam", At: now.Add(-time.Hour), Age: "1h ago"}},
			}},
		},
	}
}

func hasNote(notes []string, want string) bool {
	for _, n := range notes {
		if strings.Contains(n, want) {
			return true
		}
	}
	return false
}

func TestSnapshotShape(t *testing.T) {
	s := Snapshot(deltaReport())
	if len(s.PRs) != 2 {
		t.Fatalf("snapshot has %d entries, want 2", len(s.PRs))
	}
	e := s.PRs["acme/widgets#2"]
	if e.Owner != OwnerThem || e.ResponseAuthor != "sam" || e.Status != "blocked" {
		t.Fatalf("unexpected entry: %+v", e)
	}
}

func TestDiffDetectsChanges(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	prev := state.Snapshot{SavedAt: now.Add(-2 * time.Hour), PRs: map[string]state.Entry{
		"acme/widgets#1": {Status: "blocked", Owner: OwnerThem, Title: "Add widget", URL: "https://x/1"},
		"acme/widgets#2": {Status: "blocked", Owner: OwnerThem, Title: "Fix widget", ResponseAuthor: "sam", ResponseAt: now.Add(-time.Hour)},
		"acme/gone#9":    {Status: "up to date", Owner: OwnerThem, Title: "Old PR", URL: "https://x/9"},
	}}
	d := Diff(deltaReport(), prev, func(string, int) (bool, bool, error) {
		return true, false, nil // the vanished one was merged
	})
	if d.Total != 2 || d.UpToYou != 1 || d.Waiting != 1 {
		t.Fatalf("counts wrong: %+v", d)
	}
	notes := map[int][]string{}
	for _, c := range d.Changes {
		notes[c.Number] = c.Notes
	}
	if !hasNote(notes[1], "now up to you") {
		t.Fatalf("#1 notes = %v", notes[1])
	}
	if _, ok := notes[2]; ok {
		t.Fatalf("#2 should be unchanged, got %v", notes[2])
	}
	merged := false
	for _, c := range d.Changes {
		if c.Number == 9 && c.Repo == "acme/gone" && hasNote(c.Notes, "merged") {
			merged = true
		}
	}
	if !merged {
		t.Fatalf("vanished PR not reported as merged: %+v", d.Changes)
	}
}

func TestDiffNewResponseAndStatus(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	prev := state.Snapshot{SavedAt: now.Add(-time.Hour), PRs: map[string]state.Entry{
		"acme/widgets#1": {Status: "behind base", Owner: OwnerYou, Title: "Add widget"},
		"acme/widgets#2": {Status: "blocked", Owner: OwnerThem, ResponseAuthor: "old", ResponseAt: now.Add(-2 * time.Hour)},
	}}
	d := Diff(deltaReport(), prev, nil)
	seen := 0
	for _, c := range d.Changes {
		switch c.Number {
		case 1:
			if !hasNote(c.Notes, "status: behind base -> conflicts") {
				t.Fatalf("#1 = %v", c.Notes)
			}
			seen++
		case 2:
			if !hasNote(c.Notes, "new response from @sam") {
				t.Fatalf("#2 = %v", c.Notes)
			}
			seen++
		}
	}
	if seen != 2 {
		t.Fatalf("expected both changes, got %d: %+v", seen, d.Changes)
	}
}

func TestDiffFirstRunMarksNew(t *testing.T) {
	d := Diff(deltaReport(), state.Snapshot{PRs: map[string]state.Entry{}}, nil)
	if len(d.Changes) != 2 {
		t.Fatalf("first run should list both as new: %+v", d.Changes)
	}
	for _, c := range d.Changes {
		if !hasNote(c.Notes, "new pull request") {
			t.Fatalf("missing new marker: %+v", c)
		}
	}
}

func TestRenderDelta(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	d := &DeltaReport{
		User: "andy", GeneratedAt: now, PrevSavedAt: now.Add(-2 * time.Hour),
		UpToYou: 1, Waiting: 1, Total: 2,
		Changes: []Change{{Repo: "acme/widgets", Number: 1, URL: "https://x/1", Owner: OwnerYou, Notes: []string{"now up to you"}}},
	}
	md, err := RenderDelta(d, report.KindMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"# PR delta for andy",
		"Changed since the last run (2h ago)",
		"acme/widgets #1 · now up to you",
		"https://x/1",
		"1 change(s) · 1 up to you · 1 waiting · 2 seen.",
	} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown missing %q\n%s", want, md)
		}
	}
	js, err := RenderDelta(d, report.KindJSON)
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		UpToYou int `json:"up_to_you"`
		Changes []struct {
			Repo  string   `json:"repo"`
			Notes []string `json:"notes"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(js, &got); err != nil {
		t.Fatal(err)
	}
	if got.UpToYou != 1 || len(got.Changes) != 1 || got.Changes[0].Notes[0] != "now up to you" {
		t.Fatalf("unexpected JSON: %+v", got)
	}
}

func TestDiffVanishedStillOpen(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	prev := state.Snapshot{SavedAt: now, PRs: map[string]state.Entry{
		"acme/widgets#7": {Status: "up to date", Owner: OwnerThem, Title: "Old"},
	}}
	d := Diff(&Report{GeneratedAt: now, User: "andy"}, prev, func(string, int) (bool, bool, error) {
		return false, false, nil // the lookup says it is still open
	})
	if len(d.Changes) != 1 || !hasNote(d.Changes[0].Notes, "still open, but filtered out") {
		t.Fatalf("want a still-open note, got %+v", d.Changes)
	}
}

func TestRenderDeltaNothing(t *testing.T) {
	now := time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)
	d := &DeltaReport{User: "andy", GeneratedAt: now, PrevSavedAt: now.Add(-time.Hour), Waiting: 48, Total: 48}
	md, err := RenderDelta(d, report.KindMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"- nothing", "0 change(s) · 0 up to you · 48 waiting · 48 seen."} {
		if !strings.Contains(string(md), want) {
			t.Errorf("markdown missing %q\n%s", want, md)
		}
	}
}

func TestRenderDeltaUnknownKind(t *testing.T) {
	if _, err := RenderDelta(&DeltaReport{}, report.Kind("xlsx")); err == nil {
		t.Fatal("want an error for an unknown kind")
	}
}
