package prs

import (
	"reflect"
	"testing"

	"github.com/andyst-dev/gh-scout/internal/github"
)

func TestMissingOnBase(t *testing.T) {
	files := []github.FileChange{
		{Path: "a/moved.ts", Status: "modified"},
		{Path: "b/kept.ts", Status: "modified"},
		{Path: "c/new.ts", Status: "added"},
		{Path: "d/gone.ts", Status: "removed"},
		{Path: "", Status: "modified"},
	}
	// On base, only b/kept.ts still exists.
	onBase := map[string]bool{"b/kept.ts": true}
	exists := func(p string) bool { return onBase[p] }

	got := missingOnBase(files, exists)
	want := []string{"a/moved.ts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("missingOnBase = %v, want %v", got, want)
	}
}

func TestAddedLines(t *testing.T) {
	patch := "@@ -1,2 +1,3 @@\n context\n-removed\n+added substantive line\n+  \n+tiny\n+++ b/file"
	got := addedLines(patch)
	want := []string{"added substantive line"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("addedLines = %v, want %v", got, want)
	}
}

func TestChangeAlreadyOnBase(t *testing.T) {
	// Five substantive added lines; only merged.ts has them all on base.
	patch := "@@\n+a1aaaaaaaaaa\n+a2aaaaaaaaaa\n+a3aaaaaaaaaa\n+a4aaaaaaaaaa\n+a5aaaaaaaaaa\n-x"
	files := []github.FileChange{
		{Path: "merged.ts", Status: "modified", Patch: patch},
		{Path: "partial.ts", Status: "modified", Patch: patch},
		{Path: "too-small.ts", Status: "modified", Patch: "@@\n+only-one-line\n"},
	}
	base := map[string]string{
		"merged.ts":  "a1aaaaaaaaaa\na2aaaaaaaaaa\na3aaaaaaaaaa\na4aaaaaaaaaa\na5aaaaaaaaaa\n",
		"partial.ts": "a1aaaaaaaaaa\na2aaaaaaaaaa\n", // missing the rest
	}
	content := func(p string) (string, bool) { s, ok := base[p]; return s, ok }

	got := changeAlreadyOnBase(files, content)
	want := []string{"merged.ts"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("changeAlreadyOnBase = %v, want %v", got, want)
	}
}

func TestNamesCollapses(t *testing.T) {
	if got := names([]string{"a", "b"}); got != "a, b" {
		t.Fatalf("names(2) = %q", got)
	}
	if got := names([]string{"a", "b", "c", "d", "e"}); got != "a, b, c and 2 more" {
		t.Fatalf("names(5) = %q", got)
	}
}
