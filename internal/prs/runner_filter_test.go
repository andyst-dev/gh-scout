package prs

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
	"github.com/andyst-dev/gh-scout/internal/report"
)

// multiPRServer serves a general-purpose PR scout endpoint set. items is the
// raw JSON payload (the `.items` array) for /search/issues; failSuffix, when
// non-empty, makes any path whose request URL ends with it return 500 (to
// exercise the error/skip paths).
func multiPRServer(t *testing.T, items string, failSuffix string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if failSuffix != "" && strings.HasSuffix(r.URL.Path, failSuffix) {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		switch {
		case r.URL.Path == "/search/issues":
			_, _ = w.Write([]byte(`{"total_count":3,"items":` + items + `}`))
		case strings.Contains(r.URL.Path, "/reviews"), strings.Contains(r.URL.Path, "/comments"):
			_, _ = w.Write([]byte(`[]`))
		case strings.Contains(r.URL.Path, "/commits/"):
			_, _ = w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-02T00:00:00Z"}}}`))
		case strings.Contains(r.URL.Path, "/pulls/"):
			// head SHA is derived from the PR number so individual commits can
			// be failed in tests.
			parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
			sha := "SHA" + parts[len(parts)-1]
			_, _ = w.Write([]byte(`{"mergeable":true,"mergeable_state":"clean","head":{"sha":"` + sha + `"},"base":{"ref":"main"},"updated_at":"2024-01-05T00:00:00Z"}`))
		case r.URL.Path == "/graphql":
			_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewDecision":"","reviewThreads":{"nodes":[]}}}}}`))
		default:
			http.Error(w, "unexpected "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
}

func prsItem(repo string, number int, title, updatedAt string) string {
	return fmt.Sprintf(`{"repository_url":"https://api.github.com/repos/%s","number":%d,"title":%q,"html_url":"https://x/%d","updated_at":%q}`,
		repo, number, title, number, updatedAt)
}

func runWith(t *testing.T, srv *httptest.Server, opts Options) *Report {
	t.Helper()
	c := github.New("test-token").SetBaseURL(srv.URL)
	rep, err := NewRunner(c).Run(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	return rep
}

func TestRunnerFiltersRepos(t *testing.T) {
	srv := multiPRServer(t, `[`+
		prsItem("acme/widgets", 1, "One", "2024-01-05T00:00:00Z")+`,`+
		prsItem("acme/other", 2, "Two", "2024-01-04T00:00:00Z")+`]`, "")
	defer srv.Close()

	rep := runWith(t, srv, Options{User: "andy", Repos: []string{"acme/widgets"}, Max: 10})
	if len(rep.Repos) != 1 || rep.Repos[0].Name != "acme/widgets" {
		t.Fatalf("repo filter failed: %+v", rep.Repos)
	}
	if n := len(rep.Repos[0].PRs); n != 1 {
		t.Fatalf("got %d PRs, want 1", n)
	}
}

func TestRunnerMaxCaps(t *testing.T) {
	srv := multiPRServer(t, `[`+
		prsItem("acme/widgets", 3, "Three", "2024-01-05T00:00:00Z")+`,`+
		prsItem("acme/other", 4, "Four", "2024-01-04T00:00:00Z")+`]`, "")
	defer srv.Close()

	rep := runWith(t, srv, Options{User: "andy", Max: 1})
	if len(rep.Repos) != 1 || len(rep.Repos[0].PRs) != 1 {
		t.Fatalf("max cap failed: %+v", rep.Repos)
	}
	if rep.Repos[0].PRs[0].Number != 3 {
		t.Fatalf("expected first PR by search order, got %d", rep.Repos[0].PRs[0].Number)
	}
}

func TestRunnerSinceDropsStale(t *testing.T) {
	fresh := time.Now().Add(-1 * time.Hour).UTC().Format(time.RFC3339)
	stale := time.Now().Add(-100 * time.Hour).UTC().Format(time.RFC3339)
	srv := multiPRServer(t, `[`+
		prsItem("acme/widgets", 5, "Fresh", fresh)+`,`+
		prsItem("acme/other", 6, "Stale", stale)+`]`, "")
	defer srv.Close()

	rep := runWith(t, srv, Options{User: "andy", Since: 24 * time.Hour, Max: 10})
	if len(rep.Repos) != 1 || len(rep.Repos[0].PRs) != 1 {
		t.Fatalf("since filter failed: %+v", rep.Repos)
	}
	if rep.Repos[0].PRs[0].Number != 5 {
		t.Fatalf("expected only the fresh PR, got %+v", rep.Repos)
	}
}

func TestRunnerSkippedWhenCommitDateFails(t *testing.T) {
	// Both PRs are in the same repo; #8's head commit (SHA8) fails while #7's
	// succeeds, so the repo gains one PR plus one skip note.
	srv := multiPRServer(t, `[`+
		prsItem("acme/widgets", 8, "Fail", "2024-01-05T00:00:00Z")+`,`+
		prsItem("acme/widgets", 7, "Ok", "2024-01-05T00:00:00Z")+`]`, "/commits/SHA8")
	defer srv.Close()

	rep := runWith(t, srv, Options{User: "andy", Max: 10})
	if len(rep.Repos) != 1 || rep.Repos[0].Name != "acme/widgets" {
		t.Fatalf("grouping failed: %+v", rep.Repos)
	}
	if len(rep.Repos[0].PRs) != 1 || rep.Repos[0].PRs[0].Number != 7 {
		t.Fatalf("expected PR #7 only, got %+v", rep.Repos[0].PRs)
	}
	if rep.Repos[0].Skipped != 1 || len(rep.Repos[0].Notes) != 1 {
		t.Fatalf("expected one skip note, got %+v", rep.Repos[0])
	}
}

func TestRunnerSkipsWhenActivityFails(t *testing.T) {
	srv := multiPRServer(t, `[`+prsItem("acme/widgets", 9, "Nine", "2024-01-05T00:00:00Z")+`]`, "/reviews")
	defer srv.Close()

	rep := runWith(t, srv, Options{User: "andy", Max: 10})
	// With no ready PR entries the repo appears only through Skip.
	if len(rep.Repos) != 1 || rep.Repos[0].Skipped != 1 {
		t.Fatalf("expected one skipped PR via activity failure, got %+v", rep.Repos)
	}
	if len(rep.Repos[0].PRs) != 0 {
		t.Fatalf("expected no PRs when activity fails, got %+v", rep.Repos[0].PRs)
	}
}

func TestRenderJSONViaRender(t *testing.T) {
	js, err := Render(prsFixture(), report.KindJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(js), `"user": "andy"`) {
		t.Fatalf("json render wrong:\n%s", js)
	}
}
