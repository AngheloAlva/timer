package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/format"
	"github.com/AngheloAlva/timer/internal/service"
)

type dashboardModel struct {
	timerSvc *service.TimerService
	styles   Styles

	now         time.Time
	active      []domain.Timer
	todayTotal  int64
	loadErr     error
	width       int
	height      int
	initialized bool
}

func newDashboardModel(svc *service.TimerService, styles Styles) dashboardModel {
	return dashboardModel{
		timerSvc: svc,
		styles:   styles,
		now:      time.Now(),
	}
}

type tickMsg time.Time

type dataMsg struct {
	active     []domain.Timer
	todayTotal int64
	err        error
}

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func loadDashboardCmd(svc *service.TimerService) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		active, err := svc.ListActive(ctx)
		if err != nil {
			return dataMsg{err: err}
		}

		entries, err := svc.ListEntries(ctx, service.ListEntriesOpts{
			MinStartedAt: service.StartOfDay(time.Now()),
			Max:          10000,
		})
		if err != nil {
			return dataMsg{err: err}
		}

		var total int64
		now := time.Now()
		for _, e := range entries {
			total += e.DurationSec
		}
		for _, t := range active {
			total += t.ElapsedSec(now)
		}

		return dataMsg{active: active, todayTotal: total}
	}
}

func (m dashboardModel) Init() tea.Cmd {
	return tea.Batch(loadDashboardCmd(m.timerSvc), tickCmd())
}

func (m dashboardModel) Update(msg tea.Msg) (dashboardModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.now = time.Time(msg)
		// Periodic DB refresh to catch external mutations (CLI, MCP).
		if m.now.Second()%5 == 0 {
			return m, tea.Batch(tickCmd(), loadDashboardCmd(m.timerSvc))
		}
		return m, tickCmd()

	case dataMsg:
		m.initialized = true
		m.loadErr = msg.err
		if msg.err == nil {
			m.active = msg.active
			m.todayTotal = msg.todayTotal
		}
		return m, nil
	}
	return m, nil
}

func (m dashboardModel) View() string {
	var b strings.Builder

	b.WriteString(m.styles.Section.Render("Active timers"))
	b.WriteString("\n")

	if m.loadErr != nil {
		b.WriteString(m.styles.StateBad.Render("Error: " + m.loadErr.Error()))
		b.WriteString("\n")
	} else if !m.initialized {
		b.WriteString(m.styles.Hint.Render("Loading..."))
		b.WriteString("\n")
	} else if len(m.active) == 0 {
		b.WriteString(m.styles.Hint.Render("No active timers. Use the CLI to start one."))
		b.WriteString("\n")
	} else {
		for _, t := range m.active {
			b.WriteString(m.renderTimerRow(t))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	b.WriteString(m.styles.Section.Render("Today"))
	b.WriteString("\n")
	fmt.Fprintf(&b, "  Total tracked: %s\n",
		m.styles.Bold.Render(format.Duration(m.todayTotal)))

	return b.String()
}

func (m dashboardModel) renderTimerRow(t domain.Timer) string {
	state := m.styles.StateOK.Render("● running")
	if t.IsPaused() {
		state = m.styles.StateWarn.Render("⏸ paused ")
	}

	id := m.styles.Muted.Render(format.ShortID(t.TaskID))
	project := m.styles.Bold.Render(t.ProjectSlug)
	title := t.TaskTitle
	elapsed := m.styles.Bold.Render(format.Duration(t.ElapsedSec(m.now)))

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		"  ", state, "  ", id, "  ", project, " / ", title, "  → ", elapsed,
	)
}
