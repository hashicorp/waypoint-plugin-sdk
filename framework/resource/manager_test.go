package resource

import (
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"testing"

	"github.com/hashicorp/waypoint-plugin-sdk/internal/testproto"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/stretchr/testify/require"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
)

// types used for multiple resources
type (
	testState2 testState
	testState3 testState
)

func TestManagerCreateAll(t *testing.T) {
	t.Run("with no resources", func(t *testing.T) {
		m := NewManager()
		require.NoError(t, m.CreateAll(int(42)))
	})

	t.Run("with two non-dependent resources", func(t *testing.T) {
		require := require.New(t)

		var calledA, calledB int
		m := NewManager(
			WithResource(NewResource(
				WithName("A"),
				WithCreate(func(v int) error {
					calledA = v
					return nil
				}),
			)),

			WithResource(NewResource(
				WithName("B"),
				WithCreate(func(v int) error {
					calledB = v
					return nil
				}),
			)),
		)

		// Create
		require.NoError(m.CreateAll(int(42)))

		// Ensure we called all
		require.Equal(calledA, 42)
		require.Equal(calledB, 42)
	})

	t.Run("with two dependent resources", func(t *testing.T) {
		require := require.New(t)

		var calledB int
		m := NewManager(
			WithResource(NewResource(
				WithName("A"),
				WithState(&testState{}),
				WithCreate(func(s *testState, v int) error {
					s.Value = v
					return nil
				}),
			)),

			WithResource(NewResource(
				WithName("B"),
				WithCreate(func(s *testState) error {
					calledB = s.Value
					return nil
				}),
			)),
		)

		// Create
		require.NoError(m.CreateAll(int(42)))

		// Ensure we called all
		require.Equal(calledB, 42)

		// Ensure we have state
		require.NotNil(m.State())
	})

	t.Run("With one resource and a declared resource response to populate", func(t *testing.T) {
		require := require.New(t)

		type State struct {
			InternalId string `json:"internalId"`
		}

		// Declare our expected results
		expectedState := State{InternalId: "a_id"}
		expectedStateJson, _ := json.Marshal(expectedState)
		expectedDr := pb.DeclaredResource{
			Name:                "A",
			Type:                "T",
			Platform:            "test",
			CategoryDisplayHint: pb.ResourceCategoryDisplayHint_OTHER,
			StateJson:           string(expectedStateJson),
		}

		var dcr component.DeclaredResourcesResp
		m := NewManager(
			WithDeclaredResourcesResp(&dcr),
			WithResource(NewResource(
				WithName(expectedDr.Name),
				WithType(expectedDr.Type),
				WithCreate(func(state *State) error {
					state.InternalId = expectedState.InternalId
					return nil
				}),
				WithState(&State{}),
				WithPlatform(expectedDr.Platform),
				WithCategoryDisplayHint(expectedDr.CategoryDisplayHint),
			)),
		)

		// Create
		var state State
		require.NoError(m.CreateAll(&state))

		// Ensure we populated the declared resource
		require.NotEmpty(dcr.DeclaredResources)
		declaredResource := dcr.DeclaredResources[0]

		require.NotEmpty(declaredResource.Name)
		require.Equal(declaredResource.Name, expectedDr.Name)
		require.Equal(declaredResource.Type, expectedDr.Type)
		require.Equal(declaredResource.StateJson, expectedDr.StateJson)
		require.Equal(declaredResource.CategoryDisplayHint, expectedDr.CategoryDisplayHint)
	})

	t.Run("rollback on error", func(t *testing.T) {
		require := require.New(t)

		var destroyOrder []string
		m := NewManager(
			WithResource(NewResource(
				WithName("A"),
				WithState(&testState{}),
				WithCreate(func(s *testState, v int) error {
					s.Value = v
					return nil
				}),
				WithDestroy(func() error {
					destroyOrder = append(destroyOrder, "A")
					return nil
				}),
			)),

			WithResource(NewResource(
				WithName("B"),
				WithState(&testState2{}),
				WithCreate(func(s *testState) error {
					return errors.New("whelp")
				}),
				WithDestroy(func() error {
					destroyOrder = append(destroyOrder, "B")
					return nil
				}),
			)),

			WithResource(NewResource(
				WithName("C"),
				WithCreate(func(s *testState2) error {
					return nil
				}),
				WithDestroy(func() error {
					destroyOrder = append(destroyOrder, "C")
					return nil
				}),
			)),
		)

		// Create
		err := m.CreateAll(int(42))
		require.Error(err)
		require.Equal("whelp", err.Error())

		// Ensure we called destroy
		require.Equal([]string{"B", "A"}, destroyOrder)

		// Ensure we have no state
		require.NotNil(m.State())
	})
}

