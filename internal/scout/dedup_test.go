package scout

import (
	"testing"

	"github.com/andyst-dev/gh-scout/internal/github"
)

func TestParseReferences(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []int
	}{
		{"plain fixes", "Fixes #123", []int{123}},
		{"closes", "Closes #4", []int{4}},
		{"resolves", "resolves #88", []int{88}},
		{"lowercase", "fixes #12", []int{12}},
		{"multiple", "Fixes #1 and closes #2, refs #3", []int{1, 2, 3}},
		{"past tense", "Fixed #7", []int{7}},
		{"mention within prose", "see #5 discussion", []int{5}},
		{"none", "a totally unrelated change", nil},
		{"empty", "", nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseReferences(tt.body)
			if !equalInts(got, tt.want) {
				t.Fatalf("ParseReferences(%q) = %v, want %v", tt.body, got, tt.want)
			}
		})
	}
}

func TestMatcherHardDedup(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 11, Title: "Add auth", Body: "Fixes #3"},
		{Number: 12, Title: "Fix pager", Body: "Closes #9 and #10"},
		{Number: 13, Title: "Docs", Body: "no reference here"},
	}
	m := NewMatcher(prs)

	cases := []struct {
		issueNum int
		wantPR   int
		wantOk   bool
	}{
		{3, 11, true},  // explicit ref
		{9, 12, true},  // second of multiple refs
		{10, 12, true}, // second of multiple refs
		{13, 0, false}, // not referenced by any PR
		{99, 0, false}, // unknown issue
	}
	for _, c := range cases {
		pr, ok := m.Match(github.Issue{Number: c.issueNum})
		if ok != c.wantOk {
			t.Fatalf("issue #%d ok=%v want %v", c.issueNum, ok, c.wantOk)
		}
		if ok && pr.Number != c.wantPR {
			t.Fatalf("issue #%d matched PR #%d want #%d", c.issueNum, pr.Number, c.wantPR)
		}
	}
}

func TestMatcherPossibleDuplicate(t *testing.T) {
	prs := []github.PullRequest{
		{Number: 5, Title: "fix crash on startup markup", Body: "a body without a reference"},
		{Number: 6, Title: "add dark mode", Body: "a body without a reference"},
	}
	m := NewMatcher(prs)

	if _, ok := m.PossibleDuplicate(github.Issue{Title: "crash on startup"}); !ok {
		t.Fatal("expected a possible-duplicate signal for the shared title word")
	}
	if _, ok := m.PossibleDuplicate(github.Issue{Title: "trading algorithm v2"}); ok {
		t.Fatal("did not expect a duplicate signal for unrelated titles")
	}
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
