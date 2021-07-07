package resource

import (
	"testing"

	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/ryboe/q"
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

type testState2 testState

type testState3 struct {
	Name  string
	Value int
}

// // StatusFunc implements component.Status
// func (th *thing) StatusFunc() interface{} {
// 	return th.Status
// }

// func (th *thing) Status() {}

func TestResourceStatus(t *testing.T) {
	q.Q("---------")
	q.Q("starting")
	q.Q("---------")
	defer func() {
		q.Q("---------")
		q.Q("end")
		q.Q("---------")
		q.Q("")
	}()
	require := require.New(t)

	var destroyVal int
	r := NewResource(
		WithName("test"),
		WithState(&testState3{}),
		WithCreate(func(state *testState3, v int) error {
			state.Name = "some name"
			state.Value = v
			return nil
		}),

		WithStatus(func(state *testState3, sr *pb.StatusReport_Resource) error {
			sr.Name = state.Name
			sr.Health = pb.StatusReport_ALIVE
			sr.HealthMessage = "good"
			return nil
		}),

		WithDestroy(func(state *testState3) error {
			destroyVal = state.Value
			return nil
		}),
	)

	// Create
	require.NoError(r.Create(int(42)))

	// Ensure we were called with the proper value
	state := r.State().(*testState3)
	require.NotNil(state)
	require.Equal(state.Value, 42)

	// call status manually
	healthReport := pb.StatusReport_Resource{}
	require.NoError(r.Status(state, &healthReport))
	require.Equal(state.Name, healthReport.Name)
	require.Equal(pb.StatusReport_ALIVE, healthReport.Health)
	require.Equal("good", healthReport.HealthMessage)
	// q.Q(state)

	// q.Q(r.status.Name)
	require.NotNil(r.status)
	q.Q(r.status.Name)
	require.Equal(state.Name, r.status.Name)

	// Destroy
	require.NoError(r.Destroy())
	require.Equal(destroyVal, 42)
	require.Nil(r.State())
	require.Nil(r.State().(*testState3))
}
