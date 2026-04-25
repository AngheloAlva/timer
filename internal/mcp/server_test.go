package mcp

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/AngheloAlva/timer/internal/service"
	"github.com/AngheloAlva/timer/internal/storage/gen"
	"github.com/AngheloAlva/timer/internal/storage/sqlite"
)

// testServer wires a fresh DB + every service + an in-process MCP client
// connected to NewServer. Closes everything via t.Cleanup.
type testServer struct {
	Client     *client.Client
	ProjectSvc *service.ProjectService
	TaskSvc    *service.TaskService
	TimerSvc   *service.TimerService
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	db, err := sqlite.Open(filepath.Join(t.TempDir(), "timer.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q := gen.New(db.Conn())
	projectSvc := service.NewProjectService(q)
	taskSvc := service.NewTaskService(db.Conn(), q)
	timerSvc := service.NewTimerService(db.Conn(), q)

	srv := NewServer(projectSvc, taskSvc, timerSvc)

	c, err := client.NewInProcessClient(srv)
	if err != nil {
		t.Fatalf("inprocess client: %v", err)
	}
	t.Cleanup(func() { _ = c.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Start(ctx); err != nil {
		t.Fatalf("client start: %v", err)
	}

	initReq := mcp.InitializeRequest{}
	initReq.Params.ProtocolVersion = "2024-11-05"
	initReq.Params.ClientInfo = mcp.Implementation{Name: "integration-test", Version: "0"}
	if _, err := c.Initialize(ctx, initReq); err != nil {
		t.Fatalf("initialize: %v", err)
	}

	return &testServer{
		Client:     c,
		ProjectSvc: projectSvc,
		TaskSvc:    taskSvc,
		TimerSvc:   timerSvc,
	}
}

// callTool is a tiny helper that returns the text content (or fails the
// test on RPC errors / non-text content).
func (ts *testServer) callTool(t *testing.T, name string, args map[string]any) (text string, isError bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	res, err := ts.Client.CallTool(ctx, req)
	if err != nil {
		t.Fatalf("call %s: %v", name, err)
	}
	if len(res.Content) == 0 {
		t.Fatalf("call %s: no content", name)
	}
	tc, ok := res.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("call %s: expected TextContent, got %T", name, res.Content[0])
	}
	return tc.Text, res.IsError
}

func (ts *testServer) readResource(t *testing.T, uri string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := mcp.ReadResourceRequest{}
	req.Params.URI = uri
	res, err := ts.Client.ReadResource(ctx, req)
	if err != nil {
		t.Fatalf("read %s: %v", uri, err)
	}
	if len(res.Contents) == 0 {
		t.Fatalf("read %s: no contents", uri)
	}
	tc, ok := res.Contents[0].(mcp.TextResourceContents)
	if !ok {
		t.Fatalf("read %s: expected TextResourceContents, got %T", uri, res.Contents[0])
	}
	return tc.Text
}

// --- tests ---

func TestToolsList(t *testing.T) {
	ts := newTestServer(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := ts.Client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		t.Fatalf("list tools: %v", err)
	}

	want := map[string]bool{
		"active_timer": false, "start_timer": false, "stop_timer": false,
		"pause_timer": false, "resume_timer": false, "switch_task": false,
		"create_task": false, "list_tasks": false,
		"create_project": false, "list_projects": false,
		"log_time": false, "get_summary": false,
	}
	for _, tool := range res.Tools {
		if _, ok := want[tool.Name]; ok {
			want[tool.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("tool %q not registered", name)
		}
	}
}

func TestTimerLifecycleEndToEnd(t *testing.T) {
	ts := newTestServer(t)

	// Seed a project.
	ctx := context.Background()
	if _, err := ts.ProjectSvc.Create(ctx, "API Backend"); err != nil {
		t.Fatalf("seed project: %v", err)
	}

	// list_projects sees it.
	out, isErr := ts.callTool(t, "list_projects", map[string]any{})
	if isErr {
		t.Fatalf("list_projects: %s", out)
	}
	if !strings.Contains(out, "api-backend") {
		t.Errorf("list_projects: want api-backend, got %q", out)
	}

	// start_timer creates a task on the fly.
	out, isErr = ts.callTool(t, "start_timer", map[string]any{
		"projectSlug": "api-backend",
		"taskTitle":   "Implementar login",
	})
	if isErr {
		t.Fatalf("start_timer: %s", out)
	}
	if !strings.Contains(out, "Implementar login") {
		t.Errorf("start_timer reply missing task title: %q", out)
	}

	// active_timer lists exactly one.
	out, isErr = ts.callTool(t, "active_timer", map[string]any{})
	if isErr {
		t.Fatalf("active_timer: %s", out)
	}
	if !strings.Contains(out, "Timer corriendo") {
		t.Errorf("active_timer: want running header, got %q", out)
	}

	// pause + resume.
	if _, isErr := ts.callTool(t, "pause_timer", map[string]any{}); isErr {
		t.Errorf("pause_timer should succeed with single timer")
	}
	if _, isErr := ts.callTool(t, "resume_timer", map[string]any{}); isErr {
		t.Errorf("resume_timer should succeed with single paused timer")
	}

	// stop_timer.
	out, isErr = ts.callTool(t, "stop_timer", map[string]any{})
	if isErr {
		t.Fatalf("stop_timer: %s", out)
	}
	if !strings.Contains(out, "Timer detenido") {
		t.Errorf("stop_timer: want 'Timer detenido', got %q", out)
	}
}

func TestStartTimerAlreadyRunningForTask(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	if _, err := ts.ProjectSvc.Create(ctx, "P1"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	args := map[string]any{"projectSlug": "p1", "taskTitle": "Same task"}
	if _, isErr := ts.callTool(t, "start_timer", args); isErr {
		t.Fatal("first start should succeed")
	}
	out, isErr := ts.callTool(t, "start_timer", args)
	if !isErr {
		t.Fatal("second start_timer on same task should error")
	}
	if !strings.Contains(out, "TIMER_ALREADY_RUNNING_FOR_TASK") {
		t.Errorf("want TIMER_ALREADY_RUNNING_FOR_TASK marker, got: %q", out)
	}
}

func TestLogTimeAndSummary(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	if _, err := ts.ProjectSvc.Create(ctx, "Web"); err != nil {
		t.Fatalf("seed: %v", err)
	}

	now := time.Now()
	startedAt := service.StartOfDay(now).Add(9 * time.Hour) // 09:00 today
	endedAt := startedAt.Add(90 * time.Minute)

	out, isErr := ts.callTool(t, "log_time", map[string]any{
		"projectSlug": "web",
		"taskTitle":   "Refactor router",
		"startedAt":   startedAt.UTC().Format(time.RFC3339),
		"endedAt":     endedAt.UTC().Format(time.RFC3339),
	})
	if isErr {
		t.Fatalf("log_time: %s", out)
	}
	if !strings.Contains(out, "1h 30m 00s") {
		t.Errorf("log_time: want 1h 30m 00s, got %q", out)
	}

	out, isErr = ts.callTool(t, "get_summary", map[string]any{"range": "today"})
	if isErr {
		t.Fatalf("get_summary today: %s", out)
	}
	if !strings.Contains(out, "1h 30m 00s") {
		t.Errorf("get_summary: want 1h 30m 00s in totals, got %q", out)
	}

	// Yesterday must be empty since we logged today's time only.
	out, _ = ts.callTool(t, "get_summary", map[string]any{"range": "yesterday"})
	if !strings.Contains(out, "Sin tiempo registrado") {
		t.Errorf("get_summary yesterday: want empty marker, got %q", out)
	}
}

func TestResourceActiveTimers(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	if _, err := ts.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, isErr := ts.callTool(t, "start_timer", map[string]any{
		"projectSlug": "p", "taskTitle": "Resource test",
	}); isErr {
		t.Fatal("start_timer failed")
	}

	body := ts.readResource(t, "timer://active-timers")
	var arr []map[string]any
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, body)
	}
	if len(arr) != 1 {
		t.Fatalf("want 1 active timer, got %d", len(arr))
	}
	if arr[0]["taskTitle"] != "Resource test" {
		t.Errorf("taskTitle = %v, want %q", arr[0]["taskTitle"], "Resource test")
	}
	if arr[0]["isPaused"] != false {
		t.Errorf("isPaused = %v, want false", arr[0]["isPaused"])
	}
	if _, ok := arr[0]["startedAt"].(string); !ok {
		t.Errorf("startedAt missing or not a string: %v", arr[0]["startedAt"])
	}
}

func TestResourceProjects(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	if _, err := ts.ProjectSvc.Create(ctx, "Alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := ts.ProjectSvc.Create(ctx, "Beta"); err != nil {
		t.Fatal(err)
	}

	body := ts.readResource(t, "timer://projects")
	var arr []map[string]any
	if err := json.Unmarshal([]byte(body), &arr); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, body)
	}
	// MCP server does not seed defaults (that lives in cli/openApp).
	if len(arr) != 2 {
		t.Errorf("want 2 projects, got %d: %s", len(arr), body)
	}
}

func TestResourceTodaySummary(t *testing.T) {
	ts := newTestServer(t)
	ctx := context.Background()
	if _, err := ts.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	startedAt := service.StartOfDay(now).Add(8 * time.Hour)
	if _, isErr := ts.callTool(t, "log_time", map[string]any{
		"projectSlug": "p",
		"taskTitle":   "X",
		"startedAt":   startedAt.UTC().Format(time.RFC3339),
		"endedAt":     startedAt.Add(time.Hour).UTC().Format(time.RFC3339),
	}); isErr {
		t.Fatal("log_time failed")
	}

	body := ts.readResource(t, "timer://today")
	var got struct {
		Date     string `json:"date"`
		TotalSec int64  `json:"totalSec"`
		Projects []struct {
			Slug     string `json:"slug"`
			TotalSec int64  `json:"totalSec"`
		} `json:"projects"`
	}
	if err := json.Unmarshal([]byte(body), &got); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, body)
	}
	if got.TotalSec != 3600 {
		t.Errorf("TotalSec = %d, want 3600", got.TotalSec)
	}
	if got.Date != now.Format("2006-01-02") {
		t.Errorf("Date = %q, want today", got.Date)
	}
	if len(got.Projects) != 1 || got.Projects[0].Slug != "p" {
		t.Errorf("projects = %+v, want one with slug 'p'", got.Projects)
	}
}
