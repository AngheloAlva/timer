# Migration Plan — Fases con entregables

> Plan operativo: qué se construye en cada fase, qué queda cerrado al final, qué tests se escriben.

## Fase 0 — Learning (1-2 semanas)

Ver `01-LEARNING-PATH.md`. Entregable: proyecto sandbox `notes-cli` funcionando con CLI + TUI + SQLite.

**Criterio de salida:** podés explicar los 5 puntos del checklist de Fase 0 sin googlear.

---

## Fase 1 — CLI core (2-3 semanas)

**Objetivo:** comando `timer` usable desde terminal, sin TUI todavía.

### Tareas

1. **Scaffold del repo nuevo**
   - `go mod init github.com/<user>/timer-cli`
   - Estructura de carpetas según `02-TARGET-STACK.md`.
   - `Makefile` con `dev/test/lint/build`.
   - `.golangci.yml` con linters razonables (gofmt, govet, errcheck, staticcheck, revive).
   - GitHub Actions CI: lint + test + build.

2. **Storage layer**
   - Conexión SQLite con pragmas (`foreign_keys=ON`, `journal_mode=WAL`).
   - Migrations embebidas con `golang-migrate`.
   - `sqlc` configurado generando desde `queries/*.sql`.
   - Tests del storage con DB en archivo temporal (`t.TempDir()`).

3. **Domain + Service layers**
   - Tipos `Timer`, `Task`, `Project`, `TimeEntry`, `Tag` con métodos de dominio.
   - Servicios con constructores que reciben `*sql.DB` o las queries generadas por sqlc.
   - Reglas de negocio portadas (ver `03-DATA-MODEL.md` sección "Reglas").
   - Tests unitarios con testify.

4. **CLI commands con cobra**
   - `timer start <task>` — arranca timer. Si `<task>` no existe, preguntar si crear (prompt interactivo con `charmbracelet/huh` o `survey`).
   - `timer stop [task]` — detiene timer. Sin args, detiene todos.
   - `timer pause [task]` / `timer resume [task]`.
   - `timer list` — lista timers activos.
   - `timer log` — time entries recientes (flags: `--today`, `--week`, `--project X`).
   - `timer project add|list|archive`.
   - `timer task add|list|done`.
   - `timer report --week` — tabla con totales por proyecto/tarea.
   - `timer version`.

5. **Config + XDG paths**
   - Resolver DB/config dir con `adrg/xdg`.
   - `timer init` opcional para forzar creación de dirs + DB.
   - Seed del proyecto "Inbox" la primera vez.

### Entregable de Fase 1

Binario `timer` compilado que hace todo lo del CLI, con tests, sin TUI. Se puede usar en serio desde ya.

---

## Fase 2 — TUI (2 semanas)

**Objetivo:** `timer tui` abre una UI interactiva navegable.

### Tareas

1. **Model raíz en Bubbletea**
   - `App` con estado global (vista actual, DB ref, size).
   - Router de vistas con `tea.Msg` para transicionar.

2. **Vistas**
   - **Dashboard**: timers activos arriba, tareas del día abajo, totales de hoy/semana.
   - **Tasks**: list con filtros (proyecto, status, tag), acciones (start, stop, edit, delete).
   - **Projects**: CRUD de proyectos.
   - **Reports**: tabla con totales por rango de fechas.

3. **Componentes reusables**
   - Timer counter en vivo — goroutine que emite `tickMsg` cada segundo.
   - Project picker con `bubbles/list`.
   - Task editor con `bubbles/textinput` y `bubbles/textarea`.
   - Keybindings globales (`?` help, `q` quit, `j/k` nav, etc.).

4. **Styling**
   - `lipgloss` con theme (colores, bordes, padding).
   - Soportar modo claro/oscuro detectando el terminal.

5. **Tests TUI**
   - Con `teatest` — mandar inputs, verificar outputs.
   - Tests de al menos dashboard + tasks view.

### Entregable de Fase 2

`timer tui` abre UI completa. Desde la TUI podés hacer todo lo que el CLI hace.

---

## Fase 3 — MCP en Go (1-1.5 semanas)

Ver `05-MCP-STRATEGY.md` para el plan completo.

### Tareas

