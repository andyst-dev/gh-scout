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
