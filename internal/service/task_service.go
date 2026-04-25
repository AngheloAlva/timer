package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/storage/gen"
)

// TaskService orchestrates task operations. Depends on the same generated
// queries as ProjectService — projects are resolved by slug to keep the
// CLI surface friendly.
//
// Holds *sql.DB because MarkDone needs a transaction: if the task has an
// active timer, that timer must be closed (insert time_entry + delete
// timer) atomically with the status flip.
type TaskService struct {
	db *sql.DB
	q  *gen.Queries
}

func NewTaskService(db *sql.DB, q *gen.Queries) *TaskService {
	return &TaskService{db: db, q: q}
}

// Create inserts a new task in the project identified by slug.
func (s *TaskService) Create(ctx context.Context, projectSlug, title string) (domain.Task, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		return domain.Task{}, errors.New("task title cannot be empty")
	}

	p, err := s.q.GetProjectBySlug(ctx, projectSlug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return domain.Task{}, fmt.Errorf("project %q not found", projectSlug)
		}
		return domain.Task{}, fmt.Errorf("find project: %w", err)
	}

	now := time.Now()
	params := gen.CreateTaskParams{
		ID:          uuid.NewString(),
		ProjectID:   p.ID,
		Title:       title,
		Description: nil,
		Status:      string(domain.StatusTodo),
		ExternalRef: nil,
		CreatedAt:   now.UnixMilli(),
		UpdatedAt:   now.UnixMilli(),
	}

	if err := s.q.CreateTask(ctx, params); err != nil {
		return domain.Task{}, fmt.Errorf("create task: %w", err)
	}

	return domain.Task{
		ID:          params.ID,
		ProjectID:   p.ID,
		ProjectName: p.Name,
		ProjectSlug: p.Slug,
		Title:       title,
		Status:      domain.StatusTodo,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}

// List returns tasks. Empty projectSlug lists across all projects.
// includeDone toggles whether 'done'/'archived' tasks appear.
func (s *TaskService) List(ctx context.Context, projectSlug string, includeDone bool) ([]domain.Task, error) {
	flag := int64(0)
	if includeDone {
		flag = 1
	}

	if projectSlug != "" {
		p, err := s.q.GetProjectBySlug(ctx, projectSlug)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("project %q not found", projectSlug)
			}
			return nil, fmt.Errorf("find project: %w", err)
		}

		rows, err := s.q.ListTasksByProject(ctx, gen.ListTasksByProjectParams{
			ProjectID:   p.ID,
			IncludeDone: flag,
		})
		if err != nil {
			return nil, fmt.Errorf("list tasks: %w", err)
		}

		out := make([]domain.Task, 0, len(rows))
		for _, r := range rows {
			out = append(out, taskFromListByProjectRow(r))
		}
		return out, nil
	}

	rows, err := s.q.ListTasks(ctx, flag)
	if err != nil {
		return nil, fmt.Errorf("list tasks: %w", err)
	}

	out := make([]domain.Task, 0, len(rows))
	for _, r := range rows {
		out = append(out, taskFromListRow(r))
	}
	return out, nil
}

// UpdateTitle resolves a task by id-prefix and changes its title. Empty
// title is rejected. Returns the updated task.
func (s *TaskService) UpdateTitle(ctx context.Context, idPrefix, newTitle string) (domain.Task, error) {
	idPrefix = strings.TrimSpace(idPrefix)
	newTitle = strings.TrimSpace(newTitle)
	if idPrefix == "" {
		return domain.Task{}, errors.New("task id prefix cannot be empty")
	}
	if newTitle == "" {
		return domain.Task{}, errors.New("task title cannot be empty")
	}

	matches, err := s.q.FindTasksByIDPrefix(ctx, idPrefix+"%")
	if err != nil {
		return domain.Task{}, fmt.Errorf("resolve task: %w", err)
	}
	if len(matches) == 0 {
		return domain.Task{}, fmt.Errorf("no task matches prefix %q", idPrefix)
	}
	if len(matches) > 1 {
		return domain.Task{}, fmt.Errorf("ambiguous prefix %q: matches %d tasks", idPrefix, len(matches))
	}
	t := matches[0]

	now := time.Now()
	if err := s.q.UpdateTaskTitle(ctx, gen.UpdateTaskTitleParams{
		ID:        t.ID,
		Title:     newTitle,
		UpdatedAt: now.UnixMilli(),
	}); err != nil {
		return domain.Task{}, fmt.Errorf("update title: %w", err)
	}

	out := taskFromGen(t)
	out.Title = newTitle
	out.UpdatedAt = now
	return out, nil
}

// MarkDoneResult bundles the post-done task with an optional time entry
// that was committed if the task had an active timer. Entry is nil when
// no timer was running.
type MarkDoneResult struct {
	Task  domain.Task
	Entry *domain.TimeEntry
}

