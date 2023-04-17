// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package component

import (
	"time"
)

// LogViewer returns batches of log lines. This is expected to be returned
// by a LogPlatform implementation.
type LogViewer struct {
	// This is the time horizon log entries must be beyond to be emitted.
	StartingAt time.Time

	// The maximum number of log entries to emit.
	Limit int

	// New LogEvents should be sent to this channel.
	Output chan LogEvent
}

// LogEvent represents a single log entry.
type LogEvent struct {
	Partition string
	Timestamp time.Time
	Message   string
}
