package cli

import (
	"fmt"
	"os"

	mcpserver "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/AngheloAlva/timer/internal/mcp"
)

// newMCPCmd returns the `timer mcp` subcommand: starts a Model Context
// Protocol server over stdio so AI agents (Claude Code, etc.) can drive
// the timer.
//
// Schema guard: refuses to start if the SQLite DB does not exist yet.
// Running migrations belongs to the CLI (`timer init`), not to a server
// launched by an external client. Failing fast gives the user a clear
// next step instead of silently creating a fresh DB inside a stdio session.
func newMCPCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Start the MCP server (stdio) for AI agents",
		Long: `Starts a Model Context Protocol server over stdio. Designed to be
launched by an MCP client like Claude Code.

Configure your client with:

  {
    "mcpServers": {
      "timer": { "command": "timer", "args": ["mcp"] }
    }
  }

The MCP server requires an existing timer database. If you have not run
'timer init' yet, do that first.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dbPath, err := resolveDBPath()
			if err != nil {
				return fmt.Errorf("resolve db path: %w", err)
			}
			if _, err := os.Stat(dbPath); err != nil {
				if os.IsNotExist(err) {
					return fmt.Errorf("no timer database at %s — run `timer init` first", dbPath)
				}
				return fmt.Errorf("stat db: %w", err)
			}

			app, err := openApp()
			if err != nil {
				return err
			}
			defer func() { _ = app.Close() }()

			srv := mcp.NewServer(app.ProjectSvc, app.TaskSvc, app.TimerSvc)
			return mcpserver.ServeStdio(srv)
		},
	}
}
