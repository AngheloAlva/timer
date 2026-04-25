package service

import (
	"testing"
	"time"
)

func TestStartOfDay(t *testing.T) {
	loc := time.FixedZone("AR", -3*3600)
	in := time.Date(2026, 4, 25, 14, 37, 12, 999, loc)
	got := StartOfDay(in)
	want := time.Date(2026, 4, 25, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Errorf("StartOfDay = %v, want %v", got, want)
	}
	if got.Location() != loc {
		t.Errorf("StartOfDay lost location: got %v", got.Location())
	}
}

func TestStartOfISOWeek(t *testing.T) {
	// 2026-04-25 is a Saturday → Monday of the same ISO week is 2026-04-20.
	cases := []struct {
		name string
		in   time.Time
		want time.Time
	}{
		{
			name: "saturday rolls back to monday",
			in:   time.Date(2026, 4, 25, 23, 0, 0, 0, time.UTC),
			want: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "monday is start of itself",
			in:   time.Date(2026, 4, 20, 9, 0, 0, 0, time.UTC),
			want: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "sunday rolls back six days (ISO week)",
			in:   time.Date(2026, 4, 26, 12, 0, 0, 0, time.UTC),
			want: time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := StartOfISOWeek(tc.in)
			if !got.Equal(tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
