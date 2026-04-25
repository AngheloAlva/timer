package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/format"
	"github.com/AngheloAlva/timer/internal/service"
)

// RegisterTaskTools wires task-level MCP tools.
func RegisterTaskTools(s *mcpserver.MCPServer, svc *service.TaskService) {
	s.AddTool(
		mcp.NewTool("list_tasks",
			mcp.WithDescription("List tasks. Optional filters: projectSlug (only that project) and status (todo|in_progress|done|archived). Without status, done/archived tasks are hidden."),
			mcp.WithString("projectSlug", mcp.Description("Restrict to tasks in this project.")),
			mcp.WithString("status", mcp.Enum("todo", "in_progress", "done", "archived"), mcp.Description("Filter by task status.")),
		),
		listTasksHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("create_task",
			mcp.WithDescription("Create a new task in the given project. Returns the new task with its short ID."),
			mcp.WithString("projectSlug", mcp.Required(), mcp.Description("Slug of the project that owns the task.")),
			mcp.WithString("title", mcp.Required(), mcp.Description("Task title.")),
		),
		createTaskHandler(svc),
	)
}

func listTasksHandler(svc *service.TaskService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectSlug := strings.TrimSpace(mcp.ParseString(req, "projectSlug", ""))
		statusFilter := strings.TrimSpace(mcp.ParseString(req, "status", ""))

		// includeDone=true whenever the caller asked for a specific status,
		// so we can apply the filter ourselves. Otherwise hide done/archived
		// tasks (the same default the CLI uses).
		tasks, err := svc.List(ctx, projectSlug, statusFilter != "")
		if err != nil {
			return mcp.NewToolResultErrorFromErr("list_tasks", err), nil
		}

		if statusFilter != "" {
			filtered := tasks[:0]
			for _, t := range tasks {
				if string(t.Status) == statusFilter {
					filtered = append(filtered, t)
				}
			}
			tasks = filtered
		}

		return mcp.NewToolResultText(formatTaskList(tasks, projectSlug, statusFilter)), nil
	}
}

func createTaskHandler(svc *service.TaskService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		projectSlug := strings.TrimSpace(mcp.ParseString(req, "projectSlug", ""))
		title := strings.TrimSpace(mcp.ParseString(req, "title", ""))
		if projectSlug == "" {
			return mcp.NewToolResultError("projectSlug is required"), nil
		}
		if title == "" {
			return mcp.NewToolResultError("title is required"), nil
		}
		t, err := svc.Create(ctx, projectSlug, title)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("create_task", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(
			"Tarea creada: %q en %s (id: %s).",
			t.Title, t.ProjectName, format.ShortID(t.ID),
		)), nil
	}
}

func formatTaskList(tasks []domain.Task, projectSlug, status string) string {
	header := "Tareas"
	if projectSlug != "" {
		header = fmt.Sprintf("Tareas de %q", projectSlug)
	}
	if status != "" {
		header = fmt.Sprintf("%s (status: %s)", header, status)
	}

	if len(tasks) == 0 {
		return header + ": (vacío)"
	}

	var b strings.Builder
	b.WriteString(header + ":\n")

	maxTitle := 0
	for _, t := range tasks {
		if len(t.Title) > maxTitle {
			maxTitle = len(t.Title)
		}
	}

	for _, t := range tasks {
		extra := ""
		if t.ExternalRef != "" {
			extra = fmt.Sprintf("  [%s]", t.ExternalRef)
		}
		// Show project column only when listing across all projects.
		projectCol := ""
		if projectSlug == "" && t.ProjectSlug != "" {
			projectCol = fmt.Sprintf("  (%s)", t.ProjectSlug)
		}
		fmt.Fprintf(&b, "\n  • %s  %-*s  %s%s%s",
			format.ShortID(t.ID), maxTitle, t.Title, t.Status, extra, projectCol)
	}
	return b.String()
}
