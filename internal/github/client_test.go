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

	t.Setenv("GITHUB_TOKEN", "")
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

func TestClientCurrentUser(t *testing.T) {
	c, srv := newTestClient(t, "/user", true, http.StatusOK, `{"login":"andy"}`)
	defer srv.Close()
	login, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if login != "andy" {
		t.Fatalf("got login %q, want andy", login)
	}
}

func TestClientCurrentUserNoToken(t *testing.T) {
	c := &Client{client: &http.Client{}, token: "", baseURL: "http://none"}
	if _, err := c.CurrentUser(context.Background()); err == nil {
		t.Fatal("expected an error when no token is set")
	}
}

func TestClientSearchPullRequests(t *testing.T) {
	body := `{"total_count":1,"items":[{
		"repository_url":"https://api.github.com/repos/acme/widgets",
		"number":42,"title":"Fix crash","html_url":"https://x/pr/42",
		"updated_at":"2024-01-02T03:04:05Z"}]}`
	c, srv := newTestClient(t, "/search/issues", true, http.StatusOK, body)
	defer srv.Close()
	refs, err := c.SearchPullRequests(context.Background(), "andy")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 {
		t.Fatalf("got %d refs, want 1", len(refs))
	}
	ref := refs[0]
	if ref.Repo != "acme/widgets" || ref.Number != 42 || ref.Title != "Fix crash" {
		t.Fatalf("decode mismatch: %+v", ref)
	}
	if ref.UpdatedAt.IsZero() {
		t.Fatal("updated_at not parsed")
	}
}

func TestClientPullRequest(t *testing.T) {
	body := `{"mergeable":false,"mergeable_state":"dirty",
		"head":{"sha":"abc123"},"base":{"ref":"main"},
		"updated_at":"2024-01-02T03:04:05Z"}`
	c, srv := newTestClient(t, "/pulls/42", true, http.StatusOK, body)
	defer srv.Close()
	d, err := c.PullRequest(context.Background(), "acme/widgets", 42)
	if err != nil {
		t.Fatal(err)
	}
	if d.Mergeable == nil || *d.Mergeable {
		t.Fatalf("mergeable decode wrong: %+v", d.Mergeable)
	}
	if d.MergeableState != "dirty" || d.HeadSHA != "abc123" || d.BaseRef != "main" {
		t.Fatalf("decode mismatch: %+v", d)
	}
	if d.UpdatedAt.IsZero() {
		t.Fatal("updated_at not parsed")
	}
}

func TestClientIssueCommenters(t *testing.T) {
	body := `[{"user":{"login":"alice"},"created_at":"2024-01-02T03:04:05Z"}]`
	c, srv := newTestClient(t, "/issues/5/comments", true, http.StatusOK, body)
	defer srv.Close()
	acts, err := c.IssueCommenters(context.Background(), "acme/widgets", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].Login != "alice" || acts[0].At.IsZero() {
		t.Fatalf("decode mismatch: %+v", acts)
	}
}

func TestClientReviewers(t *testing.T) {
	body := `[{"user":{"login":"bob"},"submitted_at":"2024-01-02T03:04:05Z"}]`
	c, srv := newTestClient(t, "/pulls/5/reviews", true, http.StatusOK, body)
	defer srv.Close()
	acts, err := c.Reviewers(context.Background(), "acme/widgets", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].Login != "bob" || acts[0].At.IsZero() {
		t.Fatalf("decode mismatch: %+v", acts)
	}
}

func TestClientReviewCommenters(t *testing.T) {
	body := `[{"user":{"login":"carol"},"created_at":"2024-01-02T03:04:05Z"}]`
	c, srv := newTestClient(t, "/pulls/5/comments", true, http.StatusOK, body)
	defer srv.Close()
	acts, err := c.ReviewCommenters(context.Background(), "acme/widgets", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].Login != "carol" || acts[0].At.IsZero() {
		t.Fatalf("decode mismatch: %+v", acts)
	}
}