func TestManagerDestroyAll(t *testing.T) {
	var calledB int32
	require := require.New(t)

	// init is a function so that we can reinitialize an empty manager
	// for this test to test loading state
	var destroyOrder []string
	var destroyState int32
	init := func() *Manager {
		return NewManager(
			WithResource(NewResource(
				WithName("A"),
				WithState(&testproto.Data{}),
				WithCreate(func(s *testproto.Data, v int32) error {
					s.Number = v
					return nil
				}),
				WithDestroy(func(s *testproto.Data) error {
					destroyOrder = append(destroyOrder, "A")
					destroyState = s.Number
					return nil
				}),
			)),

			WithResource(NewResource(
				WithName("B"),
				WithCreate(func(s *testproto.Data) error {
					calledB = s.Number
					return nil
				}),
				WithDestroy(func() error {
					destroyOrder = append(destroyOrder, "B")
					return nil
				}),
			)),
		)
	}

	// Create
	m := init()
	require.NoError(m.CreateAll(int32(42)))

	// Ensure we called all
	require.Equal(calledB, int32(42))

	// Create a new manager, load the state, and verify it works
	m2 := init()
	require.NoError(m2.LoadState(m.State()))

	// Grab our resource state
	actual := m2.Resource("A").State().(*testproto.Data)
	require.NotNil(actual)
	require.Equal(actual.Number, int32(42))

	// Destroy
	require.NoError(m2.DestroyAll())

	// Ensure we destroyed
	require.Equal([]string{"B", "A"}, destroyOrder)
	require.Equal(destroyState, int32(42))
}

func TestManagerDestroyAll_noDestroyFunc(t *testing.T) {
	var calledB int32
	require := require.New(t)

	// init is a function so that we can reinitialize an empty manager
	// for this test to test loading state
	var destroyOrder []string
	init := func() *Manager {
		return NewManager(
			WithResource(NewResource(
				WithName("A"),
				WithState(&testproto.Data{}),
				WithCreate(func(s *testproto.Data, v int32) error {
					s.Number = v
					return nil
				}),
			)),

			WithResource(NewResource(
				WithName("B"),
				WithCreate(func(s *testproto.Data) error {
					calledB = s.Number
					return nil
				}),
				WithDestroy(func() error {
					destroyOrder = append(destroyOrder, "B")
					return nil
				}),
			)),
		)
	}

	// Create
	m := init()
	require.NoError(m.CreateAll(int32(42)))

	// Ensure we called all
	require.Equal(calledB, int32(42))

	// Create a new manager, load the state, and verify it works
	m2 := init()
	require.NoError(m2.LoadState(m.State()))

	// Grab our resource state
	actual := m2.Resource("A").State().(*testproto.Data)
	require.NotNil(actual)
	require.Equal(actual.Number, int32(42))

	// Destroy
	require.NoError(m2.DestroyAll())

	// Ensure we destroyed
	require.Equal([]string{"B"}, destroyOrder)
}

func TestManagerDestroyAll_loadState(t *testing.T) {
	require := require.New(t)

	// init is a function so that we can reinitialize an empty manager
	// for this test to test loading state
	var destroyOrder []string
	var destroyState int32
	init := func() *Manager {
		return NewManager(
			WithResource(NewResource(
				WithName("A"),
				WithState(&testproto.Data{}),
				WithCreate(func(s *testproto.Data, v int32) error {
					s.Number = v
					return nil
				}),
				WithDestroy(func(s *testproto.Data) error {
					destroyOrder = append(destroyOrder, "A")
					destroyState = s.Number
					return nil
				}),
			)),

			WithResource(NewResource(
				WithName("B"),
				WithState(&testState{}),
				WithCreate(func(s *testproto.Data) error {
					return nil
				}),
				WithDestroy(func() error {
					destroyOrder = append(destroyOrder, "B")
					return nil
				}),
			)),
		)
	}

	// Create manager
	m := init()

	// Manually set some destroy state
	require.NoError(m.Resource("A").SetState(&testproto.Data{Number: 42}))
	require.NoError(m.Resource("B").SetState(&testState{}))

	// Destroy
	require.NoError(m.DestroyAll())

	// Ensure we destroyed
	require.Equal([]string{"B", "A"}, destroyOrder)
	require.Equal(destroyState, int32(42))
}

