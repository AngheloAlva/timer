package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/format"
	"github.com/AngheloAlva/timer/internal/service"
)

type tasksModel struct {
	taskSvc    *service.TaskService
	timerSvc   *service.TimerService
	projectSvc *service.ProjectService
	styles     Styles

	tasks       []domain.Task
	activeByTID map[string]domain.Timer
	cursor      int
	includeDone bool
	loadErr     error
	statusMsg   string
	statusErr   bool
	initialized bool

	// At most one modal is non-nil at a time; while non-nil it owns input.
	creating *taskCreateModel
	editing  *taskEditModel
}

func newTasksModel(taskSvc *service.TaskService, timerSvc *service.TimerService, projectSvc *service.ProjectService, styles Styles) tasksModel {
	return tasksModel{
		taskSvc:     taskSvc,
		timerSvc:    timerSvc,
		projectSvc:  projectSvc,
		styles:      styles,
		activeByTID: map[string]domain.Timer{},
	}
}

func (m tasksModel) IsInputCapturing() bool { return m.creating != nil || m.editing != nil }

type tasksLoadedMsg struct {
	tasks  []domain.Task
	active map[string]domain.Timer
	err    error
}

type actionMsg struct {
	status string
	isErr  bool
}

func loadTasksCmd(taskSvc *service.TaskService, timerSvc *service.TimerService, includeDone bool) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		tasks, err := taskSvc.List(ctx, "", includeDone)
		if err != nil {
			return tasksLoadedMsg{err: err}
		}
		active, err := timerSvc.ListActive(ctx)
		if err != nil {
			return tasksLoadedMsg{err: err}
		}
		idx := make(map[string]domain.Timer, len(active))
		for _, t := range active {
			idx[t.TaskID] = t
		}
		return tasksLoadedMsg{tasks: tasks, active: idx}
	}
}

func startTimerCmd(svc *service.TimerService, taskID string) tea.Cmd {
	return func() tea.Msg {
		t, err := svc.Start(context.Background(), taskID)
		if err != nil {
			if errors.Is(err, domain.ErrTimerAlreadyRunning) {
				return actionMsg{status: "Already running for this task.", isErr: true}
			}
			return actionMsg{status: "Start failed: " + err.Error(), isErr: true}
		}
		return actionMsg{status: fmt.Sprintf("Started %s / %s", t.ProjectSlug, t.TaskTitle)}
	}
}

func stopTimerCmd(svc *service.TimerService, taskID string) tea.Cmd {
	return func() tea.Msg {
		entry, err := svc.Stop(context.Background(), taskID)
		if err != nil {
			return actionMsg{status: "Stop failed: " + err.Error(), isErr: true}
		}
		return actionMsg{status: fmt.Sprintf("Stopped → %s", format.Duration(entry.DurationSec))}
	}
}

// Pause; if already paused, resume.
func toggleTimerCmd(svc *service.TimerService, taskID string) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		_, err := svc.Pause(ctx, taskID)
		if err == nil {
			return actionMsg{status: "Paused."}
		}
		if errors.Is(err, domain.ErrTimerAlreadyPaused) {
			if _, err2 := svc.Resume(ctx, taskID); err2 != nil {
				return actionMsg{status: "Resume failed: " + err2.Error(), isErr: true}
			}
			return actionMsg{status: "Resumed."}
		}
		return actionMsg{status: "Pause failed: " + err.Error(), isErr: true}
	}
}

func markDoneCmd(svc *service.TaskService, taskID string) tea.Cmd {
	return func() tea.Msg {
		res, err := svc.MarkDone(context.Background(), taskID)
		if err != nil {
			return actionMsg{status: "Done failed: " + err.Error(), isErr: true}
		}
		if res.Entry != nil {
			return actionMsg{status: fmt.Sprintf("Done (closed timer → %s)", format.Duration(res.Entry.DurationSec))}
		}
		return actionMsg{status: "Done."}
	}
}

func (m tasksModel) Init() tea.Cmd {
	return loadTasksCmd(m.taskSvc, m.timerSvc, m.includeDone)
}

