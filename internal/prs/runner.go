package prs

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// Options configures a Run.
type Options struct {
	// User is the pull request author to scout. When empty, Run auto-detects
	// it from the authenticated GitHub user.
	User string
	// Repos, when non-empty, restricts the scan to these owner/name pairs.
	Repos []string
	// Since drops pull requests not updated within this window.
	Since time.Duration
	// Max caps the total number of pull requests examined.
	Max int
}

// Runner scouts a user's open pull requests.
type Runner struct {
	client *github.Client
}

// NewRunner builds a Runner around the GitHub client.
func NewRunner(c *github.Client) *Runner {
	return &Runner{client: c}
}

// Run collects the user's open pull requests and, for each, derives whether
// someone responded after their last push and whether it is up to date with
// its base branch. Results are grouped by repository, sorted by name.
func (r *Runner) Run(ctx context.Context, opts Options) (*Report, error) {
	user := opts.User
	if user == "" {
		detected, err := r.client.CurrentUser(ctx)
		if err != nil {
			return nil, errors.New("no --user given and could not detect one: " + err.Error())
		}
		user = detected
	}

	refs, err := r.client.SearchPullRequests(ctx, user)
	if err != nil {
		return nil, err
	}

	rep := &Report{GeneratedAt: time.Now(), User: user}
	index := map[string]int{}

	examined := 0
	for _, ref := range refs {
		if opts.Max > 0 && examined >= opts.Max {
			break
		}
		if opts.Since > 0 && time.Since(ref.UpdatedAt) > opts.Since {
			continue
		}
		if len(opts.Repos) > 0 && !contains(opts.Repos, ref.Repo) {
			continue
		}
		examined++

		// Register the repository entry up front so a later failure is recorded
		// against the same grouping Skip uses.
		idx, ok := index[ref.Repo]
		if !ok {
			rep.Repos = append(rep.Repos, Repo{Name: ref.Repo})
			idx = len(rep.Repos) - 1
			index[ref.Repo] = idx
		}

		detail, err := r.client.PullRequest(ctx, ref.Repo, ref.Number)
		if err != nil {
			rep.Skip(ref.Repo, "pull request #"+strconv.Itoa(ref.Number)+": "+err.Error())
			continue
		}

		lastPush, err := r.client.CommitDate(ctx, ref.Repo, detail.HeadSHA)
		if err != nil {
			rep.Skip(ref.Repo, "head commit #"+strconv.Itoa(ref.Number)+": "+err.Error())
			continue
		}

		acts, err := r.collectActivity(ctx, ref.Repo, ref.Number)
		if err != nil {
			rep.Skip(ref.Repo, "activity #"+strconv.Itoa(ref.Number)+": "+err.Error())
			continue
		}

		rs, err := r.client.ReviewState(ctx, ref.Repo, ref.Number)
		if err != nil {
			rep.Skip(ref.Repo, "review state #"+strconv.Itoa(ref.Number)+": "+err.Error())
			continue
		}

		st := status(detail)
		rep.Repos[idx].PRs = append(rep.Repos[idx].PRs, PR{
			Number:   ref.Number,
			Title:    ref.Title,
			URL:      ref.HTMLURL,
			Status:   st,
			Response: NewestResponse(user, lastPush, time.Now(), acts),
			Action:   Classify(user, lastActivity(user, lastPush, acts), st, rs),
		})
	}

	sort.Slice(rep.Repos, func(a, b int) bool { return rep.Repos[a].Name < rep.Repos[b].Name })
	return rep, nil
}

// collectActivity unions the issue comments, formal reviews, and inline review
// comments on one pull request into a single activity list.
func (r *Runner) collectActivity(ctx context.Context, repo string, number int) ([]github.Activity, error) {
	var acts []github.Activity
	for _, fetch := range []func(context.Context, string, int) ([]github.Activity, error){
		r.client.IssueCommenters,
		r.client.Reviewers,
		r.client.ReviewCommenters,
	} {
		got, err := fetch(ctx, repo, number)
		if err != nil {
			return nil, err
		}
		acts = append(acts, got...)
	}
	return acts, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
