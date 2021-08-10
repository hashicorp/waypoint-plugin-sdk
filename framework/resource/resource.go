package resource

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync/atomic"

	"github.com/golang/protobuf/jsonpb"
	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-multierror"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// markerType is used for markerValue on Resource.
type markerType struct{}

// createState is made available internally to all our creation functions
// to track state from the creation process.
type createState struct {
	// Order is the order that creation is called by resource name.
	// This is serialized in the state and used to determine the destruction
	// order later.
	Order []string
}

// Resource is a single resource type with an associated lifecycle and state.
// A "resource" is any external thing a plugin creates such as a load balancer,
// networking primitives, files, etc. Representing these things as "resources"
// assists in lifecycle management, such as preventing dangling resources
// in the case of an error and properly cleaning them up.
type Resource struct {
	name                string
	resourceType        string
	stateType           reflect.Type
	stateValue          interface{}
	setStateClock       uint32
	createFunc          interface{}
	destroyFunc         interface{}
	platform            string
	categoryDisplayHint pb.ResourceCategoryDisplayHint
	statusFunc          interface{}

	statusResp *StatusResponse
}

// StatusResponse is a container type that holds the resources status reports. A
// resource can have 1 status response containing zero to many individual status
// reports, depending on the resource. An example would be a K8s deployment
// resource returning a single status containing a StatusReport_Resource for
// each pod it currently tracks
type StatusResponse struct {
	Resources []*pb.StatusReport_Resource
}

// NewResource creates a new resource.
//
// Callers should call Validate on the result to check for errors. If
// a resource is used in a resource manager, the resource manager Validate
// function will also validate all the resources part of it.
func NewResource(opts ...ResourceOption) *Resource {
	var r Resource
	for _, opt := range opts {
		opt(&r)
	}

	// Default resource type to the name, if not specified
	if r.resourceType == "" {
		r.resourceType = r.name
	}

	return &r
}

// Validate checks that the resource structure is configured correctly.
// This is always called prior to any operation. Users may want to call
// this during unit tests or earlier in order to provide a better user
// experience.
func (r *Resource) Validate() error {
	var result error
	if r.name == "" {
		result = multierror.Append(result, errors.New("name must be set"))
	}
	if r.createFunc == nil {
		result = multierror.Append(result, errors.New("creation function must be set"))
	}

	return result
}

// State returns the current state for this resource. This will be nil if
// the resource hasn't been created yet or has been destroyed. If this value
// is non-nil, it is guaranteed to be the same type that was given to
// WithState. If WithState was never called, this is guaranteed to always
// be nil.
//
// The returned value is also a direct pointer to the internally stored
// state so it should not be modified simultaneously to any resource
// operations.
func (r *Resource) State() interface{} {
	return r.stateValue
}

// SetState manually sets the state for this resource. This is not recommended;
// typically state should only be modified through the lifecycle functions.
// However, this function is particularly useful to transition to using the
// resource manager from a previous version that didn't use the resource manager.
//
// When calling Manager.DestroyAll after manually setting state using SetState,
// the Manager will destroy the resources in the opposite order SetState is called.
// To put it another way: try to call SetState on resources in the order they
// were created, since DestroyAll destroys in reverse creation order.
//
// The value v must be the same type as the type given for WithState.
func (r *Resource) SetState(v interface{}) error {
	if reflect.TypeOf(v) != r.stateType {
		return fmt.Errorf("state value type %T does not match expected type %s",
			v, r.stateType.String())
	}

	r.stateValue = v
	r.setStateClock = atomic.AddUint32(&setStateCallOrder, 1)
	return nil
}

// Create creates this resource. args is a list of arguments to make
// available to the creation function via dependency injection (matching
// types in the arguments).
//
// After Create is called, any state can be accessed via the State function.
// This may be populated even during failure with partial state.
func (r *Resource) Create(args ...interface{}) error {
	if err := r.Validate(); err != nil {
		return err
	}

	f, err := r.mapperForCreate(nil)
	if err != nil {
		return err
	}

	mapperArgs := make([]argmapper.Arg, len(args))
	for i, v := range args {
		mapperArgs[i] = argmapper.Typed(v)
	}

	result := f.Call(mapperArgs...)
	return result.Err()
}

