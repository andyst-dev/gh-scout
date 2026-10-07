// Package prs scouts a user's own open pull requests: for each one it reports
// whether someone else has responded since the author's last push, and whether
// the branch is up to date with its base.
package prs

import "time"

// Report is the full result of a pull-request scout run.
type Report struct {
	GeneratedAt time.Time
	// User is the login whose pull requests were scanned.
	User string
	// Repos holds the results grouped by repository, sorted by name.
	Repos []Repo
}

// Repo groups the assessed pull requests of one repository.
type Repo struct {
	Name string
	PRs  []PR
	// Skipped counts pull requests that could not be assessed (API error).
	Skipped int
	// Notes explains each skipped pull request.
	Notes []string
}

// PR is one assessed pull request.
type PR struct {
	Number int
	Title  string
	URL    string
	// Status is one of up to date, behind base, conflicts, blocked, unstable,
	// evaluating.
	Status string
	// Response is the newest reply by someone other than the author, or nil.
	Response *Response
}

// Skip records that one pull request in repo could not be assessed, keeping a
// running count and a human note.
func (rep *Report) Skip(repo, note string) {
	for i := range rep.Repos {
		if rep.Repos[i].Name != repo {
			continue
		}
		rep.Repos[i].Skipped++
		rep.Repos[i].Notes = append(rep.Repos[i].Notes, note)
		return
	}
	rep.Repos = append(rep.Repos, Repo{Name: repo, Skipped: 1, Notes: []string{note}})
}
