package service

import (
	"context"
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

// ProjectService orchestrates project operations: validates input, generates
// IDs/slugs/timestamps, calls the generated queries, and converts results
// back into domain types.
type ProjectService struct {
	q *gen.Queries
}

// NewProjectService is the constructor — receives its dependencies explicitly.
// In NestJS this would be a class with `@Injectable()`. In Go it's a struct
// with a New function. No magic.
func NewProjectService(q *gen.Queries) *ProjectService {
	return &ProjectService{q: q}
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
