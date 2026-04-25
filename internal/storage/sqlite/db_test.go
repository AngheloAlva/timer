package sqlite

import (
	"path/filepath"
	"testing"
)

func TestOpen_CreateesAllTables(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = db.Close() }()

	expected := []string{
		"projects",
		"tasks",
		"timers",
		"time_entries",
		"tags",
		"task_tags",
		"timer_tags",
		"time_entry_tags",
	}

	for _, name := range expected {
		var count int
		err := db.Conn().QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?",
			name,
		).Scan(&count)
		if err != nil {
			t.Errorf("query %q: %v", name, err)
			continue
		}

		if count != 1 {
			t.Errorf("table %q missing (count=%d)", name, count)
		}
	}
}
