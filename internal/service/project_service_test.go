package service

import (
	"context"
	"errors"
	"testing"
)

func TestProjectService_Archive_NoActiveTimers(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	if _, err := app.ProjectSvc.Create(ctx, "Timer CLI"); err != nil {
		t.Fatal(err)
	}

	res, err := app.ProjectSvc.Archive(ctx, "timer-cli")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if res.AlreadyArchived {
		t.Errorf("expected fresh archive, got AlreadyArchived=true")
	}
	if !res.Project.Archived {
		t.Errorf("returned project should be archived")
	}
	if len(res.ClosedEntries) != 0 {
		t.Errorf("expected no closed entries, got %d", len(res.ClosedEntries))
	}

	// List without --all must hide it.
	open, err := app.ProjectSvc.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 0 {
		t.Errorf("expected archived project hidden, got %d", len(open))
	}
	all, _ := app.ProjectSvc.List(ctx, true)
	if len(all) != 1 || !all[0].Archived {
		t.Errorf("expected archived project visible with includeArchived, got %v", all)
	}
}

func TestProjectService_Archive_ClosesActiveTimers(t *testing.T) {
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

	res, err := app.ProjectSvc.Archive(ctx, "p")
	if err != nil {
		t.Fatalf("archive: %v", err)
	}
	if len(res.ClosedEntries) != 1 {
		t.Fatalf("expected 1 closed entry, got %d", len(res.ClosedEntries))
	}
	if res.ClosedEntries[0].TaskID != task.ID {
		t.Errorf("closed entry task id mismatch")
	}

	// Timer must be gone.
	active, _ := app.TimerSvc.ListActive(ctx)
	if len(active) != 0 {
		t.Errorf("expected no active timers, got %d", len(active))
	}
	// Time entry must persist.
	entries, _ := app.TimerSvc.ListEntries(ctx, ListEntriesOpts{})
	if len(entries) != 1 {
		t.Errorf("expected 1 persisted time entry, got %d", len(entries))
	}
}

func TestProjectService_Archive_Idempotent(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	if _, err := app.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProjectSvc.Archive(ctx, "p"); err != nil {
		t.Fatal(err)
	}

	res, err := app.ProjectSvc.Archive(ctx, "p")
	if err != nil {
		t.Fatalf("second archive: %v", err)
	}
	if !res.AlreadyArchived {
		t.Errorf("expected AlreadyArchived=true on second archive")
	}
}

func TestProjectService_Archive_NotFound(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.ProjectSvc.Archive(context.Background(), "nope"); err == nil {
		t.Errorf("expected error on unknown slug")
	}
}

func TestProjectService_Delete_RefusesIfNotArchived(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	if _, err := app.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatal(err)
	}
	_, err := app.ProjectSvc.Delete(ctx, "p", false)
	if !errors.Is(err, ErrProjectNotArchived) {
		t.Errorf("expected ErrProjectNotArchived, got %v", err)
	}
}

func TestProjectService_Delete_AfterArchive(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	if _, err := app.ProjectSvc.Create(ctx, "P"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProjectSvc.Archive(ctx, "p"); err != nil {
		t.Fatal(err)
	}
	res, err := app.ProjectSvc.Delete(ctx, "p", false)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if res.Project.Slug != "p" {
		t.Errorf("returned project slug = %q", res.Project.Slug)
	}

	all, _ := app.ProjectSvc.List(ctx, true)
	if len(all) != 0 {
		t.Errorf("expected project gone, got %d", len(all))
	}
}

func TestProjectService_Delete_ForceCascadesChildren(t *testing.T) {
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
	// Create one closed time entry too.
	if _, err := app.TimerSvc.Stop(ctx, task.ID); err != nil {
		t.Fatal(err)
	}
	// Restart to leave an active timer in place.
	if _, err := app.TimerSvc.Start(ctx, task.ID); err != nil {
		t.Fatal(err)
	}

	res, err := app.ProjectSvc.Delete(ctx, "p", true)
	if err != nil {
		t.Fatalf("delete --force: %v", err)
	}
	if res.TaskCount != 1 {
		t.Errorf("TaskCount = %d, want 1", res.TaskCount)
	}
	if res.TimeEntryCount != 1 {
		t.Errorf("TimeEntryCount = %d, want 1", res.TimeEntryCount)
	}
	if res.ActiveTimerCount != 1 {
		t.Errorf("ActiveTimerCount = %d, want 1", res.ActiveTimerCount)
	}

	// Everything must be gone via CASCADE.
	tasks, _ := app.TaskSvc.List(ctx, "", true)
	if len(tasks) != 0 {
		t.Errorf("expected tasks cascaded, got %d", len(tasks))
	}
	active, _ := app.TimerSvc.ListActive(ctx)
	if len(active) != 0 {
		t.Errorf("expected timers cascaded, got %d", len(active))
	}
	entries, _ := app.TimerSvc.ListEntries(ctx, ListEntriesOpts{})
	if len(entries) != 0 {
		t.Errorf("expected entries cascaded, got %d", len(entries))
	}
}

func TestProjectService_Delete_NotFound(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.ProjectSvc.Delete(context.Background(), "nope", true); err == nil {
		t.Errorf("expected error on unknown slug")
	}
}

func TestProjectService_Create(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	p, err := app.ProjectSvc.Create(ctx, "Timer CLI")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if p.Slug != "timer-cli" {
		t.Errorf("slug = %q, want %q", p.Slug, "timer-cli")
	}
	if p.Archived {
		t.Errorf("new project should not be archived")
	}
}

func TestProjectService_Create_RejectsEmpty(t *testing.T) {
	app := newTestApp(t)
	if _, err := app.ProjectSvc.Create(context.Background(), "   "); err == nil {
		t.Errorf("expected error on empty name")
	}
}

func TestProjectService_Create_DuplicateSlugReturnsSentinel(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	if _, err := app.ProjectSvc.Create(ctx, "Inbox"); err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, err := app.ProjectSvc.Create(ctx, "inbox")
	if !errors.Is(err, ErrProjectExists) {
		t.Errorf("expected ErrProjectExists, got %v", err)
	}
}

func TestProjectService_List(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	if _, err := app.ProjectSvc.Create(ctx, "B Project"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProjectSvc.Create(ctx, "A Project"); err != nil {
		t.Fatal(err)
	}

	got, err := app.ProjectSvc.List(ctx, false)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	// Sorted by name COLLATE NOCASE ASC.
	if got[0].Name != "A Project" || got[1].Name != "B Project" {
		t.Errorf("unexpected order: %v", got)
	}
}

func TestProjectService_SeedDefaultsIfEmpty(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	seeded, err := app.ProjectSvc.SeedDefaultsIfEmpty(ctx)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	if !seeded {
		t.Errorf("expected seeded=true on empty db")
	}

	// Idempotent — second call must not seed again.
	seeded2, err := app.ProjectSvc.SeedDefaultsIfEmpty(ctx)
	if err != nil {
		t.Fatalf("second seed: %v", err)
	}
	if seeded2 {
		t.Errorf("expected seeded=false on second call")
	}

	projects, err := app.ProjectSvc.List(ctx, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Slug != "inbox" {
		t.Errorf("expected single Inbox project, got %v", projects)
	}
}
