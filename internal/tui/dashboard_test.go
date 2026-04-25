package tui

import (
	"strings"
	"testing"
	"time"

	"github.com/AngheloAlva/timer/internal/domain"
)

// TestDashboard_RendersWithoutPanic exercises the view paths without a
// real Bubbletea program: empty, loading, error, and populated. Catches
// nil-deref / missing-field bugs introduced when the model evolves.
func TestDashboard_RendersWithoutPanic(t *testing.T) {
	styles := NewStyles(DefaultPalette)
	base := newDashboardModel(nil, styles)

	cases := []struct {
		name  string
		model dashboardModel
	}{
		{
			name:  "loading (initial)",
			model: base,
		},
		{
			name: "empty after load",
			model: func() dashboardModel {
				m := base
				m.initialized = true
				return m
			}(),
		},
		{
			name: "error state",
			model: func() dashboardModel {
				m := base
				m.initialized = true
				m.loadErr = errString("boom")
				return m
			}(),
		},
		{
			name: "populated",
			model: func() dashboardModel {
				m := base
				m.initialized = true
				m.now = time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)
				m.todayTotal = 3725
				m.active = []domain.Timer{
					{
						ID: "11111111-aaaa-bbbb-cccc-000000000000", TaskID: "ffffffff-aaaa-bbbb-cccc-111111111111",
						TaskTitle: "Wire dashboard", ProjectSlug: "timer-cli",
						StartedAt: m.now.Add(-30 * time.Minute),
					},
					{
						ID: "22222222-aaaa-bbbb-cccc-000000000000", TaskID: "eeeeeeee-aaaa-bbbb-cccc-222222222222",
						TaskTitle: "Pause example", ProjectSlug: "timer-cli",
						StartedAt: m.now.Add(-time.Hour), PausedAt: ptrTime(m.now.Add(-5 * time.Minute)),
					},
				}
				return m
			}(),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.model.View()
			if out == "" {
				t.Errorf("empty render")
			}
			// Sanity: title section must always be present.
			if !strings.Contains(out, "Active timers") {
				t.Errorf("missing 'Active timers' header in render")
			}
		})
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

type errString string

func (e errString) Error() string { return string(e) }
