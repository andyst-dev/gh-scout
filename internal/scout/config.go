package scout

import (
	"encoding/json"
	"fmt"
)

// Weights are the point deltas applied to each scoring signal. Tuning these
// changes what gh-scout recommends first. A rule not set in a config file keeps
// its default value.
type Weights struct {
	TitleDefect      int `json:"title_defect"`
	BodyReproduction int `json:"body_reproduction"`
	MentionsTests    int `json:"mentions_tests"`
	BugLabel         int `json:"bug_label"`
	FirstGood        int `json:"good_first_issue"`
	HelpWanted       int `json:"help_wanted"`
	BodySubstantial  int `json:"body_substantial"`
	TitleQuestion    int `json:"title_question"`
	BodyEmpty        int `json:"body_empty"`
	FeatureRequest   int `json:"feature_request"`
	TitleGeneric     int `json:"title_generic"`
}

// Rules bundle every tunable scoring parameter. Zero value means "use the
// defaults", so a config file may omit any field.
type Rules struct {
	// ReadyThreshold is the minimum score for an issue to become `ready`.
	// Below it an issue is `unclear` and skipped.
	ReadyThreshold int     `json:"ready_threshold"`
	Weights        Weights `json:"weights"`
}

// DefaultRules returns the built-in ruleset, the source of truth for
// `gh-scout rules-template`.
func DefaultRules() Rules {
	return Rules{
		ReadyThreshold: 35,
		Weights: Weights{
			TitleDefect:      25,
			BodyReproduction: 20,
			MentionsTests:    15,
			BugLabel:         15,
			FirstGood:        20,
			HelpWanted:       10,
			BodySubstantial:  10,
			TitleQuestion:    -20,
			BodyEmpty:        -15,
			FeatureRequest:   -25,
			TitleGeneric:     -10,
		},
	}
}

// appliedRules returns r when it has been customised, else the defaults.
func appliedRules(r Rules) Rules {
	if r.ReadyThreshold == 0 && r.Weights == (Weights{}) {
		return DefaultRules()
	}
	return r
}

// RulesFromJSON parses a rules file. It starts from the defaults so a partial
// file only overrides the fields it sets. Comments are allowed: both `//` line
// and `/* ... */` block comments are stripped before parsing, so a rules file
// can carry inline explanations (see RulesTemplate).
func RulesFromJSON(data []byte) (Rules, error) {
	base := DefaultRules()
	if err := json.Unmarshal(stripJSONComments(data), &base); err != nil {
		return Rules{}, fmt.Errorf("parse rules file: %w", err)
	}
	if base.ReadyThreshold < 0 || base.ReadyThreshold > 100 {
		return Rules{}, fmt.Errorf("ready_threshold must be within 0-100, got %d", base.ReadyThreshold)
	}
	return base, nil
}

// RulesTemplate is the annotated file printed by `gh-scout rules-template`. It
// is valid JSONC: every line is loadable with RulesFromJSON, so it doubles as a
// runnable starting point and an inline reference.
const RulesTemplate = `{
  // Minimum score for an issue to appear in the Ready list.
  // Below it, an issue is marked "unclear" and skipped.
  "ready_threshold": 35,

  "weights": {
    // Bonus when the title names a concrete defect (crash, panic, leak, null, deadlock...).
    "title_defect": 25,
    // Bonus when the body contains a reproduction or a code sample.
    "body_reproduction": 20,
    // Bonus when the body mentions tests or expected behavior.
    "mentions_tests": 15,
    // Bonus for the "bug" label.
    "bug_label": 15,
    // Bonus for "good first issue" / "good-first-issue".
    "good_first_issue": 20,
    // Bonus for "help wanted" / "help-wanted".
    "help_wanted": 10,
    // Bonus when a long body (60+ chars) describes the problem without a reproduction.
    "body_substantial": 10,
    // Penalty when the title is a question rather than a defect.
    "title_question": -20,
    // Penalty for an empty or very thin body (< 40 chars).
    "body_empty": -15,
    // Penalty when the issue looks like a feature request.
    "feature_request": -25,
    // Penalty for a vague title ("bug", "issue", "problem"...).
    "title_generic": -10
  }
}
`

// stripJSONComments removes // line and /* */ block comments from a JSON
// document. Config values here are numbers and short labels with no
// URL-like slashes, so a minimal scanner suffices (it still respects strings,
// so a quoted "//" inside a value is untouched).
func stripJSONComments(b []byte) []byte {
	out := make([]byte, 0, len(b))
	i, n := 0, len(b)
	inStr := false
	for i < n {
		c := b[i]
		switch {
		case inStr:
			out = append(out, c)
			if c == '\\' && i+1 < n {
				out = append(out, b[i+1])
				i += 2
				continue
			}
			if c == '"' {
				inStr = false
			}
			i++
		case c == '"':
			inStr = true
			out = append(out, c)
			i++
		case c == '/' && i+1 < n && b[i+1] == '/':
			for i < n && b[i] != '\n' {
				i++
			}
		case c == '/' && i+1 < n && b[i+1] == '*':
			i += 2
			for i+1 < n && (b[i] != '*' || b[i+1] != '/') {
				i++
			}
			i += 2
		default:
			out = append(out, c)
			i++
		}
	}
	return out
}

// JSON renders the rules as an indented, editable JSON document.
func (r Rules) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
