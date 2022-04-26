package plugin

import (
	"context"
	"fmt"
	"testing"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-plugin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

	raw, err := client.Dispense("tasklauncher")
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

	_, ok := raw.(component.RunningTask)
	require.True(ok)

	require.True(called)
}

func TestTaskLauncherRun(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	called := false
	runFunc := func(ctx context.Context, args *component.Source) (*component.TaskResult, error) {
		called = true
		assert.NotNil(ctx)
		assert.Equal("foo", args.App)
		return &component.TaskResult{ExitCode: 42}, nil
	}

	mockB := &mocks.TaskLauncher{}
	mockB.On("RunTaskFunc").Return(runFunc)

	plugins := Plugins(WithComponents(mockB), WithMappers(testDefaultMappers(t)...))
	client, server := plugin.TestPluginGRPCConn(t, plugins[1])
	defer client.Close()
	defer server.Stop()

	raw, err := client.Dispense("tasklauncher")
	require.NoError(err)
	fmt.Printf("=> %T\n", raw)
	bt := raw.(component.TaskLauncher)
	f := bt.RunTaskFunc().(*argmapper.Func)
	require.NotNil(f)

	result := f.Call(
		argmapper.Typed(context.Background()),
		argmapper.Typed(&pb.Args_Source{App: "foo"}),
	)
	require.NoError(result.Err())

	raw = result.Out(0)
	require.NotNil(raw)

	taskResult, ok := raw.(*component.TaskResult)
	require.True(ok)
	require.True(called)
	require.Equal(int(42), taskResult.ExitCode)
}

func TestTaskLauncherConfig(t *testing.T) {
	mockV := &mockTaskLauncherConfigurable{}
	testConfigurable(t, "tasklauncher", mockV, &mockV.Configurable)
}

type mockTaskLauncherConfigurable struct {
	mocks.TaskLauncher
	mocks.Configurable
}
