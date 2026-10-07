package prs

import (
	"testing"
	"time"

	"github.com/andyst-dev/gh-scout/internal/github"
)

func timeAt(y, m, d, h int) time.Time {
	return time.Date(y, time.Month(m), d, h, 0, 0, 0, time.UTC)
}

func TestNewestResponse(t *testing.T) {
	lastPush := timeAt(2024, 1, 1, 10)
	now := timeAt(2024, 1, 3, 8)
	tests := []struct {
		name string
		acts []github.Activity
		want *Response
	}{
		{
			name: "no activity",
			acts: nil,
			want: nil,
		},
		{
			name: "only the author commented",
			acts: []github.Activity{{Login: "andy", At: timeAt(2024, 1, 2, 8)}},
			want: nil,
		},
		{
			name: "activity before last push does not count",
			acts: []github.Activity{{Login: "alice", At: timeAt(2024, 1, 1, 9)}},
			want: nil,
		},
		{
			name: "single response picked",
			acts: []github.Activity{{Login: "alice", At: timeAt(2024, 1, 2, 8)}},
			want: &Response{Author: "alice", Age: "1d ago"},
		},
		{
			name: "newest across issue comments, reviews and review comments",
			acts: []github.Activity{
				{Login: "alice", At: timeAt(2024, 1, 2, 8)}, // issue comment
				{Login: "andy", At: timeAt(2024, 1, 2, 12)}, // author, ignored
				{Login: "bob", At: timeAt(2024, 1, 2, 18)},  // reviewer, newest
			},
			want: &Response{Author: "bob", Age: "14h ago"},
		},
		{
			name: "response still counted when older than last push is not",
			acts: []github.Activity{{Login: "carol", At: timeAt(2024, 1, 1, 23)}},
			want: &Response{Author: "carol", Age: "1d ago"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := NewestResponse("andy", lastPush, now, tc.acts)
			if tc.want == nil {
				if got != nil {
					t.Fatalf("got %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("got nil, want a response")
			}
			if got.Author != tc.want.Author || got.Age != tc.want.Age {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestStatus(t *testing.T) {
	f := false
	tru := true
	tests := []struct {
		name string
		d    github.PRDetail
		want string
	}{
		{name: "clean", d: github.PRDetail{Mergeable: &tru, MergeableState: "clean"}, want: "up to date"},
		{name: "behind", d: github.PRDetail{Mergeable: &tru, MergeableState: "behind"}, want: "behind base"},
		{name: "dirty means conflicts", d: github.PRDetail{Mergeable: &f, MergeableState: "dirty"}, want: "conflicts"},
		{name: "mergeable false means conflicts", d: github.PRDetail{Mergeable: &f, MergeableState: "clean"}, want: "conflicts"},
		{name: "unknown defaults to evaluating", d: github.PRDetail{Mergeable: nil, MergeableState: "unknown"}, want: "evaluating"},
		{name: "null state defaults to evaluating", d: github.PRDetail{Mergeable: nil, MergeableState: ""}, want: "evaluating"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := status(tc.d); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestAgeString(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{2 * 24 * time.Hour, "2d ago"},
		{10 * 24 * time.Hour, "1w ago"},
	}
	for _, tc := range tests {
		if got := ageString(tc.d); got != tc.want {
			t.Errorf("ageString(%v) = %q, want %q", tc.d, got, tc.want)
		}
	}
}
