package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	sqlite3 "modernc.org/sqlite"
	sqlite3lib "modernc.org/sqlite/lib"

	"github.com/AngheloAlva/timer/internal/domain"
	"github.com/AngheloAlva/timer/internal/storage/gen"
)

// TimerService owns timer lifecycle (start/stop/pause/resume) and exposes
// time-entry reads. Holds *sql.DB so it can wrap multi-statement operations
// (Start, Stop) in a transaction.
type TimerService struct {
	db *sql.DB
	q  *gen.Queries
}

func NewTimerService(db *sql.DB, q *gen.Queries) *TimerService {
	return &TimerService{db: db, q: q}
}

// Start resolves a task by id-prefix, creates a timer, and bumps the task
// status to 'in_progress' if it was 'todo'. Atomic in one transaction.
func (s *TimerService) Start(ctx context.Context, taskIDPrefix string) (domain.Timer, error) {
	taskIDPrefix = strings.TrimSpace(taskIDPrefix)
	if taskIDPrefix == "" {
		return domain.Timer{}, errors.New("task id prefix cannot be empty")
	}

	matches, err := s.q.FindTasksByIDPrefix(ctx, taskIDPrefix+"%")
	if err != nil {
		return domain.Timer{}, fmt.Errorf("resolve task: %w", err)
	}
	if len(matches) == 0 {
		return domain.Timer{}, fmt.Errorf("no task matches prefix %q", taskIDPrefix)
	}
	if len(matches) > 1 {
		return domain.Timer{}, fmt.Errorf("ambiguous prefix %q: matches %d tasks", taskIDPrefix, len(matches))
	}
	task := matches[0]

	now := time.Now()
	timerID := uuid.NewString()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.Timer{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.q.WithTx(tx)

	if err := qtx.CreateTimer(ctx, gen.CreateTimerParams{
		ID:        timerID,
		TaskID:    task.ID,
		StartedAt: now.UnixMilli(),
		Note:      nil,
		Source:    string(domain.SourceCLI),
	}); err != nil {
		if isUniqueViolation(err) {
			return domain.Timer{}, domain.ErrTimerAlreadyRunning
		}
		return domain.Timer{}, fmt.Errorf("create timer: %w", err)
	}

	if task.Status == string(domain.StatusTodo) {
		if err := qtx.UpdateTaskStatus(ctx, gen.UpdateTaskStatusParams{
			ID:        task.ID,
			Status:    string(domain.StatusInProgress),
			UpdatedAt: now.UnixMilli(),
		}); err != nil {
			return domain.Timer{}, fmt.Errorf("update task status: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return domain.Timer{}, fmt.Errorf("commit: %w", err)
	}

	t, err := s.findActiveTimerByID(ctx, timerID)
	if err != nil {
		return domain.Timer{}, fmt.Errorf("reload timer: %w", err)
	}
	return t, nil
}

// Stop closes a timer: writes a time_entry with the computed duration and
// deletes the timer row. Atomic. If the timer is paused at stop time, the
// in-progress pause is included in paused_total but NOT in duration.
func (s *TimerService) Stop(ctx context.Context, taskIDPrefix string) (domain.TimeEntry, error) {
	timer, err := s.resolveActiveTimerByTaskPrefix(ctx, taskIDPrefix)
	if err != nil {
		return domain.TimeEntry{}, err
	}

	now := time.Now()
	entry, err := s.stopOne(ctx, timer, now)
	if err != nil {
		return domain.TimeEntry{}, err
	}
	return entry, nil
}

// StopAll closes every running timer. Returns the resulting entries in
// stop order. If any one stop fails, the rest still run; the first error
// is returned alongside the partial entries.
func (s *TimerService) StopAll(ctx context.Context) ([]domain.TimeEntry, error) {
	timers, err := s.ListActive(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]domain.TimeEntry, 0, len(timers))
	var firstErr error
	now := time.Now()
	for _, t := range timers {
		entry, err := s.stopOne(ctx, t, now)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		entries = append(entries, entry)
	}
	return entries, firstErr
}

func (s *TimerService) stopOne(ctx context.Context, timer domain.Timer, now time.Time) (domain.TimeEntry, error) {
	durationSec := timer.ElapsedSec(now)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.TimeEntry{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.q.WithTx(tx)

	entryID := uuid.NewString()

	if err := qtx.CreateTimeEntry(ctx, gen.CreateTimeEntryParams{
		ID:          entryID,
		TaskID:      timer.TaskID,
		StartedAt:   timer.StartedAt.UnixMilli(),
		EndedAt:     now.UnixMilli(),
		DurationSec: durationSec,
		Note:        stringPtrOrNil(timer.Note),
		Source:      string(domain.SourceCLI),
		CreatedAt:   now.UnixMilli(),
	}); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("create time entry: %w", err)
	}

	if err := qtx.DeleteTimer(ctx, timer.ID); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("delete timer: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return domain.TimeEntry{}, fmt.Errorf("commit: %w", err)
	}

	return domain.TimeEntry{
		ID:          entryID,
		TaskID:      timer.TaskID,
		TaskTitle:   timer.TaskTitle,
		ProjectName: timer.ProjectName,
		ProjectSlug: timer.ProjectSlug,
		StartedAt:   timer.StartedAt,
		EndedAt:     now,
		DurationSec: durationSec,
		Note:        timer.Note,
		Source:      domain.SourceCLI,
		CreatedAt:   now,
	}, nil
}

// Pause flips paused_at to now. No-op (with sentinel error) if already paused.
func (s *TimerService) Pause(ctx context.Context, taskIDPrefix string) (domain.Timer, error) {
	timer, err := s.resolveActiveTimerByTaskPrefix(ctx, taskIDPrefix)
	if err != nil {
		return domain.Timer{}, err
	}
	if timer.IsPaused() {
		return timer, domain.ErrTimerAlreadyPaused
	}

	now := time.Now()
	pausedMs := now.UnixMilli()
	if err := s.q.PauseTimer(ctx, gen.PauseTimerParams{
		PausedAt: &pausedMs,
		ID:       timer.ID,
	}); err != nil {
		return domain.Timer{}, fmt.Errorf("pause timer: %w", err)
	}
	timer.PausedAt = &now
	return timer, nil
}

// Resume clears paused_at and adds the pause delta to paused_total_sec.
// Errors if the timer wasn't paused.
func (s *TimerService) Resume(ctx context.Context, taskIDPrefix string) (domain.Timer, error) {
	timer, err := s.resolveActiveTimerByTaskPrefix(ctx, taskIDPrefix)
	if err != nil {
		return domain.Timer{}, err
	}
	if !timer.IsPaused() {
		return timer, domain.ErrTimerNotPaused
	}

	now := time.Now()
	extraSec := int64(now.Sub(*timer.PausedAt).Seconds())
	if extraSec < 0 {
		extraSec = 0
	}
	if err := s.q.ResumeTimer(ctx, gen.ResumeTimerParams{
		ExtraPausedSec: extraSec,
		ID:             timer.ID,
	}); err != nil {
		return domain.Timer{}, fmt.Errorf("resume timer: %w", err)
	}
	timer.PausedAt = nil
	timer.PausedTotalSec += extraSec
	return timer, nil
}

// ListActive returns every running timer with denormalized task/project info.
func (s *TimerService) ListActive(ctx context.Context) ([]domain.Timer, error) {
	rows, err := s.q.ListActiveTimers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list active timers: %w", err)
	}
	out := make([]domain.Timer, 0, len(rows))
	for _, r := range rows {
		out = append(out, timerFromActiveRow(r))
	}
	return out, nil
}

// ListEntriesOpts narrows a time-entry query. Zero values mean "no filter":
// MinStartedAt = 0 → no lower bound; ProjectSlug = "" → all projects;
// Max = 0 → default 20.
type ListEntriesOpts struct {
	MinStartedAt time.Time
	ProjectSlug  string
	Max          int
}

// ListEntries returns time entries matching the given options, newest first.
// Resolves ProjectSlug → project_id internally.
func (s *TimerService) ListEntries(ctx context.Context, opts ListEntriesOpts) ([]domain.TimeEntry, error) {
	max := opts.Max
	if max <= 0 {
		max = 20
	}

	projectID := ""
	if opts.ProjectSlug != "" {
		p, err := s.q.GetProjectBySlug(ctx, opts.ProjectSlug)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, fmt.Errorf("project %q not found", opts.ProjectSlug)
			}
			return nil, fmt.Errorf("find project: %w", err)
		}
		projectID = p.ID
	}

	var minMs int64
	if !opts.MinStartedAt.IsZero() {
		minMs = opts.MinStartedAt.UnixMilli()
	}

	rows, err := s.q.ListTimeEntries(ctx, gen.ListTimeEntriesParams{
		MinStartedAt: minMs,
		ProjectID:    projectID,
		MaxRows:      int64(max),
	})
	if err != nil {
		return nil, fmt.Errorf("list time entries: %w", err)
	}
	out := make([]domain.TimeEntry, 0, len(rows))
	for _, r := range rows {
		out = append(out, timeEntryFromListRow(r))
	}
	return out, nil
}

