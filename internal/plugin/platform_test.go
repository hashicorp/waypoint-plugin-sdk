package plugin

import (
	"context"
	"testing"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/component/mocks"
)

func TestPlatform_optionalInterfaces(t *testing.T) {
	t.Run("implements LogPlatform", func(t *testing.T) {
		require := require.New(t)

		mockV := &mockPlatformLog{}

		plugins := Plugins(WithComponents(mockV), WithMappers(testDefaultMappers(t)...))
		client, server := plugin.TestPluginGRPCConn(t, plugins[1])
		defer client.Close()
		defer server.Stop()

		raw, err := client.Dispense("platform")
		require.NoError(err)
		require.Implements((*component.Platform)(nil), raw)
		require.Implements((*component.PlatformReleaser)(nil), raw)
		require.Implements((*component.LogPlatform)(nil), raw)

		_, ok := raw.(component.Destroyer)
		require.False(ok, "should not implement")
	})
}

func TestPlatformDynamicFunc_core(t *testing.T) {
	testDynamicFunc(t, "platform", &mocks.Platform{}, func(v, f interface{}) {
		v.(*mocks.Platform).On("DeployFunc").Return(f)
	}, func(raw interface{}) interface{} {
		return raw.(component.Platform).DeployFunc()
	})
}

func TestPlatformDynamicFunc_destroy(t *testing.T) {
	testDynamicFunc(t, "platform", &mockPlatformDestroyer{}, func(v, f interface{}) {
		v.(*mockPlatformDestroyer).Destroyer.On("DestroyFunc").Return(f)
	}, func(raw interface{}) interface{} {
		return raw.(component.Destroyer).DestroyFunc()
	})
}

func TestPlatformDynamicFunc_auth(t *testing.T) {
	testDynamicFunc(t, "platform", &mockPlatformAuthenticator{}, func(v, f interface{}) {
		v.(*mockPlatformAuthenticator).Authenticator.On("AuthFunc").Return(f)
	}, func(raw interface{}) interface{} {
		return raw.(component.Authenticator).AuthFunc()
	})
}

func TestPlatformDynamicFunc_validateAuth(t *testing.T) {
	testDynamicFunc(t, "platform", &mockPlatformAuthenticator{}, func(v, f interface{}) {
		v.(*mockPlatformAuthenticator).Authenticator.On("ValidateAuthFunc").Return(f)
	}, func(raw interface{}) interface{} {
		return raw.(component.Authenticator).ValidateAuthFunc()
	})
}

func TestPlatformConfig(t *testing.T) {
	mockV := &mockPlatformConfigurable{}
	testConfigurable(t, "platform", mockV, &mockV.Configurable)
}

func TestPlatform_generation(t *testing.T) {
	require := require.New(t)

	called := false
	genFunc := func(ctx context.Context) ([]byte, error) {
		called = true
		return []byte("HELLO"), nil
	}

	mockV := &mockPlatformGeneration{}
	mockG := &mockV.Generation
	mockG.On("GenerationFunc").Return(genFunc)

	plugins := Plugins(WithComponents(mockV), WithMappers(testDefaultMappers(t)...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	raw, err := client.Dispense("platform")
	require.NoError(err)
	value := raw.(component.Generation)
	f := value.GenerationFunc().(*argmapper.Func)
	require.NotNil(f)

	result := f.Call(
		argmapper.Typed(context.Background()),
	)
	require.NoError(result.Err())

	raw = result.Out(0)
	require.NotNil(raw)

	id := raw.([]byte)
	require.Equal("HELLO", string(id))

	require.True(called)
}

func TestPlatform_generationNoImpl(t *testing.T) {
	require := require.New(t)

	mockV := &mockPlatformLog{}

	plugins := Plugins(WithComponents(mockV), WithMappers(testDefaultMappers(t)...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	raw, err := client.Dispense("platform")
	require.NoError(err)
	value := raw.(component.Generation)
	require.Nil(value.GenerationFunc())
}

type mockPlatformAuthenticator struct {
	mocks.Platform
	mocks.Authenticator
}

type mockPlatformConfigurable struct {
	mocks.Platform
	mocks.Configurable
}

type mockPlatformLog struct {
	mocks.Platform
	mocks.LogPlatform
}

type mockPlatformDestroyer struct {
	mocks.Platform
	mocks.Destroyer
}

type mockPlatformGeneration struct {
	mocks.Platform
	mocks.Generation
}
