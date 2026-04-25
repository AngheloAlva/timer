# Go Migration Docs

Documentación para migrar este proyecto de Node (NestJS + TanStack Start + MCP en TS) a Go (CLI + TUI + MCP), local-first con SQLite.

## Orden de lectura

1. [`00-OVERVIEW.md`](./00-OVERVIEW.md) — contexto, decisiones, qué se conserva y qué no.
2. [`01-LEARNING-PATH.md`](./01-LEARNING-PATH.md) — **Fase 0**. Fundamentos de Go antes de tocar código.
3. [`02-TARGET-STACK.md`](./02-TARGET-STACK.md) — librerías, layout de carpetas, decisiones técnicas.
4. [`03-DATA-MODEL.md`](./03-DATA-MODEL.md) — traducción del schema Prisma → SQLite.
5. [`04-MIGRATION-PLAN.md`](./04-MIGRATION-PLAN.md) — plan de fases con entregables.
6. [`05-MCP-STRATEGY.md`](./05-MCP-STRATEGY.md) — qué hacer con el MCP actual.
7. [`06-DISTRIBUTION.md`](./06-DISTRIBUTION.md) — Homebrew + releases.

## Cómo retomar la próxima sesión

Prompt sugerido para la próxima conversación:

> Leé `docs/go-migration/` en orden y confirmame que entendés el plan. Completé la Fase 0 (tour de Go, Effective Go, Bubbletea tutorials, proyecto sandbox). Arranquemos con Fase 1: scaffold del repo nuevo en `../timer-cli/`.

Si la Fase 0 no está completa, la próxima sesión debería ser sobre **dudas conceptuales** de Go, no sobre código del proyecto.

## Specs de referencia (no migrar hasta leer)

Estos viven en el nivel superior y son la fuente de verdad del dominio:

- [`../API_SPEC.md`](../API_SPEC.md) — reglas de negocio de timers, errores, formato de respuestas.
- [`../DATA_MODEL.md`](../DATA_MODEL.md) — modelo conceptual completo, reglas de transiciones.
- [`../MCP_SPEC.md`](../MCP_SPEC.md) — tools y resources del MCP.
- [`../ARCHITECTURE.md`](../ARCHITECTURE.md) — decisiones de la arquitectura actual (para entender el contexto del que venimos).
