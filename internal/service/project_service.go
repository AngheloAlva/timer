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

// ErrProjectExists is returned when Create hits the UNIQUE(slug) constraint.
var ErrProjectExists = errors.New("project with that slug already exists")

// ErrProjectNotArchived is returned by Delete when the caller attempts a
// hard delete on a still-active project without force=true.
var ErrProjectNotArchived = errors.New("project is not archived; archive it first or pass force=true")

// ProjectService orchestrates project operations: validates input, generates
// IDs/slugs/timestamps, calls the generated queries, and converts results
// back into domain types.
//
// Holds *sql.DB because Archive needs a transaction: it may close several
// active timers (one per task with a running timer) atomically with the
// archived flip.
type ProjectService struct {
	db *sql.DB
	q  *gen.Queries
}

// NewProjectService is the constructor — receives its dependencies explicitly.
// In NestJS this would be a class with `@Injectable()`. In Go it's a struct
// with a New function. No magic.
func NewProjectService(db *sql.DB, q *gen.Queries) *ProjectService {
	return &ProjectService{db: db, q: q}
}

// Create inserts a new project. The slug is auto-generated from the name
// if not explicitly provided.
func (s *ProjectService) Create(ctx context.Context, name string) (domain.Project, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Project{}, errors.New("project name cannot be empty")
	}

	slug := Slugify(name)
	if slug == "" {
		return domain.Project{}, fmt.Errorf("could not derive a slug from %q", name)
	}

	now := time.Now()
	p := gen.CreateProjectParams{
		ID:        uuid.NewString(),
		Name:      name,
		Slug:      slug,
		Color:     nil, // no color picker yet
		Archived:  0,
		CreatedAt: now.UnixMilli(),
		UpdatedAt: now.UnixMilli(),
	}

	if err := s.q.CreateProject(ctx, p); err != nil {
		if isUniqueViolation(err) {
			return domain.Project{}, fmt.Errorf("%w: %q", ErrProjectExists, slug)
		}
		return domain.Project{}, fmt.Errorf("create project: %w", err)
	}

	// We didn't SELECT after INSERT — we have all the data already.
	return domain.Project{
		ID:        p.ID,
		Name:      p.Name,
		Slug:      p.Slug,
		Color:     "",
		Archived:  false,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// SeedDefaultsIfEmpty creates the canonical "Inbox" project when the DB
// has zero projects. Returns true when a seed actually happened.
// Idempotent — safe to call on every app open.
func (s *ProjectService) SeedDefaultsIfEmpty(ctx context.Context) (bool, error) {
	count, err := s.q.CountProjects(ctx)
	if err != nil {
		return false, fmt.Errorf("count projects: %w", err)
	}
	if count > 0 {
		return false, nil
	}
	if _, err := s.Create(ctx, "Inbox"); err != nil {
		return false, fmt.Errorf("seed inbox: %w", err)
	}
	return true, nil
}

// List returns all projects. If includeArchived is false, archived ones
// are filtered out by the SQL query.
func (s *ProjectService) List(ctx context.Context, includeArchived bool) ([]domain.Project, error) {
	flag := int64(0)
	if includeArchived {
		flag = 1
	}

	rows, err := s.q.ListProjects(ctx, flag)
	if err != nil {
		return nil, fmt.Errorf("list projects: %w", err)
	}

	out := make([]domain.Project, 0, len(rows))
	for _, r := range rows {
		out = append(out, projectFromGen(r))
	}
	return out, nil
}

// ArchiveResult bundles the archived project with any timers that were
// auto-closed during archive. Each ClosedEntry mirrors a domain.TimeEntry
// so callers can render "stopped X for Y" feedback.
type ArchiveResult struct {
	Project         domain.Project
	ClosedEntries   []domain.TimeEntry
	AlreadyArchived bool
}

// Archive flips archived=1 on the project. If any tasks of the project have
// a running timer, those timers are closed atomically (a time_entry per
// timer is created) before the flip. Idempotent: archiving an already
// archived project is a no-op (AlreadyArchived=true, no error).
func (s *ProjectService) Archive(ctx context.Context, slug string) (ArchiveResult, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return ArchiveResult{}, errors.New("project slug cannot be empty")
	}

	p, err := s.q.GetProjectBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ArchiveResult{}, fmt.Errorf("project %q not found", slug)
		}
		return ArchiveResult{}, fmt.Errorf("find project: %w", err)
	}

	if p.Archived == 1 {
		return ArchiveResult{Project: projectFromGen(p), AlreadyArchived: true}, nil
	}

	timers, err := s.q.ListActiveTimersByProject(ctx, p.ID)
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("list active timers: %w", err)
	}

	now := time.Now()

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ArchiveResult{}, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	qtx := s.q.WithTx(tx)

	closed := make([]domain.TimeEntry, 0, len(timers))
	for _, tm := range timers {
		// Reuse the same elapsed-sec math the timer service uses.
		var paused *time.Time
		if tm.PausedAt != nil {
			t := time.UnixMilli(*tm.PausedAt)
			paused = &t
		}
		dom := domain.Timer{
			ID:             tm.ID,
			TaskID:         tm.TaskID,
			StartedAt:      time.UnixMilli(tm.StartedAt),
			PausedAt:       paused,
			PausedTotalSec: tm.PausedTotalSec,
		}
		durationSec := dom.ElapsedSec(now)
		entryID := uuid.NewString()

		if err := qtx.CreateTimeEntry(ctx, gen.CreateTimeEntryParams{
			ID:          entryID,
			TaskID:      tm.TaskID,
			StartedAt:   tm.StartedAt,
			EndedAt:     now.UnixMilli(),
			DurationSec: durationSec,
			Note:        tm.Note,
			Source:      string(domain.SourceCLI),
			CreatedAt:   now.UnixMilli(),
		}); err != nil {
			return ArchiveResult{}, fmt.Errorf("create time entry: %w", err)
		}
		if err := qtx.DeleteTimer(ctx, tm.ID); err != nil {
			return ArchiveResult{}, fmt.Errorf("delete timer: %w", err)
		}

		closed = append(closed, domain.TimeEntry{
			ID:          entryID,
			TaskID:      tm.TaskID,
			TaskTitle:   tm.TaskTitle,
			ProjectID:   p.ID,
			ProjectName: p.Name,
			ProjectSlug: p.Slug,
			StartedAt:   time.UnixMilli(tm.StartedAt),
			EndedAt:     now,
			DurationSec: durationSec,
			Note:        derefStr(tm.Note),
			Source:      domain.SourceCLI,
			CreatedAt:   now,
		})
	}

	if err := qtx.ArchiveProject(ctx, gen.ArchiveProjectParams{
		ID:        p.ID,
		UpdatedAt: now.UnixMilli(),
	}); err != nil {
		return ArchiveResult{}, fmt.Errorf("archive project: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return ArchiveResult{}, fmt.Errorf("commit: %w", err)
	}

	out := projectFromGen(p)
	out.Archived = true
	out.UpdatedAt = now
	return ArchiveResult{Project: out, ClosedEntries: closed}, nil
}

