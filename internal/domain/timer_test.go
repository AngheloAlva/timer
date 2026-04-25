package domain

import (
	"testing"
	"time"
)

func TestTimer_ElapsedSec(t *testing.T) {
	base := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		timer Timer
		now   time.Time
		want  int64
	}{
		{
			name:  "running for 60s",
			timer: Timer{StartedAt: base, PausedTotalSec: 0, PausedAt: nil},
			now:   base.Add(60 * time.Second),
			want:  60,
		},
		{
			name:  "running for 60s with 20s prior pause",
			timer: Timer{StartedAt: base, PausedTotalSec: 20, PausedAt: nil},
			now:   base.Add(60 * time.Second),
			want:  40,
		},
		{
			name: "currently paused: subtract current pause",
			timer: Timer{
				StartedAt:      base,
				PausedTotalSec: 0,
				PausedAt:       ptrTime(base.Add(30 * time.Second)),
			},
			now:  base.Add(60 * time.Second),
			want: 30,
		},
		{
			name: "paused for the entire duration",
			timer: Timer{
				StartedAt:      base,
				PausedTotalSec: 0,
				PausedAt:       ptrTime(base),
			},
			now:  base.Add(60 * time.Second),
			want: 0,
		},
		{
			name:  "negative result clamped to 0",
			timer: Timer{StartedAt: base, PausedTotalSec: 9999},
			now:   base.Add(10 * time.Second),
			want:  0,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.timer.ElapsedSec(tc.now)
			if got != tc.want {
				t.Errorf("got %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTimer_IsPaused(t *testing.T) {
	base := time.Now()
	if (Timer{}).IsPaused() {
		t.Errorf("zero-value Timer should not be paused")
	}
	if !(Timer{PausedAt: &base}).IsPaused() {
		t.Errorf("Timer with PausedAt set should be paused")
	}
}

func ptrTime(t time.Time) *time.Time { return &t }
