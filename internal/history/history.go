// Package history counts the pull requests an author has merged per
// repository. It is the long-run counterpart to the open-PR triage of
// internal/prs: a cheap total without enumerating every pull request.
package history

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// RepoPRs is the merged pull request count for one repository.
type RepoPRs struct {
	Repo   string `json:"repo"`
	Merged int    `json:"merged"`
}

// Report is the merged-PR history for an author across repositories.
type Report struct {
	Author string    `json:"author"`
	Repos  []RepoPRs `json:"repos"`
	Total  int       `json:"total"`
}

// Counter counts merged pull requests for an author in a repository.
type Counter interface {
	MergedPRCount(ctx context.Context, repo, user string) (int, error)
}

// Run counts merged pull requests authored by user in each of repos. The
// search total is exact and needs no paging, so this is one request per repo.
func Run(ctx context.Context, c Counter, user string, repos []string) (*Report, error) {
	rep := &Report{Author: user, Repos: make([]RepoPRs, 0, len(repos))}
	for _, repo := range repos {
		n, err := c.MergedPRCount(ctx, repo, user)
		if err != nil {
			return nil, err
		}
		rep.Repos = append(rep.Repos, RepoPRs{Repo: repo, Merged: n})
		rep.Total += n
	}
	return rep, nil
}

// RenderMarkdown renders the report as a compact list. The · separator is the
// only non-ASCII glyph the project allows.
func RenderMarkdown(rep *Report) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# Merged by %s\n\n", rep.Author)
	for _, r := range rep.Repos {
		fmt.Fprintf(&b, "- %s · %d merged\n", r.Repo, r.Merged)
	}
	fmt.Fprintf(&b, "\nTotal: %d\n", rep.Total)
	return []byte(b.String())
}

// RenderJSON renders the report as indented JSON (the machine interface; the
// markdown is only for humans).
func RenderJSON(rep *Report) ([]byte, error) {
	return json.MarshalIndent(rep, "", "  ")
}
