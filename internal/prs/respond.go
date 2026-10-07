package prs

import (
	"fmt"
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// Response holds the newest reply to a pull request by someone other than its
// author, made after the author's last push.
type Response struct {
	Author string
	// Age describes how long ago the reply arrived, e.g. "2d ago".
	Age string
}

// NewestResponse finds the newest qualifying activity on a pull request: a
// comment or review whose login is not the author and whose timestamp is
// strictly after the author's last push. It returns nil when there is no such
// activity.
//
// A response exists iff at least one activity is by someone other than the
// author and after lastPush; the newest such activity is reported. This is
// pure (no I/O) so it is table-testable.
func NewestResponse(author string, lastPush, now time.Time, acts []github.Activity) *Response {
	var (
		best github.Activity
		ok   bool
	)
	for _, a := range acts {
		if a.Login == author || !a.At.After(lastPush) {
			continue
		}
		if !ok || a.At.After(best.At) {
			best, ok = a, true
		}
	}
	if !ok {
		return nil
	}
	return &Response{Author: best.Login, Age: ageString(now.Sub(best.At))}
}

// status derives the pull request state from the merge details returned by the
// GitHub API. It maps each distinct mergeable_state to an actionable bucket.
func status(d github.PRDetail) string {
	switch {
	case (d.Mergeable != nil && !*d.Mergeable) || d.MergeableState == "dirty":
		return "conflicts"
	case d.MergeableState == "behind":
		return "behind base"
	case d.MergeableState == "clean":
		return "up to date"
	case d.MergeableState == "blocked":
		return "blocked"
	case d.MergeableState == "unstable":
		return "unstable"
	default:
		// unknown, or an empty state while GitHub recomputes the verdict.
		return "evaluating"
	}
}

// ageString renders a duration as "2d ago", "3h ago", etc.
func ageString(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	}
}
