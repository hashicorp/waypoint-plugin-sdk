package funcspec

import (
	"fmt"
	"reflect"

	"github.com/golang/protobuf/ptypes/any"
	"github.com/hashicorp/go-argmapper"

	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// Args is a type that will be populated with all the expected args of
// the FuncSpec. This can be used in the callback (cb) to Func.
type Args []*pb.FuncSpec_Value

// appendValue appends an argmapper.Value to Args. The value must be an *any.Any
// or a supported primitive value. If an invalid value is given this will panic.
func appendValue(args Args, v argmapper.Value) Args {
	value := &pb.FuncSpec_Value{
		Name: v.Name,
		Type: v.Subtype,
	}

	// If we have no value, use the zero value of the type. This can happen
	// when we're given a Value from an arg type, not a function call.
	if !v.Value.IsValid() {
		v.Value = reflect.Zero(v.Type)
	}

	switch v := v.Value.Interface().(type) {
	case *any.Any:
		value.Value = &pb.FuncSpec_Value_ProtoAny{ProtoAny: v}

	case bool:
		value.PrimitiveType = pb.FuncSpec_Value_BOOL
		value.Value = &pb.FuncSpec_Value_Bool{Bool: v}

	default:
		panic(fmt.Sprintf("invalid value type for args: %T", v))
	}

	return append(args, value)
}
