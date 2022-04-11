package plugincomponent

import (
	"github.com/hashicorp/opaqueany"
	"google.golang.org/protobuf/proto"
)

type RunningTask struct {
	Any        *opaqueany.Any
	ResourceId string
}

func (c *RunningTask) Proto() proto.Message { return c.Any }