func (m tasksModel) Update(msg tea.Msg) (tasksModel, tea.Cmd) {
	if m.creating != nil {
		switch msg := msg.(type) {
		case modalCancelMsg:
			m.creating = nil
			return m, nil
		case taskCreatedMsg:
			m.creating = nil
			m.statusMsg = fmt.Sprintf("Created %q in %s", msg.task.Title, msg.task.ProjectSlug)
			m.statusErr = false
			return m, loadTasksCmd(m.taskSvc, m.timerSvc, m.includeDone)
		case actionMsg:
			m.statusMsg = msg.status
			m.statusErr = msg.isErr
			m.creating.submitting = false
			return m, nil
		}
		next, cmd := m.creating.Update(msg)
		m.creating = &next
		return m, cmd
	}

	if m.editing != nil {
		switch msg := msg.(type) {
		case modalCancelMsg:
			m.editing = nil
			return m, nil
		case taskEditedMsg:
			m.editing = nil
			m.statusMsg = fmt.Sprintf("Renamed → %q", msg.task.Title)
			m.statusErr = false
			return m, loadTasksCmd(m.taskSvc, m.timerSvc, m.includeDone)
		case actionMsg:
			m.statusMsg = msg.status
			m.statusErr = msg.isErr
			m.editing.submitting = false
			return m, nil
		}
		next, cmd := m.editing.Update(msg)
		m.editing = &next
		return m, cmd
	}

	switch msg := msg.(type) {

	case tasksLoadedMsg:
		m.initialized = true
		m.loadErr = msg.err
		if msg.err == nil {
			m.tasks = msg.tasks
			m.activeByTID = msg.active
			if m.cursor >= len(m.tasks) {
				m.cursor = max0(len(m.tasks) - 1)
			}
		}
		return m, nil

	case actionMsg:
		m.statusMsg = msg.status
		m.statusErr = msg.isErr
		return m, loadTasksCmd(m.taskSvc, m.timerSvc, m.includeDone)

	case tea.KeyMsg:
		if m.loadErr != nil {
			return m, nil
		}

		switch msg.String() {
		case "j", "down":
			if len(m.tasks) > 0 && m.cursor < len(m.tasks)-1 {
				m.cursor++
			}
			m.statusMsg = ""
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
			m.statusMsg = ""
		case "g":
			m.cursor = 0
		case "G":
			m.cursor = max0(len(m.tasks) - 1)
		case "a":
			m.includeDone = !m.includeDone
			return m, loadTasksCmd(m.taskSvc, m.timerSvc, m.includeDone)
		case "r":
			return m, loadTasksCmd(m.taskSvc, m.timerSvc, m.includeDone)
		case "enter":
			if t, ok := m.selectedTask(); ok {
				return m, startTimerCmd(m.timerSvc, t.ID)
			}
		case "s":
			if t, ok := m.selectedTask(); ok {
				return m, stopTimerCmd(m.timerSvc, t.ID)
			}
		case "p":
			if t, ok := m.selectedTask(); ok {
				return m, toggleTimerCmd(m.timerSvc, t.ID)
			}
		case "d":
			if t, ok := m.selectedTask(); ok {
				return m, markDoneCmd(m.taskSvc, t.ID)
			}
		case "n":
			modal := newTaskCreateModel(m.projectSvc, m.taskSvc, m.styles)
			m.creating = &modal
			return m, modal.Init()
		case "e":
			if t, ok := m.selectedTask(); ok {
				modal := newTaskEditModel(m.taskSvc, m.styles, t)
				m.editing = &modal
				return m, modal.Init()
			}
		}
	}

	return m, nil
}