func TestClientActivitiesFollowsPagination(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "application/json")
		if hits == 1 {
			w.Header().Set("Link", `<`+srvURL(r)+`?page=2>; rel="next"`)
			_, _ = w.Write([]byte(`[{"user":{"login":"alice"},"created_at":"2024-01-02T03:04:05Z"}]`))
			return
		}
		_, _ = w.Write([]byte(`[{"user":{"login":"bob"},"created_at":"2024-01-03T03:04:05Z"}]`))
	}))
	defer srv.Close()
	t.Setenv("GITHUB_TOKEN", "")
	c := New("")
	c.baseURL = srv.URL
	acts, err := c.IssueCommenters(context.Background(), "acme/widgets", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 2 || acts[0].Login != "alice" || acts[1].Login != "bob" {
		t.Fatalf("pagination decode mismatch: %+v", acts)
	}
	if hits != 2 {
		t.Fatalf("expected 2 page requests, got %d", hits)
	}
}

func TestClientCommitDate(t *testing.T) {
	body := `{"commit":{"committer":{"date":"2024-01-02T03:04:05Z"}}}`
	c, srv := newTestClient(t, "/commits/abc123", true, http.StatusOK, body)
	defer srv.Close()
	ts, err := c.CommitDate(context.Background(), "acme/widgets", "abc123")
	if err != nil {
		t.Fatal(err)
	}
	if ts.IsZero() {
		t.Fatal("commit date not parsed")
	}
}

func TestClientNewMethodRateLimit(t *testing.T) {
	c, srv := newTestClient(t, "/pulls/42", true, http.StatusTooManyRequests, `{}`)
	defer srv.Close()
	_, err := c.PullRequest(context.Background(), "acme/widgets", 42)
	if err == nil || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("want rate-limit error, got %v", err)
	}
}

func TestClientCommitDateBadDate(t *testing.T) {
	body := `{"commit":{"committer":{"date":"not-a-date"}}}`
	c, srv := newTestClient(t, "/commits/abc123", true, http.StatusOK, body)
	defer srv.Close()
	if _, err := c.CommitDate(context.Background(), "acme/widgets", "abc123"); err == nil {
		t.Fatal("expected an error for a malformed commit date")
	}
}

func TestClientCurrentUserEmptyLogin(t *testing.T) {
	c, srv := newTestClient(t, "/user", true, http.StatusOK, `{"other":"x"}`)
	defer srv.Close()
	if _, err := c.CurrentUser(context.Background()); err == nil {
		t.Fatal("expected an error when /user has no login")
	}
}

func TestClientCurrentUserBadJSON(t *testing.T) {
	c, srv := newTestClient(t, "/user", true, http.StatusOK, `not json`)
	defer srv.Close()
	if _, err := c.CurrentUser(context.Background()); err == nil {
		t.Fatal("expected a decode error")
	}
}

func TestClientSearchPullRequestsSkipsInvalidRepo(t *testing.T) {
	body := `{"total_count":1,"items":[{"repository_url":"https://not-github/nothing","number":1}]}`
	c, srv := newTestClient(t, "/search/issues", true, http.StatusOK, body)
	defer srv.Close()
	refs, err := c.SearchPullRequests(context.Background(), "andy")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 0 {
		t.Fatalf("got %d refs, want 0 (invalid repository_url must be skipped)", len(refs))
	}
}

func TestClientActivitiesMalformedDate(t *testing.T) {
	body := `[{"user":{"login":"alice"},"created_at":"not-a-date"}]`
	c, srv := newTestClient(t, "/issues/5/comments", true, http.StatusOK, body)
	defer srv.Close()
	acts, err := c.IssueCommenters(context.Background(), "acme/widgets", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 || acts[0].Login != "alice" || !acts[0].At.IsZero() {
		t.Fatalf("malformed date should yield zero time: %+v", acts)
	}
}

func TestClientSetBaseURL(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "")
	c := New("")
	if got := c.SetBaseURL("http://override"); got != c {
		t.Fatal("SetBaseURL should return the receiver for chaining")
	}
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

	t.Setenv("GITHUB_TOKEN", "")
	c := New("") // no token
	c.baseURL = srv.URL
	if _, err := c.Issues(context.Background(), "acme/widgets"); err != nil {
		t.Fatal(err)
	}
}
