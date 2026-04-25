# Target Stack & Layout

> Stack concreto, librerías elegidas con justificación, estructura de carpetas idiomática.

## Stack final propuesto

| Capa | Librería | Por qué |
|---|---|---|
| CLI commands | `spf13/cobra` | Estándar de facto (lo usa `gh`, `kubectl`, `hugo`). Subcomandos, flags, autocomplete. |
| TUI | `charmbracelet/bubbletea` | Elm architecture, activo, el mejor del ecosistema. |
| TUI styling | `charmbracelet/lipgloss` | Lo usa Bubbletea, layouts flexbox-like. |
| TUI componentes | `charmbracelet/bubbles` | List, table, textinput, spinner, viewport ya hechos. |
| SQLite driver | `mattn/go-sqlite3` o `modernc.org/sqlite` | El primero es cgo (rápido), el segundo es puro Go (crosscompila fácil). **Empezar con `modernc.org/sqlite`** por distribución. |
| SQL queries | `sqlc` | Genera código Go type-safe desde SQL. No es ORM. Escribís SQL, te da structs y funciones. |
| Migrations | `golang-migrate/migrate` | Migrations versionadas en `.sql`, embed en binario. |
| Config | stdlib `flag` + `os.Getenv` + XDG dirs | Mantener simple. `viper` es overkill para esto. |
| Logging | stdlib `log/slog` (Go 1.21+) | Structured logging en stdlib. |
| UUIDs | `google/uuid` | Estándar. |
| Tests | stdlib `testing` + `stretchr/testify` (solo `assert`) | `testify/assert` para asserts legibles. Nada más. |
| TUI testing | `charmbracelet/x/exp/teatest` | Testing de modelos Bubbletea. |
| MCP server | `mark3labs/mcp-go` | 8.6k stars, usado por engram/linear/honeycomb, releases activas. Pineá versión exacta (v0.x.x, API aún rompe). |
| Build/release | `goreleaser/goreleaser` | Un comando genera binarios multi-arch + Homebrew formula + changelog. |

## Librerías que NO se usan (y por qué)

- ❌ **Gin/Echo/Fiber** — no hay HTTP server en Fase 1. Si después hay sync, `net/http` alcanza.
- ❌ **GORM/Ent** — abstracción innecesaria. `sqlc` da type-safety con SQL real.
- ❌ **Viper** — overkill para un CLI local. `flag` + env vars alcanzan.
- ❌ **Cobra+Viper scaffolding (`cobra-cli`)** — genera boilerplate inflado. Escribir a mano.
- ❌ **Zap/Zerolog** — `slog` del stdlib ya existe desde Go 1.21.

## Layout de carpetas

Layout idiomático de Go para un CLI:

```
timer-cli/
├── cmd/
│   └── timer/
│       └── main.go                # entrypoint — solo arma el root cmd y ejecuta
├── internal/                      # código privado, Go bloquea imports externos
│   ├── cli/                       # comandos cobra
│   │   ├── root.go
│   │   ├── start.go
│   │   ├── stop.go
│   │   ├── list.go
│   │   ├── project.go             # subcomando con sus subsubcomandos
│   │   ├── task.go
│   │   ├── tui.go                 # `timer tui` arranca la TUI
│   │   └── mcp.go                 # `timer mcp` arranca servidor MCP stdio
│   ├── mcp/                       # servidor MCP (mark3labs/mcp-go)
│   │   ├── server.go              # ensamblaje de tools/resources
│   │   ├── tools/                 # handlers por dominio
│   │   ├── resources/             # handlers de timer://*
│   │   └── format/                # formatters human-readable
│   ├── tui/                       # todo lo de Bubbletea
│   │   ├── app.go                 # Model raíz
│   │   ├── views/
│   │   │   ├── timer_view.go
│   │   │   ├── tasks_view.go
│   │   │   └── reports_view.go
│   │   ├── components/            # bubbles customizados
│   │   └── styles/                # lipgloss styles
│   ├── domain/                    # tipos + reglas de negocio, SIN deps de DB/UI
│   │   ├── timer.go
│   │   ├── task.go
│   │   ├── project.go
│   │   ├── entry.go
│   │   └── errors.go
│   ├── storage/                   # capa de persistencia
│   │   ├── sqlite/
│   │   │   ├── db.go              # abrir conexión, pragmas, ejecutar migrations
│   │   │   ├── migrations/        # *.sql embebidos con //go:embed
│   │   │   │   ├── 001_init.up.sql
│   │   │   │   └── 001_init.down.sql
│   │   │   └── queries/           # *.sql para sqlc
│   │   │       ├── timers.sql
│   │   │       ├── tasks.sql
│   │   │       └── projects.sql
│   │   └── gen/                   # código generado por sqlc (NO editar)
│   ├── service/                   # use cases que orquestan domain + storage
│   │   ├── timer_service.go
│   │   ├── task_service.go
│   │   └── report_service.go
│   ├── config/
│   │   └── config.go              # resolver XDG paths, leer env
│   └── version/
│       └── version.go             # inyectado por goreleaser con -ldflags
├── docs/
│   └── ...                        # specs portados del repo viejo
├── .goreleaser.yaml
├── sqlc.yaml
├── go.mod
├── go.sum
├── Makefile                       # comandos dev comunes
└── README.md
```