// Destroy destroys this resource. args is a list of arguments to make
// available to the destroy function via dependency injection. The state
// value will always be available as an argument (though it may be nil
// if this resource has no state).
//
// After Destroy is called successfully (without an error result), the
// state will always be nil.
func (r *Resource) Destroy(args ...interface{}) error {
	if err := r.Validate(); err != nil {
		return err
	}

	f, err := r.mapperForDestroy(nil)
	if err != nil {
		return err
	}

	mapperArgs := make([]argmapper.Arg, len(args))
	for i, v := range args {
		mapperArgs[i] = argmapper.Typed(v)
	}

	result := f.Call(mapperArgs...)
	return result.Err()
}

// DeclaredResource converts a resource to a DeclaredResource protobuf, which
// can be used in a component.DeclaredResourcesResp.
func (r *Resource) DeclaredResource() (*pb.DeclaredResource, error) {
	stateJson, err := json.Marshal(r.State())
	if err != nil {
		return nil, fmt.Errorf("state for resource is not serializable to json: %w", err)
	}

	stateProtoAny, err := component.ProtoAny(r.State())
	if err != nil {
		return nil, fmt.Errorf("state for resource is not serializable to protobuf: %w", err)
	}

	return &pb.DeclaredResource{
		Name:                r.name,
		Type:                r.resourceType,
		Platform:            r.platform,
		CategoryDisplayHint: r.categoryDisplayHint,
		State:               stateProtoAny,
		StateJson:           string(stateJson),
	}, nil
}

// Status returns a copy of this resources' internal status response, or nil if
// no status exists.
func (r *Resource) Status() *StatusResponse {
	if r.statusResp == nil {
		return nil
	}
	cp := make([]*pb.StatusReport_Resource, len(r.statusResp.Resources))
	copy(cp, r.statusResp.Resources)
	return &StatusResponse{
		Resources: cp,
	}
}

// status is a method used to populate a Resources' statusResp. At this time it
// is used for testing purposes and should otherwise not be called directly.
func (r *Resource) status(args ...interface{}) error {
	if err := r.Validate(); err != nil {
		return err
	}

	f, err := r.mapperForStatus()
	if err != nil {
		return err
	}

	mapperArgs := make([]argmapper.Arg, len(args))
	for i, v := range args {
		mapperArgs[i] = argmapper.Typed(v)
	}

	result := f.Call(mapperArgs...)
	return result.Err()
}

// mapperForCreate returns an argmapper func that takes as input the
// requirements for the createFunc and returns the state type plus an error.
// This creates a valid "mapper" we can use with Manager.
func (r *Resource) mapperForCreate(cs *createState) (*argmapper.Func, error) {
	// Create the func for the createFunc as-is. We need to get the input/output sets.
	original, err := argmapper.NewFunc(r.createFunc)
	if err != nil {
		return nil, err
	}

	// For our output, we will always output our unique marker type.
	// This unique marker type will allow our resource manager to create
	// a function chain that calls all the resources necessary.
	markerVal := markerValue(r.name)
	outputs, err := argmapper.NewValueSet([]argmapper.Value{markerVal})
	if err != nil {
		return nil, err
	}

	// Our inputs default to whatever the function requires and our
	// output defaults to nothing (only the error type). We will proceed to
	// modify these so that the output contains our state type and the input
	// does NOT contain our state type (since it'll be allocated and provided
	// by us). If we have no state type, we do nothing!
	inputs := original.Input()
	if r.stateType != nil {
		// For outputs, we will only return the state type.
		outputs, err = argmapper.NewValueSet(append(outputs.Values(), argmapper.Value{
			Type: r.stateType,
		}))
		if err != nil {
			return nil, err
		}

		// Zero our state now
		r.initState(true)

		// For input, we have to remove the state type
		inputVals := inputs.Values()
		for i := 0; i < len(inputVals); i++ {
			v := inputVals[i]
			if v.Type != r.stateType {
				// easy case, the type is not our state type
				continue
			}

			// the type IS our state type, we need to remove it. We do
			// this by swapping with the last element (order doesn't matter)
			// and decrementing i so we reloop over this value.
			inputVals[len(inputVals)-1], inputVals[i] = inputVals[i], inputVals[len(inputVals)-1]
			inputVals = inputVals[:len(inputVals)-1]
			i--
		}
		inputs, err = argmapper.NewValueSet(inputVals)
		if err != nil {
			return nil, err
		}
	}

	return argmapper.BuildFunc(inputs, outputs, func(in, out *argmapper.ValueSet) error {
		// Our available arguments are what was given to us and required
		// by our function plus our newly allocated state.
		args := in.Args()

		if r.stateType != nil {
			// Initialize our state type and add it to our available args
			args = append(args, argmapper.Typed(r.stateValue))

			// Ensure our output value for our state type is set
			if v := out.Typed(r.stateType); v != nil {
				v.Value = reflect.ValueOf(r.stateValue)
			}
		}

		// Ensure our output marker type is set
		if v := out.TypedSubtype(markerVal.Type, markerVal.Subtype); v != nil {
			v.Value = markerVal.Value
		}

		// If we have creation state, append our resource to the order.
		if cs != nil {
			cs.Order = append(cs.Order, r.name)
		}

		// Call our function. We throw away any result types except for the error.
		result := original.Call(args...)
		return result.Err()
	}, argmapper.FuncOnce())
}