// resolveActiveTimerByTaskPrefix loads the active timer whose task.id matches
// the given prefix. Since timers have UNIQUE(task_id), there is at most one
// timer per task — task-prefix resolution is unambiguous up to the task itself.
// Returned domain.Timer carries denormalized task/project info for display.
func (s *TimerService) resolveActiveTimerByTaskPrefix(ctx context.Context, taskIDPrefix string) (domain.Timer, error) {
	taskIDPrefix = strings.TrimSpace(taskIDPrefix)
	if taskIDPrefix == "" {
		return domain.Timer{}, errors.New("task id prefix cannot be empty")
	}

	timers, err := s.ListActive(ctx)
	if err != nil {
		return domain.Timer{}, err
	}

	matches := make([]domain.Timer, 0, 2)
	for _, t := range timers {
		if strings.HasPrefix(t.TaskID, taskIDPrefix) {
			matches = append(matches, t)
		}
	}
	if len(matches) == 0 {
		return domain.Timer{}, fmt.Errorf("no active timer for task prefix %q", taskIDPrefix)
	}
	if len(matches) > 1 {
		return domain.Timer{}, fmt.Errorf("ambiguous task prefix %q: matches %d active timers", taskIDPrefix, len(matches))
	}
	return matches[0], nil
}

func (s *TimerService) findActiveTimerByID(ctx context.Context, id string) (domain.Timer, error) {
	timers, err := s.ListActive(ctx)
	if err != nil {
		return domain.Timer{}, err
	}
	for _, t := range timers {
		if t.ID == id {
			return t, nil
		}
	}
	return domain.Timer{}, fmt.Errorf("timer %s not found", id)
}

