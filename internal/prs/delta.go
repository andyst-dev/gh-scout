package prs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/andyst-dev/gh-scout/internal/report"
	"github.com/andyst-dev/gh-scout/internal/state"
)

// prKey identifies a pull request across runs.
func prKey(repo string, number int) string {
	return fmt.Sprintf("%s#%d", repo, number)
}

// parseKey splits a prKey back into its repository and number.
func parseKey(k string) (string, int, bool) {
	i := strings.LastIndex(k, "#")
	if i < 0 {
		return "", 0, false
	}
	n, err := strconv.Atoi(k[i+1:])
	if err != nil {
		return "", 0, false
	}
	return k[:i], n, true
}

// Snapshot captures a report as durable state for the next run.
func Snapshot(rep *Report) state.Snapshot {
	s := state.Snapshot{SavedAt: rep.GeneratedAt, PRs: make(map[string]state.Entry)}
	for _, repo := range rep.Repos {
		for _, p := range repo.PRs {
			e := state.Entry{Status: p.Status, Owner: p.Action.Owner, Title: p.Title, URL: p.URL}
			if p.Response != nil {
				e.ResponseAuthor = p.Response.Author
				e.ResponseAt = p.Response.At
			}
			s.PRs[prKey(repo.Name, p.Number)] = e
		}
	}
	return s
}

// Change is one difference between the current run and the previous snapshot.
type Change struct {
	Repo   string
	Number int
	Title  string
	URL    string
	// Owner is the current owner (OwnerYou or OwnerThem); empty when the pull
	// request has left the open list.
	Owner string
	// Notes lists what changed, e.g. "now up to you", "new response from @sam".
	Notes []string
}

// DeltaReport lists what changed since the previous run.
type DeltaReport struct {
	User        string
	GeneratedAt time.Time
	PrevSavedAt time.Time
	Changes     []Change
	UpToYou     int
	Waiting     int
	Total       int
}

// lifecycle resolves a pull request that is no longer in the open list.
type lifecycle func(repo string, number int) (merged, closed bool, err error)

// Diff compares a report against the previous snapshot and returns what
// changed: a new pull request, a change of owner, a newer response, a status
// move, and anything that left the open list (verified through lookup when it
// is given). Pure apart from lookup, so it stays table-testable.
func Diff(rep *Report, prev state.Snapshot, lookup lifecycle) *DeltaReport {
	d := &DeltaReport{User: rep.User, GeneratedAt: rep.GeneratedAt, PrevSavedAt: prev.SavedAt}
	seen := make(map[string]bool, len(prev.PRs))
	for _, repo := range rep.Repos {
		for _, p := range repo.PRs {
			d.Total++
			if p.Action.Owner == OwnerYou {
				d.UpToYou++
			} else {
				d.Waiting++
			}
			k := prKey(repo.Name, p.Number)
			seen[k] = true
			old, known := prev.PRs[k]
			var notes []string
			if !known {
				notes = append(notes, "new pull request")
			} else {
				if old.Owner != p.Action.Owner {
					if p.Action.Owner == OwnerYou {
						notes = append(notes, "now up to you")
					} else {
						notes = append(notes, "back to waiting on others")
					}
				}
				if p.Response != nil && (p.Response.Author != old.ResponseAuthor || p.Response.At.After(old.ResponseAt)) {
					notes = append(notes, "new response from @"+p.Response.Author)
				}
				if old.Status != p.Status && p.Status != "" {
					notes = append(notes, "status: "+old.Status+" -> "+p.Status)
				}
			}
			if len(notes) > 0 {
				d.Changes = append(d.Changes, Change{
					Repo: repo.Name, Number: p.Number, Title: p.Title,
					URL: p.URL, Owner: p.Action.Owner, Notes: notes,
				})
			}
		}
	}
	for k, old := range prev.PRs {
		if seen[k] {
			continue
		}
		repo, number, ok := parseKey(k)
		if !ok {
			continue
		}
		note := "no longer listed (merged, closed, or filtered out by --days / --max / --repos)"
		if lookup != nil {
			merged, closed, err := lookup(repo, number)
			switch {
			case err != nil:
				// keep the cautious default
			case merged:
				note = "merged"
			case closed:
				note = "closed without merge"
			default:
				note = "still open, but filtered out by --days / --max / --repos"
			}
		}
		d.Changes = append(d.Changes, Change{
			Repo: repo, Number: number, Title: old.Title, URL: old.URL, Notes: []string{note},
		})
	}
	sortChanges(d.Changes)
	return d
}

