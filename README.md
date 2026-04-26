# timer

Local-first time tracker with CLI, TUI and MCP. Single Go binary, SQLite under the hood.

> Status: Phases 1 (CLI), 2 (TUI) and 3 (MCP) shipped. See [`docs/`](./docs/) for the full plan.

## Install

### Homebrew (macOS / Linux)

```bash
brew install AngheloAlva/tap/timer
timer init
```

> The fully-qualified `AngheloAlva/tap/timer` is required to avoid a name
> collision with a deprecated `timer` formula still listed in homebrew-core.
> Once that ghost is removed upstream, `brew install timer` will work too.

### Manual download

Grab the tarball for your OS/arch from
[GitHub Releases](https://github.com/AngheloAlva/timer/releases), extract,
move `timer` into your `PATH`, then run `timer init`.

On macOS, binaries are not signed. Drop the quarantine flag once after
installing manually:

```bash
xattr -d com.apple.quarantine /usr/local/bin/timer
```

(Not needed when installing via Homebrew — brew strips the flag for you.)

### From source

```bash
git clone https://github.com/AngheloAlva/timer
cd timer
make build              # produces ./bin/timer
./bin/timer init        # creates ~/.local/share/timer/timer.db
```

## CLI quick start

```bash
timer project add "API Backend"
timer task add api-backend "Implementar login"
timer task list                       # copy the 8-char task id
timer start <task-id>                 # start tracking
timer stop  <task-id>                 # stop and store a time entry
timer log --today                     # what you logged today
timer report --week                   # totals grouped by project / task
```

Override the DB path with `TIMER_DB_PATH` (useful for tests / sandboxes).

## TUI

```bash
timer tui
```

Three views (Dashboard / Tasks / Reports), keybindings shown inside.

## MCP server (Claude Code, Cursor, etc.)

`timer mcp` starts a Model Context Protocol server over stdio. Wire it into
your client by pointing at the absolute path of the binary:

```json
{
  "mcpServers": {
    "timer": {
      "command": "/usr/local/bin/timer",
      "args": ["mcp"]
    }
  }
}
```

Drop that into your client's MCP config (`~/.claude/mcp.json`,
`./.mcp.json` for project-scoped, etc.) and restart the client.

The server requires an existing database — run `timer init` first. It
will refuse to start with a clear error if no DB exists at the resolved
path, so a missing `timer init` cannot accidentally create an empty DB
inside an MCP session.

### Tools

Timer lifecycle (every tool supports the multi-timer model — there can be
multiple active timers across different tasks; tools accept `taskTitle`
substring + `projectSlug` filters to disambiguate):

| Tool | What it does |
|------|--------------|
| `start_timer` | Start a timer. Accepts `taskId`, OR `projectSlug + taskTitle`, OR just `taskTitle` (auto-detects `projectSlug` from the MCP process cwd via `git rev-parse` / basename). Creates the task on the fly when the title is new. |
| `stop_timer` | Stop a timer. Pass `all: true` to stop every active one. |
| `pause_timer` / `resume_timer` | Pause without closing / unpause. |
| `active_timer` | List running + paused timers (single-detail or compact list). |
| `switch_task` | Stop everything and start one in a new task. |

Tasks and projects:

| Tool | What it does |
|------|--------------|
| `create_task` | Create a task in a project. |
| `list_tasks` | List tasks. Optional `projectSlug` and `status` (`todo` / `in_progress` / `done` / `archived`). |
| `create_project` | Create a project (slug auto-derived from name). |
| `list_projects` | List projects. Marks the one matching the cwd auto-detect as "(este proyecto)". |

Reporting:

| Tool | What it does |
|------|--------------|
| `log_time` | Manual retroactive entry: same task arg shape as `start_timer`, plus `startedAt` / `endedAt` (ISO 8601). |
| `get_summary` | Totals grouped by project. `range` ∈ `today` / `yesterday` / `this_week` / `last_week` / `custom` (with `from` + `to`). Optional `projectSlug` filter. |

### Resources

JSON snapshots that an agent can pull as ambient context:

| URI | Contents |
|-----|----------|
| `timer://active-timers` | All running and paused timers (incl. elapsed seconds, paused flag). |
| `timer://today` | Aggregated time for today, broken down by project and task. |
| `timer://projects` | Every project (including archived). |

## Develop

```bash
make dev        # run the CLI
make test       # unit + integration tests
make build      # produce ./bin/timer
make lint       # golangci-lint
```

The MCP integration tests in [`internal/mcp/server_test.go`](./internal/mcp/server_test.go)
use mcp-go's in-process client — no real stdio, no subprocess, runs in milliseconds.

## License

MIT
