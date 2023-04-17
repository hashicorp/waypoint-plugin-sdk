// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"testing"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/component/mocks"
	"github.com/hashicorp/waypoint-plugin-sdk/internal-shared/protomappers"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/testproto"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

func init() {
	// Set our default log level lower for tests
	hclog.L().SetLevel(hclog.Debug)
}

func TestPlugins(t *testing.T) {
	require := require.New(t)

	mock := &mocks.Builder{}
	plugins := Plugins(WithComponents(mock))
	bp := plugins[1]["builder"].(*BuilderPlugin)
	require.Equal(bp.Impl, mock)
}

func testDefaultMappers(t *testing.T) []*argmapper.Func {
	var mappers []*argmapper.Func
	for _, raw := range protomappers.All {
		f, err := argmapper.NewFunc(raw)
		require.NoError(t, err)
		mappers = append(mappers, f)
	}

	return mappers
}

// testDynamicFunc ensures that the dynamic function capabilities work
// properly. This should be called for each individual dynamic function
// the component exposes.
func testDynamicFunc(
	t *testing.T,
	typ string,
	value interface{},
	setFunc func(interface{}, interface{}), // set the function on your mock
	getFunc func(interface{}) interface{}, // get the function
) {
	require := require.New(t)
	assert := assert.New(t)

	// Our callback that we verify. We specify a LOT of args here because
	// we want to verify that each one will work properly. This is the core
	// of this test.
	called := false
	setFunc(value, func(
		ctx context.Context,
		args *component.Source,
	) *testproto.Data {
		called = true
		assert.NotNil(ctx)
		assert.Equal("foo", args.App)

		return &testproto.Data{Value: "hello"}
	})

	// Get the mappers
	mappers := testDefaultMappers(t)

	// Init the plugin server
	plugins := Plugins(WithComponents(value), WithMappers(mappers...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	// Dispense the plugin
	raw, err := client.Dispense(typ)
	require.NoError(err)
	implFunc := getFunc(raw).(*argmapper.Func)

	// Call our function by building a chain. We use the chain so we
	// have access to the same level of mappers that a default plugin
	// would normally have.
	result := implFunc.Call(
		argmapper.ConverterFunc(mappers...),

		argmapper.Typed(context.Background()),
		argmapper.Typed(hclog.L()),

		argmapper.Typed(&pb.Args_Source{App: "foo"}),

		argmapper.Typed(&component.DeclaredResourcesResp{}),

		argmapper.Typed(&component.DestroyedResourcesResp{}),
	)
	require.NoError(result.Err())

	// We only require a result if the function type expects us to return
	// a result. Otherwise, we just expect nil because it is error-only.
	if result.Len() > 0 {
		require.NotNil(result.Out(0))
	}

	require.True(called)
}
