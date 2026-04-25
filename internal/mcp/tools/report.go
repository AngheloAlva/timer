package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/format"
	"github.com/AngheloAlva/timer/internal/service"
)

// RegisterReportTools wires log_time + get_summary.
func RegisterReportTools(
	s *mcpserver.MCPServer,
	timerSvc *service.TimerService,
	taskSvc *service.TaskService,
) {
	s.AddTool(
		mcp.NewTool("log_time",
			mcp.WithDescription("Record a manual retroactive time entry without using the timer (e.g. user forgot to start). Accepts the same task arg shape as start_timer (taskId, OR projectSlug+taskTitle, OR taskTitle with cwd auto-detect)."),
			mcp.WithString("taskId", mcp.Description("Existing task ID (full UUID or 8+ char prefix).")),
			mcp.WithString("projectSlug", mcp.Description("Project slug. Optional when taskTitle is provided — falls back to cwd auto-detect.")),
			mcp.WithString("taskTitle", mcp.Description("Task title. Used together with projectSlug to find/create the task.")),
			mcp.WithString("startedAt", mcp.Required(), mcp.Description("ISO 8601 start timestamp (e.g. 2026-04-21T15:00:00Z or 2026-04-21T12:00:00-03:00).")),
			mcp.WithString("endedAt", mcp.Required(), mcp.Description("ISO 8601 end timestamp. Must be strictly after startedAt.")),
			mcp.WithString("note", mcp.Description("Optional note attached to the entry.")),
		),
		logTimeHandler(timerSvc, taskSvc),
	)

	s.AddTool(
		mcp.NewTool("get_summary",
			mcp.WithDescription("Time tracked grouped by project, then by task. Choose a preset range or pass custom with from/to."),
			mcp.WithString("range", mcp.Required(),
				mcp.Enum("today", "yesterday", "this_week", "last_week", "custom"),
				mcp.Description("Time window to summarise.")),
			mcp.WithString("from", mcp.Description("ISO 8601 lower bound, only used with range=custom.")),
			mcp.WithString("to", mcp.Description("ISO 8601 upper bound (exclusive), only used with range=custom.")),
			mcp.WithString("projectSlug", mcp.Description("Restrict to entries in this project.")),
		),
		getSummaryHandler(timerSvc),
	)
}

func logTimeHandler(timerSvc *service.TimerService, taskSvc *service.TaskService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		startedAt, err := parseISO(mcp.ParseString(req, "startedAt", ""))
		if err != nil {
			return mcp.NewToolResultError("startedAt: " + err.Error()), nil
		}
		endedAt, err := parseISO(mcp.ParseString(req, "endedAt", ""))
		if err != nil {
			return mcp.NewToolResultError("endedAt: " + err.Error()), nil
		}

		task, created, err := resolveOrCreateTask(
			ctx, taskSvc,
			mcp.ParseString(req, "taskId", ""),
			mcp.ParseString(req, "projectSlug", ""),
			mcp.ParseString(req, "taskTitle", ""),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		entry, err := timerSvc.LogEntry(
			ctx, task.ID, startedAt, endedAt,
			strings.TrimSpace(mcp.ParseString(req, "note", "")),
			domain.SourceMCP,
		)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("log_time", err), nil
		}

		return mcp.NewToolResultText(formatLoggedEntry(entry, task, created)), nil
	}
}

func getSummaryHandler(svc *service.TimerService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		rng := strings.TrimSpace(mcp.ParseString(req, "range", ""))
		if rng == "" {
			return mcp.NewToolResultError("range is required (today | yesterday | this_week | last_week | custom)"), nil
		}

		from, to, label, err := resolveRange(rng,
			mcp.ParseString(req, "from", ""),
			mcp.ParseString(req, "to", ""),
			time.Now(),
		)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		projectSlug := strings.TrimSpace(mcp.ParseString(req, "projectSlug", ""))

		entries, err := svc.ListEntries(ctx, service.ListEntriesOpts{
			MinStartedAt: from,
			ProjectSlug:  projectSlug,
			Max:          10000,
		})
		if err != nil {
			return mcp.NewToolResultErrorFromErr("get_summary", err), nil
		}

		// Apply the upper bound in-memory: ListEntries only enforces the
		// lower bound. today / this_week leave `to` zero (no upper bound).
		if !to.IsZero() {
			upperMs := to.UnixMilli()
			kept := entries[:0]
			for _, e := range entries {
				if e.StartedAt.UnixMilli() < upperMs {
					kept = append(kept, e)
				}
			}
			entries = kept
		}

		return mcp.NewToolResultText(formatSummary(service.AggregateEntries(entries), label)), nil
	}
}

