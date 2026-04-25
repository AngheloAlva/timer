# MCP Strategy

> **Decisión tomada:** MCP en Go, usando `mark3labs/mcp-go`. Esta sección documenta el plan concreto. La opción de "MCP en Node leyendo SQLite" queda al final como fallback si el SDK Go rompe algo.

El MCP actual está en Node, publicado en npm (`apps/mcp`, ver `package.json` y `src/`). Se reescribe en Go para tener un solo stack, un solo binario, y reusar directamente el service layer.

## Estado actual

- Paquete: `@<user>/timer-mcp` v0.2.1 (según commits `chore(mcp): publish 0.2.1`).
- Tech: TypeScript, `tsup` para build, publicado en npm.
- Comunicación: habla HTTP con la API NestJS usando `X-API-Key`.
- Tools/resources: definidos en `docs/MCP_SPEC.md`.

## SDK elegido: `mark3labs/mcp-go`

https://github.com/mark3labs/mcp-go

Verificado al momento de esta decisión:
- **8.6k stars**, v0.49.0 publicada recientemente, releases casi semanales.
- Usado por **engram** (el MCP que el usuario ya tiene instalado), **linear.app**, **honeycomb.io**.
- Activo: commits diarios, issues respondidas rápido.
- Soporta transports stdio, SSE y StreamableHTTP.
- API idiomática Go — tipos concretos, `context.Context` en todos lados, structs para params.

**Advertencia:** sigue en `v0.x.x`, así que la API puede romper entre minor versions. Pineá la versión exacta en `go.mod` (no rangos) y actualizá con intención, no automáticamente.

### Alternativa evaluada: SDK oficial `modelcontextprotocol/go-sdk`

Existe pero tiene menos adopción y menos features probadas que `mark3labs/mcp-go` al día de hoy. Re-evaluar dentro de 6-12 meses — si el oficial madura, migrar es trivial (son ~300 líneas de glue code).

## Plan: binario único o dos binarios

### Opción elegida — `timer mcp` como subcomando

**Layout:**
```
cmd/
├── timer/
│   └── main.go          # CLI + TUI + subcomando `timer mcp`
```

Sin `cmd/timer-mcp/` separado. El binario `timer` acepta `timer mcp` como subcomando que arranca el servidor stdio.

**Por qué un solo binario:**
- Una sola cosa que instalar, una sola versión que trackear.
- Comparten todo: config, DB path, service layer, domain types.
- Goreleaser construye un solo artefacto, Homebrew formula trivial.
- Para Claude Code, el comando del MCP es simplemente `/usr/local/bin/timer mcp`.

### Rechazada — binario separado `timer-mcp`

Hubiera dado separación conceptual mayor pero duplica infra de build sin ganancia real para este proyecto. Si el MCP crecera en complejidad y tuviera un ciclo de release distinto, tiene sentido. Hoy no.

## Claude Code config (`.mcp.json`)

```json
{
  "mcpServers": {
    "timer": {
      "command": "timer",
      "args": ["mcp"]
    }
  }
}
```

Simple: el binario `timer` está en `$PATH` (instalado por Homebrew), y `timer mcp` arranca el servidor stdio. Cero `npx`, cero Node.

## Arquitectura interna

```
internal/
├── mcp/
│   ├── server.go         # arma el server con mark3labs/mcp-go
│   ├── tools/            # handlers de cada tool
│   │   ├── timer.go      # timer_start, timer_stop, timer_pause, timer_resume
│   │   ├── task.go       # task_create, task_list, task_update_status
│   │   ├── project.go    # project_create, project_list
│   │   └── report.go     # time_log, report_summary
│   ├── resources/        # handlers de resources
│   │   ├── active.go     # timer://active
│   │   ├── today.go      # timer://today
│   │   └── project.go    # timer://project/{slug}
│   └── format/           # formatters human-readable (equivalente a format.ts actual)
│       ├── duration.go
│       └── tables.go
└── cli/
    └── mcp.go            # subcomando cobra `timer mcp`
```

### Esqueleto del server

