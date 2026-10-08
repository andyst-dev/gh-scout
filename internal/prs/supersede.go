package prs

import (
	"context"
	"fmt"
	"strings"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// missingOnBase returns the paths a pull request modifies that no longer exist
// on the base branch. Files the pull request adds or removes are ignored: only
// a *modification* whose target vanished means the base moved or deleted it.
// Pure over the exists predicate, so the rule is table-testable without HTTP.
func missingOnBase(files []github.FileChange, exists func(string) bool) []string {
	var missing []string
	for _, f := range files {
		if f.Status != "modified" || f.Path == "" {
			continue
		}
		if !exists(f.Path) {
			missing = append(missing, f.Path)
		}
	}
	return missing
}

// addedLines returns the substantive lines a unified diff adds: the '+' lines
// other than the '+++' header, trimmed, with blanks and one-token lines
// dropped so a change is judged on its content, not on punctuation churn.
func addedLines(patch string) []string {
	var out []string
	for _, ln := range strings.Split(patch, "\n") {
		if !strings.HasPrefix(ln, "+") || strings.HasPrefix(ln, "+++") {
			continue
		}
		if t := strings.TrimSpace(ln[1:]); len(t) >= 8 {
			out = append(out, t)
		}
	}
	return out
}

// changeAlreadyOnBase returns the paths whose every added line is already
// present in that path's base content, i.e. the change looks already merged
// there. It stays quiet unless the diff adds at least minHits lines. Pure over
// the base predicate.
func changeAlreadyOnBase(files []github.FileChange, base func(string) (string, bool)) []string {
	const minHits = 5
	var out []string
	for _, f := range files {
		if f.Status != "modified" || f.Path == "" || f.Patch == "" {
			continue
		}
		added := addedLines(f.Patch)
		if len(added) < minHits {
			continue
		}
		content, ok := base(f.Path)
		if !ok {
			continue
		}
		all := true
		for _, line := range added {
			if !strings.Contains(content, line) {
				all = false
				break
			}
		}
		if all {
			out = append(out, f.Path)
		}
	}
	return out
}

// supersessionHint looks for the "already landed on base" shape on a
// conflicting pull request, in two steps, and returns a one-line advisory or
// "":
//
//  1. a file it modifies is gone from the base (renamed or removed there), or
//  2. every line it adds already exists in that file on the base (merged).
//
// API failures are treated as inconclusive so the hint never cries wolf on a
// transient error.
func (r *Runner) supersessionHint(ctx context.Context, repo, base string, number int) (string, error) {
	files, err := r.client.PullFiles(ctx, repo, number)
	if err != nil {
		return "", err
	}

	existsCache := map[string]bool{}
	exists := func(path string) bool {
		if v, ok := existsCache[path]; ok {
			return v
		}
		found, err := r.client.FileExistsOn(ctx, repo, path, base)
		if err != nil {
			found = true // inconclusive: assume present and stay quiet
		}
		existsCache[path] = found
		return found
	}
	if missing := missingOnBase(files, exists); len(missing) > 0 {
		return fmt.Sprintf("may be superseded: %s no longer on %s (the base may have renamed or already merged this change)",
			names(missing), base), nil
	}

	contentCache := map[string]string{}
	failed := map[string]bool{}
	baseContent := func(path string) (string, bool) {
		if v, ok := contentCache[path]; ok {
			return v, true
		}
		if failed[path] {
			return "", false
		}
		s, ok := r.client.FileContentOn(ctx, repo, path, base)
		if !ok {
			failed[path] = true
			return "", false
		}
		contentCache[path] = s
		return s, true
	}
	if merged := changeAlreadyOnBase(files, baseContent); len(merged) > 0 {
		return fmt.Sprintf("may be superseded: this change already appears on %s (%s)", base, names(merged)), nil
	}
	return "", nil
}

// names renders up to three paths, collapsing the rest into a count.
func names(paths []string) string {
	const max = 3
	if len(paths) <= max {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s and %d more", strings.Join(paths[:max], ", "), len(paths)-max)
}
