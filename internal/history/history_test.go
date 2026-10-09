package history

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type fakeCounter struct {
	counts map[string]int
	err    error
}

func (f *fakeCounter) MergedPRCount(_ context.Context, repo, _ string) (int, error) {
	if f.err != nil {
		return 0, f.err
	}
	return f.counts[repo], nil
}

func TestRun(t *testing.T) {
	c := &fakeCounter{counts: map[string]int{"acme/widgets": 15, "acme/gadgets": 12}}
	rep, err := Run(context.Background(), c, "andy", []string{"acme/widgets", "acme/gadgets"})
	if err != nil {
		t.Fatal(err)
	}
	if rep.Author != "andy" {
		t.Fatalf("author=%q", rep.Author)
	}
	if rep.Total != 27 {
		t.Fatalf("total=%d want 27", rep.Total)
	}
	if len(rep.Repos) != 2 || rep.Repos[0].Merged != 15 || rep.Repos[1].Merged != 12 {
		t.Fatalf("repos=%+v", rep.Repos)
	}
}

func TestRunError(t *testing.T) {
	c := &fakeCounter{err: errors.New("boom")}
	if _, err := Run(context.Background(), c, "andy", []string{"acme/widgets"}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRenderMarkdown(t *testing.T) {
	rep := &Report{Author: "andy", Repos: []RepoPRs{{Repo: "acme/widgets", Merged: 15}, {Repo: "acme/gadgets", Merged: 12}}, Total: 27}
	md := string(RenderMarkdown(rep))
	for _, want := range []string{"# Merged by andy", "acme/widgets · 15 merged", "acme/gadgets · 12 merged", "Total: 27"} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q\n%s", want, md)
		}
	}
}

func TestRenderJSON(t *testing.T) {
	rep := &Report{Author: "andy", Repos: []RepoPRs{{Repo: "acme/widgets", Merged: 15}}, Total: 15}
	js, err := RenderJSON(rep)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"author": "andy"`, `"merged": 15`, `"total": 15`} {
		if !strings.Contains(string(js), want) {
			t.Errorf("json missing %q\n%s", want, js)
		}
	}
}
