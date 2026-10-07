// Package github provides a minimal client for the GitHub REST API used by
// gh-scout. It intentionally covers only the endpoints gh-scout needs and
// carries no dependency beyond the standard library.
package github

import (
	"context"
	"time"
)

// Issue is the subset of a GitHub issue gh-scout cares about.
type Issue struct {
	Number    int
	Title     string
	Body      string
	CreatedAt time.Time
	HTMLURL   string
	Labels    []string
	Comments  int
}

// PullRequest is the subset of a GitHub pull request needed to detect that an
// issue is already being worked on.
type PullRequest struct {
	Number int
	Title  string
	Body   string
	URL    string
}

// IssueLister is the interface the scout depends on to fetch repository data.
// It exists so the scout can be tested against a fake and the HTTP client can
// stay a thin, swappable implementation.
type IssueLister interface {
	// Issues returns the open pull-request-free issues of a repository,
	// newest first.
	Issues(ctx context.Context, repo string) ([]Issue, error)
	// OpenPullRequests returns the open pull requests of a repository.
	OpenPullRequests(ctx context.Context, repo string) ([]PullRequest, error)
}
