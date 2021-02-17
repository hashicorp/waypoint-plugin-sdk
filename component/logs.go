package component

import (
	"time"
)

// LogViewer returns batches of log lines. This is expected to be returned
// by a LogPlatform implementation.
type LogViewer struct {
	StartingAt time.Time
	Limit      int

	Output chan LogEvent
}

// LogEvent represents a single log entry.
type LogEvent struct {
	Partition string
	Timestamp time.Time
	Message   string
}
