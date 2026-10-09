package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	apiBaseURL = "https://api.github.com"
	userAgent  = "gh-scout"
	// perPage is the page size used for list endpoints. It is the GitHub cap.
	perPage = 100
	// maxPages bounds how far a list endpoint is walked. Enough to cover any
	// realistic backlog in a single kv while keeping the CLI fast.
	maxPages = 4
)

// Client is a thin HTTP client for the GitHub REST API.
type Client struct {
	client *http.Client
	token  string
	// baseURL is overridable in tests via a local HTTP server.
	baseURL string
}

// New builds a Client. When the GitHub token is empty, requests are made
// anonymously, which GitHub rate-limits far more aggressively.
func New(token string) *Client {
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return &Client{
		client:  &http.Client{Timeout: 30 * time.Second},
		token:   token,
		baseURL: apiBaseURL,
	}
}

// SetBaseURL overrides the API base URL. It exists so tests can point a client
// at a local HTTP server.
func (c *Client) SetBaseURL(base string) *Client {
	c.baseURL = base
	return c
}

// Issues fetches the open, non-pull-request issues of repo ("owner/name"),
// newest first.
func (c *Client) Issues(ctx context.Context, repo string) ([]Issue, error) {
	path := fmt.Sprintf("/repos/%s/issues?state=open&sort=created&direction=desc", repo)
	raw, err := c.list(ctx, path)
	if err != nil {
		return nil, err
	}
	issues := make([]Issue, 0, len(raw))
	for _, it := range raw {
		if pr := it["pull_request"]; pr != nil {
			continue // pull requests appear in the issues endpoint
		}
		issues = append(issues, decodeIssue(it))
	}
	return issues, nil
}

// OpenPullRequests fetches all open pull requests of repo.
func (c *Client) OpenPullRequests(ctx context.Context, repo string) ([]PullRequest, error) {
	path := fmt.Sprintf("/repos/%s/pulls?state=open", repo)
	raw, err := c.list(ctx, path)
	if err != nil {
		return nil, err
	}
	prs := make([]PullRequest, 0, len(raw))
	for _, it := range raw {
		prs = append(prs, PullRequest{
			Number: intNum(it["number"]),
			Title:  str(it["title"]),
			Body:   str(it["body"]),
			URL:    str(it["html_url"]),
		})
	}
	return prs, nil
}

// mergedPRCap bounds how many recent closed pull requests of a repository are
// scanned for a merged reference to an issue. A fix old enough to fall outside
// this many recent closed PRs will almost always have closed its own issue.
const mergedPRCap = 200

// MergedPullRequests returns the recent closed pull requests that merged,
// newest by update first. Closed-without-merge PRs are skipped: they leave the
// issue open work. The scout uses these to drop issues a merged PR already
// implemented but that were never closed (the #2200 / #2202 lesson).
func (c *Client) MergedPullRequests(ctx context.Context, repo string) ([]PullRequest, error) {
	u := fmt.Sprintf(
		"%s/repos/%s/pulls?state=closed&sort=updated&direction=desc&per_page=100",
		c.baseURL, repo,
	)
	var prs []PullRequest
	seen := 0
	for {
		items, next, err := c.getPage(ctx, u)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if seen >= mergedPRCap {
				return prs, nil
			}
			seen++
			if str(it["merged_at"]) == "" {
				continue
			}
			prs = append(prs, PullRequest{
				Number: intNum(it["number"]),
				Title:  str(it["title"]),
				Body:   str(it["body"]),
				URL:    str(it["html_url"]),
				Merged: true,
			})
		}
		if next == "" {
			return prs, nil
		}
		u = next
	}
}

// CurrentUser returns the login of the authenticated GitHub user. It requires
// a token; anonymous requests cannot identify a user.
func (c *Client) CurrentUser(ctx context.Context) (string, error) {
	if c.token == "" {
		return "", errors.New("no GitHub token set: pass --token or set GITHUB_TOKEN to detect the user")
	}
	var raw struct {
		Login string `json:"login"`
	}
	if err := c.getObject(ctx, "/user", &raw); err != nil {
		return "", err
	}
	if raw.Login == "" {
		return "", errors.New("GitHub /user returned no login")
	}
	return raw.Login, nil
}

