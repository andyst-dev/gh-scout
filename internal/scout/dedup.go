package scout

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// Matcher decides whether an issue is already covered by an open pull request.
// It is the anti-duplication guard: an issue clearly being worked on must not
// be offered as a fresh candidate.
type Matcher struct {
	byRef map[int]github.PullRequest
	// titleOnly holds PRs whose title resembles an issue but do not reference
	// one explicitly; these yield a soft "possible duplicate" signal instead
	// of a hard skip.
	titleOnly []github.PullRequest
}

// referenceRe matches issue or PR numbers referenced in a body. Matching any
// "#N" is deliberately broad: a PR body that mentions an issue number is
// signalling that it addresses that issue, which is exactly the anti-duplication
// signal gh-scout needs ("Fixes #9 and #10", "refs #3", "see #5" all count).
var referenceRe = regexp.MustCompile(`#(\d+)\b`)

// NewMatcher builds a Matcher from the open pull requests of a repository.
func NewMatcher(prs []github.PullRequest) *Matcher {
	m := &Matcher{byRef: make(map[int]github.PullRequest, len(prs))}
	for _, pr := range prs {
		refs := ParseReferences(pr.Body)
		if len(refs) > 0 {
			for _, n := range refs {
				m.byRef[n] = pr
			}
			continue
		}
		// A body without an explicit reference is still a weak match if the
		// titles line up, but never a hard dedup.
		m.titleOnly = append(m.titleOnly, pr)
	}
	return m
}

// ParseReferences extracts the issue numbers an issue/PR body explicitly
// references. It is exported for testing the regex in isolation.
func ParseReferences(body string) []int {
	var out []int
	for _, match := range referenceRe.FindAllStringSubmatch(body, -1) {
		if n, err := strconv.Atoi(match[1]); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// Match reports the pull request that references the issue, if any.
func (m *Matcher) Match(issue github.Issue) (pr github.PullRequest, ok bool) {
	pr, ok = m.byRef[issue.Number]
	return pr, ok
}

// PossibleDuplicate reports whether any title-only PR title looks like the
// issue's, as a weak secondary signal.
func (m *Matcher) PossibleDuplicate(issue github.Issue) (github.PullRequest, bool) {
	target := titleKey(issue.Title)
	for _, pr := range m.titleOnly {
		if overlaps(titleKey(pr.Title), target) {
			return pr, true
		}
	}
	return github.PullRequest{}, false
}

// titleKey normalizes a title for loose comparison: lowercased words.
func titleKey(title string) []string {
	fields := strings.Fields(strings.ToLower(title))
	keys := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, "[](){}.,;:!?'\"")
		if f != "" {
			keys = append(keys, f)
		}
	}
	return keys
}

// overlaps reports whether two title word sets share any non-trivial word.
func overlaps(a, b []string) bool {
	seen := make(map[string]struct{}, len(a))
	for _, w := range a {
		seen[w] = struct{}{}
	}
	for _, w := range b {
		if _, ok := seen[w]; ok && len(w) > 1 {
			return true
		}
	}
	return false
}
