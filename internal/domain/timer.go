package domain

import (
	"errors"
	"time"
)

// Source identifies which surface created a timer or time entry.
type Source string

const (
	SourceCLI    Source = "cli"
	SourceTUI    Source = "tui"
	SourceMCP    Source = "mcp"
	SourceManual Source = "manual"
)

// Timer represents an in-flight time-tracking session for a task.
// Carries denormalized task/project metadata for display.
type Timer struct {
	ID             string
	TaskID         string
	TaskTitle      string
	ProjectID      string
	ProjectName    string
	ProjectSlug    string
	StartedAt      time.Time
	Note           string
	Source         Source
	PausedAt       *time.Time // nil when running
	PausedTotalSec int64      // accumulated paused seconds across prior pauses
}

// ElapsedSec returns the effective work time so far: wall time since
// StartedAt, minus PausedTotalSec, minus the in-progress pause if paused.
// Never negative.
func (t Timer) ElapsedSec(now time.Time) int64 {
	total := int64(now.Sub(t.StartedAt).Seconds()) - t.PausedTotalSec
	if t.PausedAt != nil {
		total -= int64(now.Sub(*t.PausedAt).Seconds())
	}
	if total < 0 {
		return 0
	}
	return total
}

// IsPaused is the readable form of "PausedAt != nil".
func (t Timer) IsPaused() bool { return t.PausedAt != nil }

// TimeEntry is a closed (committed) work segment.
type TimeEntry struct {
	ID          string
	TaskID      string
	TaskTitle   string
	ProjectID   string
	ProjectName string
	ProjectSlug string
	StartedAt   time.Time
	EndedAt     time.Time
	DurationSec int64
	Note        string
	Source      Source
	CreatedAt   time.Time
}

var (
	ErrTimerAlreadyRunning = errors.New("timer already running for this task")
	ErrTimerNotPaused      = errors.New("timer is not paused")
	ErrTimerAlreadyPaused  = errors.New("timer is already paused")
)
