package cli

import (
	"github.com/spf13/cobra"
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
	cmd.AddCommand(newProjectAddCmd(), newProjectListCmd())
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
			defer app.Close()

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
			defer app.Close()

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
