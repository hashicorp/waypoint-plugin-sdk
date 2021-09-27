package plugincomponent

import (
	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/any"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
)

// Artifact implements component.Artifact.
type Artifact struct {
	Any         *any.Any
	AnyJson     string
	LabelsVal   map[string]string
	TemplateVal map[string]interface{}
}

func (c *Artifact) Proto() proto.Message { return c.Any }

func (c *Artifact) Labels() map[string]string { return c.LabelsVal }

func (c *Artifact) TemplateData() map[string]interface{} { return c.TemplateVal }

var (
	_ component.Artifact = (*Artifact)(nil)
	_ component.Template = (*Artifact)(nil)
)
