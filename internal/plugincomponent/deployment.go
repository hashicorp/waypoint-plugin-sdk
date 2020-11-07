package plugincomponent

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
)

// Deployment implements component.Deployment.
type Deployment struct {
	Any         *any.Any
	TemplateVal map[string]interface{}
}

func (c *Deployment) Proto() proto.Message                 { return c.Any }
func (c *Deployment) String() string                       { return "" }
func (c *Deployment) TemplateData() map[string]interface{} { return c.TemplateVal }

var (
	_ component.Deployment = (*Deployment)(nil)
	_ component.Template   = (*Deployment)(nil)
)
