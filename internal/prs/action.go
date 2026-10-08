package prs

import (
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// Owners of the next move on a pull request.
const (
	// OwnerYou: the author owes the next action (rebase, fix, reply).
	OwnerYou = "you"
	// OwnerThem: the ball is with reviewers, CI, or the base branch.
	OwnerThem = "them"
)

// Action says who owes the next move on a pull request, with the reasons.
type Action struct {
	// Owner is OwnerYou or OwnerThem.
	Owner string
	// Reasons lists, in priority order, why the pull request is on you. Empty
	// when Owner is OwnerThem.
	Reasons []string
	// Waiting names what you are waiting on. Empty when Owner is OwnerYou.
	Waiting string
}

// Classify decides who owes the next move on a pull request from facts alone:
// the merge status, GitHub's aggregate review decision, and the open review
// threads. A pull request is YOURS when its branch conflicts with or trails
// its base, when review is CHANGES_REQUESTED, or when an unresolved review
// thread's newest comment is someone else's and post-dates your own last
// activity (your push or your last comment). Pure (no I/O), so table-testable.
func Classify(author string, lastActivity time.Time, status string, rs github.ReviewState) Action {
	var reasons []string

	// A branch that conflicts or trails its base blocks you before anything.
	switch status {
	case "conflicts":
		reasons = append(reasons, "rebase: the branch conflicts with its base")
	case "behind base":
		reasons = append(reasons, "rebase: the branch is behind its base")
	}

	// An open review thread awaits you when someone else's newest comment
	// post-dates your own last activity (your push or your last comment).
	for _, t := range rs.OpenThreads {
		if t.LastAuthor == "" || t.LastAuthor == author {
			continue
		}
		if !t.LastAt.After(lastActivity) {
			continue
		}
		reasons = append(reasons, "reply in "+t.Anchor()+" (@"+t.LastAuthor+")")
	}

	if len(reasons) > 0 {
		return Action{Owner: OwnerYou, Reasons: reasons}
	}

	// CHANGES_REQUESTED keeps the ball on you only while the change is still
	// open: an outstanding review thread, or a freshly-requested change you
	// have not acted on. Once every thread is resolved and the requesting
	// review predates your last activity, the changes are addressed and the
	// verdict simply awaits the reviewer's re-approval.
	if rs.Decision == "CHANGES_REQUESTED" {
		if len(rs.OpenThreads) > 0 {
			return Action{
				Owner:   OwnerYou,
				Reasons: []string{"change requested in review: resolve the outstanding review thread(s)"},
			}
		}
		if rs.LastReview.State == "CHANGES_REQUESTED" && rs.LastReview.At.After(lastActivity) {
			return Action{Owner: OwnerYou, Reasons: []string{"changes requested in review"}}
		}
		return Action{Owner: OwnerThem, Waiting: "reviewer to re-approve the addressed changes"}
	}

	return Action{Owner: OwnerThem, Waiting: waitingOn(status, rs.Decision)}
}

// waitingOn names what an on-them pull request is waiting for.
func waitingOn(status, decision string) string {
	if decision == "APPROVED" {
		return "merge"
	}
	switch status {
	case "blocked", "unstable":
		return "review and required checks"
	case "behind base", "conflicts":
		return "rebase" // unreachable for on-them, defensive
	default:
		return "review"
	}
}
