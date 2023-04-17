// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:build !windows
// +build !windows

package stdio

import (
	"os"
	"sync"
)

// NOTE(mitchellh): windows is in the _windows.go file suffix

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
	stdout = os.NewFile(uintptr(3), "stdout")
	stderr = os.NewFile(uintptr(3), "stdout")
}
