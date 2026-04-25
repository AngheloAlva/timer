package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AngheloAlva/timer/internal/domain"
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
