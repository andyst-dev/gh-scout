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
	// At is when the reply arrived, kept so a delta can tell a newer reply from
	// one already seen.
	At time.Time
	// Age describes how long ago the reply arrived, e.g. "2d ago".
	Age string
}

// NewestResponse finds the newest activity that still awaits the author: a
// comment or review by someone other than the author, made after the author's
// own most recent action (their last push or their last comment/review). If
// the author already replied after the other party's comment, that reply is
// the newer activity and nothing on the thread is pending. Pure (no I/O) so
// it is table-testable.
func NewestResponse(author string, lastPush, now time.Time, acts []github.Activity) *Response {
	var best github.Activity
	var ok bool
	for _, a := range acts {
		if a.Login != author && a.At.After(lastActivity(author, lastPush, acts)) && (!ok || a.At.After(best.At)) {
			best, ok = a, true
		}
	}
	if !ok {
		return nil
	}
	return &Response{Author: best.Login, At: best.At, Age: ageString(now.Sub(best.At))}
}

// lastActivity returns the author's own newest action: their last push or any
// later comment or review of theirs. A reply means they already handled
// whatever came before it, so nothing earlier counts as awaiting them.
func lastActivity(author string, lastPush time.Time, acts []github.Activity) time.Time {
	last := lastPush
	for _, a := range acts {
		if a.Login == author && a.At.After(last) {
			last = a.At
		}
	}
	return last
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
