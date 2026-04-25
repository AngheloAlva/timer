// Package mcp wires the MCP (Model Context Protocol) server that exposes
// the timer service layer to AI agents over stdio.
//
// The server is a thin adapter on top of the same services the CLI and TUI
// use — there is no business logic here. Tool and resource handlers live
// in subpackages under tools/ and resources/.
package mcp

import (
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/AngheloAlva/timer/internal/mcp/resources"
	"github.com/AngheloAlva/timer/internal/mcp/tools"
	"github.com/AngheloAlva/timer/internal/service"
	"github.com/AngheloAlva/timer/internal/version"
)

// NewServer builds the MCP server with every tool and resource registered.
// Caller serves it (typically with mcpserver.ServeStdio).
func NewServer(
	projectSvc *service.ProjectService,
	taskSvc *service.TaskService,
	timerSvc *service.TimerService,
) *mcpserver.MCPServer {
	s := mcpserver.NewMCPServer(
		"timer",
		version.Version,
		mcpserver.WithToolCapabilities(true),
		mcpserver.WithResourceCapabilities(true, true),
	)

	tools.RegisterTimerTools(s, timerSvc, taskSvc)
	tools.RegisterTaskTools(s, taskSvc)
	tools.RegisterProjectTools(s, projectSvc)
	tools.RegisterReportTools(s, timerSvc, taskSvc)

	resources.Register(s, timerSvc, projectSvc)

	return s
}
