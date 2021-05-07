package resource

import (
	"reflect"

	"github.com/hashicorp/go-argmapper"
)

// markerType is used for markerValue on Resource.
type markerType struct{}

// Resource is a single resource type with an associated lifecycle and state.
// A "resource" is any external thing a plugin creates such as a load balancer,
// networking primitives, files, etc. Representing these things as "resources"
// assists in lifecycle management, such as preventing dangling resources
// in the case of an error and properly cleaning them up.
type Resource struct {
	seq         uint64
	name        string
	stateType   reflect.Type
	stateValue  interface{}
	createFunc  interface{}
	destroyFunc interface{}
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

	return &r
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

// Create creates this resource. args is a list of arguments to make
// available to the creation function via dependency injection (matching
// types in the arguments).
//
// After Create is called, any state can be accessed via the State function.
// This may be populated even during failure with partial state.
func (r *Resource) Create(args ...interface{}) error {
	f, err := r.mapperForCreate()
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
func (r *Resource) markerValue() argmapper.Value {
	val := markerType(struct{}{})
	return argmapper.Value{
		Type:    reflect.TypeOf(val),
		Subtype: r.name,
		Value:   reflect.ValueOf(val),
	}
}

// mapperForCreate returns an argmapper func that takes as input the
// requirements for the createFunc and returns the state type plus an error.
// This creates a valid "mapper" we can use with Manager.
func (r *Resource) mapperForCreate() (*argmapper.Func, error) {
	// Create the func for the createFunc as-is. We need to get the input/output sets.
	original, err := argmapper.NewFunc(r.createFunc)
	if err != nil {
		return nil, err
	}

	// For our output, we will always output our unique marker type.
	// This unique marker type will allow our resource manager to create
	// a function chain that calls all the resources necessary.
	markerVal := r.markerValue()
	outputs, err := argmapper.NewValueSet([]argmapper.Value{markerVal})
	if err != nil {
		return nil, err
	}

	// Our inputs default to whatever the function requires and our
	// output defaulst to nothing (only the error type). We will proceed to
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
			r.stateValue = reflect.New(r.stateType.Elem()).Interface()
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

		// Call our function. We throw away any result types except for the error.
		result := original.Call(args...)
		return result.Err()
	})
}

// ResourceOption is used to configure NewResource.
type ResourceOption func(*Resource)

// WithName sets the resource name. This name is used in output meant to be
// consumed by a user so it should be descriptive but short, such as
// "security group".
func WithName(n string) ResourceOption {
	return func(r *Resource) { r.name = n }
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