// mapperForStatus returns an argmapper func that will call the resources'
// defined status function.
func (r *Resource) mapperForStatus() (*argmapper.Func, error) {
	statusFunc := r.statusFunc
	if statusFunc == nil {
		statusFunc = func() {}
	}

	// Create the func for the statusFunc as-is. We need to get the input/output sets.
	original, err := argmapper.NewFunc(statusFunc)
	if err != nil {
		return nil, err
	}

	// For our output, we will always output our unique marker type and our
	// statusReport type. This unique marker type will allow our resource manager to
	// create a function chain that calls all the resources necessary. The
	// status type ensures we output the status to be saved to the resource.
	markerVal := markerValue(r.name)
	outputs, err := argmapper.NewValueSet([]argmapper.Value{
		markerVal,
	})
	if err != nil {
		return nil, err
	}

	// For input, we have to remove the status response type because
	// we don't need it to call the func we build below because we'll
	// construct it within the buildfunc call.
	inputVals := original.Input().Values()
	for i := 0; i < len(inputVals); i++ {
		v := inputVals[i]
		if v.Type != statusResponseType {
			continue
		}

		// the type IS our status response type, we need to remove it. We do
		// this by swapping with the last element (order doesn't matter)
		// and decrementing i so we reloop over this value.
		inputVals[len(inputVals)-1], inputVals[i] = inputVals[i], inputVals[len(inputVals)-1]
		inputVals = inputVals[:len(inputVals)-1]
		i--
	}
	inputs, err := argmapper.NewValueSet(inputVals)
	if err != nil {
		return nil, err
	}

	return argmapper.BuildFunc(inputs, outputs, func(in, out *argmapper.ValueSet) error {
		args := in.Args()
		if r.statusFunc != nil {
			r.statusResp = &StatusResponse{}
		}
		args = append(args, argmapper.Typed(r.statusResp))

		// Ensure our output marker type is set
		if v := out.TypedSubtype(markerVal.Type, markerVal.Subtype); v != nil {
			v.Value = markerVal.Value
		}

		// Call our function. We throw away any result types except for the
		// error.
		result := original.Call(args...)
		return result.Err()
	}, argmapper.FuncOnce())
}

