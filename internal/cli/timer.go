package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/service"
)

func newStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start <task-id-prefix>",
		Short: "Start a timer for a task",
		Long: `Start a timer on the given task. Resolves the prefix git-style.
If the task is in 'todo' status it flips to 'in_progress' atomically
with timer creation. Multiple tasks can be tracked in parallel — one
timer per task is the only restriction.`,
		Example: `  timer task list                 # find the task short id
  timer start aa86`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			t, err := app.TimerSvc.Start(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			cmd.Printf("Started %s  %s / %s  (task %s)\n",
				t.StartedAt.Local().Format("15:04:05"),
				t.ProjectSlug, t.TaskTitle, shortID(t.TaskID))
			return nil
		},
	}
}

func newStopCmd() *cobra.Command {
	var stopAll bool

	cmd := &cobra.Command{
		Use:   "stop [task-id-prefix]",
		Short: "Stop a running timer (writes a time entry)",
		Long: `Stop a running (or paused) timer. Writes a time entry with the
elapsed work duration (excluding paused periods) and removes the
timer atomically. Use --all to stop every active timer at once.`,
		Example: `  timer stop aa86
  timer stop --all`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !stopAll && len(args) == 0 {
				return errors.New("specify a task id prefix or use --all")
			}
			if stopAll && len(args) > 0 {
				return errors.New("cannot combine --all with a task id prefix")
			}

			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			if stopAll {
				entries, err := app.TimerSvc.StopAll(cmd.Context())
				if len(entries) == 0 && err == nil {
					cmd.Println("No active timers.")
					return nil
				}
				for _, e := range entries {
					cmd.Printf("Stopped %s / %s  → %s\n", e.ProjectSlug, e.TaskTitle, formatDuration(e.DurationSec))
				}
				return err
			}

			entry, err := app.TimerSvc.Stop(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			cmd.Printf("Stopped %s / %s  → %s\n", entry.ProjectSlug, entry.TaskTitle, formatDuration(entry.DurationSec))
			return nil
		},
	}

	cmd.Flags().BoolVar(&stopAll, "all", false, "stop every active timer")
	return cmd
}

func newPauseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pause <task-id-prefix>",
		Short: "Pause the active timer of a task",
		Long: `Pause an active timer. The current pause duration is tracked and
will be subtracted from the final entry duration when stopped. Re-pausing
an already-paused timer is a no-op (no error).`,
		Example: `  timer pause aa86`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			t, err := app.TimerSvc.Pause(cmd.Context(), args[0])
			if err != nil {
				if errors.Is(err, domain.ErrTimerAlreadyPaused) {
					cmd.Printf("Timer for %s / %s already paused.\n", t.ProjectSlug, t.TaskTitle)
					return nil
				}
				return err
			}
			cmd.Printf("Paused %s / %s\n", t.ProjectSlug, t.TaskTitle)
			return nil
		},
	}
}

func newResumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "resume <task-id-prefix>",
		Short:   "Resume the paused timer of a task",
		Long:    `Resume a paused timer. The pause duration accumulated so far is preserved (not counted as work). No-op message if the timer wasn't paused.`,
		Example: `  timer resume aa86`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			t, err := app.TimerSvc.Resume(cmd.Context(), args[0])
			if err != nil {
				if errors.Is(err, domain.ErrTimerNotPaused) {
					cmd.Printf("Timer for %s / %s is not paused.\n", t.ProjectSlug, t.TaskTitle)
					return nil
				}
				return err
			}
			cmd.Printf("Resumed %s / %s\n", t.ProjectSlug, t.TaskTitle)
			return nil
		},
	}
}

func newListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"status", "ls"},
		Short:   "List active timers",
		Long:    `Snapshot of every running and paused timer with computed elapsed time. Aliased to 'status' for the "what am I doing right now" mental model.`,
		Example: `  timer list
  timer status`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			timers, err := app.TimerSvc.ListActive(cmd.Context())
			if err != nil {
				return err
			}
			if len(timers) == 0 {
				cmd.Println("No active timers. Start one with: timer start <task-id-prefix>")
				return nil
			}

			now := time.Now()
			for _, t := range timers {
				state := "running"
				if t.IsPaused() {
					state = "paused "
				}
				cmd.Printf("  %s  [%s]  %s / %s  → %s (since %s)\n",
					shortID(t.TaskID),
					state,
					t.ProjectSlug,
					t.TaskTitle,
					formatDuration(t.ElapsedSec(now)),
					t.StartedAt.Local().Format("15:04"),
				)
			}
			return nil
		},
	}
}

func newLogCmd() *cobra.Command {
	var (
		maxRows     int
		showToday   bool
		showWeek    bool
		projectSlug string
	)

	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show recent time entries",
		Long: `List time entries (closed work segments) ordered newest-first.
Defaults to the last 50 entries across all projects. Use --today,
--week, --project, and -n to narrow.`,
		Example: `  timer log
  timer log --today
  timer log --week --project timer-cli
  timer log -n 5`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			opts := service.ListEntriesOpts{
				ProjectSlug: projectSlug,
				Max:         maxRows,
			}
			now := time.Now()
			switch {
			case showToday:
				opts.MinStartedAt = service.StartOfDay(now)
			case showWeek:
				opts.MinStartedAt = service.StartOfISOWeek(now)
			}

			entries, err := app.TimerSvc.ListEntries(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if len(entries) == 0 {
				cmd.Println("No time entries match.")
				return nil
			}

			var total int64
			for _, e := range entries {
				cmd.Printf("  %s  %s → %s  %s  %s / %s\n",
					e.StartedAt.Local().Format("2006-01-02"),
					e.StartedAt.Local().Format("15:04"),
					e.EndedAt.Local().Format("15:04"),
					formatDuration(e.DurationSec),
					e.ProjectSlug,
					e.TaskTitle,
				)
				total += e.DurationSec
			}
			cmd.Printf("\n  Total: %s across %d entries\n", formatDuration(total), len(entries))
			return nil
		},
	}

	cmd.Flags().IntVarP(&maxRows, "limit", "n", 50, "max entries to show")
	cmd.Flags().BoolVar(&showToday, "today", false, "only entries started today")
	cmd.Flags().BoolVar(&showWeek, "week", false, "only entries started this ISO week (Mon-Sun)")
	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "filter by project slug")
	cmd.MarkFlagsMutuallyExclusive("today", "week")
	return cmd
}

// formatDuration renders seconds as `Hh MMm SSs`, omitting leading zero units.
func formatDuration(sec int64) string {
	if sec < 0 {
		sec = 0
	}
	h := sec / 3600
	m := (sec % 3600) / 60
	s := sec % 60
	switch {
	case h > 0:
		return fmt.Sprintf("%dh %02dm %02ds", h, m, s)
	case m > 0:
		return fmt.Sprintf("%dm %02ds", m, s)
	default:
		return fmt.Sprintf("%ds", s)
	}
}