```go
// internal/mcp/server.go
package mcp

import (
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
    "timer/internal/service"
)

func NewServer(svc *service.Container) *server.MCPServer {
    s := server.NewMCPServer(
        "timer",
        version.Version,
        server.WithToolCapabilities(true),
        server.WithResourceCapabilities(true, true),
    )

    // Tools
    s.AddTool(
        mcp.NewTool("timer_start",
            mcp.WithDescription("Arranca un timer para una tarea"),
            mcp.WithString("task", mcp.Required(), mcp.Description("Slug o ID de la tarea")),
            mcp.WithString("note", mcp.Description("Nota opcional")),
        ),
        tools.NewTimerStart(svc.Timer),
    )

    // ... resto de tools y resources
    return s
}
```

### Arranque desde el CLI

```go
// internal/cli/mcp.go
func newMCPCmd(svc *service.Container) *cobra.Command {
    return &cobra.Command{
        Use:   "mcp",
        Short: "Arranca el servidor MCP (stdio)",
        RunE: func(cmd *cobra.Command, args []string) error {
            s := mcp.NewServer(svc)
            return server.ServeStdio(s)
        },
    }
}
```

## Ventajas concretas de este approach

1. **Reuso 100% del service layer.** Los handlers del MCP llaman a `svc.Timer.Start(...)`, exactamente el mismo código que llama el CLI cuando hacés `timer start`. Cero duplicación.
2. **Un solo `brew install`** trae CLI, TUI y MCP.
3. **Testing unificado.** Los tests del service layer validan comportamiento de los tres frontends simultáneamente.
4. **Versión única.** Si el CLI está en v1.2.3, el MCP está en v1.2.3. No hay drift posible.
5. **Shared config.** `TIMER_DB_PATH` y XDG dirs se resuelven igual para todos los frontends.

## Concurrencia SQLite

Con el CLI/TUI y el MCP siendo el mismo binario ejecutado en dos procesos (uno interactivo, otro iniciado por Claude Code), comparten archivo SQLite:

1. Ambos ejecutan `PRAGMA journal_mode = WAL` al abrir la conexión.
2. Las transacciones de un proceso no bloquean reads del otro.
3. En write contention (raro en la práctica), SQLite devuelve `SQLITE_BUSY`. Configurar `busy_timeout=5000` en el driver.
4. `modernc.org/sqlite` (pure Go, sin cgo) maneja esto correctamente. Cada proceso abre su propia conexión.

## Migrations

El MCP **NO** corre migrations. Solo el CLI en startup normal. Cuando Claude Code lanza `timer mcp`, asume que el schema ya está aplicado. Si no hay DB (primera vez que el usuario usa el MCP antes que el CLI), arrancamos con un error claro: "Ejecutá `timer init` primero."

Alternativa: `timer mcp` al arrancar chequea si la DB existe; si no, la crea y corre migrations. Decidir en Fase 3 — depende de UX que queramos.

## Tools a portar (sin cambios semánticos)

Ver `docs/MCP_SPEC.md` — todas las tools y resources documentadas se mantienen con la misma forma. Lo único que cambia es la implementación interna (SQLite en vez de HTTP).

Tools que se portan tal cual:
- `timer_start`, `timer_stop`, `timer_pause`, `timer_resume`, `timer_list_active`
- `task_create`, `task_list`, `task_update_status`
- `project_create`, `project_list`
- `time_log`, `report_summary`

Resources:
- `timer://active`
- `timer://today`
- `timer://project/{slug}`

## Fallback (si el SDK Go se rompe)

Si en Fase 3 aparece un problema bloqueante con `mark3labs/mcp-go` (breaking change no resoluble, feature faltante crítica, abandono del repo), el plan B es:

1. Mantener el MCP actual en Node (`apps/mcp` del repo viejo).
2. Reemplazar su `api-client.ts` por acceso directo a SQLite con `better-sqlite3`.
3. Publicarlo como `@<user>/timer-mcp` v1.0.
4. Documentar en el README dos rutas de instalación: Homebrew para CLI/TUI + npm para MCP.

No es lo ideal pero destraba si el SDK Go falla. No se hace preventivamente — se hace solo si realmente no hay otra.
