package cli

import (
	"errors"
	"time"

	"github.com/spf13/cobra"

	"github.com/AngheloAlva/timer/internal/format"
	"github.com/AngheloAlva/timer/internal/service"
)

func newReportCmd() *cobra.Command {
	var (
		showToday   bool
		showWeek    bool
		projectSlug string
	)

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Show totals grouped by project and task",
		Long: `Produce a totals breakdown for a time range. Requires either
--today or --week. Optional --project narrows to a single project.
Entries are aggregated in Go and sorted by total duration descending,
both at the project and task level.`,
		Example: `  timer report --today
  timer report --week
  timer report --week --project timer-cli`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if !showToday && !showWeek {
				return errors.New("specify a range: --today or --week")
			}

			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			now := time.Now()
			var (
				rangeLabel string
				since      time.Time
			)
			switch {
			case showToday:
				since = service.StartOfDay(now)
				rangeLabel = "Today (" + since.Format("2006-01-02") + ")"
			case showWeek:
				since = service.StartOfISOWeek(now)
				rangeLabel = "Week of " + since.Format("2006-01-02") + " (Mon)"
			}

			summary, err := app.TimerSvc.BuildReport(cmd.Context(), service.ListEntriesOpts{
				MinStartedAt: since,
				ProjectSlug:  projectSlug,
			})
			if err != nil {
				return err
			}

			cmd.Printf("%s\n", rangeLabel)
			if len(summary.Projects) == 0 {
				cmd.Println("  No entries.")
				return nil
			}

			cmd.Printf("Total: %s\n\n", format.Duration(summary.Total))
			for _, p := range summary.Projects {
				cmd.Printf("  %-30s %s\n", p.Name+" ("+p.Slug+")", format.Duration(p.Total))
				for _, t := range p.Tasks {
					cmd.Printf("    - %-26s %s\n", t.Title, format.Duration(t.Total))
				}
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&showToday, "today", false, "totals for today")
	cmd.Flags().BoolVar(&showWeek, "week", false, "totals for this ISO week (Mon-Sun)")
	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "filter by project slug")
	cmd.MarkFlagsMutuallyExclusive("today", "week")
	return cmd
}
