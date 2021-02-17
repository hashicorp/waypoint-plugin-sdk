package component

import "time"

// LinesChunkWriter is used by a logs plugin to output a chunk of lines.
// A concrete implementation is provided to the plugin inside the LogsSessionInfo
// value.
type LinesChunkWriter interface {
	OutputLines([]string) error
}

// ExecSessionInfo contains the information required by the exec plugin
// to setup a new exec and send the data back to a client.
// A ExecSessionInfo value is passed to a plugins ExecFunc() to allow
// the function to properly create the exec session.
type LogsSessionInfo struct {
	Output LinesChunkWriter

	// The time horizon to begin showing logs from. Only show logs newer
	// than this time stamp.
	StartingFrom time.Time

	// The maximum number of lines to output.
	Limit int
}
