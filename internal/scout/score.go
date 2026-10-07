package scout

import (
	"strings"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// defectWords hint that a title describes a concrete, fixable defect.
var defectWords = []string{
	"fix", "crash", "panic", "error", "broken", "incorrect", "wrong",
	"fail", "leak", "bug", "undefined", "null", "regression", "hang", "deadlock",
}

// genericTitles are so vague that they convey no fixable surface.
var genericTitles = []string{"bug", "issue", "problem", "doesn't work", "not working"}

// featureLabels mark an issue as a request rather than a defect.
var featureLabels = []string{"feature", "enhancement", "request", "discussion", "idea"}

// scoreWith assesses how fixable an issue looks using caller-provided weights.
// Score wraps it with the default ruleset.
func scoreWith(w Weights, issue github.Issue) (int, []string) {
	total := 0
	var reasons []string

	title := strings.ToLower(issue.Title)
	body := strings.ToLower(strings.TrimSpace(issue.Body))

	switch {
	case hasAnyWord(title, defectWords):
		total += w.TitleDefect
		reasons = append(reasons, "title states a concrete defect")
	case isGenericTitle(issue.Title):
		total += w.TitleGeneric
		reasons = append(reasons, "title is vague")
	case isQuestion(title):
		total += w.TitleQuestion
		reasons = append(reasons, "title is a question, not a defect")
	}

	switch {
	case reproduces(body):
		total += w.BodyReproduction
		reasons = append(reasons, "body has a reproduction or code sample")
	case len(body) >= 60:
		total += w.BodySubstantial
		reasons = append(reasons, "body describes the problem")
	case len(body) == 0:
		total += w.BodyEmpty
		reasons = append(reasons, "no body")
	case len(body) < 40:
		total += w.BodyEmpty
		reasons = append(reasons, "body too thin to judge")
	}

	if mentionsTests(body) {
		total += w.MentionsTests
		reasons = append(reasons, "mentions tests or expected behavior")
	}

	switch {
	case hasLabel(issue.Labels, "good first issue") || hasLabel(issue.Labels, "good-first-issue"):
		total += w.FirstGood
		reasons = append(reasons, "labelled good first issue")
	case hasLabel(issue.Labels, "help wanted") || hasLabel(issue.Labels, "help-wanted"):
		total += w.HelpWanted
		reasons = append(reasons, "labelled help wanted")
	case hasLabel(issue.Labels, "bug"):
		total += w.BugLabel
		reasons = append(reasons, "labelled bug")
	}

	if hasAnyLabel(issue.Labels, featureLabels) {
		total += w.FeatureRequest
		reasons = append(reasons, "looks like a feature request")
	}

	if total > 100 {
		total = 100
	}
	if total < 0 {
		total = 0
	}
	return total, reasons
}

// Score assesses fixability with the default ruleset.
func Score(issue github.Issue) (int, []string) {
	return scoreWith(DefaultRules().Weights, issue)
}

func hasAnyWord(title string, words []string) bool {
	for _, w := range words {
		if strings.Contains(title, w) {
			return true
		}
	}
	return false
}

func isGenericTitle(title string) bool {
	t := strings.ToLower(strings.TrimSpace(title))
	for _, g := range genericTitles {
		if t == g {
			return true
		}
	}
	return false
}

func isQuestion(t string) bool {
	return strings.HasSuffix(t, "?") ||
		strings.HasPrefix(t, "how to") ||
		strings.HasPrefix(t, "should we") ||
		strings.HasPrefix(t, "why")
}

func mentionsTests(body string) bool {
	for _, w := range []string{"test", "reproduce", "repro", "expected", "expects", "expect ", "assert", "unit", "should"} {
		if strings.Contains(body, w) {
			return true
		}
	}
	return false
}

// reproduces reports whether a body gives enough to reproduce the problem: a
// code block or an explicit reproduction hint.
func reproduces(body string) bool {
	if len(body) >= 30 && containsCodeBlock(body) {
		return true
	}
	for _, w := range []string{"reproduce", "repro", "steps to", "``", "$ "} {
		if strings.Contains(body, w) {
			return true
		}
	}
	return false
}

func containsCodeBlock(body string) bool {
	return strings.Contains(body, "```") || strings.Contains(body, "$ ") ||
		strings.Contains(body, "\n  ")
}

func hasAnyLabel(labels []string, want []string) bool {
	for _, l := range labels {
		for _, w := range want {
			if stringEqualFold(l, w) {
				return true
			}
		}
	}
	return false
}

func hasLabel(labels []string, want string) bool {
	for _, l := range labels {
		if stringEqualFold(l, want) {
			return true
		}
	}
	return false
}

func stringEqualFold(a, b string) bool {
	return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b))
}
