package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AngheloAlva/timer/internal/service"
	"github.com/AngheloAlva/timer/internal/storage/gen"
	"github.com/AngheloAlva/timer/internal/storage/sqlite"
)

// appCtx bundles every long-lived resource (DB, services) for a single
// command invocation. Each command opens its own appCtx and closes it
// before returning. This is intentional: CLI commands are short-lived
// processes — we don't share connections across invocations.
type appCtx struct {
	DB         *sqlite.DB
	ProjectSvc *service.ProjectService
	TaskSvc    *service.TaskService
	TimerSvc   *service.TimerService
	// JustSeeded is true when openApp ran the first-use seed in this call.
	// Used by `timer init` to give honest feedback on fresh DBs.
	JustSeeded bool
}

// openApp resolves the DB path, opens the SQLite connection (which also
// runs migrations on first open), and wires up the service layer.
func openApp() (*appCtx, error) {
	dbPath, err := resolveDBPath()
	if err != nil {
		return nil, fmt.Errorf("resolve db path: %w", err)
	}

	db, err := sqlite.Open(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

	q := gen.New(db.Conn())
	app := &appCtx{
		DB:         db,
		ProjectSvc: service.NewProjectService(db.Conn(), q),
		TaskSvc:    service.NewTaskService(db.Conn(), q),
		TimerSvc:   service.NewTimerService(db.Conn(), q),
	}

	// First-use seed. Silent on success — `timer init` is the verbose path.
	seeded, err := app.ProjectSvc.SeedDefaultsIfEmpty(context.Background())
	if err != nil {
		_ = app.Close()
		return nil, fmt.Errorf("seed defaults: %w", err)
	}
	app.JustSeeded = seeded

	return app, nil
}

// Close releases all resources. Safe to call once per appCtx.
func (a *appCtx) Close() error {
	return a.DB.Close()
}

// resolveDBPath returns the path where timer stores its SQLite file.
// Order of resolution:
//  1. $TIMER_DB_PATH (override, useful for tests and dev)
//  2. ~/.local/share/timer/timer.db (XDG default on linux/macOS)
func resolveDBPath() (string, error) {
	if p := os.Getenv("TIMER_DB_PATH"); p != "" {
		return p, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	dir := filepath.Join(home, ".local", "share", "timer")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create data dir: %w", err)
	}

	return filepath.Join(dir, "timer.db"), nil
}