1. **Agregar dependencia** `github.com/mark3labs/mcp-go` al `go.mod`. **Pinear versión exacta** (no rango) — ejemplo: `v0.49.0`.
2. **Subcomando `timer mcp`** en `internal/cli/mcp.go`: inicializa el container de services, arma el server MCP, arranca stdio.
3. **Tools handlers** en `internal/mcp/tools/`:
   - `timer_start`, `timer_stop`, `timer_pause`, `timer_resume`, `timer_list_active`
   - `task_create`, `task_list`, `task_update_status`
   - `project_create`, `project_list`
   - `time_log`, `report_summary`
4. **Resources handlers** en `internal/mcp/resources/`:
   - `timer://active`, `timer://today`, `timer://project/{slug}`
5. **Formatters human-readable** en `internal/mcp/format/` — portar la lógica de `apps/mcp/src/format.ts` del repo viejo.
6. **Tests de integración** — levantar el server en memoria, mandar requests JSON-RPC, verificar responses. `mark3labs/mcp-go` tiene helpers para esto.
7. **Schema guard** — chequear que la DB exista al arrancar; si no, error claro "ejecutá `timer init`".
8. **Documentar en README** la config `.mcp.json` para Claude Code.

### Entregable de Fase 3

`timer mcp` arranca un servidor MCP stdio que Claude Code puede consumir. Todos los tools/resources del spec original funcionan apuntando al SQLite local.

Claude Code config:
```json
{ "mcpServers": { "timer": { "command": "timer", "args": ["mcp"] } } }
```

---

## Fase 4 — Distribución (3-4 días)

**Objetivo:** `brew install <user>/tap/timer` funciona.

### Tareas

1. **GoReleaser**
   - `.goreleaser.yaml` con builds multi-arch (darwin/amd64, darwin/arm64, linux/amd64, linux/arm64).
   - Archives `.tar.gz` con binario + README + LICENSE.
   - Changelog automático desde conventional commits.
   - SHA256 checksums.

2. **Homebrew tap**
   - Repo separado `homebrew-tap`.
   - GoReleaser genera el Formula automáticamente al taggear.

3. **GitHub Actions release workflow**
   - Trigger en tag `v*`.
   - Build, test, release con goreleaser.
   - Publicación del Formula al tap.

4. **Install script opcional**
   - `curl -sSL https://timer.dev/install.sh | sh` para usuarios sin brew.

### Entregable de Fase 4

Tag `v0.1.0` en main → binarios publicados en GitHub Releases + Homebrew formula actualizada.

---

## Fase 5 — Sync opcional (futuro, NO hacer ahora)

Solo cuando:
- Hay 10+ usuarios reales pidiéndolo.
- El core está estable y testeado.

Opciones a evaluar:
- **Turso** — SQLite distribuido con embedded replicas. Encaja perfecto local-first.
- **ElectricSQL** — CRDT-based sync sobre SQLite. Más complejo pero más "correcto".
- **Endpoint propio** — reutilizar parte de la API NestJS vieja. Más control, más laburo.
- **Git como backend** — exportar entries como JSON/YAML a un repo. Suena loco, funciona para casos simples.

## Orden de ataque recomendado dentro de Fase 1

Para no perderse, este es el orden de tareas concretas en Fase 1:

1. Repo nuevo + `go mod init` + Makefile + CI vacío.
2. Migration 001 + abrir DB + pragmas + tests de conexión.
3. `sqlc` configurado con 1 query tonta (`SELECT 1`) para verificar generation.
4. Domain types (sin lógica todavía).
5. Queries sqlc para projects → service `ProjectService` → comando `timer project add/list`.
6. Lo mismo para tasks → `timer task add/list`.
7. Lo mismo para timers → `timer start/stop` — acá se pone denso, transacciones, reglas.
8. `timer log` y `timer report`.
9. Pulir: errores legibles, colores en output, `--help` útil.

No saltear pasos. Cada paso cierra con tests verdes.

## Métricas de éxito

Al final de Fase 4, estos criterios deberían cumplirse:

- [ ] Binario <15 MB.
- [ ] Startup de `timer list` <30 ms en máquina promedio.
- [ ] Cobertura de tests >70% en `service/` y `storage/`.
- [ ] `brew install` funciona en mac Apple Silicon e Intel.
- [ ] TUI no parpadea ni pierde inputs en ejecuciones de 1+ hora.
- [ ] MCP funciona con Claude Code sin errores por 10 operaciones consecutivas.
- [ ] Un usuario nuevo puede desde cero: instalar, crear proyecto, arrancar timer, detenerlo, ver reporte, sin leer docs.
