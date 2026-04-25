package cli

import (
	"github.com/spf13/cobra"

	"github.com/AngheloAlva/timer/internal/format"
)

// newProjectCmd builds the `timer project` command tree.
// The parent has no behavior of its own — it groups the subcommands.
func newProjectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "project",
		Aliases: []string{"projects"},
		Short:   "Manage projects",
		Long:    `Projects are the top-level grouping. Tasks belong to a project; time entries inherit it.`,
	}
	cmd.AddCommand(newProjectAddCmd(), newProjectListCmd(), newProjectArchiveCmd(), newProjectDeleteCmd())
	return cmd
}

func newProjectArchiveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "archive <slug>",
		Short: "Archive a project (soft, reversible)",
		Long: `Archive a project. The project disappears from the default 'list'
output but every task and time entry is preserved. If any task of the
project has a running timer, it is closed first (a time entry is written)
in the same transaction as the archive flip — same behavior as 'task done'.

To bring it back, set archived=0 directly in the DB (a future 'unarchive'
command will do this safely).`,
		Example: `  timer project archive timer-cli`,
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			res, err := app.ProjectSvc.Archive(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if res.AlreadyArchived {
				cmd.Printf("Project %q (%s) was already archived.\n", res.Project.Name, res.Project.Slug)
				return nil
			}
			cmd.Printf("Archived %q (%s)\n", res.Project.Name, res.Project.Slug)
			for _, e := range res.ClosedEntries {
				cmd.Printf("  (closed running timer on %s → %s)\n",
					e.TaskTitle, format.Duration(e.DurationSec))
			}
			return nil
		},
	}
}

func newProjectDeleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <slug>",
		Aliases: []string{"rm"},
		Short:   "Hard-delete a project (irreversible)",
		Long: `Delete a project AND every task, timer, and time entry under it.
This is irreversible. By default refuses unless the project is already
archived — use 'project archive' first, or pass --force to bypass and
accept the data loss.`,
		Example: `  timer project archive timer-cli && timer project delete timer-cli
  timer project delete timer-cli --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			res, err := app.ProjectSvc.Delete(cmd.Context(), args[0], force)
			if err != nil {
				return err
			}
			cmd.Printf("Deleted project %q (%s) — removed %d task(s), %d time entr(ies), %d active timer(s).\n",
				res.Project.Name, res.Project.Slug,
				res.TaskCount, res.TimeEntryCount, res.ActiveTimerCount)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "delete even if the project is not archived")
	return cmd
}

func newProjectAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name>",
		Short: "Create a new project",
		Long: `Create a project. The slug is derived from the name (lowercased,
spaces → dashes) and used as the handle in commands like 'task add' and
'log --project'.`,
		Example: `  timer project add "Timer CLI"     # → slug: timer-cli
  timer project add "Side Hustle"   # → slug: side-hustle`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			p, err := app.ProjectSvc.Create(cmd.Context(), args[0])
			if err != nil {
				return err
			}

			cmd.Printf("Created %q (slug: %s)\n", p.Name, p.Slug)
			return nil
		},
	}
}

func newProjectListCmd() *cobra.Command {
	var includeArchived bool

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List projects",
		Long:    `List active projects (use --all to also show archived ones).`,
		Example: `  timer project list
  timer project list --all`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			projects, err := app.ProjectSvc.List(cmd.Context(), includeArchived)
			if err != nil {
				return err
			}

			if len(projects) == 0 {
				cmd.Println(`No projects yet. Create one with: timer project add "My Project"`)
				return nil
			}

			for _, p := range projects {
				cmd.Printf("- %s (%s)\n", p.Name, p.Slug)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&includeArchived, "all", false, "include archived projects")
	return cmd
}
