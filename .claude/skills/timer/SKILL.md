---
name: timer
description: >
  Conventions for the timer project (Go + Cobra CLI + mcp-go MCP server + SQLite/sqlc).
  Trigger: load when the user mentions "timer", "timer cli", "timer mcp", or edits files under /Users/anghelo/Dev/timer.
license: Apache-2.0
metadata:
  author: gentleman-programming
  version: "1.0"
---

## When to Use

- Adding a new CLI command, MCP tool, service method, domain type, or SQL query in the timer repo.
- Touching anything under `internal/{domain,service,storage,cli,mcp,format}/`.
- Adding migrations or regenerating sqlc code.
- DO NOT load for: TUI work (`internal/tui/`), tests (out of scope), or release tooling.

## Stack (verified)

- Module: `github.com/AngheloAlva/timer` · Go **1.26**
- CLI: `spf13/cobra` v1.10
- MCP: `github.com/mark3labs/mcp-go` v0.49
- Storage: `modernc.org/sqlite` (pure Go) + `sqlc` + `golang-migrate/v4` (embedded via `//go:embed`)

## Architecture (hexagonal — strict)

```
cmd/timer/main.go         entrypoint — wraps errors with "Error: " prefix, exit 1
internal/domain/          pure value types + sentinel errors. ONLY stdlib imports.
internal/service/         business logic. Holds *sql.DB + *gen.Queries. Constructor DI.
internal/storage/sqlite/  DB wrapper, pragmas, migrations, query files
internal/storage/gen/     sqlc-generated code (package gen) — DO NOT edit by hand
internal/cli/             Cobra commands. newXxxCmd() factories. openApp()+Close().
internal/mcp/             server.go wires services into mcp-go
internal/mcp/tools/       MCP tool registration + handlers per domain
internal/mcp/resources/   MCP resources
internal/format/          shared formatting helpers (Duration, ShortID)
internal/version/         build-time version vars
```

## File-Path Map: where new code goes

| Adding...                | Location                                                |
|--------------------------|---------------------------------------------------------|
| Domain type / errors     | `internal/domain/<name>.go`                             |
| SQL query                | `internal/storage/sqlite/queries/<name>.sql`            |
| Schema change            | `internal/storage/sqlite/migrations/NNN_<desc>.up.sql` (+ `.down.sql`) |
| Service                  | `internal/service/<name>_service.go`                    |
| CLI command              | `internal/cli/<name>.go` + register in `internal/cli/root.go` |
| MCP tool                 | `internal/mcp/tools/<name>.go` + register in `internal/mcp/server.go` |
| Output formatting helper | `internal/format/format.go` (only if reused) or local file |

## End-to-End Recipe (canonical order)

1. **Domain** — add value type + sentinel errors in `internal/domain/<name>.go`.
2. **SQL** — add queries in `internal/storage/sqlite/queries/<name>.sql`.
3. **Migration** (only if schema changes) — `migrations/NNN_<desc>.up.sql` + `.down.sql`.
4. **Generate** — run `sqlc generate` (regenerates `internal/storage/gen/`).
5. **Service** — `internal/service/<name>_service.go` with constructor + methods.
6. **Wire service** — add field + `service.NewXxxService(...)` in `internal/cli/context.go` (`openApp`).
7. **CLI** — `internal/cli/<name>.go`, register in `root.go` `AddCommand(...)`.
8. **MCP** — `internal/mcp/tools/<name>.go` exposing `RegisterXxxTools(s, svc)`, register in `internal/mcp/server.go`.

## Code Templates

### Domain (`internal/domain/foo.go`)

```go
package domain

import (
    "errors"
    "time"
)

type Foo struct {
    ID        string
    Name      string
    CreatedAt time.Time
}

// Value receiver — domain types are immutable.
func (f Foo) IsNamed() bool { return f.Name != "" }

var (
    ErrFooNotFound = errors.New("foo not found")
    ErrFooExists   = errors.New("foo already exists")
)
```

### sqlc query (`internal/storage/sqlite/queries/foos.sql`)

