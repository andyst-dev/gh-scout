package prs

import "encoding/json"

// jsonReport is the stable on-the-wire shape of a PR scout report.
type jsonReport struct {
	User        string     `json:"user"`
	GeneratedAt string     `json:"generated_at"`
	Repos       []jsonRepo `json:"repos"`
}

type jsonRepo struct {
	Name         string   `json:"name"`
	PullRequests []jsonPR `json:"pull_requests"`
	Skipped      int      `json:"skipped,omitempty"`
	Notes        []string `json:"notes,omitempty"`
}

type jsonPR struct {
	Number   int           `json:"number"`
	Title    string        `json:"title"`
	URL      string        `json:"url"`
	Status   string        `json:"status"`
	Response *jsonResponse `json:"response,omitempty"`
	Action   jsonAction    `json:"action"`
}

type jsonAction struct {
	Owner   string   `json:"owner"`
	Reasons []string `json:"reasons,omitempty"`
	Waiting string   `json:"waiting,omitempty"`
}

type jsonResponse struct {
	Author string `json:"author"`
	Age    string `json:"age"`
}

// writeJSON renders a machine-readable report.
func writeJSON(rep *Report) ([]byte, error) {
	out := jsonReport{
		User:        rep.User,
		GeneratedAt: rep.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}
	for _, repo := range rep.Repos {
		jr := jsonRepo{Name: repo.Name, Skipped: repo.Skipped, Notes: repo.Notes}
		for _, p := range repo.PRs {
			jp := jsonPR{
				Number: p.Number,
				Title:  p.Title,
				URL:    p.URL,
				Status: p.Status,
				Action: jsonAction{
					Owner:   p.Action.Owner,
					Reasons: p.Action.Reasons,
					Waiting: p.Action.Waiting,
				},
			}
			if p.Response != nil {
				jp.Response = &jsonResponse{Author: p.Response.Author, Age: p.Response.Age}
			}
			jr.PullRequests = append(jr.PullRequests, jp)
		}
		out.Repos = append(out.Repos, jr)
	}
	return json.MarshalIndent(out, "", "  ")
}