func timerFromActiveRow(r gen.ListActiveTimersRow) domain.Timer {
	var paused *time.Time
	if r.PausedAt != nil {
		t := time.UnixMilli(*r.PausedAt)
		paused = &t
	}
	return domain.Timer{
		ID:             r.ID,
		TaskID:         r.TaskID,
		TaskTitle:      r.TaskTitle,
		ProjectID:      r.ProjectID,
		ProjectName:    r.ProjectName,
		ProjectSlug:    r.ProjectSlug,
		StartedAt:      time.UnixMilli(r.StartedAt),
		Note:           derefStr(r.Note),
		Source:         domain.Source(r.Source),
		PausedAt:       paused,
		PausedTotalSec: r.PausedTotalSec,
	}
}

func timeEntryFromListRow(r gen.ListTimeEntriesRow) domain.TimeEntry {
	return domain.TimeEntry{
		ID:          r.ID,
		TaskID:      r.TaskID,
		TaskTitle:   r.TaskTitle,
		ProjectID:   r.ProjectID,
		ProjectName: r.ProjectName,
		ProjectSlug: r.ProjectSlug,
		StartedAt:   time.UnixMilli(r.StartedAt),
		EndedAt:     time.UnixMilli(r.EndedAt),
		DurationSec: r.DurationSec,
		Note:        derefStr(r.Note),
		Source:      domain.Source(r.Source),
		CreatedAt:   time.UnixMilli(r.CreatedAt),
	}
}

// isUniqueViolation detects SQLite UNIQUE constraint failures from the
// modernc.org/sqlite driver. SQLITE_CONSTRAINT_UNIQUE = 2067.
func isUniqueViolation(err error) bool {
	var sqliteErr *sqlite3.Error
	if errors.As(err, &sqliteErr) {
		return sqliteErr.Code() == sqlite3lib.SQLITE_CONSTRAINT_UNIQUE
	}
	return false
}
