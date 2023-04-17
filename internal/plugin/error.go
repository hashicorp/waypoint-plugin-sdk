// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
)

// funcErr returns a function that can be returned for any of the
// Func component calls that just returns an error. This lets us surface
// RPC errors cleanly rather than a panic.
func funcErr(err error) interface{} {
	return func(context.Context) (interface{}, error) {
		return nil, err
	}
}
