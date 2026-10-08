package prs

import (
	"testing"
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
)

func TestClassify(t *testing.T) {
	last := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	mk := func(author string, at time.Time) github.Thread {
		return github.Thread{Path: "a.ts", Line: 3, LastAuthor: author, LastAt: at}
	}
	after := time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)
	before := time.Date(2023, 12, 1, 0, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		status      string
		rs          github.ReviewState
		wantOwner   string
		wantReasons int
		wantWaiting string
	}{
		{name: "conflicts", status: "conflicts", wantOwner: OwnerYou, wantReasons: 1},
		{name: "behind base", status: "behind base", wantOwner: OwnerYou, wantReasons: 1},
		{name: "changes requested with outstanding thread", status: "clean", rs: github.ReviewState{Decision: "CHANGES_REQUESTED", OpenThreads: []github.Thread{mk("bob", before)}}, wantOwner: OwnerYou, wantReasons: 1},
		{name: "changes requested freshly, no thread", status: "clean", rs: github.ReviewState{Decision: "CHANGES_REQUESTED", LastReview: github.ReviewView{State: "CHANGES_REQUESTED", At: after}}, wantOwner: OwnerYou, wantReasons: 1},
		{name: "changes requested addressed, threads resolved, old review", status: "clean", rs: github.ReviewState{Decision: "CHANGES_REQUESTED"}, wantOwner: OwnerThem, wantWaiting: "reviewer to re-approve the addressed changes"},
		{name: "changes requested addressed, approved review last", status: "clean", rs: github.ReviewState{Decision: "CHANGES_REQUESTED", LastReview: github.ReviewView{State: "APPROVED", At: after}}, wantOwner: OwnerThem, wantWaiting: "reviewer to re-approve the addressed changes"},
		{name: "open thread awaiting", status: "clean", rs: github.ReviewState{OpenThreads: []github.Thread{mk("bob", after)}}, wantOwner: OwnerYou, wantReasons: 1},
		{name: "thread awaits but author replied after", status: "clean", rs: github.ReviewState{OpenThreads: []github.Thread{mk("bob", before)}}, wantOwner: OwnerThem, wantWaiting: "review"},
		{name: "own thread ignored", status: "clean", rs: github.ReviewState{OpenThreads: []github.Thread{mk("andy", after)}}, wantOwner: OwnerThem, wantWaiting: "review"},
		{name: "thread without author ignored", status: "clean", rs: github.ReviewState{OpenThreads: []github.Thread{{Path: "a.ts", LastAt: after}}}, wantOwner: OwnerThem, wantWaiting: "review"},
		{name: "approved waits merge", status: "clean", rs: github.ReviewState{Decision: "APPROVED"}, wantOwner: OwnerThem, wantWaiting: "merge"},
		{name: "clean waits review", status: "up to date", wantOwner: OwnerThem, wantWaiting: "review"},
		{name: "blocked waits checks", status: "blocked", wantOwner: OwnerThem, wantWaiting: "review and required checks"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Classify("andy", last, c.status, c.rs)
			if a.Owner != c.wantOwner {
				t.Fatalf("owner=%q want %q (action=%+v)", a.Owner, c.wantOwner, a)
			}
			if len(a.Reasons) != c.wantReasons {
				t.Fatalf("reasons=%v want %d", a.Reasons, c.wantReasons)
			}
			if a.Waiting != c.wantWaiting {
				t.Fatalf("waiting=%q want %q", a.Waiting, c.wantWaiting)
			}
		})
	}
}

func TestClassifyStacksReasonsByPriority(t *testing.T) {
	last := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	a := Classify("andy", last, "conflicts", github.ReviewState{
		Decision:    "CHANGES_REQUESTED",
		OpenThreads: []github.Thread{{Path: "a.ts", Line: 2, LastAuthor: "bob", LastAt: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)}},
	})
	if a.Owner != OwnerYou {
		t.Fatalf("owner=%q", a.Owner)
	}
	// rebase, then the newest awaiting thread. CHANGES_REQUESTED is implied by
	// the awaiting thread and does not add a third, less specific reason.
	if len(a.Reasons) != 2 {
		t.Fatalf("want 2 stacked reasons, got %v", a.Reasons)
	}
	if a.Reasons[0] != "rebase: the branch conflicts with its base" {
		t.Fatalf("rebase must lead, got %v", a.Reasons)
	}
	if a.Reasons[1] != "reply in a.ts:2 (@bob)" {
		t.Fatalf("thread reply must follow, got %v", a.Reasons)
	}
}

func TestThreadAnchor(t *testing.T) {
	if got := (github.Thread{Path: "a.ts", Line: 12}).Anchor(); got != "a.ts:12" {
		t.Fatalf("anchor=%q", got)
	}
	if got := (github.Thread{Path: "a.ts"}).Anchor(); got != "a.ts" {
		t.Fatalf("anchor=%q", got)
	}
	if got := (github.Thread{}).Anchor(); got != "the review thread" {
		t.Fatalf("anchor=%q", got)
	}
}
