package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReviewState(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/graphql" || r.Method != http.MethodPost {
			http.Error(w, "bad path/method", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"repository":{"pullRequest":{"reviewDecision":"CHANGES_REQUESTED","reviewThreads":{"nodes":[
			{"isResolved":false,"path":"a.ts","line":3,"comments":{"nodes":[{"author":{"login":"bot"},"createdAt":"2024-01-05T00:00:00Z"}]}},
			{"isResolved":true,"path":"b.ts","line":9,"comments":{"nodes":[{"author":{"login":"bot"},"createdAt":"2024-01-04T00:00:00Z"}]}},
			{"isResolved":false,"path":"c.ts","line":null,"comments":{"nodes":[{"author":{"login":"bot"},"createdAt":"2024-01-06T00:00:00Z"}]}}
		]}}}}}`))
	}))
	defer srv.Close()

	rs, err := New("t").SetBaseURL(srv.URL).ReviewState(context.Background(), "acme/widgets", 5)
	if err != nil {
		t.Fatal(err)
	}
	if rs.Decision != "CHANGES_REQUESTED" {
		t.Fatalf("decision=%q", rs.Decision)
	}
	if len(rs.OpenThreads) != 2 {
		t.Fatalf("open threads=%d want 2 (resolved filtered), got %+v", len(rs.OpenThreads), rs.OpenThreads)
	}
	th := rs.OpenThreads[0]
	if th.Path != "a.ts" || th.Line != 3 || th.LastAuthor != "bot" {
		t.Fatalf("thread=%+v", th)
	}
	if !th.LastAt.Equal(time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("lastAt=%v", th.LastAt)
	}
	if l := rs.OpenThreads[1].Line; l != 0 {
		t.Fatalf("null line should decode to 0, got %d", l)
	}
}

func TestReviewStateGraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errors":[{"message":"boom"}]}`))
	}))
	defer srv.Close()

	if _, err := New("t").SetBaseURL(srv.URL).ReviewState(context.Background(), "acme/widgets", 5); err == nil {
		t.Fatal("expected an error from GraphQL errors")
	}
}

func TestReviewStateBadRepo(t *testing.T) {
	if _, err := New("t").ReviewState(context.Background(), "no-slash", 5); err == nil {
		t.Fatal("expected an error for a repo without a slash")
	}
}