// SearchPullRequests finds a user's open pull requests, newest by update time
// first.
func (c *Client) SearchPullRequests(ctx context.Context, user string) ([]PRRef, error) {
	q := url.QueryEscape("author:" + user + " is:pr is:open")
	endpoint := fmt.Sprintf("/search/issues?q=%s&sort=updated&order=desc&per_page=%d", q, perPage)
	var raw struct {
		Items []map[string]any `json:"items"`
	}
	if err := c.getObject(ctx, endpoint, &raw); err != nil {
		return nil, err
	}
	refs := make([]PRRef, 0, len(raw.Items))
	for _, it := range raw.Items {
		repo := repoFromURL(str(it["repository_url"]))
		if repo == "" {
			continue
		}
		refs = append(refs, PRRef{
			Repo:      repo,
			Number:    intNum(it["number"]),
			Title:     str(it["title"]),
			HTMLURL:   str(it["html_url"]),
			UpdatedAt: parseTime(it["updated_at"]),
		})
	}
	return refs, nil
}

// MergedPRCount counts the merged pull requests authored by user in repo via
// the search API (total_count only, no paging).
func (c *Client) MergedPRCount(ctx context.Context, repo, user string) (int, error) {
	q := url.QueryEscape("repo:" + repo + " author:" + user + " is:pr is:merged")
	endpoint := fmt.Sprintf("/search/issues?q=%s&per_page=1", q)
	var raw struct {
		TotalCount int `json:"total_count"`
	}
	if err := c.getObject(ctx, endpoint, &raw); err != nil {
		return 0, err
	}
	return raw.TotalCount, nil
}

// MergedByRepo counts the user's merged pull requests per repository across
// all repositories, by enumerating the search results and aggregating by
// repository. Unlike the collection endpoints, /search/issues returns an
// object with an items array, so this pages through c.get instead of c.list.
func (c *Client) MergedByRepo(ctx context.Context, user string) (map[string]int, error) {
	q := url.QueryEscape("author:" + user + " is:pr is:merged")
	u := c.baseURL + "/search/issues?q=" + q + "&sort=updated&order=desc&per_page=" + fmt.Sprint(perPage)

	counts := make(map[string]int, 16)
	for page := 1; page <= maxPages && u != ""; page++ {
		res, err := c.get(ctx, u)
		if err != nil {
			return nil, err
		}
		if err := checkStatus(res); err != nil {
			_ = res.Body.Close()
			return nil, err
		}
		var raw struct {
			Items []map[string]any `json:"items"`
		}
		if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
			_ = res.Body.Close()
			return nil, fmt.Errorf("decode GitHub response: %w", err)
		}
		_ = res.Body.Close()
		for _, it := range raw.Items {
			repo := repoFromURL(str(it["repository_url"]))
			if repo == "" {
				continue
			}
			counts[repo]++
		}
		u = nextPageURL(res.Header.Get("Link"))
	}
	return counts, nil
}

// PullRequest fetches the merge details and head commit of one pull request.
func (c *Client) PullRequest(ctx context.Context, repo string, number int) (PRDetail, error) {
	var d PRDetail
	var raw struct {
		Mergeable      *bool  `json:"mergeable"`
		MergeableState string `json:"mergeable_state"`
		Head           struct {
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
		UpdatedAt string `json:"updated_at"`
	}
	endpoint := fmt.Sprintf("/repos/%s/pulls/%d", repo, number)
	if err := c.getObject(ctx, endpoint, &raw); err != nil {
		return d, err
	}
	d.Mergeable = raw.Mergeable
	d.MergeableState = raw.MergeableState
	d.HeadSHA = raw.Head.SHA
	d.BaseRef = raw.Base.Ref
	d.UpdatedAt = parseTime(raw.UpdatedAt)
	return d, nil
}

// PullFiles returns the paths a pull request changes, with their status.
func (c *Client) PullFiles(ctx context.Context, repo string, number int) ([]FileChange, error) {
	raw, err := c.list(ctx, fmt.Sprintf("/repos/%s/pulls/%d/files", repo, number))
	if err != nil {
		return nil, err
	}
	files := make([]FileChange, 0, len(raw))
	for _, it := range raw {
		if path := str(it["filename"]); path != "" {
			files = append(files, FileChange{Path: path, Status: str(it["status"]), Patch: str(it["patch"])})
		}
	}
	return files, nil
}

// FileExistsOn reports whether path exists on ref (a branch, tag or SHA). A
// 404 is a definitive "no", not an error.
func (c *Client) FileExistsOn(ctx context.Context, repo, path, ref string) (bool, error) {
	u, err := url.Parse(c.baseURL + "/repos/" + repo + "/contents/" + escapePath(path))
	if err != nil {
		return false, err
	}
	q := u.Query()
	q.Set("ref", ref)
	u.RawQuery = q.Encode()
	res, err := c.get(ctx, u.String())
	if err != nil {
		return false, err
	}
	defer func() { _ = res.Body.Close() }()
	switch res.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, checkStatus(res)
	}
}

