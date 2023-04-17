// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

// Package pluginargs
package pluginargs

import (
	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-plugin"
)

// Internal is a struct that is available to mappers. This is an internal-only
// type that is not possible for plugins to register for since it is only
// exported in an internal package.
type Internal struct {
	Broker  *plugin.GRPCBroker
	Mappers []*argmapper.Func
	Cleanup *Cleanup
}

// Cleanup can be used to register cleanup functions.
type Cleanup struct {
	f func()
}

// Do registers a cleanup function that will be called when the plugin RPC
// call is complete.
func (c *Cleanup) Do(f func()) {
	oldF := c.f
	c.f = func() {
		if oldF != nil {
			defer oldF()
		}
		f()
	}
}

func (c *Cleanup) Close() error {
	if c.f != nil {
		c.f()
	}

	return nil
}
