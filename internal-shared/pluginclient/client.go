// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package pluginclient

import (
	"fmt"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	internalplugin "github.com/hashicorp/waypoint-plugin-sdk/internal/plugin"
)

// ClientConfig returns the base client config to use when connecting
// to a plugin. This sets the handshake config, protocols, etc. Manually
// override any values you want to set.
func ClientConfig(log hclog.Logger, odr bool) *plugin.ClientConfig {
	odrSettings := &internalplugin.ODRSetting{Enabled: odr}

	return &plugin.ClientConfig{
		HandshakeConfig: internalplugin.Handshake,
		VersionedPlugins: internalplugin.Plugins(
			internalplugin.WithLogger(log),
			internalplugin.WithODR(odrSettings),
		),
		AllowedProtocols: []plugin.Protocol{plugin.ProtocolGRPC},

		// We always set managed to true just in case we don't properly
		// call Kill so that CleanupClients gets it. If we do properly call
		// Kill, then it is a no-op to call it again so this is safe.
		Managed: true,

		// This is super important. There appears to be a bug with AutoMTLS
		// when using GRPCBroker and listening from the _client_ side. The
		// TLS fails to negotiate. For now we just disable this but we should
		// look into fixing that later.
		AutoMTLS: false,
	}
}

// Mappers returns the mappers supported by the plugin.
func Mappers(c *plugin.Client) ([]*argmapper.Func, error) {
	rpcClient, err := c.Client()
	if err != nil {
		return nil, err
	}

	v, err := rpcClient.Dispense("mapper")
	if err != nil {
		return nil, err
	}

	client, ok := v.(*internalplugin.MapperClient)
	if !ok {
		return nil, fmt.Errorf("mapper service was unexpected type: %T", v)
	}

	return client.Mappers()
}
