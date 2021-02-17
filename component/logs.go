package component

import (
	"time"
)

// LogPlatform is responsible for reading the logs for a deployment.
// This doesn't need to be the same as the Platform but a Platform can also
// implement this interface to natively provide logs.
type LogPlatform interface {
	// LogsFunc should return an implementation of LogViewer.
	LogsFunc() interface{}
}

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
