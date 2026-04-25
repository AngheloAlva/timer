// Package resources holds MCP resource handlers.
//
// Resources are read-only views of the timer state, returned as JSON for
// AI agents to consume programmatically (vs the human-readable text the
// tools return). The URI scheme follows MCP_SPEC.md:
//
//	timer://active-timers   → all running/paused timers
//	timer://today           → time summary for today
//	timer://projects        → list of projects
package resources

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/service"
)

const mimeJSON = "application/json"

// Register attaches every read-only resource to the server.
func Register(
	s *mcpserver.MCPServer,
	timerSvc *service.TimerService,
	projectSvc *service.ProjectService,
) {
	s.AddResource(
		mcp.NewResource("timer://active-timers", "Active timers",
			mcp.WithMIMEType(mimeJSON)),
		activeTimersHandler(timerSvc),
	)
	s.AddResource(
		mcp.NewResource("timer://today", "Today's time summary",
			mcp.WithMIMEType(mimeJSON)),
		todayHandler(timerSvc),
	)
	s.AddResource(
		mcp.NewResource("timer://projects", "Projects",
			mcp.WithMIMEType(mimeJSON)),
		projectsHandler(projectSvc),
	)
}

// ---------- handlers ----------

type activeTimerJSON struct {
	ID          string `json:"id"`
	TaskID      string `json:"taskId"`
	TaskTitle   string `json:"taskTitle"`
	ProjectName string `json:"projectName"`
	ProjectSlug string `json:"projectSlug"`
	StartedAt   string `json:"startedAt"`
	ElapsedSec  int64  `json:"elapsedSec"`
	Note        string `json:"note,omitempty"`
	IsPaused    bool   `json:"isPaused"`
}

func activeTimersHandler(svc *service.TimerService) mcpserver.ResourceHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		timers, err := svc.ListActive(ctx)
		if err != nil {
			return nil, fmt.Errorf("list active: %w", err)
		}
		now := time.Now()
		out := make([]activeTimerJSON, 0, len(timers))
		for _, t := range timers {
			out = append(out, activeTimerJSON{
				ID:          t.ID,
				TaskID:      t.TaskID,
				TaskTitle:   t.TaskTitle,
				ProjectName: t.ProjectName,
				ProjectSlug: t.ProjectSlug,
				StartedAt:   t.StartedAt.UTC().Format(time.RFC3339),
				ElapsedSec:  t.ElapsedSec(now),
				Note:        t.Note,
				IsPaused:    t.IsPaused(),
			})
		}
		return jsonContents(req.Params.URI, out)
	}
}

type taskTotalJSON struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	TotalSec   int64  `json:"totalSec"`
}

type projectTotalJSON struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Slug     string          `json:"slug"`
	TotalSec int64           `json:"totalSec"`
	Tasks    []taskTotalJSON `json:"tasks"`
}

type todayJSON struct {
	Date     string             `json:"date"` // YYYY-MM-DD in local time
	TotalSec int64              `json:"totalSec"`
	Projects []projectTotalJSON `json:"projects"`
}

func todayHandler(svc *service.TimerService) mcpserver.ResourceHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		now := time.Now()
		report, err := svc.BuildReport(ctx, service.ListEntriesOpts{
			MinStartedAt: service.StartOfDay(now),
		})
		if err != nil {
			return nil, fmt.Errorf("build report: %w", err)
		}

		projects := make([]projectTotalJSON, 0, len(report.Projects))
		for _, p := range report.Projects {
			tasks := make([]taskTotalJSON, 0, len(p.Tasks))
			for _, t := range p.Tasks {
				tasks = append(tasks, taskTotalJSON{
					ID: t.ID, Title: t.Title, TotalSec: t.Total,
				})
			}
			projects = append(projects, projectTotalJSON{
				ID: p.ID, Name: p.Name, Slug: p.Slug,
				TotalSec: p.Total, Tasks: tasks,
			})
		}

		return jsonContents(req.Params.URI, todayJSON{
			Date:     now.Format("2006-01-02"),
			TotalSec: report.Total,
			Projects: projects,
		})
	}
}

type projectJSON struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Color    string `json:"color,omitempty"`
	Archived bool   `json:"archived"`
}

func projectsHandler(svc *service.ProjectService) mcpserver.ResourceHandlerFunc {
	return func(ctx context.Context, req mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		projects, err := svc.List(ctx, true)
		if err != nil {
			return nil, fmt.Errorf("list projects: %w", err)
		}
		out := make([]projectJSON, 0, len(projects))
		for _, p := range projects {
			out = append(out, projectFromDomain(p))
		}
		return jsonContents(req.Params.URI, out)
	}
}

func projectFromDomain(p domain.Project) projectJSON {
	return projectJSON{
		ID: p.ID, Name: p.Name, Slug: p.Slug, Color: p.Color, Archived: p.Archived,
	}
}

// jsonContents marshals payload and wraps it as a single TextResourceContents
// at the requested URI. The protocol allows multiple contents per resource;
// JSON resources only need one.
func jsonContents(uri string, payload any) ([]mcp.ResourceContents, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: mimeJSON,
			Text:     string(body),
		},
	}, nil
}
