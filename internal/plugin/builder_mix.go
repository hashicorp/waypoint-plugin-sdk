// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"github.com/hashicorp/waypoint-plugin-sdk/component"
)

type mix_Builder_Authenticator struct {
	component.Authenticator
	component.ConfigurableNotify
	component.Builder
	component.Documented
}
