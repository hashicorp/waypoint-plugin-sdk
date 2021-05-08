package resource

import (
	"fmt"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// Manager manages the lifecycle and state of one or more resources.
//
// A resource manager makes it easy to define the set of resources you want
// to create, create them, handle any errors, and persist the state of the
// resources you've created (such as IDs or other metadata) so that you can
// update or destroy the resources later.
//
// Create a Manager with NewManager and a set of options.
type Manager struct {
	resources   map[string]*Resource
	createState *createState
	logger      hclog.Logger
}

// NewManager creates a new resource manager.
//
// Callers should call Validate on the result to check for errors.
func NewManager(opts ...ManagerOption) *Manager {
	var m Manager
	m.resources = map[string]*Resource{}
	m.logger = hclog.L()
	for _, opt := range opts {
		opt(&m)
	}
	return &m
}

// Resource returns the resource with the given name. This will return nil
// if the resource is not known.
func (m *Manager) Resource(n string) *Resource {
	return m.resources[n]
}

// LoadState loads the serialized state from Proto.
func (m *Manager) LoadState(v *any.Any) error {
	var s pb.Framework_ResourceManagerState
	if err := component.ProtoAnyUnmarshal(v, &s); err != nil {
		return err
	}

	// Initialize our creation state from the serialized state
	m.createState = &createState{Order: s.CreateOrder}

	// Go through each resource and populate their state
	for _, sr := range s.Resources {
		r, ok := m.resources[sr.Name]
		if !ok {
			return fmt.Errorf(
				"failed to deserialize state: unknown resource %q", sr.Name)
		}

		if err := r.loadState(sr); err != nil {
			return err
		}
	}

	return nil
}

// State returns the serialized state for this manager and all the resources
// that are part of this manager. This is a `google.protobuf.Any` type and
// plugin authors are expected to serialize this type directly into their
// return values. This is an opaque type; plugin authors should make no attempt
// to deserialize this.
func (m *Manager) State() *any.Any {
	result, err := component.ProtoAny(m.proto())
	if err != nil {
		// This should never happen. Errors that happen are usually encoded
		// into the state as messages or a panic occurs if it is critical.
		// We don't expect this to ever panic because Validate should test
		// this.
		panic(err)
	}

	return result
}

func (m *Manager) proto() *pb.Framework_ResourceManagerState {
	var result pb.Framework_ResourceManagerState
	for _, r := range m.resources {
		result.Resources = append(result.Resources, r.proto())
	}

	// If we have creation station, then track the order. We will use
	// this to construct the destroy order later.
	if cs := m.createState; cs != nil {
		result.CreateOrder = cs.Order
	}

	return &result
}

// CreateAll creates all the resources for this manager.
//
// The ordering will be determined based on the creation function dependencies
// for each resource.
//
// Create will initialize brand new state. This will not reuse existing state.
// If there is any existing state loaded, this will return an error immediately
// because it risks that state being lost.
func (m *Manager) CreateAll(args ...interface{}) error {
	// We need to build up the final function in our argmapper chain. This
	// function will do nothing, but will take as an input all the marker
	// values for the resources we want to create. This will force argmapper
	// to call all our create functions for all our resources.
	finalInputs := make([]argmapper.Value, 0, len(m.resources))
	for _, r := range m.resources {
		finalInputs = append(finalInputs, markerValue(r.name))
	}

	finalInputSet, err := argmapper.NewValueSet(finalInputs)
	if err != nil {
		return err
	}

	finalFunc, err := argmapper.BuildFunc(
		finalInputSet, nil,
		func(in, out *argmapper.ValueSet) error {
			// no-op on purpose. This function only exists to set the
			// required inputs for argmapper to create the correct call
			// graph.
			return nil
		},
	)
	if err != nil {
		return err
	}

	// Reset our creation state if we're creating
	m.createState = &createState{}

	// Start building our arguments
	var mapperArgs []argmapper.Arg
	for _, arg := range args {
		mapperArgs = append(mapperArgs, argmapper.Typed(arg))
	}
	for _, r := range m.resources {
		createFunc, err := r.mapperForCreate(m.createState)
		if err != nil {
			return err
		}

		mapperArgs = append(mapperArgs, argmapper.ConverterFunc(createFunc))
	}

	// Setup additional options
	mapperArgs = append(mapperArgs, argmapper.Logger(m.logger))

	result := finalFunc.Call(mapperArgs...)
	return result.Err()
}

// DestroyAll destroys all the resources under management. This will call
// Destroy in the reverse order of Create. All the state that was created
// via Create will be available to the Destroy callbacks. Note that after
// a resource is destroyed, their state is also set to nil.
//
// Only resources that have been created will be destroyed. This means
// that if Create partially failed, then only the resources that attempted
// creation will have Destroy called. Resources that were never called to
// Create will do nothing.
func (m *Manager) DestroyAll(args ...interface{}) error {
	cs := m.createState
	if cs == nil || len(cs.Order) == 0 {
		// We have no creation that was ever done so we have nothing to destroy.
		return nil
	}

	var finalInputs []argmapper.Value
	mapperArgs := []argmapper.Arg{
		argmapper.Logger(m.logger),
	}

	// Go through our creation order and create all our destroyers.
	for i := 0; i < len(cs.Order); i++ {
		r := m.Resource(cs.Order[i])
		if r == nil {
			// We are missing a resource that we should be destroying.
			return fmt.Errorf(
				"destroy failed: missing resource definition %q",
				cs.Order[i],
			)
		}

		// The dependencies are the resources that were created after
		// this resource.
		var deps []string
		if next := i + 1; next < len(cs.Order) {
			deps = cs.Order[next:]
		}

		// Create the mapper for destroy. The dependencies are the set of
		// created resources in the creation order that were ahead of this one.
		f, err := r.mapperForDestroy(deps)
		if err != nil {
			return err
		}
		mapperArgs = append(mapperArgs, argmapper.ConverterFunc(f))

		// Ensure that our final func is dependent on the marker for
		// this resource so that it definitely gets called.
		finalInputs = append(finalInputs, markerValue(r.name))
	}

	// Create our final target function. This has as dependencies all the
	// markers for the resources that should be destroyed.
	finalInputSet, err := argmapper.NewValueSet(finalInputs)
	if err != nil {
		return err
	}

	finalFunc, err := argmapper.BuildFunc(
		finalInputSet, nil,
		func(in, out *argmapper.ValueSet) error {
			// no-op on purpose. This function only exists to set the
			// required inputs for argmapper to create the correct call
			// graph.
			return nil
		},
	)
	if err != nil {
		return err
	}

	// Call it
	result := finalFunc.Call(mapperArgs...)

	// If this was successful, then we clear out our creation state.
	if result.Err() == nil {
		m.createState = nil
	}

	return result.Err()
}

// ManagerOption is used to configure NewManager.
type ManagerOption func(*Manager)

// WithResource specifies a resource for the manager. This can be called
// multiple times and the resources will be appended to the manager.
func WithResource(r *Resource) ManagerOption {
	return func(m *Manager) {
		m.resources[r.name] = r
	}
}
