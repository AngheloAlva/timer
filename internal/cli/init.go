package cli

import (
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Create the data directory and seed the default Inbox project",
		Long: `Initialize the timer database explicitly.

The DB is also created on first use of any command, but running 'init'
gives you a clear path to where the data lives and confirms the default
'Inbox' project was seeded. Useful when setting up a new machine or
verifying that everything is wired correctly.`,
		Example: `  timer init
  TIMER_DB_PATH=/tmp/sandbox.db timer init`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := resolveDBPath()
			if err != nil {
				return err
			}

			app, err := openApp()
			if err != nil {
				return err
			}
			defer app.Close()

			cmd.Printf("Database ready at %s\n", path)
			if app.JustSeeded {
				cmd.Println(`Seeded default project "Inbox".`)
			} else {
				cmd.Println("Existing data preserved (no seed needed).")
			}
			return nil
		},
	}
}
