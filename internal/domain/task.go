package domain

import "time"

type TaskStatus string

const (
	StatusTodo       TaskStatus = "todo"
	StatusInProgress TaskStatus = "in_progress"
	StatusDone       TaskStatus = "done"
	StatusArchived   TaskStatus = "archived"
)

// Task is the domain representation of a task. It carries denormalized
// project name/slug for display purposes — the list views use them and
// fetching them separately in Go would mean an extra query per task.
type Task struct {
	ID          string
	ProjectID   string
	ProjectName string
	ProjectSlug string
	Title       string
	Description string
	Status      TaskStatus
	ExternalRef string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
