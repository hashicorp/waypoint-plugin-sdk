package funcspec

import (
	"context"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/hashicorp/go-argmapper"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// Spec takes a function pointer and generates a FuncSpec from it. The
// function must only take arguments that are proto.Message implementations
// or have a chain of converters that directly convert to a proto.Message.
func Spec(fn interface{}, args ...argmapper.Arg) (*pb.FuncSpec, error) {
	if fn == nil {
		return nil, status.Errorf(codes.Unimplemented, "required plugin type not implemented")
	}

	filterProto := argmapper.FilterType(protoMessageType)

	// Outparameters do not need to be supplied by core, and should
	// be omitted from the advertised function spec.
	filterOutParameter := argmapper.FilterType(outParameterType)

	// Copy our args cause we're going to use append() and we don't
	// want to modify our caller.
	args = append([]argmapper.Arg{
		argmapper.FilterOutput(filterProto),
	}, args...)

	// Build our function
	f, err := argmapper.NewFunc(fn)
	if err != nil {
		return nil, err
	}

	filter := argmapper.FilterOr(
		argmapper.FilterType(contextType),
		filterPrimitive,
		filterProto,
		filterOutParameter,
	)

	// Redefine the function in terms of protobuf messages. "Redefine" changes
	// the inputs of a function to only require values that match our filter
	// function. In our case, that is protobuf messages.
	f, err = f.Redefine(append(args,
		argmapper.FilterInput(filter),
	)...)
	if err != nil {
		return nil, err
	}

	// Grab the input set of the function and build up our funcspec
	result := pb.FuncSpec{Name: f.Name()}
	for _, v := range f.Input().Values() {
		if !filterProto(v) && !filterPrimitive(v) {
			continue
		}

		val := &pb.FuncSpec_Value{Name: v.Name}
		switch {
		case filterProto(v):
			val.Type = typeToMessage(v.Type)

		case filterPrimitive(v):
			val.PrimitiveType = pb.FuncSpec_Value_PrimitiveType(v.Type.Kind())
		}

		result.Args = append(result.Args, val)
	}

	// Grab the output set and store that
	for _, v := range f.Output().Values() {
		// We only advertise proto types in output since those are the only
		// types we can send across the plugin boundary.
		if !filterProto(v) {
			continue
		}

		result.Result = append(result.Result, &pb.FuncSpec_Value{
			Name: v.Name,
			Type: typeToMessage(v.Type),
		})
	}

	return &result, nil
}

func typeToMessage(typ reflect.Type) string {
	return proto.MessageName(reflect.Zero(typ).Interface().(proto.Message))
}

func filterPrimitive(v argmapper.Value) bool {
	_, ok := validPrimitive[v.Type.Kind()]
	return ok
}

var (
	contextType      = reflect.TypeOf((*context.Context)(nil)).Elem()
	protoMessageType = reflect.TypeOf((*proto.Message)(nil)).Elem()
	outParameterType = reflect.TypeOf((*component.OutParameter)(nil)).Elem()

	// validPrimitive is the map of primitive types we support coming
	// over the plugin boundary. To add a new type to this, you must
	// update:
	//
	//  1. the Primitive enum in plugin.proto
	//  2. appendValue in args.go
	//  3. value.Type setting in func.go Func
	//  4. arg decoding in internal/plugin/dynamic_call.go
	//
	validPrimitive = map[reflect.Kind]struct{}{
		reflect.Bool:   {},
		reflect.Int:    {},
		reflect.Int8:   {},
		reflect.Int16:  {},
		reflect.Int32:  {},
		reflect.Int64:  {},
		reflect.Uint:   {},
		reflect.Uint8:  {},
		reflect.Uint16: {},
		reflect.Uint32: {},
		reflect.Uint64: {},
		reflect.String: {},
	}
)
