// Package tools holds MCP tool handlers. Each handler is a thin adapter
// over the service layer that formats the result as text for an AI agent.
package tools

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/format"
	"github.com/AngheloAlva/timer/internal/service"
)

// RegisterTimerTools attaches every timer-lifecycle MCP tool to the server.
// Both TimerService and TaskService are needed because start_timer can
// resolve / create tasks by title.
func RegisterTimerTools(
	s *mcpserver.MCPServer,
	timerSvc *service.TimerService,
	taskSvc *service.TaskService,
) {
	s.AddTool(
		mcp.NewTool("active_timer",
			mcp.WithDescription("List the user's currently active timers (running or paused)."),
		),
		activeTimerHandler(timerSvc),
	)

	s.AddTool(
		mcp.NewTool("start_timer",
			mcp.WithDescription("Start a timer for a task. Accepts taskId, OR (projectSlug + taskTitle), OR just taskTitle (projectSlug is auto-inferred from the MCP process cwd via git/basename). Creates the task on the fly when (projectSlug, taskTitle) does not match any existing task. Errors with TIMER_ALREADY_RUNNING_FOR_TASK if a timer is already active on that exact task."),
			mcp.WithString("taskId", mcp.Description("Existing task ID (full UUID or 8+ char prefix).")),
			mcp.WithString("projectSlug", mcp.Description("Project slug. Optional when taskTitle is provided — falls back to cwd auto-detect.")),
			mcp.WithString("taskTitle", mcp.Description("Task title. Used together with projectSlug to find/create the task.")),
		),
		startTimerHandler(timerSvc, taskSvc),
	)

	s.AddTool(
		mcp.NewTool("stop_timer",
			mcp.WithDescription("Stop an active timer and record a time entry. With no filters and exactly one active timer, stops it. Use taskTitle (substring, case-insensitive) or projectSlug to disambiguate when multiple are active. Pass all=true to stop every active timer."),
			mcp.WithString("taskTitle", mcp.Description("Substring filter on task title (case-insensitive).")),
			mcp.WithString("projectSlug", mcp.Description("Restrict to timers in this project.")),
			mcp.WithBoolean("all", mcp.Description("Stop every active timer.")),
		),
		stopTimerHandler(timerSvc),
	)

	s.AddTool(
		mcp.NewTool("pause_timer",
			mcp.WithDescription("Pause an active timer without closing it. Same disambiguation rules as stop_timer. Errors if the matched timer is already paused."),
			mcp.WithString("taskTitle", mcp.Description("Substring filter on task title (case-insensitive).")),
			mcp.WithString("projectSlug", mcp.Description("Restrict to timers in this project.")),
		),
		pauseTimerHandler(timerSvc),
	)

	s.AddTool(
		mcp.NewTool("resume_timer",
			mcp.WithDescription("Resume a paused timer. Same disambiguation rules as stop_timer. Errors if the matched timer is not paused."),
			mcp.WithString("taskTitle", mcp.Description("Substring filter on task title (case-insensitive).")),
			mcp.WithString("projectSlug", mcp.Description("Restrict to timers in this project.")),
		),
		resumeTimerHandler(timerSvc),
	)

	s.AddTool(
		mcp.NewTool("switch_task",
			mcp.WithDescription("Stop every active timer and start a new one on the given task. Same arg shape as start_timer. Useful when the user changes focus."),
			mcp.WithString("taskId", mcp.Description("Existing task ID.")),
			mcp.WithString("projectSlug", mcp.Description("Project slug; optional when taskTitle is provided.")),
			mcp.WithString("taskTitle", mcp.Description("Task title; used with projectSlug or with cwd auto-detect.")),
		),
		switchTaskHandler(timerSvc, taskSvc),
	)
}

// ---------- handlers ----------

func activeTimerHandler(svc *service.TimerService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, _ mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		timers, err := svc.ListActive(ctx)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("list active timers", err), nil
		}
		return mcp.NewToolResultText(formatActiveTimers(timers, time.Now())), nil
	}
}

func startTimerHandler(timerSvc *service.TimerService, taskSvc *service.TaskService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		task, created, err := resolveOrCreateTask(
			ctx, taskSvc,
			mcp.ParseString(req, "taskId", ""),
			mcp.ParseString(req, "projectSlug", ""),
			mcp.ParseString(req, "taskTitle", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		timer, err := timerSvc.Start(ctx, task.ID)
		if err != nil {
			if errors.Is(err, domain.ErrTimerAlreadyRunning) {
				return mcp.NewToolResultError(
					"TIMER_ALREADY_RUNNING_FOR_TASK: ya hay un timer activo en esa tarea.",
				), nil
			}
			return mcp.NewToolResultErrorFromErr("start_timer", err), nil
		}

		return mcp.NewToolResultText(formatStartedTimer(timer, created)), nil
	}
}

func stopTimerHandler(svc *service.TimerService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		if mcp.ParseBoolean(req, "all", false) {
			entries, err := svc.StopAll(ctx)
			if err != nil {
				return mcp.NewToolResultErrorFromErr("stop_timer", err), nil
			}
			return mcp.NewToolResultText(formatStoppedAll(entries)), nil
		}

		f := timerFilters{
			TaskTitle:   mcp.ParseString(req, "taskTitle", ""),
			ProjectSlug: mcp.ParseString(req, "projectSlug", ""),
		}

		active, err := svc.ListActive(ctx)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("stop_timer", err), nil
		}
		target, err := pickActive(active, f, time.Now())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		entry, err := svc.Stop(ctx, target.TaskID)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("stop_timer", err), nil
		}
		return mcp.NewToolResultText(formatStoppedOne(entry)), nil
	}
}