// TestStatus_Manager tests the Manager's ability to call resource status
// methods and present them for creating a report
func TestStatus_Manager(t *testing.T) {
	require := require.New(t)

	init := func() *Manager {
		return NewManager(
			// state with status
			WithResource(NewResource(
				WithName("A"),
				WithState(&testState{}),
				WithCreate(func(s *testState, v int) error {
					s.Value = v
					return nil
				}),
				WithStatus(func(s *testState, sr *StatusResponse) error {
					rr := &pb.StatusReport_Resource{
						Name:   fmt.Sprintf(statusNameTpl, s.Value),
						Health: pb.StatusReport_READY,
					}
					sr.Resources = append(sr.Resources, rr)
					return nil
				}),
			)),

			// no state, with status
			WithResource(NewResource(
				WithName("B"),
				WithCreate(func(s *testState) error {
					// no-op
					return nil
				}),
				WithStatus(func(sr *StatusResponse) error {
					rr := &pb.StatusReport_Resource{
						Name:   "no state here",
						Health: pb.StatusReport_DOWN,
					}
					sr.Resources = append(sr.Resources, rr)
					return nil
				}),
			)),
			// state and multiple status reports
			WithResource(NewResource(
				WithName("C"),
				WithState(&testState2{}),
				WithCreate(func(s *testState2, vs string) error {
					v, _ := strconv.Atoi(vs)
					s.Value = v
					return nil
				}),
				WithStatus(func(s *testState2, sr *StatusResponse) error {
					rr := &pb.StatusReport_Resource{
						Name:   fmt.Sprintf(statusNameTpl, s.Value),
						Health: pb.StatusReport_ALIVE,
					}
					// make sure we can return more than 1 StatusReport_Resource
					// in a single Resource Status method
					rr2 := &pb.StatusReport_Resource{
						Name:   fmt.Sprintf(statusNameTpl, s.Value+1),
						Health: pb.StatusReport_ALIVE,
					}
					sr.Resources = append(sr.Resources, rr, rr2)
					return nil
				}),
			)),
			// state, no status
			WithResource(NewResource(
				WithName("D"),
				WithState(&testState3{}),
				WithCreate(func(s *testState3) error {
					s.Value = 0
					return nil
				}),
			)),
		)
	}

	// Create
	m := init()
	require.NoError(m.CreateAll(42, "13"))

	// Get status for each resource
	reports, err := m.StatusAll()
	require.NoError(err)

	require.Len(reports, 4)
	sort.Sort(byName(reports))

	require.Equal("no state here", reports[0].Name)
	require.Equal(fmt.Sprintf(statusNameTpl, 13), reports[1].Name)
	require.Equal(fmt.Sprintf(statusNameTpl, 14), reports[2].Name)
	require.Equal(fmt.Sprintf(statusNameTpl, 42), reports[3].Name)

	// Generate overall status report
	statusReport, err := m.StatusReport()
	require.NoError(err)
	require.NotNil(statusReport)

	require.True(statusReport.External)
	require.NotNil(statusReport.GeneratedTime)
	require.Equal(statusReport.Health, pb.StatusReport_PARTIAL)
	require.Equal(statusReport.HealthMessage, "2 C ALIVE, 1 A READY, 1 B DOWN")

	// Destroy
	require.NoError(m.DestroyAll())
}

