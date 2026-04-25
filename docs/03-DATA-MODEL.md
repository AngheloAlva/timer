# Data Model — Prisma/Postgres → SQLite

> Traducción del schema actual (`apps/api/prisma/schema.prisma`) al mundo local-first con SQLite.

## Principios de la traducción

1. **Un solo usuario por DB.** La DB vive en `~/.local/share/timer/timer.db`. El modelo `User` se elimina. Todo lo que tenía `userId` lo pierde.
2. **Nada de auth.** Se eliminan: `User`, `RefreshToken`, `ApiKey`. No hay login local.
3. **UUIDs como TEXT.** SQLite no tiene tipo UUID. Se guarda como `TEXT` generado con `google/uuid`.
4. **Timestamps como INTEGER Unix epoch (ms).** SQLite no tiene `DATETIME` nativo útil. Guardar como `INTEGER` evita líos de zona horaria. La conversión a `time.Time` se hace en Go.
5. **Enums como TEXT + CHECK constraint.** SQLite no tiene ENUM; simulamos con `CHECK (status IN ('todo', 'in_progress', ...))`.
6. **Foreign keys ON.** SQLite tiene FKs desactivadas por default. Ejecutar `PRAGMA foreign_keys = ON;` al abrir la conexión.
7. **WAL mode.** `PRAGMA journal_mode = WAL;` para permitir lecturas concurrentes mientras se escribe (relevante cuando el CLI y el MCP acceden a la misma DB).

## Schema SQLite propuesto

Archivo: `internal/storage/sqlite/migrations/001_init.up.sql`

```sql
-- Pragmas (aplicar al abrir conexión, no en migration)
-- PRAGMA foreign_keys = ON;
-- PRAGMA journal_mode = WAL;
-- PRAGMA synchronous = NORMAL;

CREATE TABLE projects (
    id          TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    slug        TEXT NOT NULL UNIQUE,
    color       TEXT,
    archived    INTEGER NOT NULL DEFAULT 0,  -- boolean como 0/1
    created_at  INTEGER NOT NULL,             -- unix ms
    updated_at  INTEGER NOT NULL
);

CREATE INDEX idx_projects_archived ON projects(archived);

CREATE TABLE tasks (
    id            TEXT PRIMARY KEY,
    project_id    TEXT NOT NULL,
    title         TEXT NOT NULL,
    description   TEXT,
    status        TEXT NOT NULL DEFAULT 'todo'
                    CHECK (status IN ('todo', 'in_progress', 'done', 'archived')),
    external_ref  TEXT,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL,
    FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE
);

CREATE INDEX idx_tasks_project_status ON tasks(project_id, status);
CREATE INDEX idx_tasks_external_ref ON tasks(external_ref);

CREATE TABLE timers (
    id                TEXT PRIMARY KEY,
    task_id           TEXT NOT NULL UNIQUE,  -- un timer activo por tarea
    started_at        INTEGER NOT NULL,
    note              TEXT,
    source            TEXT NOT NULL
                        CHECK (source IN ('cli', 'tui', 'mcp', 'manual')),
    paused_at         INTEGER,
    paused_total_sec  INTEGER NOT NULL DEFAULT 0,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE TABLE time_entries (
    id           TEXT PRIMARY KEY,
    task_id      TEXT NOT NULL,
    started_at   INTEGER NOT NULL,
    ended_at     INTEGER NOT NULL,
    duration_sec INTEGER NOT NULL,
    note         TEXT,
    source       TEXT NOT NULL
                   CHECK (source IN ('cli', 'tui', 'mcp', 'manual')),
    created_at   INTEGER NOT NULL,
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
);

CREATE INDEX idx_time_entries_started ON time_entries(started_at);
CREATE INDEX idx_time_entries_task_started ON time_entries(task_id, started_at);

CREATE TABLE tags (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL UNIQUE,
    color      TEXT NOT NULL DEFAULT '#94a3b8',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);

-- Join tables M:N
CREATE TABLE task_tags (
    task_id TEXT NOT NULL,
    tag_id  TEXT NOT NULL,
    PRIMARY KEY (task_id, tag_id),
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE timer_tags (
    timer_id TEXT NOT NULL,
    tag_id   TEXT NOT NULL,
    PRIMARY KEY (timer_id, tag_id),
    FOREIGN KEY (timer_id) REFERENCES timers(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);

CREATE TABLE time_entry_tags (
    time_entry_id TEXT NOT NULL,
    tag_id        TEXT NOT NULL,
    PRIMARY KEY (time_entry_id, tag_id),
    FOREIGN KEY (time_entry_id) REFERENCES time_entries(id) ON DELETE CASCADE,
    FOREIGN KEY (tag_id) REFERENCES tags(id) ON DELETE CASCADE
);
```

## Diferencias vs. schema Prisma original

