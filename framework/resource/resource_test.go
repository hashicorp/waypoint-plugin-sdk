package resource

import (
	"fmt"
	"testing"

	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/stretchr/testify/require"
)

func TestResourceCreate_state(t *testing.T) {
	require := require.New(t)

	var destroyVal int
	r := NewResource(
		WithName("test"),
		WithState(&testState{}),
		WithCreate(func(state *testState, v int) error {
			state.Value = v
			return nil
		}),
		WithDestroy(func(state *testState) error {
			destroyVal = state.Value
			return nil
		}),
	)

	// Create
	require.NoError(r.Create(int(42)))

	// Ensure we were called with the proper value
	state := r.State().(*testState)
	require.NotNil(state)
	require.Equal(state.Value, 42)

	// Destroy
	require.NoError(r.Destroy())
	require.Equal(destroyVal, 42)
	require.Nil(r.State())
	require.Nil(r.State().(*testState))
}

func TestResourceCreate_stateNoDestroy(t *testing.T) {
	require := require.New(t)

	r := NewResource(
		WithName("test"),
		WithState(&testState{}),
		WithCreate(func(state *testState, v int) error {
			state.Value = v
			return nil
		}),
	)

	// Create
	require.NoError(r.Create(int(42)))

	// Ensure we were called with the proper value
	state := r.State().(*testState)
	require.NotNil(state)
	require.Equal(state.Value, 42)

	// Destroy
	require.NoError(r.Destroy())
	require.Nil(r.State())
	require.Nil(r.State().(*testState))
}

func TestResourceCreate_noStateNoDestroy(t *testing.T) {
	require := require.New(t)

	r := NewResource(
		WithName("test"),
		WithCreate(func(v int) error {
			return nil
		}),
	)

	// Create
	require.NoError(r.Create(int(42)))

	// Destroy
	require.NoError(r.Destroy())
	require.Nil(r.State())
}

func TestResourceCreate_noState(t *testing.T) {
	require := require.New(t)

	var actual int
	var destroyCalled bool
	r := NewResource(
		WithName("test"),
		WithCreate(func(v int) error {
			actual = v
			return nil
		}),
		WithDestroy(func() error {
			destroyCalled = true
			return nil
		}),
	)

	// Create
	require.NoError(r.Create(int(42)))

	// Ensure we were called with the proper value
	require.Equal(actual, int(42))

	// Destroy
	require.NoError(r.Destroy())
	require.True(destroyCalled)
}

type testState struct {
	Value int
}

type (
	testState2 testState
	testState3 testState
)

var (
	statusNameTpl    = "status-%d"
	healthMessageTpl = "alive-%d"
)

func TestStatus_Resource(t *testing.T) {
	require := require.New(t)

	r := NewResource(
		WithName("test"),
		WithState(&testState{}),
		WithCreate(func(state *testState, v int) error {
			state.Value = v
			return nil
		}),

		WithStatus(func(state *testState, sr *pb.StatusReport_Resource) error {
			sr.Name = fmt.Sprintf(statusNameTpl, state.Value)
			sr.Health = pb.StatusReport_ALIVE
			sr.HealthMessage = fmt.Sprintf(healthMessageTpl, state.Value)
			return nil
		}),

		WithDestroy(func(state *testState) error {
			return nil
		}),
	)

	// Create
	require.NoError(r.Create(int(42)))

	// Ensure we were called with the proper value
	state := r.State().(*testState)
	require.NotNil(state)
	require.Equal(state.Value, 42)

	// call status manually
	require.Nil(r.status)
	require.NoError(r.GetStatus(state, &pb.StatusReport_Resource{}))
	require.NotNil(r.status)

	require.Equal(fmt.Sprintf(statusNameTpl, state.Value), r.status.Name)
	require.Equal(pb.StatusReport_ALIVE, r.status.Health)
	require.Equal(fmt.Sprintf(healthMessageTpl, state.Value), r.status.HealthMessage)

	// Destroy
	require.NoError(r.Destroy())
	require.Nil(r.State())
	require.Nil(r.State().(*testState))
	// make sure status is cleared
	require.Nil(r.status)
}
