package plugin

import (
	"context"
	"testing"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/component/mocks"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

func TestConfigSourcerRead(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	called := false
	readFunc := func(ctx context.Context) []*pb.ConfigSource_Value {
		called = true
		assert.NotNil(ctx)
		return []*pb.ConfigSource_Value{
			{
				Name: "hello",
			},
		}
	}

	mockB := &mocks.ConfigSourcer{}
	mockB.On("ReadFunc").Return(readFunc)

	plugins := Plugins(WithComponents(mockB), WithMappers(testDefaultMappers(t)...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	raw, err := client.Dispense("configsourcer")
	require.NoError(err)
	source := raw.(component.ConfigSourcer)
	f := source.ReadFunc().(*argmapper.Func)
	require.NotNil(f)

	result := f.Call(
		argmapper.Typed(context.Background()),
	)
	require.NoError(result.Err())

	raw = result.Out(0)
	require.NotNil(raw)

	values := raw.([]*pb.ConfigSource_Value)
	require.Len(values, 1)

	require.True(called)
}

func TestConfigSourcerStop(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	called := false
	stopFunc := func(ctx context.Context) error {
		called = true
		assert.NotNil(ctx)
		return nil
	}

	mockB := &mocks.ConfigSourcer{}
	mockB.On("StopFunc").Return(stopFunc)

	plugins := Plugins(WithComponents(mockB), WithMappers(testDefaultMappers(t)...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	raw, err := client.Dispense("configsourcer")
	require.NoError(err)
	source := raw.(component.ConfigSourcer)
	f := source.StopFunc().(*argmapper.Func)
	require.NotNil(f)

	result := f.Call(
		argmapper.Typed(context.Background()),
	)
	require.NoError(result.Err())

	require.True(called)
}

func TestConfigSourcerConfig(t *testing.T) {
	mockV := &mockConfigSourcerConfigurable{}
	testConfigurable(t, "configsourcer", mockV, &mockV.Configurable)
}

type mockConfigSourcerAuthenticator struct {
	mocks.ConfigSourcer
	mocks.Authenticator
}

type mockConfigSourcerConfigurable struct {
	mocks.ConfigSourcer
	mocks.Configurable
}
