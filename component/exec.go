package component

import "io"

// ExecSessionInfo contains the information required by the exec plugin
// to setup a new exec and send the data back to a client.
type ExecSessionInfo struct {
	Input  io.Reader // effectively the stdin from the user (stdin)
	Output io.Writer // the output from the session (stdout)
	Error  io.Writer // an error output from the session (stderr)

	IsTTY bool // indicates if the input/output is a terminal

	// If this is a TTY, this is the terminal type (ie, the value of the TERM
	// environment variable)
	Term string

	// If this is a TTY, this is the initial window size.
	InitialWindowSize WindowSize

	// a channel that is sent the size of the users window. A new value
	// is sent on start and on each detection of window change.
	WindowSizeUpdates <-chan WindowSize

	// arguments to pass to the session. Normally interpreted as the first value
	// being the command to run and the rest arguments to that command.
	Arguments []string
}

// WindowSize provides information about the size of the terminal
// window.
type WindowSize struct {
	Height int // the height (in lines) of the terminal
	Width  int // the width (in lines) of the terminal
}
