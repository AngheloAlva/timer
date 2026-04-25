package domain

import "time"

// Project is the domain representation of a project — what the CLI, TUI,
// and MCP layers consume. It uses Go-native types (time.Time, bool, plain
// string) regardless of how the storage layer encodes things.
type Project struct {
	ID        string
	Name      string
	Slug      string
	Color     string // empty string when unset
	Archived  bool
	CreatedAt time.Time
	UpdatedAt time.Time
}
