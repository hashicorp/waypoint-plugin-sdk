package plugincomponent

import (
	"github.com/evanphx/opaqueany"
	"google.golang.org/protobuf/proto"
)

type RunningTask struct {
	Any *opaqueany.Any
}

func (c *RunningTask) Proto() proto.Message { return c.Any }