// parseISO accepts RFC3339 (with or without seconds, with timezone offset
// or Z). Any other format is rejected — better than silently parsing only
// part of the input.
func parseISO(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("required ISO 8601 timestamp is empty")
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	if t, err := time.Parse("2006-01-02T15:04:05", s); err == nil {
		// Naive timestamp — assume local time. Document this in the tool desc.
		return t, nil
	}
	return time.Time{}, fmt.Errorf("not a valid ISO 8601 timestamp: %q", s)
}

// resolveRange maps the named ranges to (from, to, label). Empty `to` means
// "no upper bound" (today / this_week). label is human-readable for output.
func resolveRange(name, fromArg, toArg string, now time.Time) (from, to time.Time, label string, err error) {
	switch name {
	case "today":
		return service.StartOfDay(now), time.Time{}, "Hoy", nil
	case "yesterday":
		sod := service.StartOfDay(now)
		return sod.AddDate(0, 0, -1), sod, "Ayer", nil
	case "this_week":
		return service.StartOfISOWeek(now), time.Time{}, "Esta semana", nil
	case "last_week":
		thisWk := service.StartOfISOWeek(now)
		return thisWk.AddDate(0, 0, -7), thisWk, "Semana pasada", nil
	case "custom":
		f, err := parseISO(fromArg)
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("from: %w", err)
		}
		t, err := parseISO(toArg)
		if err != nil {
			return time.Time{}, time.Time{}, "", fmt.Errorf("to: %w", err)
		}
		if !t.After(f) {
			return time.Time{}, time.Time{}, "", fmt.Errorf("to must be strictly after from")
		}
		return f, t, fmt.Sprintf("%s → %s", f.Format("2006-01-02 15:04"), t.Format("2006-01-02 15:04")), nil
	default:
		return time.Time{}, time.Time{}, "", fmt.Errorf("unknown range %q", name)
	}
}

// ---------- formatters ----------

func formatLoggedEntry(e domain.TimeEntry, task domain.Task, taskCreated bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Entrada registrada: %s en %q (%s).",
		format.Duration(e.DurationSec), task.Title, task.ProjectName)
	fmt.Fprintf(&b, "\n  %s → %s",
		e.StartedAt.Format("2006-01-02 15:04"),
		e.EndedAt.Format("15:04"))
	if e.Note != "" {
		fmt.Fprintf(&b, "\n  📝 %s", e.Note)
	}
	if taskCreated {
		b.WriteString("\n  📝 Tarea creada en este momento.")
	}
	return b.String()
}

func formatSummary(r service.ReportSummary, label string) string {
	if r.Total == 0 {
		return fmt.Sprintf("Resumen — %s\n\nSin tiempo registrado.", label)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Resumen — %s\n\nTotal: %s\n", label, format.Duration(r.Total))

	// Per-project breakdown with percentage.
	if len(r.Projects) > 0 {
		b.WriteString("\nPor proyecto:")
		maxName := 0
		for _, p := range r.Projects {
			if len(p.Name) > maxName {
				maxName = len(p.Name)
			}
		}
		for _, p := range r.Projects {
			pct := 0
			if r.Total > 0 {
				pct = int((p.Total * 100) / r.Total)
			}
			fmt.Fprintf(&b, "\n  %-*s  %s  (%d%%)",
				maxName, p.Name, format.Duration(p.Total), pct)
		}
	}
	return b.String()
}
