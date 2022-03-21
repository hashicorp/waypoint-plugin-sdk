package plugin

import (
	"context"
	"testing"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/opaqueany"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/component/mocks"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/testproto"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

func TestBuilderBuild(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	called := false
	buildFunc := func(ctx context.Context, args *component.Source) *testproto.Data {
		called = true
		assert.NotNil(ctx)
		assert.Equal("foo", args.App)
		return &testproto.Data{Value: "hello"}
	}

	mockB := &mocks.Builder{}
	mockB.On("BuildFunc").Return(buildFunc)

	plugins := Plugins(WithComponents(mockB), WithMappers(testDefaultMappers(t)...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	raw, err := client.Dispense("builder")
	require.NoError(err)
	builder := raw.(component.Builder)
	f := builder.BuildFunc().(*argmapper.Func)
	require.NotNil(f)

	result := f.Call(
		argmapper.Typed(context.Background()),
		argmapper.Typed(&pb.Args_Source{App: "foo"}),
	)
	require.NoError(result.Err())

	raw = result.Out(0)
	require.NotNil(raw)
	require.Implements((*component.Artifact)(nil), raw)

	anyVal := raw.(component.ProtoMarshaler).Proto().(*opaqueany.Any)
	name := anyVal.MessageName()
	require.NoError(err)
	require.Equal("testproto.Data", string(name))

	require.True(called)
}

func TestBuilderDynamicFunc_auth(t *testing.T) {
	testDynamicFunc(t, "builder", &mockBuilderAuthenticator{}, func(v, f interface{}) {
		v.(*mockBuilderAuthenticator).Authenticator.On("AuthFunc").Return(f)
	}, func(raw interface{}) interface{} {
		return raw.(component.Authenticator).AuthFunc()
	})
}

func TestBuilderDynamicFunc_validateAuth(t *testing.T) {
	testDynamicFunc(t, "builder", &mockBuilderAuthenticator{}, func(v, f interface{}) {
		v.(*mockBuilderAuthenticator).Authenticator.On("ValidateAuthFunc").Return(f)
	}, func(raw interface{}) interface{} {
		return raw.(component.Authenticator).ValidateAuthFunc()
	})
}

func TestBuilderConfig(t *testing.T) {
	mockV := &mockBuilderConfigurable{}
	testConfigurable(t, "builder", mockV, &mockV.Configurable)
}

type mockBuilderAuthenticator struct {
	mocks.Builder
	mocks.Authenticator
}

type mockBuilderConfigurable struct {
	mocks.Builder
	mocks.Configurable
}
