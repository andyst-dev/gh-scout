package prs

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/andyst-dev/gh-scout/internal/report"
)

func prsFixture() *Report {
	now := time.Now()
	return &Report{
		GeneratedAt: now,
		User:        "andy",
		Repos: []Repo{
			{
				Name: "acme/widgets",
				PRs: []PR{
					{Number: 42, Title: "Fix crash", URL: "https://x/pr/42", Status: "up to date",
						Response: &Response{Author: "alice", Age: "2d ago"},
						Action:   Action{Owner: OwnerThem, Waiting: "review"}},
					{Number: 43, Title: "Add feature", URL: "https://x/pr/43", Status: "conflicts",
						Action: Action{Owner: OwnerYou, Reasons: []string{"rebase: the branch conflicts with its base"}},
						Hint:   "may be superseded: old/path.ts no longer exists on main"},
				},
			},
			{
				Name:    "acme/gadgets",
				Skipped: 1,
				Notes:   []string{"pull request #7: GitHub API 500"},
			},
		},
	}
}

func TestWriteMarkdown(t *testing.T) {
	out, err := writeMarkdown(prsFixture())
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{
		"# PR scout for andy ·",
		"## Up to you (1)",
		"acme/widgets #43 · conflicts | rebase: the branch conflicts with its base",
		"## Waiting on others (1)",
		"acme/widgets #42 · up to date | review (response: @alice 2d ago)",
		"## acme/widgets",
		"## acme/gadgets",
		"[#42 · up to date] Fix crash",
		"(response: @alice 2d ago)",
		"[#43 · conflicts] Add feature",
		"may be superseded: old/path.ts no longer exists on main",
		"Skipped 1 pull request(s).",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("markdown missing %q\n%s", want, s)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	out, err := writeJSON(prsFixture())
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		User  string `json:"user"`
		Repos []struct {
			Name         string `json:"name"`
			PullRequests []struct {
				Number   int    `json:"number"`
				Status   string `json:"status"`
				Hint     string `json:"hint"`
				Response *struct {
					Author string `json:"author"`
					Age    string `json:"age"`
				} `json:"response"`
			} `json:"pull_requests"`
			Skipped int `json:"skipped"`
		} `json:"repos"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
	if got.User != "andy" || len(got.Repos) != 2 {
		t.Fatalf("unexpected JSON payload: %+v", got)
	}
	first := got.Repos[0]
	if first.PullRequests[0].Number != 42 || first.PullRequests[0].Status != "up to date" {
		t.Fatalf("unexpected first PR: %+v", first.PullRequests[0])
	}
	if first.PullRequests[0].Response == nil || first.PullRequests[0].Response.Author != "alice" {
		t.Fatalf("response not rendered: %+v", first.PullRequests[0])
	}
	if first.PullRequests[1].Response != nil {
		t.Fatalf("response of PR 43 should be omitted, got %+v", first.PullRequests[1].Response)
	}
	if first.PullRequests[1].Hint == "" {
		t.Fatal("hint of PR 43 should be rendered in JSON")
	}
	if got.Repos[1].Skipped != 1 {
		t.Fatalf("skipped count wrong: %+v", got.Repos[1])
	}
}

func TestRenderUnknownKind(t *testing.T) {
	if _, err := Render(prsFixture(), report.Kind("xlsx")); err == nil {
		t.Fatal("expected an error for an unknown kind")
	}
}

func TestRenderDispatch(t *testing.T) {
	md, err := Render(prsFixture(), report.KindMarkdown)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(md), "# PR scout") {
		t.Fatalf("markdown dispatch wrong:\n%s", md)
	}
}