// mapperForDestroy returns an argmapper func that will call the destroy
// function. The deps given will be created as input dependencies to ensure
// that they are destroyed first. The value of deps should be the name of
// the resource.
func (r *Resource) mapperForDestroy(deps []string) (*argmapper.Func, error) {
	// The destroy function is optional (some resources aren't destroyed
	// or are destroyed via some other functions). If so, just set it to
	// a no-op since we still want to execute and do our state logic and so on.
	destroyFunc := r.destroyFunc
	if destroyFunc == nil {
		destroyFunc = func() {}
	}

	// Create the func for the destroyFunc as-is. We need to get the input/output sets.
	original, err := argmapper.NewFunc(destroyFunc)
	if err != nil {
		return nil, err
	}

	// For our output, we will always output our unique marker type.
	// This unique marker type will allow our resource manager to create
	// a function chain that calls all the resources necessary.
	markerVal := markerValue(r.name)
	outputs, err := argmapper.NewValueSet([]argmapper.Value{markerVal})
	if err != nil {
		return nil, err
	}

	// We have to modify our inputs to add the set of dependencies to this.
	inputVals := original.Input().Values()
	for _, d := range deps {
		if d == r.name {
			// This shouldn't happen, this would be an infinite loop. If this
			// happened it means there is a bug or corruption somewhere. We
			// panic so that we can track this bug down.
			panic("resource dependent on itself for destroy")
		}

		inputVals = append(inputVals, markerValue(d))
	}
	inputs, err := argmapper.NewValueSet(inputVals)
	if err != nil {
		return nil, err
	}

	// Ensure we have the state available as an argument. If it is
	// nil then we initialize it.
	var buildArgs []argmapper.Arg
	if r.stateType != nil {
		if r.stateValue == nil {
			r.initState(true)
		}
		buildArgs = append(buildArgs, argmapper.Typed(r.stateValue))
	}

	// We want to ensure that the destroy function is called at most once.
	buildArgs = append(buildArgs, argmapper.FuncOnce())

	return argmapper.BuildFunc(inputs, outputs, func(in, out *argmapper.ValueSet) error {
		// Our available arguments are what was given to us and required
		// by our function plus our newly allocated state.
		args := in.Args()

		// Ensure our output marker type is set
		if v := out.TypedSubtype(markerVal.Type, markerVal.Subtype); v != nil {
			v.Value = markerVal.Value
		}

		// Call our function. We throw away any result types except for the error.
		result := original.Call(args...)
		err := result.Err()

		// If the destroy was successful, we clear our state and status
		if err == nil {
			r.initState(false)
			r.statusResp = nil
		}

		return err
	}, buildArgs...)
}

// initState sets the r.stateValue to a new, empty state value.
// If zero is true, this will get set to a non-nil value. If zero is
// false, the state will be a nil pointer type to the state type.
func (r *Resource) initState(zero bool) {
	if r.stateType != nil {
		if zero {
			r.stateValue = reflect.New(r.stateType.Elem()).Interface()
		} else {
			r.stateValue = reflect.New(r.stateType).Elem().Interface()
		}
	}
}

// loadState is the inverse of proto. This repopulates the state from the
// serialized proto format. This will discard any previous state that is
// currently loaded.
func (r *Resource) loadState(s *pb.Framework_ResourceState) error {
	// If we have no raw value in the state then ignore it.
	if s == nil || s.Raw == nil {
		return nil
	}

	// We try to unmarshal directly into a state value
	r.initState(true)
	if r.stateValue == nil {
		return fmt.Errorf(
			"resource %q: can't unserialize state because the resource "+
				"has no defined state type", r.name)
	}

	pm, ok := r.stateValue.(proto.Message)
	if !ok {
		return fmt.Errorf(
			"resource %q: can't unserialize state because the resource "+
				"state type is not a protobuf message.", r.name)
	}
	return component.ProtoAnyUnmarshal(s.Raw, pm)
}

// proto returns the protobuf message for the state of this resource.
func (r *Resource) proto() *pb.Framework_ResourceState {
	stateProto, err := component.Proto(r.stateValue)
	if err != nil {
		// This shouldn't happen.
		panic(err)
	}

	// This means we have no state value, we return just the name.
	if stateProto == nil {
		return &pb.Framework_ResourceState{Name: r.name}
	}

	// Encode our state
	anyVal, err := component.ProtoAny(stateProto)
	if err != nil {
		// This shouldn't happen.
		panic(err)
	}

	var m jsonpb.Marshaler
	m.Indent = "\t" // make it human-readable
	jsonVal, err := m.MarshalToString(stateProto)
	if err != nil {
		jsonVal = fmt.Sprintf(`{"error": %q}`, err)
	}

	return &pb.Framework_ResourceState{
		Name: r.name,
		Raw:  anyVal,
		Json: jsonVal,
	}
}

