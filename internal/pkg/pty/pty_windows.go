// +build windows

package pty

import (
	"github.com/hashicorp/waypoint-plugin-sdk/internal/pkg/conpty"
)

func newPty() (Pty, error) {
	return conpty.New(80, 80)
}
