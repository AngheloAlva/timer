package cli

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/AngheloAlva/timer/internal/tui"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Open the interactive terminal UI",
		Long: `Launch the full-screen terminal UI. The dashboard shows active
timers with live elapsed counters, plus today's total. More views
(tasks, projects, reports) will land in subsequent iterations.

Quit with q, Esc, or Ctrl+C.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			model := tui.NewApp(app.ProjectSvc, app.TaskSvc, app.TimerSvc)
			p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(cmd.Context()))
			_, err = p.Run()
			return err
		},
	}
}
