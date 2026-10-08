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
// issue is already being worked on (open PR) or already shipped (merged PR).
type PullRequest struct {
	Number int
	Title  string
	Body   string
	URL    string
	// Merged is true only for pull requests that actually merged (closed PRs
	// without a merge leave the issue open work).
	Merged bool
}

// PRRef is one of a user's open pull requests as found by the search endpoint.
type PRRef struct {
	Repo      string
	Number    int
	Title     string
	HTMLURL   string
	UpdatedAt time.Time
}

// PRDetail is the subset of a single pull request needed to assess its state.
type PRDetail struct {
	// Mergeable is nil when GitHub could not (or will not) compute a
	// mergeability verdict; false means the branch conflicts with its base.
	Mergeable      *bool
	MergeableState string
	// HeadSHA is the commit SHA of the pull request's head branch.
	HeadSHA string
	// BaseRef is the name of the branch the pull request targets.
	BaseRef   string
	UpdatedAt time.Time
}

// FileChange is one path a pull request touches, with GitHub's change status
// (added, modified, removed, renamed).
type FileChange struct {
	Path   string
	Status string
	// Patch is the unified diff hunks GitHub returns for the file, or "" when
	// the diff is too large for the REST payload to carry.
	Patch string
}

// Activity is one comment or review left by a user on a pull request.
type Activity struct {
	Login string
	At    time.Time
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
	// MergedPullRequests returns recent closed pull requests that merged,
	// newest first, so an issue a merged PR already implemented can be dropped.
	MergedPullRequests(ctx context.Context, repo string) ([]PullRequest, error)
}
