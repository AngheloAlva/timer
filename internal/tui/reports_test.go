package tui

import (
	"strings"
	"testing"

	"github.com/AngheloAlva/timer/internal/service"
)

func TestReportsView_RendersAllStates(t *testing.T) {
	styles := NewStyles(DefaultPalette)
	base := newReportsModel(nil, styles)

	cases := []struct {
		name     string
		model    reportsModel
		mustHave string
	}{
		{name: "loading", model: base, mustHave: "Loading"},
		{
			name: "error",
			model: func() reportsModel {
				m := base
				m.loading = false
				m.initialized = true
				m.loadErr = errString("boom")
				return m
			}(),
			mustHave: "boom",
		},
		{
			name: "empty",
			model: func() reportsModel {
				m := base
				m.loading = false
				m.initialized = true
				m.rangeLabel = "Today (2026-04-25)"
				return m
			}(),
			mustHave: "No entries",
		},
		{
			name: "populated",
			model: func() reportsModel {
				m := base
				m.loading = false
				m.initialized = true
				m.rangeLabel = "Today (2026-04-25)"
				m.summary = service.ReportSummary{
					Total: 90,
					Projects: []service.ProjectSummary{
						{
							ID: "p", Name: "Timer CLI", Slug: "timer-cli", Total: 90,
							Tasks: []service.TaskSummary{
								{ID: "t1", Title: "Implement", Total: 90},
							},
						},
					},
				}
				return m
			}(),
			mustHave: "Implement",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.model.View()
			if !strings.Contains(out, tc.mustHave) {
				t.Errorf("missing %q in render:\n%s", tc.mustHave, out)
			}
		})
	}
}

func TestReportsView_ToggleTriggersReload(t *testing.T) {
	styles := NewStyles(DefaultPalette)
	m := newReportsModel(nil, styles)
	m.loading = false
	m.initialized = true

	// Press 'w' → range becomes week, loading flips on, returns a Cmd.
	m, cmd := m.Update(keyMsg("w"))
	if m.rng != rangeWeek {
		t.Errorf("range = %v, want week", m.rng)
	}
	if !m.loading {
		t.Errorf("expected loading=true after toggle")
	}
	if cmd == nil {
		t.Errorf("expected reload Cmd")
	}

	// Pressing 'w' again is a no-op.
	m, cmd = m.Update(keyMsg("w"))
	if cmd != nil {
		t.Errorf("repeated toggle should not fire a reload")
	}
}
