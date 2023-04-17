// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"fmt"
	"reflect"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/opaqueany"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/hashicorp/waypoint-plugin-sdk/internal/funcspec"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// callDynamicFunc calls a dynamic (mapper-based) function with the
// given input arguments. This is a helper that is expected to be used
// by most component gRPC servers to implement their function calls.
func callDynamicFunc2(
	f interface{},
	args funcspec.Args,
	callArgs ...argmapper.Arg,
) (interface{}, error) {
	// Decode our *opaqueany.Any values.
	for _, arg := range args {
		var value interface{}
		var err error
		switch v := arg.Value.(type) {
		case *pb.FuncSpec_Value_ProtoAny:
			value, err = argProtoAny(arg)

		case *pb.FuncSpec_Value_Bool:
			value = v.Bool

		case *pb.FuncSpec_Value_Int:
			switch arg.PrimitiveType {
			case pb.FuncSpec_Value_INT8:
				value = int8(v.Int)
			case pb.FuncSpec_Value_INT16:
				value = int16(v.Int)
			case pb.FuncSpec_Value_INT32:
				value = int32(v.Int)
			case pb.FuncSpec_Value_INT64:
				value = int64(v.Int)
			case pb.FuncSpec_Value_INT:
				fallthrough
			default:
				// Fallback to int as a default
				value = int(v.Int)
			}

		case *pb.FuncSpec_Value_Uint:
			switch arg.PrimitiveType {
			case pb.FuncSpec_Value_INT8:
				value = uint8(v.Uint)
			case pb.FuncSpec_Value_INT16:
				value = uint16(v.Uint)
			case pb.FuncSpec_Value_INT32:
				value = uint32(v.Uint)
			case pb.FuncSpec_Value_INT64:
				value = uint64(v.Uint)
			case pb.FuncSpec_Value_INT:
				fallthrough
			default:
				// Fallback to uint as a default
				value = uint(v.Uint)
			}

		case *pb.FuncSpec_Value_String_:
			value = v.String_

		default:
			return nil, fmt.Errorf("internal error! invalid argument value: %#v",
				arg.Value)
		}
		if err != nil {
			return nil, err
		}

		callArgs = append(callArgs,
			argmapper.NamedSubtype(arg.Name, value, arg.Type),
		)
	}

	mapF, err := argmapper.NewFunc(f)
	if err != nil {
		return nil, err
	}

	result := mapF.Call(callArgs...)
	if err := result.Err(); err != nil {
		return nil, err
	}

	return result.Out(0), nil
}

// callDynamicFuncAny is callDynamicFunc that automatically encodes the
// result to an *opaqueany.Any.
func callDynamicFuncAny2(
	f interface{},
	args funcspec.Args,
	callArgs ...argmapper.Arg,
) (*opaqueany.Any, string, interface{}, error) {
	result, err := callDynamicFunc2(f, args, callArgs...)
	if err != nil {
		return nil, "", nil, err
	}

	// We expect the final result to always be a proto message so we can
	// send it back over the wire.
	//
	// NOTE(mitchellh): If we wanted to in the future, we can probably change
	// this to be any type that has a mapper that can take it to be a
	// proto.Message.
	msg, ok := result.(proto.Message)
	if !ok {
		return nil, "", nil, fmt.Errorf(
			"result of plugin-based function must be a proto.Message, got %T", msg)
	}

	anyVal, err := opaqueany.New(msg)
	if err != nil {
		return nil, "", nil, err
	}

	anyJson, err := protojson.Marshal(msg)
	if err != nil {
		return nil, "", nil, err
	}

	return anyVal, string(anyJson), result, err
}

func argProtoAny(arg *pb.FuncSpec_Value) (interface{}, error) {
	anyVal := arg.Value.(*pb.FuncSpec_Value_ProtoAny).ProtoAny

	name := anyVal.MessageName()

	mt, err := protoregistry.GlobalTypes.FindMessageByName(name)
	if err != nil {
		return nil, fmt.Errorf("cannot decode type: %s", name)
	}

	typ := reflect.TypeOf(proto.Message(mt.Zero().Interface()))

	// Allocate the message type. If it is a pointer we want to
	// allocate the actual structure and not the pointer to the structure.
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	v := reflect.New(typ)
	v.Elem().Set(reflect.Zero(typ))

	// Unmarshal directly into our newly allocated structure.
	if err := anyVal.UnmarshalTo(v.Interface().(proto.Message)); err != nil {
		return nil, err
	}

	return v.Interface(), nil
}
