package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/service"
)

type taskCreateModel struct {
	projectSvc *service.ProjectService
	taskSvc    *service.TaskService
	styles     Styles

	step       createStep
	projects   []domain.Project
	projectIdx int

	titleInput textinput.Model
	loadErr    error
	submitting bool
}

type createStep int

const (
	createStepLoading createStep = iota
	createStepProject
	createStepTitle
)

func newTaskCreateModel(projectSvc *service.ProjectService, taskSvc *service.TaskService, styles Styles) taskCreateModel {
	ti := textinput.New()
	ti.Placeholder = "Task title"
	ti.CharLimit = 200
	ti.Width = 40

	return taskCreateModel{
		projectSvc: projectSvc,
		taskSvc:    taskSvc,
		styles:     styles,
		step:       createStepLoading,
		titleInput: ti,
	}
}

type projectsLoadedMsg struct {
	projects []domain.Project
	err      error
}

type taskCreatedMsg struct {
	task domain.Task
}

type modalCancelMsg struct{}

func loadProjectsCmd(svc *service.ProjectService) tea.Cmd {
	return func() tea.Msg {
		ps, err := svc.List(context.Background(), false)
		return projectsLoadedMsg{projects: ps, err: err}
	}
}

func createTaskCmd(svc *service.TaskService, projectSlug, title string) tea.Cmd {
	return func() tea.Msg {
		t, err := svc.Create(context.Background(), projectSlug, title)
		if err != nil {
			return actionMsg{status: "Create failed: " + err.Error(), isErr: true}
		}
		return taskCreatedMsg{task: t}
	}
}

func (m taskCreateModel) Init() tea.Cmd {
	return loadProjectsCmd(m.projectSvc)
}

func (m taskCreateModel) Update(msg tea.Msg) (taskCreateModel, tea.Cmd) {
	switch msg := msg.(type) {

	case projectsLoadedMsg:
		m.loadErr = msg.err
		if msg.err != nil {
			return m, nil
		}
		m.projects = msg.projects
		if len(msg.projects) == 0 {
			m.loadErr = errors.New("no projects available — create one first via 'timer project add'")
			return m, nil
		}
		// Single-project shortcut: jump straight to typing the title.
		if len(msg.projects) == 1 {
			m.step = createStepTitle
			m.titleInput.Focus()
			return m, textinput.Blink
		}
		m.step = createStepProject
		return m, nil

	case tea.KeyMsg:
		switch m.step {
		case createStepLoading:
			if msg.String() == "esc" {
				return m, sendModalCancel()
			}
			return m, nil

		case createStepProject:
			switch msg.String() {
			case "esc":
				return m, sendModalCancel()
			case "j", "down":
				if m.projectIdx < len(m.projects)-1 {
					m.projectIdx++
				}
			case "k", "up":
				if m.projectIdx > 0 {
					m.projectIdx--
				}
			case "enter", "tab":
				m.step = createStepTitle
				m.titleInput.Focus()
				return m, textinput.Blink
			}
			return m, nil

		case createStepTitle:
			switch msg.String() {
			case "esc":
				return m, sendModalCancel()
			case "shift+tab":
				if len(m.projects) > 1 {
					m.titleInput.Blur()
					m.step = createStepProject
				}
				return m, nil
			case "enter":
				title := strings.TrimSpace(m.titleInput.Value())
				if title == "" {
					return m, nil
				}
				if m.submitting {
					return m, nil
				}
				m.submitting = true
				return m, createTaskCmd(m.taskSvc, m.projects[m.projectIdx].Slug, title)
			}
		}
	}

	if m.step == createStepTitle {
		var cmd tea.Cmd
		m.titleInput, cmd = m.titleInput.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m taskCreateModel) View() string {
	header := m.styles.Bold.Render("New task")

	var body string
	switch m.step {
	case createStepLoading:
		if m.loadErr != nil {
			body = m.styles.StateBad.Render("Error: " + m.loadErr.Error())
		} else {
			body = m.styles.Hint.Render("Loading projects...")
		}

	case createStepProject:
		var b strings.Builder
		b.WriteString(m.styles.Hint.Render("Pick a project — j/k to move, Enter/Tab to confirm"))
		b.WriteString("\n\n")
		for i, p := range m.projects {
			cursor := "  "
			label := fmt.Sprintf("%s (%s)", p.Name, p.Slug)
			if i == m.projectIdx {
				cursor = m.styles.Key.Render("▸ ")
				label = m.styles.Bold.Render(label)
			}
			b.WriteString(cursor + label + "\n")
		}
		body = b.String()

	case createStepTitle:
		var b strings.Builder
		b.WriteString(m.styles.Hint.Render(
			fmt.Sprintf("Project: %s", m.projects[m.projectIdx].Slug)))
		b.WriteString("\n\n")
		b.WriteString(m.titleInput.View())
		if m.submitting {
			b.WriteString("\n")
			b.WriteString(m.styles.Hint.Render("Saving..."))
		}
		body = b.String()
	}

	return m.styles.Panel.Render(header + "\n\n" + body)
}

func (m taskCreateModel) FooterHints() []string {
	switch m.step {
	case createStepProject:
		return []string{
			m.styles.Key.Render("j/k") + m.styles.KeyDesc.Render(" move"),
			m.styles.Key.Render("Enter") + m.styles.KeyDesc.Render(" pick"),
			m.styles.Key.Render("Esc") + m.styles.KeyDesc.Render(" cancel"),
		}
	case createStepTitle:
		hints := []string{
			m.styles.Key.Render("Enter") + m.styles.KeyDesc.Render(" save"),
			m.styles.Key.Render("Esc") + m.styles.KeyDesc.Render(" cancel"),
		}
		if len(m.projects) > 1 {
			hints = append([]string{
				m.styles.Key.Render("Shift+Tab") + m.styles.KeyDesc.Render(" back"),
			}, hints...)
		}
		return hints
	default:
		return []string{
			m.styles.Key.Render("Esc") + m.styles.KeyDesc.Render(" cancel"),
		}
	}
}

func sendModalCancel() tea.Cmd {
	return func() tea.Msg { return modalCancelMsg{} }
}
