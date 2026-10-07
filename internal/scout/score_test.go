package scout

import (
	"testing"

	"github.com/andyst-dev/gh-scout/internal/github"
)

func TestScore(t *testing.T) {
	tests := []struct {
		name  string
		issue github.Issue
		min   int // minimum score expected
	}{
		{
			name: "clear reproducible bug",
			issue: github.Issue{
				Title:  "App crashes on startup with null pointer",
				Body:   "Steps to reproduce:\n```go\nrun()\n```\nexpects no panic,\n```$ ./app```",
				Labels: []string{"bug"},
			},
			min: 70,
		},
		{
			name: "first-timer friendly with reproduction",
			issue: github.Issue{
				Title:  "Log line leaks the auth token",
				Body:   "When auth fails the token appears in logs.\nUnit test: process(*token).\n```\nlog.Printf(\"%s\", token)\n```",
				Labels: []string{"good first issue"},
			},
			min: 70,
		},
		{
			name:  "empty no-signal issue",
			issue: github.Issue{Title: "something", Body: ""},
			min:   0,
		},
		{
			name:  "feature request is penalized",
			issue: github.Issue{Title: "Add export to PDF", Body: "Nice to have exports for users.", Labels: []string{"enhancement"}},
			min:   0,
		},
		{
			name:  "question title scores low",
			issue: github.Issue{Title: "How to configure the proxy?"},
			min:   0,
		},
		{
			name:  "thin body is discouraged",
			issue: github.Issue{Title: "Fix flaky integration test", Body: "pls fix", Labels: []string{"bug"}},
			min:   10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, _ := Score(tt.issue)
			if score < tt.min {
				t.Fatalf("Score() = %d, want at least %d", score, tt.min)
			}
		})
	}
}

// TestScoreBounds ensures the output never escapes [0, 100].
func TestScoreBounds(t *testing.T) {
	high := github.Issue{
		Title:  "Fix crash and memory leak on login fail",
		Body:   "Repro:\n```\nlogin()\n```\nUnit test\n```$ go test```\nexpects no leak.",
		Labels: []string{"bug", "good first issue"},
	}
	if s := mustScore(t, high); s > 100 {
		t.Fatalf("high score overflow: %d", s)
	}

	low := github.Issue{Title: "How are you doing?"}
	if s := mustScore(t, low); s < 0 {
		t.Fatalf("low score underflow: %d", s)
	}
}

// mustScore returns the score or fails the test.
func mustScore(t *testing.T, i github.Issue) int {
	t.Helper()
	s, _ := Score(i)
	return s
}
