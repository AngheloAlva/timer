package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/AngheloAlva/timer/internal/service"
)

type view int

const (
	viewDashboard view = iota
	viewTasks
	viewReports
)

const numViews = 3

type AppModel struct {
	styles  Styles
	current view

	dashboard dashboardModel
	tasks     tasksModel
	reports   reportsModel

	width, height int
	quitting      bool
}

func NewApp(projectSvc *service.ProjectService, taskSvc *service.TaskService, timerSvc *service.TimerService) AppModel {
	styles := NewStyles(DefaultPalette)
	return AppModel{
		styles:    styles,
		current:   viewDashboard,
		dashboard: newDashboardModel(timerSvc, styles),
		tasks:     newTasksModel(taskSvc, timerSvc, projectSvc, styles),
		reports:   newReportsModel(timerSvc, styles),
	}
}

func (m AppModel) Init() tea.Cmd {
	// Pre-init all subviews so switching is instant.
	return tea.Batch(m.dashboard.Init(), m.tasks.Init(), m.reports.Init())
}

func (m AppModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		// Skip global nav keys when a modal owns input.
		if !m.activeIsCapturing() {
			switch msg.String() {
			case "q":
				m.quitting = true
				return m, tea.Quit
			case "tab":
				m.current = (m.current + 1) % numViews
				return m, nil
			case "1":
				m.current = viewDashboard
				return m, nil
			case "2":
				m.current = viewTasks
				return m, nil
			case "3":
				m.current = viewReports
				return m, nil
			}
		}
	}

	// KeyMsg only reaches the active view (so j/k/enter don't trigger
	// background views). Everything else (load results, ticks, window
	// resizes) broadcasts to all three so their async chains stay alive.
	if _, isKey := msg.(tea.KeyMsg); isKey {
		var cmd tea.Cmd
		switch m.current {
		case viewDashboard:
			m.dashboard, cmd = m.dashboard.Update(msg)
		case viewTasks:
			m.tasks, cmd = m.tasks.Update(msg)
		case viewReports:
			m.reports, cmd = m.reports.Update(msg)
		}
		return m, cmd
	}

	var cmds []tea.Cmd
	var cmd tea.Cmd
	m.dashboard, cmd = m.dashboard.Update(msg)
	cmds = append(cmds, cmd)
	m.tasks, cmd = m.tasks.Update(msg)
	cmds = append(cmds, cmd)
	m.reports, cmd = m.reports.Update(msg)
	cmds = append(cmds, cmd)
	return m, tea.Batch(cmds...)
}

func (m AppModel) View() string {
	if m.quitting {
		return ""
	}

	var b strings.Builder
	b.WriteString(m.styles.Title.Render("⏱  Timer"))
	b.WriteString("   ")
	b.WriteString(m.tabs())
	b.WriteString("\n\n")

	switch m.current {
	case viewDashboard:
		b.WriteString(m.dashboard.View())
	case viewTasks:
		b.WriteString(m.tasks.View())
	case viewReports:
		b.WriteString(m.reports.View())
	}

	b.WriteString("\n")
	b.WriteString(m.footer())
	b.WriteString("\n")
	return b.String()
}

func (m AppModel) tabs() string {
	tab := func(label string, active bool) string {
		if active {
			return m.styles.Bold.Render(label)
		}
		return m.styles.Muted.Render(label)
	}
	return tab("[1] Dashboard", m.current == viewDashboard) + "  " +
		tab("[2] Tasks", m.current == viewTasks) + "  " +
		tab("[3] Reports", m.current == viewReports)
}

func (m AppModel) footer() string {
	var keys []string
	switch m.current {
	case viewDashboard:
		keys = []string{}
	case viewTasks:
		keys = m.tasks.FooterHints()
	case viewReports:
		keys = m.reports.FooterHints()
	}

	if !m.activeIsCapturing() {
		keys = append(keys,
			m.styles.Key.Render("Tab")+m.styles.KeyDesc.Render(" switch"),
			m.styles.Key.Render("q")+m.styles.KeyDesc.Render(" quit"),
		)
	}
	return m.styles.Footer.Render(strings.Join(keys, "   "))
}

func (m AppModel) activeIsCapturing() bool {
	switch m.current {
	case viewTasks:
		return m.tasks.IsInputCapturing()
	default:
		return false
	}
}