func (m tasksModel) View() string {
	var b strings.Builder
	b.WriteString(m.styles.Section.Render("Tasks"))
	b.WriteString("\n")

	switch {
	case m.loadErr != nil:
		b.WriteString(m.styles.StateBad.Render("Error: " + m.loadErr.Error()))
		b.WriteString("\n")
	case !m.initialized:
		b.WriteString(m.styles.Hint.Render("Loading..."))
		b.WriteString("\n")
	case len(m.tasks) == 0:
		b.WriteString(m.styles.Hint.Render("No tasks. Create one with: timer task add <project-slug> <title>"))
		b.WriteString("\n")
	default:
		currentSlug := ""
		for i, t := range m.tasks {
			if t.ProjectSlug != currentSlug {
				if currentSlug != "" {
					b.WriteString("\n")
				}
				b.WriteString(m.styles.Bold.Render(t.ProjectName + " (" + t.ProjectSlug + ")"))
				b.WriteString("\n")
				currentSlug = t.ProjectSlug
			}
			b.WriteString(m.renderTaskRow(t, i == m.cursor))
			b.WriteString("\n")
		}
	}

	if m.statusMsg != "" {
		b.WriteString("\n")
		if m.statusErr {
			b.WriteString(m.styles.StateBad.Render("✗ " + m.statusMsg))
		} else {
			b.WriteString(m.styles.StateOK.Render("✓ " + m.statusMsg))
		}
		b.WriteString("\n")
	}

	if m.creating != nil {
		b.WriteString("\n")
		b.WriteString(m.creating.View())
		b.WriteString("\n")
	} else if m.editing != nil {
		b.WriteString("\n")
		b.WriteString(m.editing.View())
		b.WriteString("\n")
	}

	return b.String()
}

func (m tasksModel) FooterHints() []string {
	if m.creating != nil {
		return m.creating.FooterHints()
	}
	if m.editing != nil {
		return m.editing.FooterHints()
	}
	return []string{
		m.styles.Key.Render("j/k") + m.styles.KeyDesc.Render(" move"),
		m.styles.Key.Render("Enter") + m.styles.KeyDesc.Render(" start"),
		m.styles.Key.Render("s") + m.styles.KeyDesc.Render(" stop"),
		m.styles.Key.Render("p") + m.styles.KeyDesc.Render(" pause/resume"),
		m.styles.Key.Render("d") + m.styles.KeyDesc.Render(" done"),
		m.styles.Key.Render("n") + m.styles.KeyDesc.Render(" new"),
		m.styles.Key.Render("e") + m.styles.KeyDesc.Render(" edit"),
		m.styles.Key.Render("a") + m.styles.KeyDesc.Render(" toggle done"),
		m.styles.Key.Render("r") + m.styles.KeyDesc.Render(" reload"),
	}
}

func (m tasksModel) renderTaskRow(t domain.Task, selected bool) string {
	cursor := "  "
	if selected {
		cursor = m.styles.Key.Render("▸ ")
	}

	id := m.styles.Muted.Render(format.ShortID(t.ID))

	statusTag := m.statusBadge(t.Status)

	timerHint := ""
	if tm, ok := m.activeByTID[t.ID]; ok {
		if tm.IsPaused() {
			timerHint = "  " + m.styles.StateWarn.Render("⏸")
		} else {
			timerHint = "  " + m.styles.StateOK.Render("●")
		}
	}

	title := t.Title
	if selected {
		title = m.styles.Bold.Render(title)
	}

	return fmt.Sprintf("%s%s  %s  %s%s", cursor, id, statusTag, title, timerHint)
}

func (m tasksModel) statusBadge(s domain.TaskStatus) string {
	switch s {
	case domain.StatusInProgress:
		return m.styles.StateOK.Render("[in_progress]")
	case domain.StatusDone:
		return m.styles.Muted.Render("[done       ]")
	case domain.StatusArchived:
		return m.styles.Muted.Render("[archived   ]")
	default:
		return m.styles.KeyDesc.Render("[todo       ]")
	}
}

func (m tasksModel) selectedTask() (domain.Task, bool) {
	if m.cursor < 0 || m.cursor >= len(m.tasks) {
		return domain.Task{}, false
	}
	return m.tasks[m.cursor], true
}

func max0(n int) int {
	if n < 0 {
		return 0
	}
	return n
}
