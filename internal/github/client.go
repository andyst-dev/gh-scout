package github

import (
	"context"
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

func intNum(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}
