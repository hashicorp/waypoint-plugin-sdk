// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugincomponent

import (
	"github.com/hashicorp/opaqueany"
	"google.golang.org/protobuf/proto"
)

type RunningTask struct {
	Any *opaqueany.Any
}

func (c *RunningTask) Proto() proto.Message { return c.Any }