// sortChanges keeps the report useful: what is on you first, then what left the
// list, then the rest, each by repository and number.
func sortChanges(cs []Change) {
	rank := func(c Change) int {
		switch c.Owner {
		case OwnerYou:
			return 0
		case "":
			return 1
		default:
			return 2
		}
	}
	sort.SliceStable(cs, func(i, j int) bool {
		if rank(cs[i]) != rank(cs[j]) {
			return rank(cs[i]) < rank(cs[j])
		}
		if cs[i].Repo != cs[j].Repo {
			return cs[i].Repo < cs[j].Repo
		}
		return cs[i].Number < cs[j].Number
	})
}

// RenderDelta renders a delta report in the requested format.
func RenderDelta(d *DeltaReport, kind report.Kind) ([]byte, error) {
	switch kind {
	case report.KindMarkdown:
		return writeDeltaMarkdown(d), nil
	case report.KindJSON:
		return writeDeltaJSON(d)
	default:
		return nil, errors.New("unknown report format " + string(kind))
	}
}

func writeDeltaMarkdown(d *DeltaReport) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# PR delta for %s · %s\n\n", d.User, d.GeneratedAt.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "## Changed since the last run (%s)\n\n", ageString(d.GeneratedAt.Sub(d.PrevSavedAt)))
	if len(d.Changes) == 0 {
		b.WriteString("- nothing\n\n")
	} else {
		for _, c := range d.Changes {
			fmt.Fprintf(&b, "- %s #%d · %s\n", c.Repo, c.Number, strings.Join(c.Notes, "; "))
			if c.URL != "" {
				fmt.Fprintf(&b, "  - %s\n", c.URL)
			}
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "%d change(s) · %d up to you · %d waiting · %d seen.\n", len(d.Changes), d.UpToYou, d.Waiting, d.Total)
	return b.Bytes()
}

type jsonDelta struct {
	User        string       `json:"user"`
	GeneratedAt string       `json:"generated_at"`
	PreviousRun string       `json:"previous_run"`
	UpToYou     int          `json:"up_to_you"`
	Waiting     int          `json:"waiting"`
	Total       int          `json:"total"`
	Changes     []jsonChange `json:"changes"`
}

type jsonChange struct {
	Repo   string   `json:"repo"`
	Number int      `json:"number"`
	Title  string   `json:"title,omitempty"`
	URL    string   `json:"url,omitempty"`
	Notes  []string `json:"notes"`
}

func writeDeltaJSON(d *DeltaReport) ([]byte, error) {
	out := jsonDelta{
		User:        d.User,
		GeneratedAt: d.GeneratedAt.UTC().Format("2006-01-02T15:04:05Z"),
		PreviousRun: d.PrevSavedAt.UTC().Format("2006-01-02T15:04:05Z"),
		UpToYou:     d.UpToYou,
		Waiting:     d.Waiting,
		Total:       d.Total,
		Changes:     make([]jsonChange, 0, len(d.Changes)),
	}
	for _, c := range d.Changes {
		out.Changes = append(out.Changes, jsonChange{
			Repo: c.Repo, Number: c.Number, Title: c.Title, URL: c.URL, Notes: c.Notes,
		})
	}
	return json.MarshalIndent(out, "", "  ")
}