// TestStatus_Manager_LoopRepro is a regression test for a loop discovered while
// implementing StatusAll involving using Resource Manager with a single
// Resource that reports a status.
// See https://github.com/hashicorp/waypoint-plugin-sdk/pull/43 for additional
// background.
func TestStatus_Manager_LoopRepro(t *testing.T) {
	require := require.New(t)

	init := func() *Manager {
		return NewManager(
			WithResource(NewResource(
				WithName("C"),
				WithState(&testState{}),
				WithCreate(func(s *testState, vs string) error {
					v, _ := strconv.Atoi(vs)
					s.Value = v
					return nil
				}),
				WithStatus(func(s *testState, sr *StatusResponse) error {
					rr := &pb.StatusReport_Resource{
						Name: fmt.Sprintf(statusNameTpl, s.Value),
					}
					// make sure we can return more than 1 StatusReport_Resource
					// in a single Resource Status method
					rr2 := &pb.StatusReport_Resource{
						Name: fmt.Sprintf(statusNameTpl, s.Value+1),
					}
					sr.Resources = append(sr.Resources, rr, rr2)
					return nil
				}),
			)),
		)
	}

	// Create
	m := init()
	require.NoError(m.CreateAll(42, "13"))

	reports, err := m.StatusAll()
	require.NoError(err)

	require.Len(reports, 2)
	sort.Sort(byName(reports))

	require.Equal(fmt.Sprintf(statusNameTpl, 13), reports[0].Name)
	require.Equal(fmt.Sprintf(statusNameTpl, 14), reports[1].Name)

	// Destroy
	require.NoError(m.DestroyAll())
}

func Test_healthSummary(t *testing.T) {
	tests := []struct {
		name                     string
		resources                []*pb.StatusReport_Resource
		wantOverallHealth        pb.StatusReport_Health
		wantOverallHealthMessage string
		wantErr                  bool
	}{
		{
			name: "All resources have same health",
			resources: []*pb.StatusReport_Resource{
				{
					Health: pb.StatusReport_READY,
					Type:   "network",
				},
				{
					Health: pb.StatusReport_READY,
					Type:   "container",
				},
			},
			wantOverallHealth:        pb.StatusReport_READY,
			wantOverallHealthMessage: "All 2 resources are reporting READY",
			wantErr:                  false,
		},
		{
			name: "Resources have different healths",
			resources: []*pb.StatusReport_Resource{
				{
					Health: pb.StatusReport_READY,
					Type:   "deployment",
				},
				{
					Health: pb.StatusReport_READY,
					Type:   "pod",
				},
				{
					Health: pb.StatusReport_READY,
					Type:   "pod",
				},
				{
					Health: pb.StatusReport_DOWN,
					Type:   "pod",
				},
			},
			wantOverallHealth:        pb.StatusReport_PARTIAL,
			wantOverallHealthMessage: "1 deployment READY, 2 pod READY, 1 pod DOWN",
		},
		{
			name:      "fails given no resources",
			resources: []*pb.StatusReport_Resource{},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotOverallHealth, gotOverallHealthMessage, err := healthSummary(tt.resources)
			if (err != nil) != tt.wantErr {
				t.Errorf("healthSummary() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotOverallHealth != tt.wantOverallHealth {
				t.Errorf("healthSummary() gotOverallHealth = %v, want %v", gotOverallHealth, tt.wantOverallHealth)
			}
			if gotOverallHealthMessage != tt.wantOverallHealthMessage {
				t.Errorf("healthSummary() gotOverallHealthMessage = %v, want %v", gotOverallHealthMessage, tt.wantOverallHealthMessage)
			}
		})
	}
}

func TestManagerDestroyAll_repro(t *testing.T) {
	require := require.New(t)

	init := func() *Manager {
		type S struct{}
		return NewManager(
			WithResource(NewResource(
				WithName("A"),
				WithState(&S{}),
				WithCreate(func() {}),
				WithDestroy(func(_ int) {}),
			)),
			WithResource(NewResource(
				WithName("B"),
				WithState(&S{}),
				WithCreate(func() {}),
				WithDestroy(func(_ int) {}),
			)),
			WithResource(NewResource(
				WithName("C"),
				WithState(&S{}),
				WithCreate(func() {}),
				WithDestroy(func(_ int) {}),
			)),
		)
	}

	for i := 0; i < 100; i++ {
		t.Log(fmt.Sprintf("Iteration %d", i))
		rm := init()
		require.NoError(rm.CreateAll())

		rm2 := init()
		require.NoError(rm2.LoadState(rm.State()))

		require.NoError(rm2.DestroyAll(1))
	}
}

// byName implements sort.Interface for sorting the results from calling
// Status(), to ensure ordering when validating the tests
type byName []*pb.StatusReport_Resource

func (a byName) Len() int           { return len(a) }
func (a byName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a byName) Less(i, j int) bool { return a[i].Name < a[j].Name }