```sql
-- name: CreateFoo :exec
INSERT INTO foos (id, name, created_at)
VALUES (sqlc.arg(id), sqlc.arg(name), sqlc.arg(created_at));

-- name: GetFoo :one
SELECT * FROM foos WHERE id = sqlc.arg(id) LIMIT 1;

-- name: ListFoos :many
SELECT * FROM foos ORDER BY created_at DESC;
```

Annotations: `:exec` (no return), `:one` (single row), `:many` (slice). Always `sqlc.arg(name)`.

### Migration (`internal/storage/sqlite/migrations/00X_add_foos.up.sql`)

```sql
CREATE TABLE foos (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    created_at INTEGER NOT NULL
);
CREATE INDEX idx_foos_name ON foos(name);
```

Pair with a `.down.sql` that reverses it. Migrations are auto-applied at `Open()`.

### Service (`internal/service/foo_service.go`)

```go
package service

import (
    "context"
    "database/sql"
    "fmt"
    "strings"
    "time"

    "github.com/AngheloAlva/timer/internal/domain"
    "github.com/AngheloAlva/timer/internal/storage/gen"
)

type FooService struct {
    db *sql.DB
    q  *gen.Queries
}

func NewFooService(db *sql.DB, q *gen.Queries) *FooService {
    return &FooService{db: db, q: q}
}

func (s *FooService) Create(ctx context.Context, name string) (domain.Foo, error) {
    name = strings.TrimSpace(name)
    if name == "" {
        return domain.Foo{}, fmt.Errorf("name cannot be empty")
    }
    id := newID() // existing helper
    now := time.Now()

    if err := s.q.CreateFoo(ctx, gen.CreateFooParams{
        ID: id, Name: name, CreatedAt: now.UnixMilli(),
    }); err != nil {
        if isUniqueViolation(err) {
            return domain.Foo{}, fmt.Errorf("%w: %q", domain.ErrFooExists, name)
        }
        return domain.Foo{}, fmt.Errorf("create foo: %w", err)
    }
    return domain.Foo{ID: id, Name: name, CreatedAt: now}, nil
}
```

**Transaction template** (multi-statement writes):

```go
tx, err := s.db.BeginTx(ctx, nil)
if err != nil { return domain.Foo{}, fmt.Errorf("begin tx: %w", err) }
defer func() { _ = tx.Rollback() }()
qtx := s.q.WithTx(tx)

// ... use qtx.XxxQuery(ctx, ...) ...

if err := tx.Commit(); err != nil {
    return domain.Foo{}, fmt.Errorf("commit: %w", err)
}
```

### CLI command (`internal/cli/foo.go`)

```go
package cli

import "github.com/spf13/cobra"

func newFooCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:     "foo",
        Aliases: []string{"foos"},
        Short:   "Manage foos",
    }
    cmd.AddCommand(newFooAddCmd(), newFooListCmd())
    return cmd
}

func newFooAddCmd() *cobra.Command {
    return &cobra.Command{
        Use:   "add <name>",
        Short: "Create a foo",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            app, err := openApp()
            if err != nil { return err }
            defer func() { _ = app.Close() }()

            f, err := app.FooSvc.Create(cmd.Context(), args[0])
            if err != nil { return err }

            cmd.Printf("Created %q (id: %s)\n", f.Name, format.ShortID(f.ID))
            return nil
        },
    }
}
```

Then in `internal/cli/root.go`:
```go
cmd.AddCommand(/* ... */, newFooCmd())
```
And in `internal/cli/context.go` (`openApp`):
```go
FooSvc: service.NewFooService(db.Conn(), q),
```

### MCP tool (`internal/mcp/tools/foo.go`)