// ResourceOption is used to configure NewResource.
type ResourceOption func(*Resource)

// WithName sets the resource name. This name is used in output meant to be
// consumed by a user so it should be descriptive but short, such as
// "security group" or "app container". It must be unique among resources you create.
func WithName(n string) ResourceOption {
	return func(r *Resource) { r.name = n }
}

// WithType optionally sets the type of the resource according to the platform.
// E.g. "container", "instance", "autoscaling group", "pod", etc.
// If not specified, type will default to the resource's name.
// Multiple resources may share the same type, and generally one resource
// will have a type that matches the DeclaredResource corresponding to this resource.
func WithType(t string) ResourceOption {
	return func(r *Resource) { r.resourceType = t }
}

// WithCreate sets the creation function for this resource.
//
// The function may take as inputs any arguments it requires and can return
// any values it wants. The inputs will be automatically populated with
// available values that are configured on the resource manager. As a special
// case, the arguments may also accept the state type specified for WithState
// (if any) to get access to an allocated, empty state structure.
//
// The return values are ignored, except for a final "error" value. A final
// "error" type value will be used to determine success or failure of the
// function call.
//
// If a resource wants to access or share information with other resources,
// it must do so via the specified state type argument. This argument can be
// modified as the function runs and it will be made available to subsequent
// resources.
//
// The creation function will be called for EACH deployment operation so if
// a resource is shared across deployments (such as a VPC), the creation function
// should be idempotent and look up that existing resource.
func WithCreate(f interface{}) ResourceOption {
	return func(r *Resource) { r.createFunc = f }
}

// WithDestroy sets the function to destroy this resource.
//
// Please see the docs for WithCreate since the semantics are very similar.
//
// One important difference for the destruction function is that the state
// argument will be populated with the value of the state set during WithCreate.
func WithDestroy(f interface{}) ResourceOption {
	return func(r *Resource) { r.destroyFunc = f }
}

// WithState specifies the state type for this resource. The state type
// must either by a proto.Message or implement the ProtoMarshaler interface.
//
// An allocated zero value of this type will be made available during
// creation. The value given as v is NOT used directly; it is only used to
// determine the type. Therefore, the value v is ignored and shouldn't be used
// for initialization.
func WithState(v interface{}) ResourceOption {
	return func(r *Resource) { r.stateType = reflect.TypeOf(v) }
}

// WithPlatform specifies the name of the platform this resource is being created on
// (i.e. kubernetes, docker, etc).
//
// Corresponds to the protobuf DeclaredResource.Platform field
func WithPlatform(platform string) ResourceOption {
	return func(r *Resource) { r.platform = platform }
}

// WithCategoryDisplayHint specifies the category this resource belongs to.
// Used for display purposes only.
//
// Corresponds to the protobuf DeclaredResource.CategoryDisplayHint field
func WithCategoryDisplayHint(categoryDisplayHint pb.ResourceCategoryDisplayHint) ResourceOption {
	return func(r *Resource) { r.categoryDisplayHint = categoryDisplayHint }
}

func WithStatus(f interface{}) ResourceOption {
	return func(r *Resource) { r.statusFunc = f }
}

// markerValue returns a argmapper.Value that is unique to this resource.
// This is used by the resource manager to ensure that all resource
// lifecycle functions are called.
//
// Details on how this works: argmapper only calls the functions in its
// chain that are necessary to call the final function in the chain. In order
// to ensure a function is called, you must depend on a unique value that
// it outputs. The resource manager works by adding these unique marker values
// as dependencies on the final function in the chain, thus ensuring that
// the intermediate functions are called.
func markerValue(n string) argmapper.Value {
	val := markerType(struct{}{})
	return argmapper.Value{
		Type:    reflect.TypeOf(val),
		Subtype: n,
		Value:   reflect.ValueOf(val),
	}
}

var statusResponseType = reflect.TypeOf((*StatusResponse)(nil))

// setStateCallOrder is a weird global that is only used to construct an
// ordering for destroy when Resource.SetState is manually called. See the
// docs on SetState for more info. We expect this is a legacy thing hence
// its a bit hacky, but over time we can probably remove it.
var setStateCallOrder uint32
