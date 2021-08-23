package plugincomponent

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"
)

// AccessInfo provides raw access the value returned by AccessInfoFunc
// as an any, to allow it to be decoded by a target plugin that needs it.
type AccessInfo struct {
	Any *any.Any
}

func (c *AccessInfo) Proto() proto.Message { return c.Any }
func (c *AccessInfo) TypedAny() *any.Any   { return c.Any }
