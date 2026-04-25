package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/AngheloAlva/timer/internal/format"
	"github.com/AngheloAlva/timer/internal/service"
)

type reportRange int

const (
	rangeToday reportRange = iota
	rangeWeek
)

func (r reportRange) String() string {
	if r == rangeToday {
		return "today"
	}
	return "week"
}

type reportsModel struct {
	timerSvc *service.TimerService
	styles   Styles

	rng         reportRange
	summary     service.ReportSummary
	rangeLabel  string
	loadErr     error
	loading     bool
	initialized bool
}

func newReportsModel(timerSvc *service.TimerService, styles Styles) reportsModel {
	return reportsModel{
		timerSvc: timerSvc,
		styles:   styles,
		rng:      rangeToday,
		loading:  true,
	}
}

type reportLoadedMsg struct {
	rng     reportRange
	summary service.ReportSummary
	label   string
	err     error
}

func loadReportCmd(svc *service.TimerService, rng reportRange) tea.Cmd {
	return func() tea.Msg {
		now := time.Now()
		var (
			since time.Time
			label string
		)
		switch rng {
		case rangeToday:
			since = service.StartOfDay(now)
			label = "Today (" + since.Format("2006-01-02") + ")"
		case rangeWeek:
			since = service.StartOfISOWeek(now)
			label = "Week of " + since.Format("2006-01-02") + " (Mon)"
		}

		summary, err := svc.BuildReport(context.Background(), service.ListEntriesOpts{
			MinStartedAt: since,
		})
		return reportLoadedMsg{rng: rng, summary: summary, label: label, err: err}
	}
}

func (m reportsModel) Init() tea.Cmd {
	return loadReportCmd(m.timerSvc, m.rng)
}

func (m reportsModel) Update(msg tea.Msg) (reportsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case reportLoadedMsg:
		// Drop stale loads (user toggled mid-flight).
		if msg.rng != m.rng {
			return m, nil
		}
		m.initialized = true
		m.loading = false
		m.loadErr = msg.err
		if msg.err == nil {
			m.summary = msg.summary
			m.rangeLabel = msg.label
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "t":
			if m.rng != rangeToday {
				m.rng = rangeToday
				m.loading = true
				return m, loadReportCmd(m.timerSvc, m.rng)
			}
		case "w":
			if m.rng != rangeWeek {
				m.rng = rangeWeek
				m.loading = true
				return m, loadReportCmd(m.timerSvc, m.rng)
			}
		case "r":
			m.loading = true
			return m, loadReportCmd(m.timerSvc, m.rng)
		}
	}
	return m, nil
}

func (m reportsModel) View() string {
	var b strings.Builder

	header := "Reports — " + m.rangeTab(rangeToday, "Today") + "  " + m.rangeTab(rangeWeek, "Week")
	b.WriteString(m.styles.Section.Render(header))
	b.WriteString("\n")

	switch {
	case m.loadErr != nil:
		b.WriteString(m.styles.StateBad.Render("Error: " + m.loadErr.Error()))
		b.WriteString("\n")
		return b.String()
	case m.loading || !m.initialized:
		b.WriteString(m.styles.Hint.Render("Loading..."))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(m.styles.Muted.Render(m.rangeLabel))
	b.WriteString("\n")

	if len(m.summary.Projects) == 0 {
		b.WriteString(m.styles.Hint.Render("  No entries in this range."))
		b.WriteString("\n")
		return b.String()
	}

	b.WriteString(fmt.Sprintf("Total: %s\n\n", m.styles.Bold.Render(format.Duration(m.summary.Total))))
	for _, p := range m.summary.Projects {
		b.WriteString(fmt.Sprintf("  %s  %s\n",
			m.styles.Bold.Render(fmt.Sprintf("%-28s", p.Name+" ("+p.Slug+")")),
			m.styles.StateOK.Render(format.Duration(p.Total)),
		))
		for _, t := range p.Tasks {
			b.WriteString(fmt.Sprintf("    %s  %s\n",
				fmt.Sprintf("%-30s", "- "+t.Title),
				m.styles.Muted.Render(format.Duration(t.Total)),
			))
		}
	}

	return b.String()
}

func (m reportsModel) rangeTab(r reportRange, label string) string {
	if m.rng == r {
		return m.styles.Bold.Render("[" + label + "]")
	}
	return m.styles.Muted.Render(" " + label + " ")
}

func (m reportsModel) FooterHints() []string {
	return []string{
		m.styles.Key.Render("t") + m.styles.KeyDesc.Render(" today"),
		m.styles.Key.Render("w") + m.styles.KeyDesc.Render(" week"),
		m.styles.Key.Render("r") + m.styles.KeyDesc.Render(" reload"),
	}
}
