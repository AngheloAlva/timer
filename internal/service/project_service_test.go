package service

import (
	"context"
	"errors"
	"testing"
)

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
