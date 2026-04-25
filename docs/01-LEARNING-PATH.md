# Fase 0 — Learning Path

> No tocar el proyecto Go hasta completar esta fase. Si lo hacés antes, vas a escribir "TypeScript con sintaxis de Go" y el resultado va a ser malo.

## Por qué esta fase existe

Go piensa distinto a TypeScript. Los errores típicos del que viene de TS y arranca a tirar código sin estudiar:

1. **Punteros mal usados.** `*string` cuando `string` alcanza. `*Struct` pasado a todos lados "por si acaso".
2. **Interfaces gigantes.** En Go las interfaces son chiquitas (1-2 métodos). `io.Reader` tiene UN método. `io.Writer` tiene UNO. Eso es idiomático.
3. **`panic` como si fuera `throw`.** `panic` en Go es para errores irrecuperables (index out of bounds, nil dereference). Los errores de negocio se devuelven con `error`, no se lanzan.
4. **Goroutines sin channels.** Arrancar goroutines con estado compartido y `sync.Mutex` por todos lados cuando un channel lo resuelve limpio.
5. **Buscar un "NestJS de Go".** No existe. El stdlib (`net/http`, `database/sql`, `encoding/json`) ES el framework. Hay routers (chi, gin), pero no hay DI containers de verdad y no hacen falta.

## Checklist — no se pasa a Fase 1 hasta completar TODO esto

### 1. Tour oficial de Go — 2-3 días

https://go.dev/tour/

Completo. No salteás. Al terminar deberías poder explicar sin dudar:
- Diferencia entre `var x int` vs `x := 0`.
- Cuándo `T` y cuándo `*T` (value receivers vs pointer receivers).
- Qué es un `interface` en Go (spoiler: satisfacción estructural, no `implements`).
- Qué es un `channel` y qué diferencia hay entre buffered y unbuffered.
- Cómo se maneja un error (retornar `(T, error)`, chequear `if err != nil`).

### 2. Effective Go — 1 día

https://go.dev/doc/effective_go

Lectura entera de una vez. Te va a caer la ficha sobre naming conventions, package design, cuándo usar qué. Después lo volvés a consultar como referencia.

### 3. Learning Go (Jon Bodner, 2ª ed.) — capítulos 1-8

Es el mejor libro para alguien que viene de otro lenguaje. No hay atajo.

Capítulos críticos:
- Cap 3: Composite Types (slices, maps, structs) — cómo piensa memoria Go.
- Cap 6: Pointers — cuándo sí, cuándo no.
- Cap 7: Types, Methods, Interfaces — EL capítulo.
- Cap 8: Errors — error wrapping con `%w`, `errors.Is`, `errors.As`.

### 4. Concurrencia — 1 día

- Video: **"Rob Pike — Concurrency is not Parallelism"** (YouTube, 30 min). Obligatorio.
- Después: https://go.dev/blog/pipelines — pipelines con goroutines + channels.

Al final deberías poder responder: "¿cómo hago un timer que corre en background mientras el TUI sigue respondiendo a inputs?" La respuesta usa goroutines + channels. Si no sabés cómo, no estás listo.

### 5. Bubbletea — 2 días

https://github.com/charmbracelet/bubbletea/tree/master/tutorials

Orden:
1. `tutorials/basics` — el modelo Update/View.
2. `tutorials/commands` — comandos async (I/O sin bloquear el UI).
3. Después leer 2-3 ejemplos de `bubbletea/examples/` — especialmente `list-default`, `table`, `textinput`.

Conceptos a tener ASIMILADOS antes de pasar a Fase 1:
- **The Elm Architecture**: `Model` + `Update(msg) → Model` + `View() → string`. Todo lo que pasa es un `Msg`.
- **Commands (`tea.Cmd`)**: operaciones async (HTTP, timers, lectura de archivos) que devuelven un `Msg` cuando terminan.
- Por qué **no se hace I/O en el `Update`** directamente — se devuelve un `Cmd`.

### 6. Proyecto sandbox — 3-4 días

**Antes de tocar el timer real**, escribí un proyecto chiquito que ejercite todo lo anterior. Sugerencia:

**`notes-cli`** — un gestor de notas minimalista:
- `notes add "texto"` — agrega una nota (comando CLI con `cobra` o `flag`).
- `notes list` — lista tabla en stdout (usar `lipgloss` o `go-pretty`).
- `notes tui` — abre TUI con Bubbletea para navegar/editar (list + textarea).
- Storage: SQLite con `database/sql` crudo + `mattn/go-sqlite3`. **Sin ORM.**
- Tests: `go test` con al menos 3 tests del storage layer.

Si podés escribir esto sin googlear cada 5 minutos, estás listo para el timer.

## Material complementario (opcional pero recomendado)

- **"Go by Example"**: https://gobyexample.com/ — referencia rápida con snippets.
- **"100 Go Mistakes and How to Avoid Them"** (Teiva Harsanyi) — libro que te salva de errores típicos cuando ya sabés lo básico.
- **Charm ecosystem deep dive**: ver `charmbracelet/lipgloss`, `charmbracelet/bubbles` (componentes reusables), `charmbracelet/glamour` (markdown rendering).

## Criterio de "listo"

Podés pasar a Fase 1 cuando puedas explicar sin dudar, en voz alta:

1. Cómo modelar un timer activo en Go sin race conditions (goroutine + channel, no mutex).
2. Cómo Bubbletea renderiza sin parpadeo (diffing del view + alt screen).
3. Por qué no uso un ORM en este proyecto (SQL crudo + `sqlc` si quiero type-safety).
4. Cuándo una función devuelve `*Timer` vs `Timer` (regla: pointer si la función muta o si la struct es grande; value si es inmutable y chica).
5. Cómo se testea código con SQLite embebido (DB en memoria o archivo temporal por test, `t.TempDir()`).

Si alguna de estas te cuesta, volvé al material hasta que no te cueste.

## Qué NO hacer en Fase 0

- ❌ Abrir el proyecto nuevo y empezar a tirar código "para ir calentando".
- ❌ Pedirle a Claude que te genere scaffolding antes de entender el stdlib.
- ❌ Elegir frameworks grandes (Gin, Echo, Fiber) porque "se ven parecidos a Nest". No los necesitás.
- ❌ Usar GORM o Ent "para ahorrar tiempo". SQL crudo primero, ORM después si duele.
