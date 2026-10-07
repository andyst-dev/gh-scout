package scout

import (
	"context"
	"testing"
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// fakeLister is an observable IssueLister used to exercise the runner.
type fakeLister struct {
	issues []github.Issue
	prs    []github.PullRequest
}

func (f *fakeLister) Issues(_ context.Context, _ string) ([]github.Issue, error) {
	return f.issues, nil
}

func (f *fakeLister) OpenPullRequests(_ context.Context, _ string) ([]github.PullRequest, error) {
	return f.prs, nil
}

func TestRunnerStatuses(t *testing.T) {
	now := time.Now()
	l := &fakeLister{
		issues: []github.Issue{
			{Number: 1, Title: "Clear crash on empty config", Body: "Repro:\n```\nrun()\n```\nUnit test.\nexpects no panic.", Labels: []string{"bug"}, CreatedAt: now},
			{Number: 2, Title: "Logs leak the auth token", Body: "Auth token printed on failure.\nUnit test: redact and log.", Labels: []string{"bug"}, CreatedAt: now},
			{Number: 3, Title: "Meh", Body: "", CreatedAt: now},          // unclear
			{Number: 4, Title: "Vague one", Body: "idk", CreatedAt: now}, // unclear
		},
		prs: []github.PullRequest{
			{Number: 100, Title: "Redact token", Body: "Fixes #2"},
		},
	}

	runner := NewRunner(l)
	rep, err := runner.Run(context.Background(), []string{"acme/widgets"}, Options{})
	if err != nil {
		t.Fatal(err)
	}

	byNum := map[int]Candidate{}
	for _, c := range rep.Candidates {
		byNum[c.Number] = c
	}

	if had := byNum[2]; had.Status != StatusAddressed {
		t.Fatalf("issue #2 expected %s got %s", StatusAddressed, had.Status)
	}
	if had := byNum[1]; had.Status != StatusReady {
		t.Fatalf("issue #1 expected %s got %s (%q, score %d)", StatusReady, had.Status, had.Reason, had.Score)
	}
	if had := byNum[3]; had.Status != StatusUnclear {
		t.Fatalf("issue #3 expected %s got %s", StatusUnclear, had.Status)
	}
	if had := byNum[4]; had.Status != StatusUnclear {
		t.Fatalf("issue #4 expected %s got %s", StatusUnclear, had.Status)
	}
	if byNum[1].Score <= byNum[4].Score {
		t.Fatalf("issue #1 (score %d) should outrank #4 (score %d)", byNum[1].Score, byNum[4].Score)
	}
}

func TestFilterIssues(t *testing.T) {
	now := time.Now()
	old := github.Issue{Number: 1, CreatedAt: now.Add(-10 * 24 * time.Hour), Labels: nil}
	new := github.Issue{Number: 2, CreatedAt: now, Labels: []string{"bug"}}
	noLabel := github.Issue{Number: 3, CreatedAt: now, Labels: []string{"docs"}}

	got := filterIssues([]github.Issue{old, new, noLabel}, Options{
		Since:      5 * 24 * time.Hour,
		Labels:     []string{"bug"},
		MaxPerRepo: 1,
	})
	if len(got) != 1 || got[0].Number != 2 {
		t.Fatalf("filterIssues kept %d, want exactly [#2]", len(got))
	}
}
