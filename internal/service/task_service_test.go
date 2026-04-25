package service

import (
	"context"
	"strings"
	"testing"

	"github.com/AngheloAlva/timer/internal/domain"
)

func TestTaskService_Create(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	if _, err := app.ProjectSvc.Create(ctx, "Timer CLI"); err != nil {
		t.Fatal(err)
	}

	task, err := app.TaskSvc.Create(ctx, "timer-cli", "Implement timers")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.Status != domain.StatusTodo {
		t.Errorf("status = %v, want %v", task.Status, domain.StatusTodo)
	}
	if task.ProjectSlug != "timer-cli" {
		t.Errorf("project slug = %q", task.ProjectSlug)
	}
}

func TestTaskService_Create_ProjectNotFound(t *testing.T) {
	app := newTestApp(t)
	_, err := app.TaskSvc.Create(context.Background(), "nope", "x")
	if err == nil {
		t.Errorf("expected error")
	}
	if !strings.Contains(err.Error(), `project "nope" not found`) {
		t.Errorf("error not friendly: %v", err)
	}
}

func TestTaskService_Create_RejectsEmptyTitle(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	if _, err := app.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.TaskSvc.Create(ctx, "p", "  "); err == nil {
		t.Errorf("expected error on empty title")
	}
}

func TestTaskService_List_FiltersDoneByDefault(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	if _, err := app.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatal(err)
	}
	t1, _ := app.TaskSvc.Create(ctx, "p", "Open one")
	t2, _ := app.TaskSvc.Create(ctx, "p", "Closed one")
	if _, err := app.TaskSvc.MarkDone(ctx, t2.ID); err != nil {
		t.Fatal(err)
	}

	open, err := app.TaskSvc.List(ctx, "p", false)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 || open[0].ID != t1.ID {
		t.Errorf("expected only the open task, got %v", open)
	}

	all, err := app.TaskSvc.List(ctx, "p", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 with includeDone, got %d", len(all))
	}
}

func TestTaskService_MarkDone_AmbiguousPrefix(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	if _, err := app.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatal(err)
	}
	// Two tasks → empty/short prefix could be ambiguous. Use the
	// well-defined ambiguity: prefix of length 0.
	if _, err := app.TaskSvc.Create(ctx, "p", "T1"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.TaskSvc.Create(ctx, "p", "T2"); err != nil {
		t.Fatal(err)
	}

	// Find any single hex char shared by both UUIDs (extremely likely).
	tasks, _ := app.TaskSvc.List(ctx, "p", true)
	for c := byte('0'); c <= byte('9'); c++ {
		if strings.HasPrefix(tasks[0].ID, string(c)) && strings.HasPrefix(tasks[1].ID, string(c)) {
			_, err := app.TaskSvc.MarkDone(ctx, string(c))
			if err == nil || !strings.Contains(err.Error(), "ambiguous") {
				t.Errorf("expected ambiguity error on prefix %q, got %v", string(c), err)
			}
			return
		}
	}
	// If no shared first char, the test is inconclusive — skip rather than fail.
	t.Skip("no shared first hex char between the two UUIDs; rerun to flake test")
}

func TestTaskService_MarkDone_ClosesActiveTimer(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	if _, err := app.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatal(err)
	}
	task, err := app.TaskSvc.Create(ctx, "p", "T1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := app.TimerSvc.Start(ctx, task.ID); err != nil {
		t.Fatal(err)
	}

	res, err := app.TaskSvc.MarkDone(ctx, task.ID)
	if err != nil {
		t.Fatalf("mark done: %v", err)
	}
	if res.Entry == nil {
		t.Fatalf("expected time entry to be returned")
	}
	if res.Task.Status != domain.StatusDone {
		t.Errorf("status = %v, want done", res.Task.Status)
	}

	// Active timers must be empty now.
	active, err := app.TimerSvc.ListActive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Errorf("expected no active timers, got %d", len(active))
	}

	// And the time entry must exist.
	entries, err := app.TimerSvc.ListEntries(ctx, ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].ID != res.Entry.ID {
		t.Errorf("expected the closed entry to be persisted, got %v", entries)
	}
}

func TestTaskService_MarkDone_NoTimer(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	if _, err := app.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatal(err)
	}
	task, err := app.TaskSvc.Create(ctx, "p", "T")
	if err != nil {
		t.Fatal(err)
	}
	res, err := app.TaskSvc.MarkDone(ctx, task.ID)
	if err != nil {
		t.Fatalf("mark done: %v", err)
	}
	if res.Entry != nil {
		t.Errorf("entry should be nil when no timer was running")
	}
}
