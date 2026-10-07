package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer returns a client pointed at an httptest server whose handler
// asserts on the request (authorization, expected path) and responds with the
// given status and body.
func newTestClient(t *testing.T, wantPath string, wantAuth bool, status int, body string) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, wantPath) {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusInternalServerError)
			return
		}
		gotAuth := r.Header.Get("Authorization") != ""
		if gotAuth != wantAuth {
			http.Error(w, "authorization mismatch", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	c := New("test-token")
	c.baseURL = srv.URL
	return c, srv
}

func TestClientIssues(t *testing.T) {
	body := `[
	  {"number": 3, "title": "crash", "body": "steps", "created_at": "2024-01-02T03:04:05Z",
	   "html_url": "https://x/i/3", "comments": 2,
	   "labels": [{"name": "bug"}]},
	  {"number": 4, "title": "a PR, not an issue", "body": "", "pull_request": {}}
	]`
	c, srv := newTestClient(t, "/issues", true, http.StatusOK, body)
	defer srv.Close()

	issues, err := c.Issues(context.Background(), "acme/widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1 (the PR must be filtered out)", len(issues))
	}
	it := issues[0]
	if it.Number != 3 || it.Title != "crash" || len(it.Labels) != 1 || it.Labels[0] != "bug" {
		t.Fatalf("decode mismatch: %+v", it)
	}
	if it.CreatedAt.IsZero() {
		t.Fatal("created_at was not parsed")
	}
	if !strings.Contains(it.HTMLURL, "3") {
		t.Fatalf("html_url not decoded: %q", it.HTMLURL)
	}
}

func TestClientPullRequests(t *testing.T) {
	body := `[{"number": 9, "title": "fix crash", "body": "Fixes #3", "html_url": "https://x/pr/9"}]`
	c, srv := newTestClient(t, "/pulls", true, http.StatusOK, body)
	defer srv.Close()

	prs, err := c.OpenPullRequests(context.Background(), "acme/widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(prs) != 1 || prs[0].Number != 9 || prs[0].Body != "Fixes #3" {
		t.Fatalf("decode mismatch: %+v", prs)
	}
}

func TestClientRateLimit(t *testing.T) {
	c, srv := newTestClient(t, "/issues", true, http.StatusTooManyRequests, `{}`)
	defer srv.Close()
	_, err := c.Issues(context.Background(), "acme/widgets")
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("want rate-limit error, got %v", err)
	}
}

func TestClientServerError(t *testing.T) {
	c, srv := newTestClient(t, "/issues", true, http.StatusServiceUnavailable, ``)
	defer srv.Close()
	_, err := c.Issues(context.Background(), "acme/widgets")
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("want 503 error, got %v", err)
	}
}

func TestClientFollowsPagination(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		if hits == 1 {
			w.Header().Set("Link", `<`+srvURL(r)+`?page=2>; rel="next"`)
			_, _ = w.Write([]byte(`[{"number":1}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"number":2}]`))
	}))
	defer srv.Close()

	c := New("")
	c.baseURL = srv.URL
	issues, err := c.Issues(context.Background(), "acme/widgets")
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 2 || issues[0].Number != 1 || issues[1].Number != 2 {
		t.Fatalf("pagination decode mismatch: %+v", issues)
	}
	if hits != 2 {
		t.Fatalf("expected 2 page requests, got %d", hits)
	}
}

// srvURL reconstructs the test server URL from an incoming request.
func srvURL(r *http.Request) string {
	return "http://" + r.Host
}

func TestClientIssuesWithoutToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			http.Error(w, "unexpected auth header", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := New("") // no token
	c.baseURL = srv.URL
	if _, err := c.Issues(context.Background(), "acme/widgets"); err != nil {
		t.Fatal(err)
	}
}