// DeleteResult reports what was destroyed by a hard delete. Counts are
// captured before the DELETE so callers can echo "deleted N tasks, M time
// entries" — useful since CASCADE removes everything in one shot.
type DeleteResult struct {
	Project          domain.Project
	TaskCount        int64
	TimeEntryCount   int64
	ActiveTimerCount int64
}

// Delete hard-deletes a project. By default refuses unless the project is
// already archived (call Archive first). Pass force=true to bypass the
// guard and accept the data loss — CASCADE will remove every task, timer,
// and time_entry under it.
func (s *ProjectService) Delete(ctx context.Context, slug string, force bool) (DeleteResult, error) {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return DeleteResult{}, errors.New("project slug cannot be empty")
	}

	p, err := s.q.GetProjectBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return DeleteResult{}, fmt.Errorf("project %q not found", slug)
		}
		return DeleteResult{}, fmt.Errorf("find project: %w", err)
	}

	if p.Archived == 0 && !force {
		return DeleteResult{}, fmt.Errorf("%w (slug: %s)", ErrProjectNotArchived, slug)
	}

	taskCount, err := s.q.CountTasksByProject(ctx, p.ID)
	if err != nil {
		return DeleteResult{}, fmt.Errorf("count tasks: %w", err)
	}
	entryCount, err := s.q.CountTimeEntriesByProject(ctx, p.ID)
	if err != nil {
		return DeleteResult{}, fmt.Errorf("count time entries: %w", err)
	}
	activeTimers, err := s.q.ListActiveTimersByProject(ctx, p.ID)
	if err != nil {
		return DeleteResult{}, fmt.Errorf("list active timers: %w", err)
	}

	if err := s.q.DeleteProject(ctx, p.ID); err != nil {
		return DeleteResult{}, fmt.Errorf("delete project: %w", err)
	}

	return DeleteResult{
		Project:          projectFromGen(p),
		TaskCount:        taskCount,
		TimeEntryCount:   entryCount,
		ActiveTimerCount: int64(len(activeTimers)),
	}, nil
}

// projectFromGen converts the storage representation into the domain type.
// Lives next to the service because this is where the boundary is crossed.
func projectFromGen(r gen.Project) domain.Project {
	return domain.Project{
		ID:        r.ID,
		Name:      r.Name,
		Slug:      r.Slug,
		Color:     derefStr(r.Color),
		Archived:  r.Archived == 1,
		CreatedAt: time.UnixMilli(r.CreatedAt),
		UpdatedAt: time.UnixMilli(r.UpdatedAt),
	}
}
