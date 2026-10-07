package scout

import (
	"strings"
	"testing"

	"github.com/andyst-dev/gh-scout/internal/github"
)

func TestGuidanceDifficulty(t *testing.T) {
	tests := []struct {
		name string
		it   github.Issue
		want string
	}{
		{
			name: "good first issue is easy",
			it:   github.Issue{Title: "Fix config loading", Labels: []string{"good first issue"}, Body: "Steps: 1. run. 2. boom. ```code```"},
			want: DifficultyEasy,
		},
		{
			name: "typo is easy",
			it:   github.Issue{Title: "Typo in error message"},
			want: DifficultyEasy,
		},
		{
			name: "repro plus tests is easy",
			it:   github.Issue{Title: "Null deref in parser", Body: "```\npanic\n```\nexpected: no panic"},
			want: DifficultyEasy,
		},
		{
			name: "no repro is hard",
			it:   github.Issue{Title: "App breaks on load", Body: "It used to work and I don't know why, quite involved across modules"},
			want: DifficultyHard,
		},
		{
			name: "feature request is hard",
			it:   github.Issue{Title: "Add dark mode", Labels: []string{"enhancement"}, Body: "Would be nice to support themes across the board"},
			want: DifficultyHard,
		},
		{
			name: "medium default",
			it:   github.Issue{Title: "Error in deploy script", Body: "The deploy step fails after upload with an unclear exit code."},
			want: DifficultyMedium,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, _ := Guidance(tt.it)
			if d != tt.want {
				t.Fatalf("Guidance difficulty = %q, want %q", d, tt.want)
			}
		})
	}
}

func TestGuidanceHint(t *testing.T) {
	tests := []struct {
		name string
		it   github.Issue
		want string
	}{
		{
			name: "defect hint names the path",
			it:   github.Issue{Title: "Deadlock in request queue", Body: "Sometimes hangs."},
			want: "deadlock",
		},
		{
			name: "repro hint",
			it:   github.Issue{Title: "Pagination skips records", Body: "```\ncurl ...\n```"},
			want: "reproduce",
		},
		{
			name: "test-first hint",
			it:   github.Issue{Title: "Wrong sort order", Body: "expected ascending, got descending"},
			want: "test",
		},
		{
			name: "typo hint",
			it:   github.Issue{Title: "Spelling error in banner"},
			want: "tiny",
		},
		{
			name: "fallback hint mentions reference",
			it:   github.Issue{Title: "Strange behaviour", Body: "Not sure what causes this yet."},
			want: "Fixes #N",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, h := Guidance(tt.it)
			if !strings.Contains(h, tt.want) {
				t.Fatalf("Guidance hint %q does not contain %q", h, tt.want)
			}
		})
	}
}
