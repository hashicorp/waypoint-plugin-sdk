package plugin

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/anypb"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/component/mocks"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/testproto"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

func TestTaskLauncherStart(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	called := false
	startFunc := func(ctx context.Context, args *component.Source) *testproto.Data {
		called = true
		assert.NotNil(ctx)
		assert.Equal("foo", args.App)
		return &testproto.Data{Value: "hello"}
	}

	mockB := &mocks.TaskLauncher{}
	mockB.On("StartTaskFunc").Return(startFunc)
	mockB.On("StopTaskFunc").Return(startFunc)

	plugins := Plugins(WithComponents(mockB), WithMappers(testDefaultMappers(t)...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	raw, err := client.Dispense("tasktauncher")
	require.NoError(err)
	fmt.Printf("=> %T\n", raw)
	bt := raw.(component.TaskLauncher)
	f := bt.StartTaskFunc().(*argmapper.Func)
	require.NotNil(f)

	result := f.Call(
		argmapper.Typed(context.Background()),
		argmapper.Typed(&pb.Args_Source{App: "foo"}),
	)
	require.NoError(result.Err())

	raw = result.Out(0)
	require.NotNil(raw)

	_, ok := raw.(*anypb.Any)
	require.True(ok)

	require.True(called)
}

func TestTaskLauncherDynamicFunc_auth(t *testing.T) {
	testDynamicFunc(t, "tasktauncher", &mockTaskLaunchererAuthenticator{}, func(v, f interface{}) {
		v.(*mockTaskLaunchererAuthenticator).Authenticator.On("AuthFunc").Return(f)
	}, func(raw interface{}) interface{} {
		return raw.(component.Authenticator).AuthFunc()
	})
}

func TestTaskLauncherDynamicFunc_validateAuth(t *testing.T) {
	testDynamicFunc(t, "tasktauncher", &mockTaskLaunchererAuthenticator{}, func(v, f interface{}) {
		v.(*mockTaskLaunchererAuthenticator).Authenticator.On("ValidateAuthFunc").Return(f)
	}, func(raw interface{}) interface{} {
		return raw.(component.Authenticator).ValidateAuthFunc()
	})
}

func TestTaskLauncherConfig(t *testing.T) {
	mockV := &mockTaskLauncherConfigurable{}
	testConfigurable(t, "tasktauncher", mockV, &mockV.Configurable)
}

type mockTaskLaunchererAuthenticator struct {
	mocks.TaskLauncher
	mocks.Authenticator
}

type mockTaskLauncherConfigurable struct {
	mocks.TaskLauncher
	mocks.Configurable
}
