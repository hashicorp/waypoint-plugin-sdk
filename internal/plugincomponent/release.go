package plugincomponent

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// Release implements component.Release.
type Release struct {
	Any         *any.Any
	AnyJson     string
	Release     *pb.Release
	TemplateVal map[string]interface{}
}

func (c *Release) Proto() proto.Message                 { return c.Any }
func (c *Release) URL() string                          { return c.Release.Url }
func (c *Release) TemplateData() map[string]interface{} { return c.TemplateVal }

var (
	_ component.Release  = (*Release)(nil)
	_ component.Template = (*Release)(nil)
)
