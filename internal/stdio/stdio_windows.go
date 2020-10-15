// +build windows

package stdio

import (
	"os"
	"sync"
	"syscall"
)

// We cache the stdout/stderr files because we need to use the same *os.File
// or we'll get a hang.
var (
	once           sync.Once
	stdout, stderr *os.File
)

// Stdout returns the stdout file that was passed as an extra file descriptor
// to the plugin. We do this so that we can get access to a real TTY if
// possible for subprocess output.
func Stdout() *os.File {
	once.Do(initFds)
	return stdout
}

// Stderr. See stdout for details.
func Stderr() *os.File {
	once.Do(initFds)
	return stderr
}

func initFds() {
	stdout = openConsole("CONOUT$")
	stderr = stdout
}

// This is used to get the exact console handle instead of the redirected
// handles from panicwrap.
func openConsole(name string) *os.File {
	// Convert to UTF16
	path, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		panic(err)
	}

	// Determine the share mode
	var shareMode uint32
	switch name {
	case "CONIN$":
		shareMode = syscall.FILE_SHARE_READ
	case "CONOUT$":
		shareMode = syscall.FILE_SHARE_WRITE
	}

	// Get the file
	h, err := syscall.CreateFile(
		path,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		shareMode,
		nil,
		syscall.OPEN_EXISTING,
		0, 0)
	if err != nil {
		panic(err)
	}

	// Create the Go file
	return os.NewFile(uintptr(h), name)
}
