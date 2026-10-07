package report

import (
	"encoding/json"

	"github.com/andyst-dev/gh-scout/internal/scout"
)

// jsonWriter renders a machine-readable report. It is also how other tools
// (lint bots, schedulers) consume gh-scout.
type jsonWriter struct{}

// jsonReport is the stable on-the-wire shape.
type jsonReport struct {
	GeneratedAt  string          `json:"generated_at"`
	Repositories []string        `json:"repositories"`
	Ready        []jsonCandidate `json:"ready"`
	Addressed    []jsonCandidate `json:"addressed"`
	Unclear      []jsonCandidate `json:"unclear"`
	Summary      jsonSummary     `json:"summary"`
}

type jsonCandidate struct {
	Repository string   `json:"repository"`
	Number     int      `json:"number"`
	Title      string   `json:"title"`
	URL        string   `json:"url"`
	CreatedAt  string   `json:"created_at"`
	Labels     []string `json:"labels"`
	Score      int      `json:"score"`
	Difficulty string   `json:"difficulty,omitempty"`
	Hint       string   `json:"hint,omitempty"`
	Reason     string   `json:"reason"`
}

type jsonSummary struct {
	Examined  int `json:"examined"`
	Ready     int `json:"ready"`
	Addressed int `json:"addressed"`
	Unclear   int `json:"unclear"`
}

func (jsonWriter) Write(rep *scout.Report) ([]byte, error) {
	ready, addressed, unclear := partition(rep.Candidates)

	out := jsonReport{
		GeneratedAt:  rep.GeneratedAt.Format("2006-01-02T15:04:05Z"),
		Repositories: rep.Repos,
		Ready:        toJSON(ready),
		Addressed:    toJSON(addressed),
		Unclear:      toJSON(unclear[:min(10, len(unclear))]),
		Summary: jsonSummary{
			Examined:  len(rep.Candidates),
			Ready:     len(ready),
			Addressed: len(addressed),
			Unclear:   len(unclear),
		},
	}
	return json.MarshalIndent(out, "", "  ")
}

func toJSON(cs []scout.Candidate) []jsonCandidate {
	out := make([]jsonCandidate, 0, len(cs))
	for _, c := range cs {
		out = append(out, jsonCandidate{
			Repository: c.Repository,
			Number:     c.Number,
			Title:      c.Title,
			URL:        c.URL,
			CreatedAt:  c.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			Labels:     c.Labels,
			Score:      c.Score,
			Difficulty: c.Difficulty,
			Hint:       c.Hint,
			Reason:     c.Reason,
		})
	}
	return out
}
