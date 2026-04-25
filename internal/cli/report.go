package cli

import (
	"errors"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/AngheloAlva/timer/internal/domain"
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
			defer app.Close()

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

			entries, err := app.TimerSvc.ListEntries(cmd.Context(), service.ListEntriesOpts{
				MinStartedAt: since,
				ProjectSlug:  projectSlug,
				Max:          10000, // effectively unbounded for a week of work
			})
			if err != nil {
				return err
			}

			cmd.Printf("%s\n", rangeLabel)
			if len(entries) == 0 {
				cmd.Println("  No entries.")
				return nil
			}

			renderReport(cmd, entries)
			return nil
		},
	}

	cmd.Flags().BoolVar(&showToday, "today", false, "totals for today")
	cmd.Flags().BoolVar(&showWeek, "week", false, "totals for this ISO week (Mon-Sun)")
	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "filter by project slug")
	cmd.MarkFlagsMutuallyExclusive("today", "week")
	return cmd
}

// projectAgg holds the running totals for a single project, plus per-task
// breakdowns. We use a map keyed by taskID inside, which we sort before
// rendering.
type projectAgg struct {
	id    string
	name  string
	slug  string
	total int64
	tasks map[string]*taskAgg
}

type taskAgg struct {
	id    string
	title string
	total int64
}

func renderReport(cmd *cobra.Command, entries []domain.TimeEntry) {
	projects := map[string]*projectAgg{}
	var grand int64

	for _, e := range entries {
		p, ok := projects[e.ProjectID]
		if !ok {
			p = &projectAgg{
				id:    e.ProjectID,
				name:  e.ProjectName,
				slug:  e.ProjectSlug,
				tasks: map[string]*taskAgg{},
			}
			projects[e.ProjectID] = p
		}
		p.total += e.DurationSec

		t, ok := p.tasks[e.TaskID]
		if !ok {
			t = &taskAgg{id: e.TaskID, title: e.TaskTitle}
			p.tasks[e.TaskID] = t
		}
		t.total += e.DurationSec

		grand += e.DurationSec
	}

	cmd.Printf("Total: %s\n\n", formatDuration(grand))

	// Sort projects by total desc, tasks within by total desc.
	pAggs := make([]*projectAgg, 0, len(projects))
	for _, p := range projects {
		pAggs = append(pAggs, p)
	}
	sort.SliceStable(pAggs, func(i, j int) bool { return pAggs[i].total > pAggs[j].total })

	for _, p := range pAggs {
		cmd.Printf("  %-30s %s\n", p.name+" ("+p.slug+")", formatDuration(p.total))

		tAggs := make([]*taskAgg, 0, len(p.tasks))
		for _, t := range p.tasks {
			tAggs = append(tAggs, t)
		}
		sort.SliceStable(tAggs, func(i, j int) bool { return tAggs[i].total > tAggs[j].total })

		for _, t := range tAggs {
			cmd.Printf("    - %-26s %s\n", t.title, formatDuration(t.total))
		}
	}
}
