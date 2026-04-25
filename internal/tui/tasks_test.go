package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/AngheloAlva/timer/internal/domain"
)

// keyMsg builds a tea.KeyMsg whose String() returns the given single-rune
// key. Used to drive Update without a real Bubbletea program.
func keyMsg(s string) tea.KeyMsg {
	return tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune(s)})
}

func TestTasksView_RendersWithoutPanic(t *testing.T) {
	styles := NewStyles(DefaultPalette)
	base := newTasksModel(nil, nil, nil, styles)

	now := time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name  string
		model tasksModel
	}{
		{name: "loading", model: base},
		{
			name: "empty",
			model: func() tasksModel {
				m := base
				m.initialized = true
				return m
			}(),
		},
		{
			name: "error",
			model: func() tasksModel {
				m := base
				m.initialized = true
				m.loadErr = errString("boom")
				return m
			}(),
		},
		{
			name: "populated with running and paused timers",
			model: func() tasksModel {
				m := base
				m.initialized = true
				m.tasks = []domain.Task{
					{ID: "aaaaaaaa-1111", Title: "Task A", Status: domain.StatusInProgress, ProjectName: "P", ProjectSlug: "p"},
					{ID: "bbbbbbbb-2222", Title: "Task B", Status: domain.StatusTodo, ProjectName: "P", ProjectSlug: "p"},
					{ID: "cccccccc-3333", Title: "Task C done", Status: domain.StatusDone, ProjectName: "P", ProjectSlug: "p"},
				}
				m.activeByTID = map[string]domain.Timer{
					"aaaaaaaa-1111": {TaskID: "aaaaaaaa-1111", StartedAt: now.Add(-5 * time.Minute)},
					"bbbbbbbb-2222": {TaskID: "bbbbbbbb-2222", StartedAt: now.Add(-10 * time.Minute), PausedAt: ptrTime(now.Add(-1 * time.Minute))},
				}
				m.cursor = 1
				m.statusMsg = "Started P / Task B"
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
			if !strings.Contains(out, "Tasks") {
				t.Errorf("missing 'Tasks' header in render")
			}
		})
	}
}

func TestTasksView_NavigationClampsCursor(t *testing.T) {
	styles := NewStyles(DefaultPalette)
	m := newTasksModel(nil, nil, nil, styles)
	m.initialized = true
	m.tasks = []domain.Task{
		{ID: "1", Title: "A"},
		{ID: "2", Title: "B"},
	}

	// Cursor stays at 0 when pressing k from top.
	m, _ = m.Update(keyMsg("k"))
	if m.cursor != 0 {
		t.Errorf("cursor=%d, want 0", m.cursor)
	}

	// Down twice → 1, third stays at 1 (last index).
	m, _ = m.Update(keyMsg("j"))
	if m.cursor != 1 {
		t.Errorf("cursor=%d, want 1", m.cursor)
	}
	m, _ = m.Update(keyMsg("j"))
	if m.cursor != 1 {
		t.Errorf("cursor stuck at last, got %d", m.cursor)
	}
}
