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

// RulesFromJSON parses a plain JSON rules file. It starts from the defaults so
// a partial file only overrides the fields it sets. The fields are documented
// in the README's "Rules reference".
func RulesFromJSON(data []byte) (Rules, error) {
	base := DefaultRules()
	if err := json.Unmarshal(data, &base); err != nil {
		return Rules{}, fmt.Errorf("parse rules file: %w", err)
	}
	if base.ReadyThreshold < 0 || base.ReadyThreshold > 100 {
		return Rules{}, fmt.Errorf("ready_threshold must be within 0-100, got %d", base.ReadyThreshold)
	}
	return base, nil
}

// JSON renders the rules as an indented, editable JSON document.
func (r Rules) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
