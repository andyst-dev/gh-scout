package prs

import (
	"bytes"
	"fmt"
	"strings"
)

// writeMarkdown renders a human-readable summary led by the action verdict:
// which pull requests are on the author ("Up to you") and which are not
// ("Waiting on others"), then the per-repository detail below.
func writeMarkdown(rep *Report) ([]byte, error) {
	var b bytes.Buffer
	fmt.Fprintf(&b, "# PR scout for %s · %s\n\n", rep.User, rep.GeneratedAt.Format("2006-01-02 15:04"))

	type line struct {
		repo string
		pr   PR
	}
	var yours, theirs []line
	for _, repo := range rep.Repos {
		for _, p := range repo.PRs {
			ln := line{repo: repo.Name, pr: p}
			if p.Action.Owner == OwnerYou {
				yours = append(yours, ln)
			} else {
				theirs = append(theirs, ln)
			}
		}
	}

	if len(yours) == 0 {
		b.WriteString("## Up to you (0)\n\n- nothing\n\n")
	} else {
		fmt.Fprintf(&b, "## Up to you (%d)\n\n", len(yours))
		for _, ln := range yours {
			fmt.Fprintf(&b, "- %s #%d · %s | %s\n", ln.repo, ln.pr.Number, ln.pr.Status, strings.Join(ln.pr.Action.Reasons, "; "))
			if ln.pr.URL != "" {
				fmt.Fprintf(&b, "  - %s\n", ln.pr.URL)
			}
			if ln.pr.Hint != "" {
				fmt.Fprintf(&b, "  - %s\n", ln.pr.Hint)
			}
			b.WriteString("\n")
		}
	}

	if len(theirs) == 0 {
		b.WriteString("## Waiting on others (0)\n\n- nothing\n\n")
	} else {
		fmt.Fprintf(&b, "## Waiting on others (%d)\n\n", len(theirs))
		for _, ln := range theirs {
			why := ln.pr.Action.Waiting
			if ln.pr.Response != nil {
				why += fmt.Sprintf(" (response: @%s %s)", ln.pr.Response.Author, ln.pr.Response.Age)
			}
			fmt.Fprintf(&b, "- %s #%d · %s | %s\n", ln.repo, ln.pr.Number, ln.pr.Status, why)
			if ln.pr.URL != "" {
				fmt.Fprintf(&b, "  - %s\n", ln.pr.URL)
			}
			b.WriteString("\n")
		}
	}

	b.WriteString("## By repository\n\n")
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
			if p.Hint != "" {
				fmt.Fprintf(&b, "  - %s\n", p.Hint)
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
