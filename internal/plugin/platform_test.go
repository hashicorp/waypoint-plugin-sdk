package plugin

import (
	"context"
	"testing"

	"github.com/golang/protobuf/ptypes"
	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/component/mocks"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
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

func TestPlatform_status(t *testing.T) {
	require := require.New(t)

	called := false
	statusFunc := func(ctx context.Context) (*pb.StatusReport, error) {
		called = true
		resources := []*pb.StatusReport_Resource{{
			Name:          "web",
			Health:        pb.StatusReport_READY,
			HealthMessage: "all fine",
		}}
		return &pb.StatusReport{
			Resources:     resources,
			External:      true,
			HealthMessage: "ready to go",
			GeneratedTime: ptypes.TimestampNow(),
			Health:        pb.StatusReport_READY,
		}, nil
	}

	mockV := &mockPlatformStatus{}
	mockG := &mockV.Status
	mockG.On("StatusFunc").Return(statusFunc)

	plugins := Plugins(WithComponents(mockV), WithMappers(testDefaultMappers(t)...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	raw, err := client.Dispense("platform")
	require.NoError(err)
	value := raw.(component.Status)
	f := value.StatusFunc().(*argmapper.Func)
	require.NotNil(f)

	result := f.Call(
		argmapper.Typed(context.Background()),
	)
	require.NoError(result.Err())

	raw = result.Out(0)
	require.NotNil(raw)

	report := raw.(*pb.StatusReport)
	require.Equal(1, len(report.Resources))
	require.Equal("web", report.Resources[0].Name)
	require.Equal("all fine", report.Resources[0].HealthMessage)
	require.Equal(pb.StatusReport_READY, report.Resources[0].Health)
	require.Equal(true, report.External)
	require.Equal("ready to go", report.HealthMessage)
	require.Equal(pb.StatusReport_READY, report.Health)

	require.True(called)
}

func TestPlatform_statusNoImpl(t *testing.T) {
	require := require.New(t)

	mockV := &mockPlatformLog{}

	plugins := Plugins(WithComponents(mockV), WithMappers(testDefaultMappers(t)...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	raw, err := client.Dispense("platform")
	require.NoError(err)
	value := raw.(component.Status)
	require.Nil(value.StatusFunc())
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

type mockPlatformStatus struct {
	mocks.Platform
	mocks.Status
}
