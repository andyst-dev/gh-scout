package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ReviewState is a pull request's review verdict plus its open (unresolved)
// review threads. Both come from the GraphQL API: REST exposes neither the
// aggregate review_decision nor whether a review thread was resolved.
type ReviewState struct {
	// Decision is APPROVED, CHANGES_REQUESTED or REVIEW_REQUIRED; empty when
	// the repository does not require reviews.
	Decision string
	// OpenThreads are the review threads that were never resolved.
	OpenThreads []Thread
	// LastReview is the pull request's most recent submitted review, used to
	// tell an answered-and-addressed CHANGES_REQUESTED (the reviewer merely has
	// to re-approve) from a freshly requested change the author has not acted
	// on yet.
	LastReview ReviewView
}

// ReviewView is one submitted review reduced to the state and the time it was
// submitted.
type ReviewView struct {
	State string
	At    time.Time
}

// Thread is one unresolved review thread reduced to what gh-scout needs: where
// it is anchored and who spoke last.
type Thread struct {
	Path string
	// Line is 0 when the anchor is not tied to a line (outdated or file-level).
	Line int
	// LastAuthor is the login of the thread's newest comment, empty when the
	// thread has no comments.
	LastAuthor string
	// LastAt is when that newest comment was written.
	LastAt time.Time
}

// Anchor renders the thread location, e.g. "libs/a.ts:12".
func (t Thread) Anchor() string {
	if t.Line > 0 {
		return fmt.Sprintf("%s:%d", t.Path, t.Line)
	}
	if t.Path != "" {
		return t.Path
	}
	return "the review thread"
}

// reviewStateQuery fetches the review verdict and the open review threads of
// one pull request. Threads are resolved only by a human click, so the query
// returns every thread and lets the caller filter on isResolved.
const reviewStateQuery = `query($owner:String!,$name:String!,$number:Int!){repository(owner:$owner,name:$name){pullRequest(number:$number){reviewDecision reviewThreads(first:100){nodes{isResolved path line comments(last:1){nodes{author{login} createdAt}}}} reviews(last:1){nodes{state submittedAt}}}}}`

// ReviewState fetches the review decision and the unresolved review threads of
// one pull request via the GraphQL API.
func (c *Client) ReviewState(ctx context.Context, repo string, number int) (ReviewState, error) {
	owner, name, ok := splitOwnerName(repo)
	if !ok {
		return ReviewState{}, fmt.Errorf("repository %q must be owner/name", repo)
	}

	payload := map[string]any{
		"query": reviewStateQuery,
		"variables": map[string]any{
			"owner":  owner,
			"name":   name,
			"number": number,
		},
	}
	var raw struct {
		Data struct {
			Repository struct {
				PullRequest struct {
					ReviewDecision string `json:"reviewDecision"`
					ReviewThreads  struct {
						Nodes []struct {
							IsResolved bool   `json:"isResolved"`
							Path       string `json:"path"`
							Line       *int   `json:"line"`
							Comments   struct {
								Nodes []struct {
									Author struct {
										Login string `json:"login"`
									} `json:"author"`
									CreatedAt string `json:"createdAt"`
								} `json:"nodes"`
							} `json:"comments"`
						} `json:"nodes"`
					} `json:"reviewThreads"`
					Reviews struct {
						Nodes []struct {
							State       string `json:"state"`
							SubmittedAt string `json:"submittedAt"`
						} `json:"nodes"`
					} `json:"reviews"`
				} `json:"pullRequest"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := c.postGraphQL(ctx, payload, &raw); err != nil {
		return ReviewState{}, err
	}
	if len(raw.Errors) > 0 {
		return ReviewState{}, fmt.Errorf("GitHub GraphQL: %s", raw.Errors[0].Message)
	}

	rs := ReviewState{Decision: raw.Data.Repository.PullRequest.ReviewDecision}
	if n := raw.Data.Repository.PullRequest.Reviews.Nodes; len(n) > 0 {
		last := n[len(n)-1]
		rs.LastReview.State = last.State
		rs.LastReview.At, _ = time.Parse(time.RFC3339, last.SubmittedAt)
	}
	for _, n := range raw.Data.Repository.PullRequest.ReviewThreads.Nodes {
		if n.IsResolved {
			continue
		}
		thread := Thread{Path: n.Path}
		if n.Line != nil {
			thread.Line = *n.Line
		}
		if len(n.Comments.Nodes) > 0 {
			last := n.Comments.Nodes[len(n.Comments.Nodes)-1]
			thread.LastAuthor = last.Author.Login
			thread.LastAt, _ = time.Parse(time.RFC3339, last.CreatedAt)
		}
		rs.OpenThreads = append(rs.OpenThreads, thread)
	}
	return rs, nil
}

// postGraphQL performs one authenticated POST to the GraphQL endpoint and
// decodes the JSON body into dest.
func (c *Client) postGraphQL(ctx context.Context, payload any, dest any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/graphql", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	res, err := c.client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = res.Body.Close() }()
	if err := checkStatus(res); err != nil {
		return err
	}
	if err := json.NewDecoder(res.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode GitHub response: %w", err)
	}
	return nil
}

// splitOwnerName splits "owner/name" into its two parts.
func splitOwnerName(repo string) (string, string, bool) {
	owner, name, found := strings.Cut(repo, "/")
	if !found || owner == "" || name == "" {
		return "", "", false
	}
	return owner, name, true
}
