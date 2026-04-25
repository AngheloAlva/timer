package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/format"
	"github.com/AngheloAlva/timer/internal/mcp/projectdetect"
	"github.com/AngheloAlva/timer/internal/service"
)

// timerFilters are the common parameters on stop/pause/resume tools.
// Empty fields mean "no filter".
type timerFilters struct {
	TaskTitle   string // substring, case-insensitive
	ProjectSlug string // exact match
}

func (f timerFilters) matches(t domain.Timer) bool {
	if f.ProjectSlug != "" && t.ProjectSlug != f.ProjectSlug {
		return false
	}
	if f.TaskTitle != "" {
		needle := strings.ToLower(f.TaskTitle)
		hay := strings.ToLower(t.TaskTitle)
		if !strings.Contains(hay, needle) {
			return false
		}
	}
	return true
}

// pickActive picks exactly one active timer using the filters.
//
// Behaviour follows MCP_SPEC.md:
//   - 0 active timers → "no hay timers activos"
//   - 1 active timer, no filters → return it
//   - filters match exactly 1 → return it
//   - multiple matches (or 0 with filters) → ambiguity error with the
//     candidate list rendered for the agent
func pickActive(timers []domain.Timer, f timerFilters, now time.Time) (domain.Timer, error) {
	if len(timers) == 0 {
		return domain.Timer{}, errors.New("no hay timers activos")
	}

	matches := make([]domain.Timer, 0, len(timers))
	for _, t := range timers {
		if f.matches(t) {
			matches = append(matches, t)
		}
	}

	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return domain.Timer{}, fmt.Errorf(
			"ningún timer activo coincide con los filtros. Timers activos:\n%s",
			renderTimerList(timers, now),
		)
	default:
		return domain.Timer{}, fmt.Errorf(
			"hay %d timers activos y no pude desambiguar. Especificá taskTitle o projectSlug:\n%s",
			len(matches), renderTimerList(matches, now),
		)
	}
}

// renderTimerList formats a list of active timers using the spec markers
// (▶ running, ⏸ paused). Prefixed with two spaces and a leading newline so
// it can be appended to error/success messages directly.
func renderTimerList(timers []domain.Timer, now time.Time) string {
	var b strings.Builder
	for _, t := range timers {
		marker := "▶"
		if t.IsPaused() {
			marker = "⏸"
		}
		fmt.Fprintf(&b, "\n  %s %s (%s) · %s",
			marker, t.TaskTitle, t.ProjectName, format.Duration(t.ElapsedSec(now)))
	}
	return strings.TrimLeft(b.String(), "\n")
}

// resolveOrCreateTask maps the start_timer / log_time / switch_task argument
// shape (taskId | projectSlug+taskTitle | taskTitle+cwd-detect) into a
// concrete domain.Task with denormalized project info, creating it on the
// fly when (projectSlug, taskTitle) does not match anything.
//
// Returns the task and a flag indicating whether a fresh task was created
// so the caller can mention it in the response.
func resolveOrCreateTask(
	ctx context.Context,
	taskSvc *service.TaskService,
	taskID, projectSlug, taskTitle string,
) (domain.Task, bool, error) {
	taskID = strings.TrimSpace(taskID)
	taskTitle = strings.TrimSpace(taskTitle)
	projectSlug = strings.TrimSpace(projectSlug)

	// taskId path: scan all tasks (the only list query that carries
	// denormalized project info) and match by exact ID or unambiguous prefix.
	if taskID != "" {
		all, err := taskSvc.List(ctx, "", true)
		if err != nil {
			return domain.Task{}, false, fmt.Errorf("listar tareas: %w", err)
		}
		var matches []domain.Task
		for _, t := range all {
			if t.ID == taskID || strings.HasPrefix(t.ID, taskID) {
				matches = append(matches, t)
			}
		}
		switch len(matches) {
		case 0:
			return domain.Task{}, false, fmt.Errorf("no task matches id %q", taskID)
		case 1:
			return matches[0], false, nil
		default:
			return domain.Task{}, false, fmt.Errorf("ambiguous task id %q: matches %d tasks", taskID, len(matches))
		}
	}

	if taskTitle == "" {
		return domain.Task{}, false, errors.New("se requiere taskId, o (projectSlug + taskTitle), o taskTitle (con auto-detect del cwd)")
	}

	if projectSlug == "" {
		d := projectdetect.Detect("")
		if d.InferredSlug == "" {
			return domain.Task{}, false, errors.New("no se pudo inferir un proyecto desde el cwd; pasá projectSlug explícito")
		}
		projectSlug = d.InferredSlug
	}

	tasks, err := taskSvc.List(ctx, projectSlug, true)
	if err != nil {
		return domain.Task{}, false, fmt.Errorf("listar tareas del proyecto %q: %w", projectSlug, err)
	}

	needle := strings.ToLower(taskTitle)
	for _, t := range tasks {
		if strings.ToLower(t.Title) == needle {
			return t, false, nil
		}
	}

	created, err := taskSvc.Create(ctx, projectSlug, taskTitle)
	if err != nil {
		return domain.Task{}, false, fmt.Errorf("crear tarea: %w", err)
	}
	return created, true, nil
}