```go
package tools

import (
    "context"
    "errors"
    "fmt"

    "github.com/mark3labs/mcp-go/mcp"
    mcpserver "github.com/mark3labs/mcp-go/server"

    "github.com/AngheloAlva/timer/internal/domain"
    "github.com/AngheloAlva/timer/internal/service"
)

func RegisterFooTools(s *mcpserver.MCPServer, svc *service.FooService) {
    s.AddTool(
        mcp.NewTool("create_foo",
            mcp.WithDescription("Create a new foo. Provide a non-empty name. Returns the foo id and name. Names are unique; duplicates fail with FOO_EXISTS."),
            mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable name. Trimmed; must be non-empty.")),
        ),
        createFooHandler(svc),
    )
}

func createFooHandler(svc *service.FooService) mcpserver.ToolHandlerFunc {
    return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
        name := mcp.ParseString(req, "name", "")
        f, err := svc.Create(ctx, name)
        if err != nil {
            if errors.Is(err, domain.ErrFooExists) {
                return mcp.NewToolResultError("FOO_EXISTS: a foo with that name already exists"), nil
            }
            return mcp.NewToolResultErrorFromErr("create_foo", err), nil
        }
        return mcp.NewToolResultText(fmt.Sprintf("Foo created: %s (id: %s)", f.Name, f.ID)), nil
    }
}
```

Then in `internal/mcp/server.go`:
```go
tools.RegisterFooTools(s, fooSvc)
```
(plus add `fooSvc *service.FooService` to `NewServer` parameters and pass it from `cmd/timer/main.go`).

## DO / DON'T

### DO
- Pass `ctx context.Context` as the FIRST arg of every service method.
- Return value types + error from services (`(domain.Foo, error)`, not `(*domain.Foo, error)`).
- Wrap errors with `fmt.Errorf("verb noun: %w", err)`.
- Define sentinel errors in `internal/domain/` and check with `errors.Is(...)`.
- Use `cmd.Printf` / `cmd.Println` for CLI output.
- Use `mcp.NewToolResultError("UPPER_SNAKE_CODE: ...")` for known domain errors; `NewToolResultErrorFromErr(toolName, err)` for the rest.
- Use snake_case for MCP tool names; write 200+ char descriptions — agents need context.
- Use value receivers on domain methods.
- Always `openApp()` + `defer func() { _ = app.Close() }()` at the top of every CLI `RunE`.

### DON'T
- ❌ Call `gen.Queries` from CLI or MCP — only services touch the DB.
- ❌ Use `fmt.Print*` in CLI commands — use `cmd.Printf` / `cmd.Println`.
- ❌ Put validation, DB calls, or formatting in domain types.
- ❌ Forget to register the new command in `internal/cli/root.go` or the new tool group in `internal/mcp/server.go`.
- ❌ Skip `ctx` on service methods.
- ❌ Use pointer receivers on domain methods.
- ❌ Write raw SQL outside `internal/storage/sqlite/queries/*.sql`.
- ❌ Add a logging library — project is intentionally quiet, errors bubble as return values.
- ❌ Touch `internal/storage/gen/` by hand — regenerate with `sqlc generate`.
- ❌ Change `MaxOpenConns(1)` or the SQLite pragmas in `db.go`.
- ❌ Import `internal/cli`, `internal/mcp`, or `internal/tui` from `internal/service` (or vice versa upward).

## Layering rules (HARD)

```
domain   ← imports: stdlib only
service  ← imports: domain, storage/gen, stdlib
cli      ← imports: service, domain, format, storage (via openApp wiring)
mcp      ← imports: service, domain, format
tui      ← imports: service, domain, format
```

If you find yourself wanting to import `cli`/`mcp`/`tui` from `service`, STOP — restructure.

## Commands

```bash
# Regenerate sqlc code after editing queries or migrations
sqlc generate

# Format
gofmt -w .

# Lint (config: .golangci.yml — v2 schema)
golangci-lint run

# Run the CLI locally
go run ./cmd/timer -- <args>
```

(Don't `go build` after every change — the user prefers checking via lint/run when needed.)

## Resources

- Existing references for patterns:
  - CLI: `internal/cli/project.go`, `internal/cli/timer.go`, `internal/cli/root.go`, `internal/cli/context.go`
  - MCP: `internal/mcp/tools/project.go`, `internal/mcp/tools/timer.go`, `internal/mcp/server.go`
  - Service: `internal/service/project_service.go`, `internal/service/timer_service.go`
  - Storage: `internal/storage/sqlite/db.go`, `internal/storage/sqlite/queries/`, `internal/storage/sqlite/migrations/`
  - Domain: `internal/domain/timer.go`, `internal/domain/project.go`