### Decisiones clave del layout

**`internal/` en vez de `pkg/`:**
- `internal/` es un keyword de Go: lo que vive ahí NO puede ser importado desde fuera del módulo.
- Como esto es un CLI (no una librería para consumo externo), todo va en `internal/`.
- `pkg/` sería para cuando se quiera exponer algo reutilizable (hoy no aplica).

**Capas (domain → service → storage → cli/tui):**
- `domain/` no importa NADA de `storage/` ni `cli/`. Solo tipos y reglas.
- `service/` importa `domain` y `storage`. Orquesta, valida, aplica reglas.
- `cli/` y `tui/` importan `service`. NUNCA tocan `storage` directo.
- Esto es la misma separación que tenías en NestJS, pero sin DI container — en Go se pasa por constructor.

**`cmd/timer/main.go` minimalista:**
```go
func main() {
    if err := cli.NewRootCmd().Execute(); err != nil {
        os.Exit(1)
    }
}
```
Nada más ahí. Todo el armado vive en `internal/cli/`.

## Archivos de configuración mínimos

### `sqlc.yaml`

```yaml
version: "2"
sql:
  - engine: "sqlite"
    queries: "internal/storage/sqlite/queries"
    schema: "internal/storage/sqlite/migrations"
    gen:
      go:
        package: "gen"
        out: "internal/storage/gen"
        emit_json_tags: true
        emit_prepared_queries: false
        emit_interface: true
        emit_pointers_for_null_types: true
```

### `Makefile` inicial

```makefile
.PHONY: dev test lint sqlc migrate

dev:
	go run ./cmd/timer tui

test:
	go test ./... -race -cover

lint:
	golangci-lint run

sqlc:
	sqlc generate

build:
	CGO_ENABLED=0 go build -o bin/timer ./cmd/timer
```

## Dónde vive la data del usuario

Usar XDG Base Directory Spec:

- **DB**: `$XDG_DATA_HOME/timer/timer.db` (fallback: `~/.local/share/timer/timer.db`)
- **Config**: `$XDG_CONFIG_HOME/timer/config.toml` (fallback: `~/.config/timer/config.toml`)
- **Logs**: `$XDG_STATE_HOME/timer/timer.log` (fallback: `~/.local/state/timer/timer.log`)

Librería: `adrg/xdg` resuelve esto en una línea. O escribirlo a mano, son 20 líneas.

## Build tags / flags

Al compilar con `goreleaser`, inyectar versión y commit:

```go
// internal/version/version.go
package version

var (
    Version = "dev"
    Commit  = "none"
    Date    = "unknown"
)
```

```bash
go build -ldflags "-X 'timer/internal/version.Version=v0.1.0' -X 'timer/internal/version.Commit=$(git rev-parse HEAD)'" ./cmd/timer
```

## Qué NO hacer en el layout

- ❌ Un único paquete `main` con todo adentro.
- ❌ `models/` o `types/` como carpeta global — los tipos viven con el dominio al que pertenecen.
- ❌ Imports circulares. Si pasa, el diseño está mal. Go los prohíbe, así que el compilador te lo avisa.
- ❌ Nombres de paquete con guiones (`my-pkg` está mal, va `mypkg`).
- ❌ Nombres de paquete redundantes (`package utils` adentro de `utils/`, ok; `package UtilsPackage`, no).
