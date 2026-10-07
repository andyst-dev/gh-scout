package scout

import (
	"context"
	"strings"
	"testing"

	"github.com/andyst-dev/gh-scout/internal/github"
)

func TestDefaultRulesJSONRoundTrip(t *testing.T) {
	def := DefaultRules()
	data, err := def.JSON()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := RulesFromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ReadyThreshold != def.ReadyThreshold || parsed.Weights != def.Weights {
		t.Fatalf("round-trip mismatch:\n got %+v\nwant %+v", parsed, def)
	}
}

func TestRulesFromJSONPartialFillsNotNull(t *testing.T) {
	parsed, err := RulesFromJSON([]byte(`{"weights":{"title_defect":40}}`))
	if err != nil {
		t.Fatal(err)
	}
	def := DefaultRules()
	if parsed.Weights.TitleDefect != 40 {
		t.Fatalf("overridden weight = %d, want 40", parsed.Weights.TitleDefect)
	}
	// untouched fields keep the default.
	if parsed.Weights.BodyReproduction != def.Weights.BodyReproduction {
		t.Fatalf("untouched weight changed: %+v", parsed.Weights)
	}
	if parsed.ReadyThreshold != def.ReadyThreshold {
		t.Fatalf("untouched threshold changed: %d", parsed.ReadyThreshold)
	}
}

// RulesTemplate itself is JSONC and must load to the defaults.
func TestRulesTemplateLoadsToDefaults(t *testing.T) {
	parsed, err := RulesFromJSON([]byte(RulesTemplate))
	if err != nil {
		t.Fatalf("annotated template must parse: %v", err)
	}
	def := DefaultRules()
	if parsed.ReadyThreshold != def.ReadyThreshold || parsed.Weights != def.Weights {
		t.Fatalf("template != defaults:\n got %+v\nwant %+v", parsed, def)
	}
}

// A commented file overrides fields but keeps unset defaults.
func TestRulesFromJSONWithComments(t *testing.T) {
	data := []byte(`{
  // raise the bar
  "ready_threshold": 50,
  "weights": {
    /* title defects matter more */
    "title_defect": 40,
    // a key with a slash-like value must survive stripping
    "feature_request": -25
  }
}`)
	parsed, err := RulesFromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.ReadyThreshold != 50 || parsed.Weights.TitleDefect != 40 {
		t.Fatalf("commented override not applied: %+v", parsed)
	}
	if parsed.Weights.BodyReproduction != DefaultRules().Weights.BodyReproduction {
		t.Fatalf("commented parse lost an unset default: %+v", parsed)
	}
}

func TestRulesFromJSONRejectsBadThreshold(t *testing.T) {
	_, err := RulesFromJSON([]byte(`{"ready_threshold":150}`))
	if err == nil || !strings.Contains(err.Error(), "0-100") {
		t.Fatalf("want threshold-range error, got %v", err)
	}
}

func TestRunnerHonorsCustomRules(t *testing.T) {
	// This issue scores well under the default rules (repro + tests).
	l := fakeLister{
		issues: []github.Issue{
			{Number: 7, Title: "App crashes on startup", Body: "Steps to reproduce:\n```\nrun()\n```\nexpected: no panic"},
		},
		prs: []github.PullRequest{},
	}
	// A high ready threshold forces it below the bar -> unclear.
	rep, err := NewRunner(&l).Run(context.Background(), []string{"acme/x"}, Options{
		Rules: Rules{ReadyThreshold: 90},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Candidates) != 1 || rep.Candidates[0].Status != StatusUnclear {
		t.Fatalf("expected the issue to be unclear under threshold 90, got %+v", rep.Candidates)
	}
}
