package plugincomponent

import (
	"encoding/json"

	"github.com/hashicorp/opaqueany"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"google.golang.org/protobuf/proto"
)

// Deployment implements component.Deployment.
type Deployment struct {
	Any         *opaqueany.Any
	AnyJson     string
	Deployment  *pb.Deploy
	TemplateVal map[string]interface{}
}

func (c *Deployment) Proto() proto.Message { return c.Any }
func (c *Deployment) URL() string {
	if c.Deployment == nil {
		return ""
	}
	return c.Deployment.Url
}
func (c *Deployment) String() string                       { return "" }
func (c *Deployment) TemplateData() map[string]interface{} { return c.TemplateVal }

func (c *Deployment) MarshalJSON() ([]byte, error) { return []byte(c.AnyJson), nil }

var (
	_ component.Deployment = (*Deployment)(nil)
	_ component.Template   = (*Deployment)(nil)
	_ json.Marshaler       = (*Deployment)(nil)
)