| Elemento | Prisma (actual) | SQLite (target) | Motivo |
|---|---|---|---|
| `User` | Modelo completo | **Eliminado** | Single-user local |
| `ApiKey`, `RefreshToken` | Modelos | **Eliminados** | No hay auth |
| `userId` en todas | FK a User | **Eliminado** | Single-user |
| `id` | `String @default(uuid())` | `TEXT PRIMARY KEY`, generado en app | SQLite no genera UUIDs |
| `createdAt`/`updatedAt` | `DateTime @default(now())` | `INTEGER NOT NULL` (unix ms), seteado por la app | SQLite/tiempo |
| `archived` | `Boolean` | `INTEGER 0/1` | SQLite no tiene bool |
| `TaskStatus` enum | Enum Prisma | `TEXT + CHECK` | SQLite sin enums |
| `TimerSource` | `web, mcp, api, manual` | `cli, tui, mcp, manual` | Cambia el mundo de surfaces |
| `Project.slug` | `@@unique([userId, slug])` | `UNIQUE` global | Single-user |
| `Tag.name` | `@@unique([userId, name])` | `UNIQUE` global | Single-user |
| M:N tags | `Tag[]` implícito de Prisma | Tablas join explícitas | SQL crudo |

## Domain types en Go

Ejemplo de cómo se vería el tipo `Timer` en `internal/domain/timer.go`:

```go
package domain

import (
    "errors"
    "time"
)

type Source string

const (
    SourceCLI    Source = "cli"
    SourceTUI    Source = "tui"
    SourceMCP    Source = "mcp"
    SourceManual Source = "manual"
)

type Timer struct {
    ID             string
    TaskID         string
    StartedAt      time.Time
    Note           string
    Source         Source
    PausedAt       *time.Time
    PausedTotalSec int
}

// ElapsedSec calcula los segundos efectivamente trackeados
// (descuenta tiempo pausado y pausa actual si aplica).
func (t *Timer) ElapsedSec(now time.Time) int {
    total := int(now.Sub(t.StartedAt).Seconds()) - t.PausedTotalSec
    if t.PausedAt != nil {
        total -= int(now.Sub(*t.PausedAt).Seconds())
    }
    if total < 0 {
        return 0
    }
    return total
}

var (
    ErrTimerAlreadyRunning = errors.New("timer already running for this task")
    ErrTimerNotRunning     = errors.New("timer is not running")
    ErrTimerNotPaused      = errors.New("timer is not paused")
)
```

Puntos a notar:
- `Source` es un `string` typed con constantes — el equivalente idiomático de un enum TS.
- `PausedAt *time.Time` — pointer porque es nullable. `PausedTotalSec int` no es pointer porque 0 es valor válido.
- Errores como variables `Err...` exportadas — se comparan con `errors.Is(err, domain.ErrTimerAlreadyRunning)`.

## Reglas de negocio a portar

Del repo actual (confirmar leyendo `apps/api/src/timers/` y `docs/API_SPEC.md`):

1. **Un timer activo por tarea** (UNIQUE constraint en `task_id`).
2. **Multi-timer**: múltiples timers simultáneos permitidos, cada uno en tarea distinta.
3. **Pausa**: `pausedAt` marca inicio de la pausa actual; al reanudar, el delta se suma a `pausedTotalSec` y se limpia `pausedAt`.
4. **Detener timer** = DELETE del registro `timers` + INSERT en `time_entries` con `duration_sec` calculado. Atómico en una transacción.
5. **Slug de project** se auto-genera del `name` si no viene explícito. Lowercase, kebab-case, sin acentos.
6. **Tags se crean on-the-fly** si no existen al asociarlas (upsert por `name`).

## Migrations con golang-migrate

Estructura de archivos:

```
internal/storage/sqlite/migrations/
├── 001_init.up.sql
├── 001_init.down.sql
├── 002_add_whatever.up.sql
└── 002_add_whatever.down.sql
```

Embed en el binario:

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS
```

Ejecutar al startup:

```go
func RunMigrations(db *sql.DB) error {
    driver, err := sqlite.WithInstance(db, &sqlite.Config{})
    if err != nil { return err }
    source, err := iofs.New(migrationsFS, "migrations")
    if err != nil { return err }
    m, err := migrate.NewWithInstance("iofs", source, "sqlite", driver)
    if err != nil { return err }
    if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
        return err
    }
    return nil
}
```

## sqlc — ejemplo de query

Archivo `internal/storage/sqlite/queries/timers.sql`:

```sql
-- name: CreateTimer :exec
INSERT INTO timers (id, task_id, started_at, note, source, paused_total_sec)
VALUES (?, ?, ?, ?, ?, 0);

-- name: GetActiveTimers :many
SELECT * FROM timers
ORDER BY started_at DESC;

-- name: GetTimerByTaskID :one
SELECT * FROM timers WHERE task_id = ? LIMIT 1;

-- name: PauseTimer :exec
UPDATE timers SET paused_at = ? WHERE id = ?;

-- name: ResumeTimer :exec
UPDATE timers
SET paused_at = NULL,
    paused_total_sec = paused_total_sec + ?
WHERE id = ?;

-- name: DeleteTimer :exec
DELETE FROM timers WHERE id = ?;
```

`sqlc generate` lee esto y te da funciones Go type-safe tipo `CreateTimer(ctx, arg CreateTimerParams) error`.

## Seed inicial

Al crear la DB por primera vez, crear un proyecto default `"inbox"` para que el usuario pueda empezar a trackear sin configurar nada:

```sql
INSERT INTO projects (id, name, slug, color, archived, created_at, updated_at)
VALUES (?, 'Inbox', 'inbox', '#64748b', 0, ?, ?);
```

Esto se hace en código al detectar DB vacía, no en una migration (las migrations no deberían tener data).
