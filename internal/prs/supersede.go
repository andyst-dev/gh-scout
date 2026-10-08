package prs

import (
	"context"
	"fmt"

	"github.com/andyst-dev/gh-scout/internal/github"
)

// missingOnBase returns the paths a pull request modifies that no longer exist
// on the base branch, in the order GitHub listed them. Files the pull request
// adds or removes are ignored: only a *modification* whose target vanished
// means the base moved or deleted it. Pure over the exists predicate, so the
// rule is table-testable without any HTTP.
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

// supersessionHint looks for the "already landed on base" shape on a
// conflicting pull request: a file it modifies no longer exists on the base
// branch, so the change may have been renamed away or re-implemented there.
// It returns "" when nothing looks superseded. API failures are treated as
// "still exists" so the hint never cries wolf on a transient error.
func (r *Runner) supersessionHint(ctx context.Context, repo, base string, number int) (string, error) {
	files, err := r.client.PullFiles(ctx, repo, number)
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	exists := func(path string) bool {
		if v, ok := seen[path]; ok {
			return v
		}
		found, err := r.client.FileExistsOn(ctx, repo, path, base)
		if err != nil {
			found = true
		}
		seen[path] = found
		return found
	}
	missing := missingOnBase(files, exists)
	if len(missing) == 0 {
		return "", nil
	}
	return fmt.Sprintf("may be superseded: %s no longer exists on %s (the base may have moved or already merged this change)", missing[0], base), nil
}
