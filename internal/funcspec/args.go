package funcspec

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/opaqueany"

	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// Args is a type that will be populated with all the expected args of
// the FuncSpec. This can be used in the callback (cb) to Func.
type Args []*pb.FuncSpec_Value

// appendValue appends an argmapper.Value to Args. The value must be an *opaqueany.Any
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
	case *opaqueany.Any:
		value.Value = &pb.FuncSpec_Value_ProtoAny{ProtoAny: v}

	case bool:
		value.PrimitiveType = pb.FuncSpec_Value_BOOL
		value.Value = &pb.FuncSpec_Value_Bool{Bool: v}

	case int:
		value.PrimitiveType = pb.FuncSpec_Value_INT
		value.Value = &pb.FuncSpec_Value_Int{Int: int64(v)}
	case int8:
		value.PrimitiveType = pb.FuncSpec_Value_INT8
		value.Value = &pb.FuncSpec_Value_Int{Int: int64(v)}
	case int16:
		value.PrimitiveType = pb.FuncSpec_Value_INT16
		value.Value = &pb.FuncSpec_Value_Int{Int: int64(v)}
	case int32:
		value.PrimitiveType = pb.FuncSpec_Value_INT32
		value.Value = &pb.FuncSpec_Value_Int{Int: int64(v)}
	case int64:
		value.PrimitiveType = pb.FuncSpec_Value_INT64
		value.Value = &pb.FuncSpec_Value_Int{Int: int64(v)}

	case uint:
		value.PrimitiveType = pb.FuncSpec_Value_UINT
		value.Value = &pb.FuncSpec_Value_Uint{Uint: uint64(v)}
	case uint8:
		value.PrimitiveType = pb.FuncSpec_Value_UINT8
		value.Value = &pb.FuncSpec_Value_Uint{Uint: uint64(v)}
	case uint16:
		value.PrimitiveType = pb.FuncSpec_Value_UINT16
		value.Value = &pb.FuncSpec_Value_Uint{Uint: uint64(v)}
	case uint32:
		value.PrimitiveType = pb.FuncSpec_Value_UINT32
		value.Value = &pb.FuncSpec_Value_Uint{Uint: uint64(v)}
	case uint64:
		value.PrimitiveType = pb.FuncSpec_Value_UINT64
		value.Value = &pb.FuncSpec_Value_Uint{Uint: uint64(v)}

	case string:
		value.PrimitiveType = pb.FuncSpec_Value_STRING
		value.Value = &pb.FuncSpec_Value_String_{String_: v}

	default:
		panic(fmt.Sprintf("invalid value type for args: %T", v))
	}

	return append(args, value)
}
