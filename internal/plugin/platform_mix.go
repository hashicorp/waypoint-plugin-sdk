// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"github.com/hashicorp/waypoint-plugin-sdk/component"
)

type mix_Platform_Authenticator struct {
	component.Authenticator
	component.ConfigurableNotify
	component.Documented
	component.Platform
	component.PlatformReleaser
	component.WorkspaceDestroyer
	component.LogPlatform
	component.Generation
	component.Status
}

type mix_Platform_Destroy struct {
	component.Authenticator
	component.ConfigurableNotify
	component.Documented
	component.Platform
	component.PlatformReleaser
	component.Execer
	component.LogPlatform
	component.Destroyer
	component.WorkspaceDestroyer
	component.Generation
	component.Status
}

type mix_Platform_Exec struct {
	component.Authenticator
	component.ConfigurableNotify
	component.Documented
	component.Platform
	component.PlatformReleaser
	component.LogPlatform
	component.Execer
	component.Generation
	component.Status
}
