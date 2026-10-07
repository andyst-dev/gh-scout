package prs

import (
	"bytes"
	"fmt"
)

// writeMarkdown renders the human-readable summary.
func writeMarkdown(rep *Report) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# PR scout for %s · %s\n\n", rep.User, rep.GeneratedAt.Format("2006-01-02 15:04"))

	for _, repo := range rep.Repos {
		fmt.Fprintf(&b, "## %s\n\n", repo.Name)
		for _, p := range repo.PRs {
			line := fmt.Sprintf("- [#%d · %s] %s", p.Number, p.Status, p.Title)
			if p.Response != nil {
				line += fmt.Sprintf(" (response: @%s %s)", p.Response.Author, p.Response.Age)
			}
			b.WriteString(line + "\n")
			if p.URL != "" {
				fmt.Fprintf(&b, "  - %s\n", p.URL)
			}
			b.WriteString("\n")
		}
		if repo.Skipped > 0 {
			fmt.Fprintf(&b, "Skipped %d pull request(s).\n", repo.Skipped)
			for _, note := range repo.Notes {
				fmt.Fprintf(&b, "- %s\n", note)
			}
			b.WriteString("\n")
		}
	}
	return b.Bytes(), nil
}
