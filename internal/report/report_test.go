package report

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/andyst-dev/gh-scout/internal/scout"
)

func newFixture() *scout.Report {
	now := time.Now()
	return &scout.Report{
		GeneratedAt: now,
		Repos:       []string{"acme/widgets", "acme/gadgets"},
		Candidates: []scout.Candidate{
			{Repository: "acme/widgets", Number: 41, Title: "Crash on empty config", Status: scout.StatusReady, Score: 82, Reason: "clear defect; has repro", URL: "https://x/41", CreatedAt: now},
			{Repository: "acme/widgets", Number: 9, Title: "Meh", Status: scout.StatusUnclear, Score: 5},
			{Repository: "acme/gadgets", Number: 3, Title: "Old bug", Status: scout.StatusAddressed, Reason: "open PR #12 already references it"},
		},
	}
}

func TestMarkdownWriter(t *testing.T) {
	w, err := New(KindMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	out, err := w.Write(newFixture())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)

	for _, want := range []string{"# Contribution scouting", "## Ready targets", "#41", "acme/gadgets#3", "open PR #12"} {
		if !strings.Contains(s, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}

func TestJSONWriter(t *testing.T) {
	w, err := New(KindJSON)
	if err != nil {
		t.Fatal(err)
	}
	out, err := w.Write(newFixture())
	if err != nil {
		t.Fatal(err)
	}

	var got struct {
		Repositories []string `json:"repositories"`
		Ready        []struct {
			Number int `json:"number"`
		} `json:"ready"`
		Summary struct {
			Examined int `json:"examined"`
			Ready    int `json:"ready"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if len(got.Repositories) != 2 || got.Ready[0].Number != 41 {
		t.Fatalf("unexpected JSON payload: %+v", got)
	}
	if got.Summary.Examined != 3 || got.Summary.Ready != 1 {
		t.Fatalf("summary wrong: %+v", got.Summary)
	}
}

func TestNewUnknownKind(t *testing.T) {
	if _, err := New(Kind("xlsx")); err == nil {
		t.Fatal("expected an error for an unknown kind")
	}
}
