package plugincomponent

import (
	"github.com/evanphx/opaqueany"
	"google.golang.org/protobuf/proto"
)

// AccessInfo provides raw access the value returned by AccessInfoFunc
// as an any, to allow it to be decoded by a target plugin that needs it.
type AccessInfo struct {
	Any *opaqueany.Any
}

func (c *AccessInfo) Proto() proto.Message     { return c.Any }
func (c *AccessInfo) TypedAny() *opaqueany.Any { return c.Any }
