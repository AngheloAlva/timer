package tui

import (
	"context"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/service"
)

type taskEditModel struct {
	taskSvc *service.TaskService
	styles  Styles

	taskID     string
	titleInput textinput.Model
	submitting bool
}

func newTaskEditModel(taskSvc *service.TaskService, styles Styles, task domain.Task) taskEditModel {
	ti := textinput.New()
	ti.Placeholder = "Task title"
	ti.CharLimit = 200
	ti.Width = 40
	ti.SetValue(task.Title)
	ti.CursorEnd()
	ti.Focus()

	return taskEditModel{
		taskSvc:    taskSvc,
		styles:     styles,
		taskID:     task.ID,
		titleInput: ti,
	}
}

type taskEditedMsg struct {
	task domain.Task
}

func updateTitleCmd(svc *service.TaskService, taskID, title string) tea.Cmd {
	return func() tea.Msg {
		t, err := svc.UpdateTitle(context.Background(), taskID, title)
		if err != nil {
			return actionMsg{status: "Update failed: " + err.Error(), isErr: true}
		}
		return taskEditedMsg{task: t}
	}
}

func (m taskEditModel) Init() tea.Cmd { return textinput.Blink }

func (m taskEditModel) Update(msg tea.Msg) (taskEditModel, tea.Cmd) {
	if km, ok := msg.(tea.KeyMsg); ok {
		switch km.String() {
		case "esc":
			return m, sendModalCancel()
		case "enter":
			title := strings.TrimSpace(m.titleInput.Value())
			if title == "" || m.submitting {
				return m, nil
			}
			m.submitting = true
			return m, updateTitleCmd(m.taskSvc, m.taskID, title)
		}
	}

	var cmd tea.Cmd
	m.titleInput, cmd = m.titleInput.Update(msg)
	return m, cmd
}

func (m taskEditModel) View() string {
	header := m.styles.Bold.Render("Edit task title")
	body := m.titleInput.View()
	if m.submitting {
		body += "\n" + m.styles.Hint.Render("Saving...")
	}
	return m.styles.Panel.Render(header + "\n\n" + body)
}

func (m taskEditModel) FooterHints() []string {
	return []string{
		m.styles.Key.Render("Enter") + m.styles.KeyDesc.Render(" save"),
		m.styles.Key.Render("Esc") + m.styles.KeyDesc.Render(" cancel"),
	}
}
