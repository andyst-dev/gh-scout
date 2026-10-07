package scout

import (
	"strings"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// Guidance reads an issue's concrete cues (title, body, labels) and returns an
// estimated Difficulty plus a one-line suggestion for the PR to open. It is a
// heuristic aid, not a verdict: the human still reads the issue.
func Guidance(issue github.Issue) (string, string) {
	title := strings.ToLower(issue.Title)
	body := strings.ToLower(strings.TrimSpace(issue.Body))

	smallBtn := hasLabel(issue.Labels, "good first issue") ||
		hasLabel(issue.Labels, "good-first-issue")
	helped := hasLabel(issue.Labels, "help wanted") || hasLabel(issue.Labels, "help-wanted")
	feature := hasAnyLabel(issue.Labels, []string{"feature", "enhancement", "request", "discussion", "idea"})
	hasRepro := reproduces(body)
	hasTests := mentionsTests(body)
	isTypo := strings.Contains(title, "typo") || strings.Contains(title, "spelling")
	defect := matchedDefect(title)

	difficulty := DifficultyMedium
	switch {
	case smallBtn || helped || isTypo || (hasRepro && hasTests):
		difficulty = DifficultyEasy
	case feature ||
		hasAnyWord(title, []string{"refactor", "redesign", "architect", "support"}) ||
		strings.Contains(body, "across") || strings.Contains(body, "all platforms") ||
		(strings.Contains(body, "unknown cause") || (!hasRepro && len(body) >= 160)):
		difficulty = DifficultyHard
	}

	var hint string
	switch {
	case isTypo:
		hint = "tiny one-line fix (typo/wording) - open the PR directly"
	case defect != "":
		hint = "fix the " + defect + " path: add a regression test, then the patch"
	case hasRepro && hasTests:
		hint = "reproduce, add a failing test, then implement the fix"
	case hasRepro:
		hint = "reproduce the issue, then open a focused fix PR"
	case hasTests:
		hint = "write the expected-behaviour test, then implement"
	case smallBtn || helped:
		hint = "well-specified, small surface - confirm scope and open a focused PR"
	default:
		hint = "read the issue, confirm scope, open a focused PR referencing it (Fixes #N)"
	}
	return difficulty, hint
}

// matchedDefect returns the first defect keyword present in the title, or "".
func matchedDefect(title string) string {
	for _, w := range defectWords {
		if strings.Contains(title, w) {
			return w
		}
	}
	return ""
}
