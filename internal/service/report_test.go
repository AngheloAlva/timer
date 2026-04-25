package service

import (
	"testing"

	"github.com/AngheloAlva/timer/internal/domain"
)

func TestAggregateEntries_GroupsAndSorts(t *testing.T) {
	entries := []domain.TimeEntry{
		{ProjectID: "p1", ProjectName: "Alpha", ProjectSlug: "alpha", TaskID: "t1", TaskTitle: "A1", DurationSec: 60},
		{ProjectID: "p1", ProjectName: "Alpha", ProjectSlug: "alpha", TaskID: "t1", TaskTitle: "A1", DurationSec: 30},
		{ProjectID: "p1", ProjectName: "Alpha", ProjectSlug: "alpha", TaskID: "t2", TaskTitle: "A2", DurationSec: 200},
		{ProjectID: "p2", ProjectName: "Beta", ProjectSlug: "beta", TaskID: "t3", TaskTitle: "B1", DurationSec: 1000},
	}

	got := AggregateEntries(entries)

	if got.Total != 1290 {
		t.Errorf("total = %d, want 1290", got.Total)
	}

	if len(got.Projects) != 2 {
		t.Fatalf("projects len = %d, want 2", len(got.Projects))
	}

	// Beta has more total → comes first.
	if got.Projects[0].Slug != "beta" {
		t.Errorf("first project = %q, want beta", got.Projects[0].Slug)
	}
	if got.Projects[0].Total != 1000 {
		t.Errorf("beta total = %d, want 1000", got.Projects[0].Total)
	}

	alpha := got.Projects[1]
	if alpha.Slug != "alpha" {
		t.Errorf("second project = %q, want alpha", alpha.Slug)
	}
	if alpha.Total != 290 {
		t.Errorf("alpha total = %d, want 290", alpha.Total)
	}
	// Within alpha: A2 (200) > A1 (90).
	if len(alpha.Tasks) != 2 || alpha.Tasks[0].Title != "A2" {
		t.Errorf("alpha tasks misordered: %+v", alpha.Tasks)
	}
}

func TestAggregateEntries_Empty(t *testing.T) {
	got := AggregateEntries(nil)
	if got.Total != 0 || len(got.Projects) != 0 {
		t.Errorf("empty input should produce empty summary, got %+v", got)
	}
}
