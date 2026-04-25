package tui

import (
	"strings"
	"testing"

	"github.com/AngheloAlva/timer/internal/domain"
)

func TestTaskCreateModal_RendersAllSteps(t *testing.T) {
	styles := NewStyles(DefaultPalette)
	base := newTaskCreateModel(nil, nil, styles)

	cases := []struct {
		name      string
		model     taskCreateModel
		mustHave  string
		mustNot   string
	}{
		{name: "loading", model: base, mustHave: "Loading projects", mustNot: "Pick a project"},
		{
			name: "project picker",
			model: func() taskCreateModel {
				m := base
				m.step = createStepProject
				m.projects = []domain.Project{{Name: "A", Slug: "a"}, {Name: "B", Slug: "b"}}
				return m
			}(),
			mustHave: "Pick a project",
			mustNot:  "Task title",
		},
		{
			name: "title input (single project)",
			model: func() taskCreateModel {
				m := base
				m.step = createStepTitle
				m.projects = []domain.Project{{Name: "Only", Slug: "only"}}
				return m
			}(),
			mustHave: "Project: only",
		},
		{
			name: "load error",
			model: func() taskCreateModel {
				m := base
				m.loadErr = errString("boom")
				return m
			}(),
			mustHave: "boom",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := tc.model.View()
			if !strings.Contains(out, tc.mustHave) {
				t.Errorf("missing %q in render:\n%s", tc.mustHave, out)
			}
			if tc.mustNot != "" && strings.Contains(out, tc.mustNot) {
				t.Errorf("unexpected %q in render", tc.mustNot)
			}
		})
	}
}

func TestTaskCreateModal_SingleProjectSkipsPicker(t *testing.T) {
	styles := NewStyles(DefaultPalette)
	m := newTaskCreateModel(nil, nil, styles)

	next, _ := m.Update(projectsLoadedMsg{
		projects: []domain.Project{{Name: "Inbox", Slug: "inbox"}},
	})
	if next.step != createStepTitle {
		t.Errorf("expected createStepTitle, got %d", next.step)
	}
}

func TestTaskCreateModal_MultipleProjectsLandsOnPicker(t *testing.T) {
	styles := NewStyles(DefaultPalette)
	m := newTaskCreateModel(nil, nil, styles)

	next, _ := m.Update(projectsLoadedMsg{
		projects: []domain.Project{{Name: "A", Slug: "a"}, {Name: "B", Slug: "b"}},
	})
	if next.step != createStepProject {
		t.Errorf("expected createStepProject, got %d", next.step)
	}
}
