package report

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/andyst-dev/gh-scout/internal/scout"
)

// markdownWriter renders the human-readable summary.
type markdownWriter struct{}

func (markdownWriter) Write(rep *scout.Report) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# Contribution scouting · %s\n\n", rep.GeneratedAt.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "Repositories: %s\n\n", strings.Join(rep.Repos, ", "))

	ready, addressed, unclear := partition(rep.Candidates)

	if len(ready) == 0 {
		b.WriteString("## Ready\n\nNo confident fix targets found in the scanned window.\n\n")
	} else {
		fmt.Fprintf(&b, "## Ready targets · %d\n\n", len(ready))
		for _, c := range ready {
			fmt.Fprintf(&b, "- **[#%d · score %d · %s]** %s\n", c.Number, c.Score, c.Difficulty, c.Title)
			fmt.Fprintf(&b, "  - `%s` · %s\n", c.Repository, c.URL)
			if c.Hint != "" {
				fmt.Fprintf(&b, "  - suggested PR: %s\n", c.Hint)
			}
			fmt.Fprintf(&b, "  - %s\n", c.Reason)
		}
		b.WriteString("\n")
	}

	if len(addressed) > 0 {
		fmt.Fprintf(&b, "### Excluded (%d already addressed)\n\n", len(addressed))
		for _, c := range addressed {
			fmt.Fprintf(&b, "- %s#%d %s (%s)\n", c.Repository, c.Number, c.Title, c.Reason)
		}
		b.WriteString("\n")
	}

	if len(unclear) > 0 {
		fmt.Fprintf(&b, "### Skipped (%d, too little signal)\n", len(unclear))
		for _, c := range unclear[:min(5, len(unclear))] {
			fmt.Fprintf(&b, "- %s#%d %s\n", c.Repository, c.Number, c.Title)
		}
		if len(unclear) > 5 {
			fmt.Fprintf(&b, "- … and %d more\n", len(unclear)-5)
		}
		b.WriteString("\n")
	}

	total := len(ready) + len(addressed) + len(unclear)
	fmt.Fprintf(&b, "_%d issue(s) examined, %d ready._\n", total, len(ready))
	b.WriteString("\n_Score 0-100 = how fixable an issue looks: clear defect, reproduction, tests and help labels help. Statuses: ready / addressed / duplicate._\n")
	return b.Bytes(), nil
}

// partition splits candidates by status.
func partition(cs []scout.Candidate) (ready, addressed, unclear []scout.Candidate) {
	for _, c := range cs {
		switch c.Status {
		case scout.StatusReady:
			ready = append(ready, c)
		case scout.StatusAddressed:
			addressed = append(addressed, c)
		default:
			unclear = append(unclear, c)
		}
	}
	return ready, addressed, unclear
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
