# Go Migration — Overview

> Documento maestro. Si leés uno solo, que sea este. Los demás desarrollan cada pieza.

## Contexto

El proyecto arrancó como **monorepo Node** (NestJS + TanStack Start + MCP en TS) pensado para ser una web app con backend cloud. Después de iterar lo suficiente para entender el dominio y aprender cómo publicar un MCP en npm, la conclusión fue:

- Como **producto**, un timer web con login no aporta nada nuevo. Hay 500.
- Como **dev tool local-first** (CLI + TUI + MCP, todo local, SQLite embebido), se parece a las herramientas que los devs realmente usamos: `lazygit`, `k9s`, `gh`, `atuin`, `engram`.
- El usuario objetivo es un dev solo (o equipos chicos) en una máquina. No necesita multi-device hoy. El día que lo necesite, se agrega sync — sin romper nada.

## Decisión

**Reescribir en Go** como CLI + TUI nativo, distribuido por Homebrew y npm (el MCP), con SQLite local como source of truth.

### Por qué Go y no seguir con Node

| Criterio | Node (Ink + pkg) | Go (Bubbletea + binario nativo) |
|---|---|---|
| Tamaño binario | 40-80 MB | 8-15 MB |
| Startup time | 100-300 ms | 5-20 ms |
| Ecosistema TUI | Ink (React) — lindo, limitado | Bubbletea + Lipgloss + Glamour — SOTA |
| Distribución Homebrew | Compleja | Trivial (GoReleaser) |
| Concurrencia (timer en background) | Event loop | Goroutines + channels |
| Curva de aprendizaje | 0 (ya sabés TS) | 2-3 meses |
| Aprendizaje nuevo | Bajo | Alto (lenguaje compilado, concurrencia, Elm architecture) |

El criterio decisivo fue **"ya no veo qué más aprender"** — Go resuelve eso. No se hace por performance, se hace por aprendizaje + calidad de la herramienta final.

## Respuesta a "¿repo nuevo?"

**Sí, repo nuevo.** Razones:
1. Mezclar Node y Go en el mismo monorepo genera fricción de build systems.
2. Go tiene layout idiomático distinto (`cmd/`, `internal/`, `pkg/`) que choca con `apps/`/`packages/`.
3. El repo viejo queda como **referencia viva**: `docs/API_SPEC.md`, `docs/DATA_MODEL.md`, `docs/MCP_SPEC.md` son la fuente de verdad del dominio que se va a portar.

Sugerencia de nombre: `timer-cli` o `<nombre>-cli`. Mantenelo corto porque va a ser el comando que se tipea.

## Qué se conserva del repo viejo

**Se porta (dominio/semántica):**
- Modelo de datos (ver `docs/DATA_MODEL.md`) — se traduce de Prisma/Postgres a SQLite.
- Reglas de negocio (transiciones de timer, pausa, multi-timer) — ver `docs/API_SPEC.md`.
- Tools y resources del MCP — ver `docs/MCP_SPEC.md`.
- Comandos y slugs de proyectos/tareas.

**NO se porta (tech específica):**
- Código NestJS (controllers, services, DTOs, guards).
- Prisma schema (se reescribe como SQL + migrations).
- Auth cloud (JWT, refresh tokens, API keys) — en local-first no hace falta hasta que haya sync.
- TanStack Start / frontend web — el "frontend" ahora es TUI. Si después se quiere web, será una segunda superficie opcional.

**Decidido:**
- MCP se reescribe en Go con `mark3labs/mcp-go` como subcomando `timer mcp`. Un solo binario, un solo stack. Ver `05-MCP-STRATEGY.md`.

## Fases del proyecto

| Fase | Qué | Duración estimada |
|---|---|---|
| **0. Fundamentos Go** | Tour + Effective Go + Bubbletea tutorials + proyecto sandbox | 1-2 semanas |
| **1. CLI core** | Comandos base (`timer start/stop/list`), SQLite, dominio | 2-3 semanas |
| **2. TUI** | Vista interactiva con Bubbletea + Lipgloss | 2 semanas |
| **3. MCP** | Decidir estrategia y wirear | 1 semana |
| **4. Distribución** | GoReleaser + Homebrew tap + CI | 3-4 días |
| **5. Sync opcional** (futuro) | Turso/ElectricSQL/endpoint propio | — |

**Total realista hasta v1 usable: 8-10 semanas** a ritmo side project (10h/semana).

## Índice de documentos

- `00-OVERVIEW.md` — este archivo
- `01-LEARNING-PATH.md` — Fase 0, lo que hay que aprender antes de tocar código
- `02-TARGET-STACK.md` — librerías elegidas, layout de carpetas, decisiones técnicas
- `03-DATA-MODEL.md` — traducción del schema Prisma → SQLite
- `04-MIGRATION-PLAN.md` — plan por fases con entregables concretos
- `05-MCP-STRATEGY.md` — qué hacer con el MCP actual
- `06-DISTRIBUTION.md` — Homebrew tap + releases

## Principios que se mantienen

1. **TypeScript estricto** ya no aplica, pero el espíritu sí: **Go estricto**. Sin `interface{}` (ahora `any`) si no hace falta. Tipos concretos siempre que se pueda.
2. **No inventar**. Si el spec (`API_SPEC.md`, `DATA_MODEL.md`, `MCP_SPEC.md`) no cubre algo, preguntar antes de decidir.
3. **Conventional commits** — se mantiene.
4. **Tests en el mismo PR** que la feature — se mantiene.
5. **Conceptos > código**. No se toca una línea de Go hasta completar Fase 0.
