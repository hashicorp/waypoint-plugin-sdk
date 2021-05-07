package resource

import (
	"fmt"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/hashicorp/go-argmapper"

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
}

// NewManager creates a new resource manager.
//
// Callers should call Validate on the result to check for errors.
func NewManager(opts ...ManagerOption) *Manager {
	var m Manager
	m.resources = map[string]*Resource{}
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
		finalInputs = append(finalInputs, r.markerValue())
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

	result := finalFunc.Call(mapperArgs...)
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
