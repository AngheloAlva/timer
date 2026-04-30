package cli

import (
	"github.com/AngheloAlva/timer/internal/version"
	"github.com/spf13/cobra"
)

// NewRootCmd builds the top-level `timer` command.
// Subcommands (start, stop, project, task, mcp, tui, ...) get attached here
// as the project grows.
func NewRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "timer",
		Short: "Local-first time tracker (CLI + TUI + MCP)",
		Long: `Timer is a local-first time tracker. Everything lives in a single
SQLite file under ~/.local/share/timer/timer.db; no server, no account.

Quick start:
  timer init                            # create DB + seed Inbox project
  timer project add "My Project"
  timer task add my-project "Fix login"
  timer task list                       # copy the 8-char task id
  timer start <task-id>                 # start tracking
  timer stop <task-id>                  # stop and store a time entry
  timer log --today                     # see what you logged today
  timer report --week                   # totals grouped by project / task

Override the DB path with TIMER_DB_PATH (useful for tests / sandboxes).`,
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version.Version,
	}

	cmd.AddCommand(
		newVersionCmd(),
		newInitCmd(),
		newProjectCmd(),
		newTaskCmd(),
		newStartCmd(),
		newStopCmd(),
		newPauseCmd(),
		newResumeCmd(),
		newListCmd(),
		newLogCmd(),
		newReportCmd(),
		newTUICmd(),
		newMCPCmd(),
		newUpdateCmd(),
	)

	return cmd
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Printf("timer %s (commit %s, built %s)\n",
				version.Version, version.Commit, version.Date)
			return nil
		},
	}
}
