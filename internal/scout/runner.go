package scout

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// Runner evaluates repositories into candidates.
type Runner struct {
	lister github.IssueLister
}

// NewRunner builds a Runner around an IssueLister (the real GitHub client or
// a fake in tests).
func NewRunner(l github.IssueLister) *Runner {
	return &Runner{lister: l}
}

// Run scouts every repository and returns the ranked candidates.
func (r *Runner) Run(ctx context.Context, repos []string, opts Options) (*Report, error) {
	rep := &Report{Repos: repos, GeneratedAt: time.Now()}

	for _, repo := range repos {
		issues, err := r.lister.Issues(ctx, repo)
		if err != nil {
			return nil, err
		}
		prs, err := r.lister.OpenPullRequests(ctx, repo)
		if err != nil {
			return nil, err
		}

		matcher := NewMatcher(prs)
		for _, issue := range filterIssues(issues, opts) {
			rep.Candidates = append(rep.Candidates, r.evaluate(repo, issue, matcher, opts))
		}
	}

	sort.SliceStable(rep.Candidates, func(a, b int) bool {
		return rep.Candidates[a].Score > rep.Candidates[b].Score
	})
	return rep, nil
}

// evaluate turns a single issue into a candidate.
func (r *Runner) evaluate(repo string, issue github.Issue, m *Matcher, opts Options) Candidate {
	rules := appliedRules(opts.Rules)
	c := Candidate{
		Repository: repo,
		Number:     issue.Number,
		Title:      issue.Title,
		URL:        issue.HTMLURL,
		CreatedAt:  issue.CreatedAt,
		Labels:     issue.Labels,
	}

	// Hard dedup: an open PR already references the issue. Never offer it.
	if pr, ok := m.Match(issue); ok {
		c.Status = StatusAddressed
		c.Reason = "open PR #" + strconv.Itoa(pr.Number) + " already references it"
		return c
	}

	// Soft signal: a PR with a similar title but no explicit reference.
	if pr, ok := m.PossibleDuplicate(issue); ok {
		c.Reason = "possible duplicate: #" + strconv.Itoa(pr.Number) + " \"" + pr.Title + "\""
	}

	score, reasons := scoreWith(rules.Weights, issue)
	c.Score = score
	if c.Reason == "" {
		c.Reason = strings.Join(reasons, "; ")
	}

	switch {
	case score < rules.ReadyThreshold:
		c.Status = StatusUnclear
	case opts.MinScore > 0 && score < opts.MinScore:
		c.Status = StatusUnclear
		if c.Reason == "" {
			c.Reason = "score below --min-score"
		}
	default:
		c.Status = StatusReady
		c.Difficulty, c.Hint = Guidance(issue)
	}
	return c
}

// filterIssues applies the Options filters (recency, labels, cap).
func filterIssues(issues []github.Issue, opts Options) []github.Issue {
	out := make([]github.Issue, 0, len(issues))
	for _, i := range issues {
		if opts.Since > 0 && time.Since(i.CreatedAt) > opts.Since {
			continue
		}
		if len(opts.Labels) > 0 && !hasAnyLabel(i.Labels, opts.Labels) {
			continue
		}
		out = append(out, i)
	}
	if opts.MaxPerRepo > 0 && len(out) > opts.MaxPerRepo {
		out = out[:opts.MaxPerRepo]
	}
	return out
}
