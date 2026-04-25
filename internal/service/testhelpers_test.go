package service

import (
	"path/filepath"
	"testing"

	"github.com/AngheloAlva/timer/internal/storage/gen"
	"github.com/AngheloAlva/timer/internal/storage/sqlite"
)

// testApp bundles a fresh, isolated database with all services wired up.
// One per test — t.Cleanup closes the DB.
type testApp struct {
	DB         *sqlite.DB
	Q          *gen.Queries
	ProjectSvc *ProjectService
	TaskSvc    *TaskService
	TimerSvc   *TimerService
}

func newTestApp(t *testing.T) *testApp {
	t.Helper()

	dbPath := filepath.Join(t.TempDir(), "timer.db")
	db, err := sqlite.Open(dbPath)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	q := gen.New(db.Conn())
	return &testApp{
		DB:         db,
		Q:          q,
		ProjectSvc: NewProjectService(db.Conn(), q),
		TaskSvc:    NewTaskService(db.Conn(), q),
		TimerSvc:   NewTimerService(db.Conn(), q),
	}
}
