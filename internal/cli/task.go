package cli

import (
	"github.com/spf13/cobra"

	"github.com/AngheloAlva/timer/internal/format"
)

// newTaskCmd builds the `timer task` command tree.
func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "task",
		Aliases: []string{"tasks"},
		Short:   "Manage tasks",
		Long:    `Tasks live inside a project. They have a status (todo, in_progress, done, archived) and an 8-char short id used by the timer commands.`,
	}
	cmd.AddCommand(newTaskAddCmd(), newTaskListCmd(), newTaskDoneCmd(), newTaskDeleteCmd())
	return cmd
}

func newTaskDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <task-id-prefix>",
		Aliases: []string{"rm"},
		Short:   "Hard-delete a task (irreversible)",
		Long: `Delete a task AND its active timer (if any) and every time entry
of the task. This is irreversible. By default refuses if the task has any
time entry or an active timer — pass --force to bypass and accept the data
loss. Tasks with no history can be deleted without --force.`,
		Example: `  timer task delete aa86             # only if the task has no history
  timer task delete aa86 --force     # nuke timer + entries`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			res, err := app.TaskSvc.Delete(cmd.Context(), args[0], force)
			if err != nil {
				return err
			}
			cmd.Printf("Deleted task %s  %q  — removed %d time entr(ies)",
				format.ShortID(res.Task.ID), res.Task.Title, res.TimeEntryCount)
			if res.HadActiveTimer {
				cmd.Print(", and 1 active timer")
			}
			cmd.Println(".")
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete even if the task has time entries or an active timer")
	return cmd
}

func newTaskAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <project-slug> <title>",
		Short: "Create a new task in a project",
		Long: `Create a new task in the given project. The task starts in 'todo'
status and gets a UUID — only the first 8 chars are shown and used in
subsequent commands.`,
		Example: `  timer task add timer-cli "Implement timers"
  timer task add inbox "Buy milk"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			t, err := app.TaskSvc.Create(cmd.Context(), args[0], args[1])
			if err != nil {
				return err
			}

			cmd.Printf("Created %q in %s (id: %s)\n", t.Title, t.ProjectSlug, format.ShortID(t.ID))
			return nil
		},
	}
}

func newTaskListCmd() *cobra.Command {
	var projectSlug string
	var includeAll bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List tasks (grouped by project)",
		Long: `List tasks grouped by project. By default hides 'done' and
'archived' tasks — use --all to include them.`,
		Example: `  timer task list
  timer task list --project timer-cli
  timer task list --all`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			tasks, err := app.TaskSvc.List(cmd.Context(), projectSlug, includeAll)
			if err != nil {
				return err
			}

			if len(tasks) == 0 {
				cmd.Println(`No tasks. Create one with: timer task add <project-slug> "My task"`)
				return nil
			}

			currentSlug := ""
			for _, t := range tasks {
				if t.ProjectSlug != currentSlug {
					if currentSlug != "" {
						cmd.Println()
					}
					cmd.Printf("%s (%s)\n", t.ProjectName, t.ProjectSlug)
					currentSlug = t.ProjectSlug
				}
				cmd.Printf("  %s  [%-11s]  %s\n", format.ShortID(t.ID), t.Status, t.Title)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&projectSlug, "project", "p", "", "filter by project slug")
	cmd.Flags().BoolVar(&includeAll, "all", false, "include done and archived tasks")
	return cmd
}

func newTaskDoneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "done <task-id-prefix>",
		Short: "Mark a task as done (closes any active timer first)",
		Long: `Mark a task as done. Resolves the prefix git-style: any unique
prefix of the task's UUID works. If the task has a running timer, it
is closed atomically (a time entry gets written) before the status
flip — so 'task done' is a one-shot "I'm finished here" command.`,
		Example: `  timer task done aa86            # 4 chars are usually enough
  timer task done aa866ddf        # the full short id from 'task list'`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			res, err := app.TaskSvc.MarkDone(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			cmd.Printf("Done: %s  %s\n", format.ShortID(res.Task.ID), res.Task.Title)
			if res.Entry != nil {
				cmd.Printf("  (closed running timer → %s)\n", format.Duration(res.Entry.DurationSec))
			}
			return nil
		},
	}
}