func pauseTimerHandler(svc *service.TimerService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		f := timerFilters{
			TaskTitle:   mcp.ParseString(req, "taskTitle", ""),
			ProjectSlug: mcp.ParseString(req, "projectSlug", ""),
		}
		active, err := svc.ListActive(ctx)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("pause_timer", err), nil
		}
		target, err := pickActive(active, f, time.Now())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		paused, err := svc.Pause(ctx, target.TaskID)
		if err != nil {
			if errors.Is(err, domain.ErrTimerAlreadyPaused) {
				return mcp.NewToolResultError(
					fmt.Sprintf("El timer de %q ya estaba pausado.", target.TaskTitle),
				), nil
			}
			return mcp.NewToolResultErrorFromErr("pause_timer", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(
			"Timer pausado en %q (%s) · %s tracked.",
			paused.TaskTitle, paused.ProjectName, format.Duration(paused.ElapsedSec(time.Now())),
		)), nil
	}
}

func resumeTimerHandler(svc *service.TimerService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		f := timerFilters{
			TaskTitle:   mcp.ParseString(req, "taskTitle", ""),
			ProjectSlug: mcp.ParseString(req, "projectSlug", ""),
		}
		active, err := svc.ListActive(ctx)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("resume_timer", err), nil
		}
		// resume_timer narrows to the paused subset before disambiguating —
		// otherwise picking among running+paused would be confusing.
		paused := make([]domain.Timer, 0, len(active))
		for _, t := range active {
			if t.IsPaused() {
				paused = append(paused, t)
			}
		}
		if len(paused) == 0 {
			return mcp.NewToolResultText("No hay timers pausados."), nil
		}
		target, err := pickActive(paused, f, time.Now())
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		resumed, err := svc.Resume(ctx, target.TaskID)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("resume_timer", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(
			"Timer reanudado en %q (%s).",
			resumed.TaskTitle, resumed.ProjectName,
		)), nil
	}
}

func switchTaskHandler(timerSvc *service.TimerService, taskSvc *service.TaskService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		task, created, err := resolveOrCreateTask(
			ctx, taskSvc,
			mcp.ParseString(req, "taskId", ""),
			mcp.ParseString(req, "projectSlug", ""),
			mcp.ParseString(req, "taskTitle", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		stopped, stopErr := timerSvc.StopAll(ctx)
		if stopErr != nil {
			return mcp.NewToolResultErrorFromErr("switch_task: stop_all", stopErr), nil
		}

		timer, err := timerSvc.Start(ctx, task.ID)
		if err != nil {
			if errors.Is(err, domain.ErrTimerAlreadyRunning) {
				return mcp.NewToolResultError(
					"TIMER_ALREADY_RUNNING_FOR_TASK: ya hay un timer activo en esa tarea (los anteriores ya fueron detenidos).",
				), nil
			}
			return mcp.NewToolResultErrorFromErr("switch_task: start", err), nil
		}

		return mcp.NewToolResultText(formatSwitched(stopped, timer, created)), nil
	}
}

// ---------- formatters ----------

func formatActiveTimers(timers []domain.Timer, now time.Time) string {
	if len(timers) == 0 {
		return "No hay timers activos."
	}

	if len(timers) == 1 {
		t := timers[0]
		var b strings.Builder
		header := "Timer corriendo"
		if t.IsPaused() {
			header = "Timer pausado"
		}
		fmt.Fprintf(&b, "%s · %s tracked:\n", header, format.Duration(t.ElapsedSec(now)))
		fmt.Fprintf(&b, "  Tarea: %s\n", t.TaskTitle)
		fmt.Fprintf(&b, "  Proyecto: %s", t.ProjectName)
		if t.Note != "" {
			fmt.Fprintf(&b, "\n  Nota: %s", t.Note)
		}
		return b.String()
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%d timers activos:\n\n%s", len(timers), renderTimerList(timers, now))
	return b.String()
}

func formatStartedTimer(t domain.Timer, created bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Timer iniciado en %q (proyecto: %s).\n", t.TaskTitle, t.ProjectName)
	fmt.Fprintf(&b, "⏱  Empezó: %s", t.StartedAt.Format("15:04:05"))
	if created {
		fmt.Fprintf(&b, "\n📝 Tarea creada en este momento.")
	}
	return b.String()
}

func formatStoppedOne(e domain.TimeEntry) string {
	return fmt.Sprintf(
		"Timer detenido. Se registraron %s en %q (%s).",
		format.Duration(e.DurationSec), e.TaskTitle, e.ProjectName,
	)
}

func formatStoppedAll(entries []domain.TimeEntry) string {
	if len(entries) == 0 {
		return "No había timers activos."
	}
	var total int64
	for _, e := range entries {
		total += e.DurationSec
	}
	return fmt.Sprintf(
		"Detenidos %d timer(s) (total: %s).",
		len(entries), format.Duration(total),
	)
}

func formatSwitched(stopped []domain.TimeEntry, started domain.Timer, taskCreated bool) string {
	var b strings.Builder
	b.WriteString("Cambiando de tarea:\n")
	if len(stopped) == 0 {
		b.WriteString("  ⏹  (no había timers activos)\n")
	} else {
		var total int64
		for _, e := range stopped {
			total += e.DurationSec
		}
		fmt.Fprintf(&b, "  ⏹  Detenidos %d timer(s) (total: %s)\n", len(stopped), format.Duration(total))
	}
	fmt.Fprintf(&b, "  ▶  %q (%s) → timer activo", started.TaskTitle, started.ProjectName)
	if taskCreated {
		b.WriteString("\n  📝 Tarea creada en este momento.")
	}
	return b.String()
}
