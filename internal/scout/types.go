// Package scout turns a repository's raw issues into a ranked list of
// contribution candidates by filtering out issues already addressed by an open
// pull request and scoring how "fixable" what remains looks.
package scout

import (
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// Status states why a candidate wound up in the report.
type Status string

const (
	// StatusReady marks an issue with a clean fixable surface and no open PR
	// already targeting it.
	StatusReady Status = "ready"
	// StatusAddressed marks an issue an open pull request already references,
	// and therefore is excluded from the ready set.
	StatusAddressed Status = "addressed"
	// StatusMerged marks an issue a merged pull request already references: the
	// fix shipped, the issue was just never closed, so it is excluded too.
	StatusMerged Status = "merged"
	// StatusUnclear marks an issue with no contact with a PR but too little
	// signal to confidently call it fixable.
	StatusUnclear Status = "unclear"
)

// Candidate is one issue evaluated by the scout.
type Candidate struct {
	Repository string
	Number     int
	Title      string
	URL        string
	CreatedAt  time.Time
	Labels     []string
	Status     Status
	Reason     string
	Score      int
	// Difficulty is a coarse 3-step estimate of how approachable a fix looks.
	Difficulty string
	// Hint is a one-line suggestion of the kind of PR to open.
	Hint string
}

// Difficulty levels attached to ready candidates.
const (
	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

// Report is the full result of a scout run across several repositories.
type Report struct {
	GeneratedAt time.Time
	Repos       []string
	Candidates  []Candidate
}

// Options configures a Run.
type Options struct {
	// Since truncates issues to those created within this window. Zero means
	// no filtering.
	Since time.Duration
	// Labels, when non-empty, keeps only issues carrying at least one of them.
	Labels []string
	// MaxPerRepo caps how many issues are examined per repository.
	MaxPerRepo int
	// MinScore drops candidates scoring below this value from the ready set.
	MinScore int
	// Rules overrides the default scoring weights and ready threshold. The
	// zero value (the usual case) uses the built-in defaults.
	Rules Rules
}

// IssueLister is what Run needs: the github.IssueLister subset is enough.
type IssueLister = github.IssueLister
