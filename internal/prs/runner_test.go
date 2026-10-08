package prs

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// prsTestServer serves the full set of endpoints a PR scout run touches for a
// single open pull request (PR #5, repo acme/widgets, head HEAD123).
func prsTestServer(t *testing.T, author string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/user" {
			_, _ = w.Write([]byte(`{"login":"` + author + `"}`))
			return
		}
		switch r.URL.Path {
		case "/search/issues":
			_, _ = w.Write([]byte(`{"total_count":1,"items":[{"repository_url":"https://api.github.com/repos/acme/widgets","number":5,"title":"Fix crash","html_url":"https://x/pr/5","updated_at":"2024-01-05T00:00:00Z"}]}`))
		case "/repos/acme/widgets/pulls/5":
			_, _ = w.Write([]byte(`{"mergeable":true,"mergeable_state":"behind","head":{"sha":"HEAD123"},"base":{"ref":"main"},"updated_at":"2024-01-05T00:00:00Z"}`))
		case "/repos/acme/widgets/pulls/5/reviews":
			_, _ = w.Write([]byte(`[{"user":{"login":"bob"},"submitted_at":"2024-01-04T00:00:00Z"}]`))
		case "/repos/acme/widgets/pulls/5/comments":
			_, _ = w.Write([]byte(`[]`))
		case "/repos/acme/widgets/issues/5/comments":
			_, _ = w.Write([]byte(`[{"user":{"login":"alice"},"created_at":"2024-01-06T00:00:00Z"}]`))
		case "/repos/acme/widgets/commits/HEAD123":
			_, _ = w.Write([]byte(`{"commit":{"committer":{"date":"2024-01-03T00:00:00Z"}}}`))
		case "/graphql":
			_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewDecision":"APPROVED","reviewThreads":{"nodes":[{"isResolved":false,"path":"libs/app.ts","line":12,"comments":{"nodes":[{"author":{"login":"bob"},"createdAt":"2024-01-07T00:00:00Z"}]}}]}}}}}`))
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
}

func TestRunnerRun(t *testing.T) {
	srv := prsTestServer(t, "andy")
	defer srv.Close()
	c := github.New("test-token").SetBaseURL(srv.URL)

	rep, err := NewRunner(c).Run(context.Background(), Options{User: "andy", Max: 50})
	if err != nil {
		t.Fatal(err)
	}
	if rep.User != "andy" {
		t.Fatalf("got user %q, want andy", rep.User)
	}
	if len(rep.Repos) != 1 || rep.Repos[0].Name != "acme/widgets" {
		t.Fatalf("got repos %+v, want acme/widgets", rep.Repos)
	}
	pr := rep.Repos[0].PRs
	if len(pr) != 1 {
		t.Fatalf("got %d PRs, want 1", len(pr))
	}
	if pr[0].Number != 5 || pr[0].Status != "behind base" {
		t.Fatalf("PR decode mismatch: %+v", pr[0])
	}
	// The user pushed 2024-01-03, then bob reviewed 01-04 and alice commented
	// 01-06; the newest response is alice's.
	if pr[0].Response == nil || pr[0].Response.Author != "alice" {
		t.Fatalf("response not detected: %+v", pr[0].Response)
	}
	if rep.Repos[0].Skipped != 0 {
		t.Fatalf("expected no skips, got %d", rep.Repos[0].Skipped)
	}
	// The branch is behind base (rebase) and an open thread has bob speaking
	// last (01-07) after andy's push (01-03) - both make the PR the author's.
	if pr[0].Action.Owner != OwnerYou {
		t.Fatalf("expected action on the author, got %+v", pr[0].Action)
	}
}

func TestRunnerRunAutoDetectsUser(t *testing.T) {
	srv := prsTestServer(t, "detected-user")
	defer srv.Close()
	c := github.New("test-token").SetBaseURL(srv.URL)

	rep, err := NewRunner(c).Run(context.Background(), Options{Max: 50})
	if err != nil {
		t.Fatal(err)
	}
	if rep.User != "detected-user" {
		t.Fatalf("got user %q, want detected-user", rep.User)
	}
}

func TestRunnerSkipsFailedPR(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/search/issues":
			_, _ = w.Write([]byte(`{"total_count":1,"items":[{"repository_url":"https://api.github.com/repos/acme/widgets","number":7,"title":"Broken","html_url":"https://x/pr/7","updated_at":"2024-01-05T00:00:00Z"}]}`))
		default:
			http.Error(w, "boom", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	c := github.New("test-token").SetBaseURL(srv.URL)

	rep, err := NewRunner(c).Run(context.Background(), Options{User: "andy", Max: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Repos) != 1 || rep.Repos[0].Skipped != 1 {
		t.Fatalf("expected one skipped PR, got %+v", rep.Repos)
	}
	if len(rep.Repos[0].Notes) != 1 {
		t.Fatalf("expected one skip note, got %+v", rep.Repos[0].Notes)
	}
}