// FileContentOn returns the text of path on ref, and whether it could be read.
// A 404, a directory or an over-large blob yields "", false: callers treat it
// as inconclusive, never as absent.
func (c *Client) FileContentOn(ctx context.Context, repo, path, ref string) (string, bool) {
	u, err := url.Parse(c.baseURL + "/repos/" + repo + "/contents/" + escapePath(path))
	if err != nil {
		return "", false
	}
	q := u.Query()
	q.Set("ref", ref)
	u.RawQuery = q.Encode()
	res, err := c.get(ctx, u.String())
	if err != nil {
		return "", false
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return "", false
	}
	var raw struct {
		Content  string `json:"content"`
		Encoding string `json:"encoding"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return "", false
	}
	if raw.Encoding == "base64" {
		dec, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(raw.Content, "\n", ""))
		if err != nil {
			return "", false
		}
		return string(dec), true
	}
	return raw.Content, true
}

// PullLifecycle reports whether a pull request has been merged or closed, so a
// delta can explain one that left the open list. A 404 (deleted repository)
// reads as neither.
func (c *Client) PullLifecycle(ctx context.Context, repo string, number int) (merged, closed bool, err error) {
	res, err := c.get(ctx, fmt.Sprintf("%s/repos/%s/pulls/%d", c.baseURL, repo, number))
	if err != nil {
		return false, false, err
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode == http.StatusNotFound {
		return false, false, nil
	}
	if err := checkStatus(res); err != nil {
		return false, false, err
	}
	var raw struct {
		State    string `json:"state"`
		MergedAt any    `json:"merged_at"`
	}
	if err := json.NewDecoder(res.Body).Decode(&raw); err != nil {
		return false, false, fmt.Errorf("decode GitHub response: %w", err)
	}
	return raw.MergedAt != nil, raw.State == "closed", nil
}

// escapePath percent-escapes each segment of a repository path, leaving the
// slashes that separate them.
func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return strings.Join(parts, "/")
}

// IssueCommenters returns the issue-thread comments on a pull request.
func (c *Client) IssueCommenters(ctx context.Context, repo string, number int) ([]Activity, error) {
	return c.activities(ctx, fmt.Sprintf("/repos/%s/issues/%d/comments", repo, number), "created_at")
}

// Reviewers returns the formal reviews submitted on a pull request.
func (c *Client) Reviewers(ctx context.Context, repo string, number int) ([]Activity, error) {
	return c.activities(ctx, fmt.Sprintf("/repos/%s/pulls/%d/reviews", repo, number), "submitted_at")
}

// ReviewCommenters returns the inline review comments on a pull request.
func (c *Client) ReviewCommenters(ctx context.Context, repo string, number int) ([]Activity, error) {
	return c.activities(ctx, fmt.Sprintf("/repos/%s/pulls/%d/comments", repo, number), "created_at")
}

// CommitDate returns the commit date of the given commit SHA.
func (c *Client) CommitDate(ctx context.Context, repo, sha string) (time.Time, error) {
	var raw struct {
		Commit struct {
			Committer struct {
				Date string `json:"date"`
			} `json:"committer"`
		} `json:"commit"`
	}
	endpoint := fmt.Sprintf("/repos/%s/commits/%s", repo, sha)
	if err := c.getObject(ctx, endpoint, &raw); err != nil {
		return time.Time{}, err
	}
	t, err := time.Parse(time.RFC3339, raw.Commit.Committer.Date)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse commit date %q: %w", raw.Commit.Committer.Date, err)
	}
	return t, nil
}

// activities fetches a paginated collection endpoint and maps each item to an
// Activity, taking the author from .user.login and the timestamp from atField.
func (c *Client) activities(ctx context.Context, endpoint, atField string) ([]Activity, error) {
	raw, err := c.list(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	acts := make([]Activity, 0, len(raw))
	for _, it := range raw {
		user, _ := it["user"].(map[string]any)
		acts = append(acts, Activity{
			Login: str(user["login"]),
			At:    parseTime(it[atField]),
		})
	}
	return acts, nil
}

// getObject performs one unauthenticated-aware GET and decodes the JSON body
// into dest (a pointer). It shares the list() error and rate-limit handling.
func (c *Client) getObject(ctx context.Context, endpoint string, dest any) error {
	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return err
	}
	res, err := c.get(ctx, u.String())
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

// get performs a GET with gh-scout's standard headers and returns the response
// body for the caller to decode. The body must be closed by the caller.
func (c *Client) get(ctx context.Context, reqURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.client.Do(req)
}

// list GETs a paginated collection endpoint and returns all decoded items.
func (c *Client) list(ctx context.Context, endpoint string) ([]map[string]any, error) {
	u, err := url.Parse(c.baseURL + endpoint)
	if err != nil {
		return nil, err
	}
	query := u.Query()
	query.Set("per_page", fmt.Sprintf("%d", perPage))
	u.RawQuery = query.Encode()

	var all []map[string]any
	next := u.String()
	for page := 1; page <= maxPages && next != ""; page++ {
		items, linkNext, err := c.getPage(ctx, next)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		next = linkNext
	}
	return all, nil
}

// getPage performs one GET and returns the decoded items plus the URL of the
// next page, empty when there is none.
func (c *Client) getPage(ctx context.Context, reqURL string) ([]map[string]any, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", userAgent)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	res, err := c.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == http.StatusTooManyRequests {
		return nil, "", errors.New("GitHub rate limit exceeded (set GITHUB_TOKEN for a higher quota)")
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return nil, "", fmt.Errorf("GitHub API %s: %s", res.Status, strings.TrimSpace(string(body)))
	}

	var items []map[string]any
	if err := json.NewDecoder(res.Body).Decode(&items); err != nil {
		return nil, "", fmt.Errorf("decode GitHub response: %w", err)
	}
	return items, nextPageURL(res.Header.Get("Link")), nil
}

// nextPageURL extracts the URL of the next Link page, empty when absent.
func nextPageURL(link string) string {
	for _, part := range strings.Split(link, ",") {
		seg := strings.SplitN(strings.TrimSpace(part), ";", 2)
		if len(seg) != 2 || !strings.Contains(seg[1], `rel="next"`) {
			continue
		}
		return strings.Trim(seg[0], "<>")
	}
	return ""
}

// decodeIssue maps a raw API item onto Issue.
func decodeIssue(it map[string]any) Issue {
	labels := make([]string, 0, 8)
	if rawLabels, ok := it["labels"].([]any); ok {
		for _, l := range rawLabels {
			if label, ok := l.(map[string]any); ok {
				labels = append(labels, str(label["name"]))
			}
		}
	}
	var created time.Time
	if ts := str(it["created_at"]); ts != "" {
		created, _ = time.Parse(time.RFC3339, ts)
	}
	return Issue{
		Number:    intNum(it["number"]),
		Title:     str(it["title"]),
		Body:      str(it["body"]),
		CreatedAt: created,
		HTMLURL:   str(it["html_url"]),
		Labels:    labels,
		Comments:  intNum(it["comments"]),
	}
}

func str(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

// parseTime parses an RFC3339 timestamp from a raw API item, returning the
// zero time when the value is absent or malformed.
func parseTime(v any) time.Time {
	if s := str(v); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// repoFromURL extracts the "owner/name" of a repository from its API
// repository_url, stripping the leading /repos/ prefix.
func repoFromURL(repositoryURL string) string {
	if idx := strings.Index(repositoryURL, "/repos/"); idx >= 0 {
		return strings.TrimPrefix(repositoryURL[idx:], "/repos/")
	}
	return ""
}

// checkStatus validates a GitHub response, mapping the shared error cases
// (rate limit, non-2xx) onto an error without reading the body.
func checkStatus(res *http.Response) error {
	if res.StatusCode == http.StatusTooManyRequests {
		return errors.New("GitHub rate limit exceeded (set GITHUB_TOKEN for a higher quota)")
	}
	if res.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return fmt.Errorf("GitHub API %s: %s", res.Status, strings.TrimSpace(string(body)))
	}
	return nil
}

func intNum(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
