package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/AngheloAlva/timer/internal/domain"
)

// startedTask is a small helper: creates project P + task T and returns the task.
func startedTask(t *testing.T, app *testApp, taskTitle string) domain.Task {
	t.Helper()
	ctx := context.Background()
	if _, err := app.ProjectSvc.Create(ctx, "P"); err != nil && !errors.Is(err, ErrProjectExists) {
		t.Fatalf("create project: %v", err)
	}
	task, err := app.TaskSvc.Create(ctx, "p", taskTitle)
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	return task
}

func TestTimerService_Start(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	task := startedTask(t, app, "T")

	timer, err := app.TimerSvc.Start(ctx, task.ID)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if timer.TaskID != task.ID {
		t.Errorf("task id mismatch")
	}
	if timer.IsPaused() {
		t.Errorf("new timer should not be paused")
	}
}

func TestTimerService_Start_FlipsTodoToInProgress(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	task := startedTask(t, app, "T")
	if task.Status != domain.StatusTodo {
		t.Fatalf("precondition: task should be todo")
	}

	if _, err := app.TimerSvc.Start(ctx, task.ID); err != nil {
		t.Fatal(err)
	}

	tasks, err := app.TaskSvc.List(ctx, "p", false)
	if err != nil {
		t.Fatal(err)
	}
	if tasks[0].Status != domain.StatusInProgress {
		t.Errorf("expected in_progress after start, got %v", tasks[0].Status)
	}
}

func TestTimerService_Start_DuplicateReturnsSentinel(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	task := startedTask(t, app, "T")

	if _, err := app.TimerSvc.Start(ctx, task.ID); err != nil {
		t.Fatal(err)
	}
	_, err := app.TimerSvc.Start(ctx, task.ID)
	if !errors.Is(err, domain.ErrTimerAlreadyRunning) {
		t.Errorf("expected ErrTimerAlreadyRunning, got %v", err)
	}
}

func TestTimerService_Stop(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	task := startedTask(t, app, "T")
	if _, err := app.TimerSvc.Start(ctx, task.ID); err != nil {
		t.Fatal(err)
	}

	entry, err := app.TimerSvc.Stop(ctx, task.ID)
	if err != nil {
		t.Fatalf("stop: %v", err)
	}
	if entry.TaskID != task.ID {
		t.Errorf("entry task id mismatch")
	}
	if entry.DurationSec < 0 {
		t.Errorf("duration should not be negative")
	}

	active, _ := app.TimerSvc.ListActive(ctx)
	if len(active) != 0 {
		t.Errorf("active timers should be empty after stop")
	}
}

func TestTimerService_StopAll(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	t1 := startedTask(t, app, "T1")
	t2 := startedTask(t, app, "T2")

	if _, err := app.TimerSvc.Start(ctx, t1.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := app.TimerSvc.Start(ctx, t2.ID); err != nil {
		t.Fatal(err)
	}

	entries, err := app.TimerSvc.StopAll(ctx)
	if err != nil {
		t.Fatalf("stop all: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}

	active, _ := app.TimerSvc.ListActive(ctx)
	if len(active) != 0 {
		t.Errorf("expected no active timers")
	}
}

func TestTimerService_PauseResumeIdempotent(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	task := startedTask(t, app, "T")
	if _, err := app.TimerSvc.Start(ctx, task.ID); err != nil {
		t.Fatal(err)
	}

	// Pause once → ok.
	if _, err := app.TimerSvc.Pause(ctx, task.ID); err != nil {
		t.Fatalf("pause: %v", err)
	}

	// Pause again → ErrTimerAlreadyPaused (caller can ignore).
	_, err := app.TimerSvc.Pause(ctx, task.ID)
	if !errors.Is(err, domain.ErrTimerAlreadyPaused) {
		t.Errorf("expected ErrTimerAlreadyPaused, got %v", err)
	}

	// Resume → ok.
	if _, err := app.TimerSvc.Resume(ctx, task.ID); err != nil {
		t.Fatalf("resume: %v", err)
	}

	// Resume again → ErrTimerNotPaused.
	_, err = app.TimerSvc.Resume(ctx, task.ID)
	if !errors.Is(err, domain.ErrTimerNotPaused) {
		t.Errorf("expected ErrTimerNotPaused, got %v", err)
	}
}

func TestTimerService_PauseAccumulatesPausedTotal(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()
	task := startedTask(t, app, "T")
	if _, err := app.TimerSvc.Start(ctx, task.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := app.TimerSvc.Pause(ctx, task.ID); err != nil {
		t.Fatal(err)
	}

	// Sleep a tick so that resume sees a non-zero pause delta. Keep
	// tight to avoid slow tests.
	time.Sleep(1100 * time.Millisecond)

	resumed, err := app.TimerSvc.Resume(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.PausedTotalSec < 1 {
		t.Errorf("expected paused_total_sec >= 1 after 1.1s pause, got %d", resumed.PausedTotalSec)
	}
	if resumed.IsPaused() {
		t.Errorf("resumed timer should not be paused")
	}
}

func TestTimerService_ListEntries_Filters(t *testing.T) {
	app := newTestApp(t)
	ctx := context.Background()

	if _, err := app.ProjectSvc.Create(ctx, "Alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := app.ProjectSvc.Create(ctx, "Beta"); err != nil {
		t.Fatal(err)
	}

	a, _ := app.TaskSvc.Create(ctx, "alpha", "A1")
	b, _ := app.TaskSvc.Create(ctx, "beta", "B1")

	for _, id := range []string{a.ID, b.ID, a.ID} {
		if _, err := app.TimerSvc.Start(ctx, id); err != nil {
			t.Fatal(err)
		}
		if _, err := app.TimerSvc.Stop(ctx, id); err != nil {
			t.Fatal(err)
		}
	}

	all, err := app.TimerSvc.ListEntries(ctx, ListEntriesOpts{})
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 entries, got %d", len(all))
	}

	alphaOnly, err := app.TimerSvc.ListEntries(ctx, ListEntriesOpts{ProjectSlug: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if len(alphaOnly) != 2 {
		t.Errorf("expected 2 alpha entries, got %d", len(alphaOnly))
	}

	// MinStartedAt in the future → empty.
	future, err := app.TimerSvc.ListEntries(ctx, ListEntriesOpts{
		MinStartedAt: time.Now().Add(1 * time.Hour),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(future) != 0 {
		t.Errorf("expected 0 entries in the future, got %d", len(future))
	}

	// Project not found → friendly error.
	if _, err := app.TimerSvc.ListEntries(ctx, ListEntriesOpts{ProjectSlug: "nope"}); err == nil {
		t.Errorf("expected error for unknown project")
	}
}
