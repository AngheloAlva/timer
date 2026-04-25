package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/format"
	"github.com/AngheloAlva/timer/internal/mcp/projectdetect"
	"github.com/AngheloAlva/timer/internal/service"
)

// RegisterProjectTools wires project-level MCP tools.
func RegisterProjectTools(s *mcpserver.MCPServer, svc *service.ProjectService) {
	s.AddTool(
		mcp.NewTool("list_projects",
			mcp.WithDescription("List the user's projects. Marks the project whose slug matches the auto-detected cwd as '(este proyecto)' so the agent can preselect it."),
			mcp.WithBoolean("includeArchived", mcp.Description("Include archived projects in the result. Default false.")),
		),
		listProjectsHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("create_project",
			mcp.WithDescription("Create a new project. The slug is auto-derived from the name (lowercase, alphanumeric, dashes)."),
			mcp.WithString("name", mcp.Required(), mcp.Description("Human-readable project name.")),
		),
		createProjectHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("archive_project",
			mcp.WithDescription("Archive a project (soft delete). The project is hidden from the default list but tasks and time entries are preserved. If any task has a running timer, it is closed atomically before the archive. Idempotent."),
			mcp.WithString("slug", mcp.Required(), mcp.Description("Slug of the project to archive.")),
		),
		archiveProjectHandler(svc),
	)

	s.AddTool(
		mcp.NewTool("delete_project",
			mcp.WithDescription("HARD-delete a project AND every task, timer, and time entry under it. IRREVERSIBLE. By default refuses unless the project is already archived — call archive_project first, or pass force=true to bypass and accept the data loss. Requires confirm=true on every call as a safety check."),
			mcp.WithString("slug", mcp.Required(), mcp.Description("Slug of the project to delete.")),
			mcp.WithBoolean("confirm", mcp.Required(), mcp.Description("Must be true to actually delete. Safety guard against accidental calls.")),
			mcp.WithBoolean("force", mcp.Description("If true, skip the 'must be archived' guard. Default false.")),
		),
		deleteProjectHandler(svc),
	)
}

func listProjectsHandler(svc *service.ProjectService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		includeArchived := mcp.ParseBoolean(req, "includeArchived", false)
		projects, err := svc.List(ctx, includeArchived)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("list_projects", err), nil
		}
		cwdSlug := projectdetect.Detect("").InferredSlug
		return mcp.NewToolResultText(formatProjectList(projects, cwdSlug)), nil
	}
}

func createProjectHandler(svc *service.ProjectService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		name := strings.TrimSpace(mcp.ParseString(req, "name", ""))
		if name == "" {
			return mcp.NewToolResultError("name is required"), nil
		}
		p, err := svc.Create(ctx, name)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("create_project", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(
			"Proyecto creado: %s (slug: %s).", p.Name, p.Slug,
		)), nil
	}
}

func archiveProjectHandler(svc *service.ProjectService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug := strings.TrimSpace(mcp.ParseString(req, "slug", ""))
		if slug == "" {
			return mcp.NewToolResultError("slug is required"), nil
		}
		res, err := svc.Archive(ctx, slug)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("archive_project", err), nil
		}
		if res.AlreadyArchived {
			return mcp.NewToolResultText(fmt.Sprintf(
				"El proyecto %q (%s) ya estaba archivado.", res.Project.Name, res.Project.Slug,
			)), nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "Proyecto archivado: %s (%s).", res.Project.Name, res.Project.Slug)
		for _, e := range res.ClosedEntries {
			fmt.Fprintf(&b, "\n  • timer cerrado en %q → %s", e.TaskTitle, format.Duration(e.DurationSec))
		}
		return mcp.NewToolResultText(b.String()), nil
	}
}

func deleteProjectHandler(svc *service.ProjectService) mcpserver.ToolHandlerFunc {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		slug := strings.TrimSpace(mcp.ParseString(req, "slug", ""))
		if slug == "" {
			return mcp.NewToolResultError("slug is required"), nil
		}
		if !mcp.ParseBoolean(req, "confirm", false) {
			return mcp.NewToolResultError("confirm must be true to delete a project (safety guard)"), nil
		}
		force := mcp.ParseBoolean(req, "force", false)

		res, err := svc.Delete(ctx, slug, force)
		if err != nil {
			return mcp.NewToolResultErrorFromErr("delete_project", err), nil
		}
		return mcp.NewToolResultText(fmt.Sprintf(
			"Proyecto eliminado: %s (%s). Se borraron %d tarea(s), %d entrada(s) de tiempo y %d timer(s) activo(s).",
			res.Project.Name, res.Project.Slug,
			res.TaskCount, res.TimeEntryCount, res.ActiveTimerCount,
		)), nil
	}
}

func formatProjectList(projects []domain.Project, cwdSlug string) string {
	if len(projects) == 0 {
		return "No tenés proyectos. Creá uno con create_project."
	}

	maxSlug := 0
	for _, p := range projects {
		if len(p.Slug) > maxSlug {
			maxSlug = len(p.Slug)
		}
	}

	var b strings.Builder
	b.WriteString("Tus proyectos:\n")
	for _, p := range projects {
		hint := ""
		if cwdSlug != "" && p.Slug == cwdSlug {
			hint = " (este proyecto)"
		}
		archived := ""
		if p.Archived {
			archived = " [archivado]"
		}
		fmt.Fprintf(&b, "\n  • %-*s  → %s%s%s", maxSlug, p.Slug, p.Name, hint, archived)
	}
	return b.String()
}