// MarkDone resolves a task by ID prefix (git-style), atomically closes
// any active timer on it (writing a time_entry), and flips its status to
// 'done'. Errors out on no match or ambiguous prefix.
func (s *TaskService) MarkDone(ctx context.Context, idPrefix string) (MarkDoneResult, error) {
	idPrefix = strings.TrimSpace(idPrefix)
	if idPrefix == "" {
		return MarkDoneResult{}, errors.New("task id prefix cannot be empty")
	}

	matches, err := s.q.FindTasksByIDPrefix(ctx, idPrefix+"%")
	if err != nil {
		return MarkDoneResult{}, fmt.Errorf("resolve task: %w", err)
	}
	if len(matches) == 0 {
		return MarkDoneResult{}, fmt.Errorf("no task matches prefix %q", idPrefix)
	}
	if len(matches) > 1 {
		return MarkDoneResult{}, fmt.Errorf("ambiguous prefix %q: matches %d tasks", idPrefix, len(matches))
	}
	t := matches[0]

	// Probe for an active timer before opening the transaction. The probe
	// uses ErrNoRows to mean "no timer to close".
	var (
		timer    gen.Timer
		hasTimer bool
	)
	if tm, err := s.q.GetTimerByTaskID(ctx, t.ID); err == nil {
		timer = tm
		hasTimer = true
	} else if !errors.Is(err, sql.ErrNoRows) {
		return MarkDoneResult{}, fmt.Errorf("probe timer: %w", err)
	}

	now := time.Now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return MarkDoneResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.q.WithTx(tx)

	var entry *domain.TimeEntry
	if hasTimer {
		dom := timerFromGen(timer)
		durationSec := dom.ElapsedSec(now)
		entryID := uuid.NewString()

		if err := qtx.CreateTimeEntry(ctx, gen.CreateTimeEntryParams{
			ID:          entryID,
			TaskID:      timer.TaskID,
			StartedAt:   timer.StartedAt,
			EndedAt:     now.UnixMilli(),
			DurationSec: durationSec,
			Note:        timer.Note,
			Source:      string(domain.SourceCLI),
			CreatedAt:   now.UnixMilli(),
		}); err != nil {
			return MarkDoneResult{}, fmt.Errorf("create time entry: %w", err)
		}
		if err := qtx.DeleteTimer(ctx, timer.ID); err != nil {
			return MarkDoneResult{}, fmt.Errorf("delete timer: %w", err)
		}
		entry = &domain.TimeEntry{
			ID:          entryID,
			TaskID:      timer.TaskID,
			StartedAt:   time.UnixMilli(timer.StartedAt),
			EndedAt:     now,
			DurationSec: durationSec,
			Source:      domain.SourceCLI,
			CreatedAt:   now,
		}
	}

	if err := qtx.UpdateTaskStatus(ctx, gen.UpdateTaskStatusParams{
		ID:        t.ID,
		Status:    string(domain.StatusDone),
		UpdatedAt: now.UnixMilli(),
	}); err != nil {
		return MarkDoneResult{}, fmt.Errorf("update task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return MarkDoneResult{}, fmt.Errorf("commit: %w", err)
	}

	out := taskFromGen(t)
	out.Status = domain.StatusDone
	out.UpdatedAt = now
	return MarkDoneResult{Task: out, Entry: entry}, nil
}

// timerFromGen converts a gen.Timer into a partially-filled domain.Timer
// (no project/task display info). Used internally by MarkDone for elapsed
// calculation; we don't need denormalized fields there.
func timerFromGen(t gen.Timer) domain.Timer {
	var paused *time.Time
	if t.PausedAt != nil {
		p := time.UnixMilli(*t.PausedAt)
		paused = &p
	}
	return domain.Timer{
		ID:             t.ID,
		TaskID:         t.TaskID,
		StartedAt:      time.UnixMilli(t.StartedAt),
		Note:           derefStr(t.Note),
		Source:         domain.Source(t.Source),
		PausedAt:       paused,
		PausedTotalSec: t.PausedTotalSec,
	}
}

func taskFromGen(t gen.Task) domain.Task {
	return domain.Task{
		ID:          t.ID,
		ProjectID:   t.ProjectID,
		Title:       t.Title,
		Description: derefStr(t.Description),
		Status:      domain.TaskStatus(t.Status),
		ExternalRef: derefStr(t.ExternalRef),
		CreatedAt:   time.UnixMilli(t.CreatedAt),
		UpdatedAt:   time.UnixMilli(t.UpdatedAt),
	}
}

func taskFromListRow(r gen.ListTasksRow) domain.Task {
	return domain.Task{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		ProjectName: r.ProjectName,
		ProjectSlug: r.ProjectSlug,
		Title:       r.Title,
		Description: derefStr(r.Description),
		Status:      domain.TaskStatus(r.Status),
		ExternalRef: derefStr(r.ExternalRef),
		CreatedAt:   time.UnixMilli(r.CreatedAt),
		UpdatedAt:   time.UnixMilli(r.UpdatedAt),
	}
}

func taskFromListByProjectRow(r gen.ListTasksByProjectRow) domain.Task {
	return domain.Task{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		ProjectName: r.ProjectName,
		ProjectSlug: r.ProjectSlug,
		Title:       r.Title,
		Description: derefStr(r.Description),
		Status:      domain.TaskStatus(r.Status),
		ExternalRef: derefStr(r.ExternalRef),
		CreatedAt:   time.UnixMilli(r.CreatedAt),
		UpdatedAt:   time.UnixMilli(r.UpdatedAt),
	}
}
