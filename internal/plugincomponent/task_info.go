package plugincomponent

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
)

type RunningTask struct {
	Any *any.Any
}

func (c *RunningTask) Proto() proto.Message { return c.Any }
