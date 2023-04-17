// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package funcspec

import (
	"reflect"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/opaqueany"

	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// Func takes a FuncSpec and returns a *mapper.Func that can be called
// to invoke this function. The callback can have an argument type of Args
// in order to get access to the required dynamic proto.Any types of the
// FuncSpec.
func Func(s *pb.FuncSpec, cb interface{}, args ...argmapper.Arg) *argmapper.Func {
	// Build a Func around our callback so that we can inspect the
	// input/output sets since we want to merge with that.
	cbFunc, err := argmapper.NewFunc(cb)
	if err != nil {
		panic(err)
	}

	// Create the argmapper input values. All our args are expected to be
	// protobuf Any types that have a subtype matching our string name.
	// We append them directly to our expected values for the callback.
	// This lets us get our callback types in addition to our funcspec types.
	inputValues := cbFunc.Input().Values()
	for _, arg := range s.Args {
		value := argmapper.Value{Name: arg.Name, Subtype: arg.Type, Type: anyType}

		// If we have a primitive type set, then we set the proper type.
		switch arg.PrimitiveType {
		case pb.FuncSpec_Value_BOOL:
			value.Type = reflect.TypeOf(false)
		case pb.FuncSpec_Value_INT:
			value.Type = reflect.TypeOf(int(0))
		case pb.FuncSpec_Value_INT8:
			value.Type = reflect.TypeOf(int8(0))
		case pb.FuncSpec_Value_INT16:
			value.Type = reflect.TypeOf(int16(0))
		case pb.FuncSpec_Value_INT32:
			value.Type = reflect.TypeOf(int32(0))
		case pb.FuncSpec_Value_INT64:
			value.Type = reflect.TypeOf(int64(0))

		case pb.FuncSpec_Value_UINT:
			value.Type = reflect.TypeOf(uint(0))
		case pb.FuncSpec_Value_UINT8:
			value.Type = reflect.TypeOf(uint8(0))
		case pb.FuncSpec_Value_UINT16:
			value.Type = reflect.TypeOf(uint16(0))
		case pb.FuncSpec_Value_UINT32:
			value.Type = reflect.TypeOf(uint32(0))
		case pb.FuncSpec_Value_UINT64:
			value.Type = reflect.TypeOf(uint64(0))

		case pb.FuncSpec_Value_STRING:
			value.Type = reflect.TypeOf("")

		case pb.FuncSpec_Value_INVALID:
			// Ignore
		}

		inputValues = append(inputValues, value)
	}

	// Remove the Args value if there is one, since we're going to populate
	// that later and we don't need it for the initial call.
	for i, v := range inputValues {
		if v.Type == argsType {
			inputValues[i] = inputValues[len(inputValues)-1]
			inputValues = inputValues[:len(inputValues)-1]
			break
		}
	}

	inputSet, err := argmapper.NewValueSet(inputValues)
	if err != nil {
		panic(err)
	}

	// Build our output set. By default this just matches our output function.
	outputSet := cbFunc.Output()

	// If we have results specified on the Spec, then we expect this to represent
	// a mapper. Mapper callbacks MUST return *opaqueany.Any or []*opaqueany.Any. When we
	// have a mapper, we change the output type to be all the values we're
	// mapping to.
	if len(s.Result) > 0 {
		var outputValues []argmapper.Value
		for _, result := range s.Result {
			outputValues = append(outputValues, argmapper.Value{
				Name:    result.Name,
				Type:    anyType,
				Subtype: result.Type,
			})
		}

		outputSet, err = argmapper.NewValueSet(outputValues)
		if err != nil {
			panic(err)
		}
	}

	result, err := argmapper.BuildFunc(inputSet, outputSet, func(in, out *argmapper.ValueSet) error {
		callArgs := make([]argmapper.Arg, 0, len(args)+len(in.Values()))

		// Build up our callArgs which we'll pass to our callback. We pass
		// through all args except for *opaqueany.Any values. For *any values, we
		// add them to our Args list.
		var args Args
		for _, v := range in.Values() {
			// Append any *opaqueany.Any types or supported primitive to the Args
			_, okPrim := validPrimitive[v.Type.Kind()]
			if v.Type == anyType || okPrim {
				args = appendValue(args, v)
			}

			// If we have any other type, then we set it directly.
			callArgs = append(callArgs, v.Arg())
		}

		// Add our grouped Args type.
		callArgs = append(callArgs, argmapper.Typed(args))

		// Call into our callback. This populates our callback function output.
		cbOut := cbFunc.Output()
		if err := cbOut.FromResult(cbFunc.Call(callArgs...)); err != nil {
			return err
		}

		// If we aren't a mapper, we return now since we've populated our callback.
		if len(s.Result) == 0 {
			return nil
		}

		// We're a mapper, so we have to go through our values and look
		// for the *opaqueany.Any value or []*opaqueany.Any and populate our expected
		// outputs.
		for _, v := range cbOut.Values() {
			switch v.Type {
			case anyType:
				// We're seeing an *opaqueany.Any. So we encode this and try
				// to match it to any value that we have.
				anyVal := v.Value.Interface().(*opaqueany.Any)
				st := anyVal.MessageName()

				expected := out.TypedSubtype(v.Type, string(st))
				if expected == nil {
					continue
				}

				expected.Value = v.Value
			}
		}

		// Go through our callback output looking
		return nil
	}, append([]argmapper.Arg{
		argmapper.FuncName(s.Name),
		argmapper.ConverterGen(anyConvGen),
	}, args...)...)
	if err != nil {
		panic(err)
	}

	return result
}

var (
	anyType  = reflect.TypeOf((*opaqueany.Any)(nil))
	argsType = reflect.TypeOf(Args(nil))
)
