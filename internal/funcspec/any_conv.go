package funcspec

import (
	"reflect"

	"github.com/evanphx/opaqueany"
	"github.com/hashicorp/go-argmapper"
	"google.golang.org/protobuf/proto"
)

// anyConvGen is an argmapper.ConverterGenFunc that dynamically creates
// converters to *opaqueany.Any for types that implement proto.Message. This
// allows automatic conversion to *opaqueopaqueany.Any.
//
// This is automatically injected for all funcspec.Func calls.
func anyConvGen(v argmapper.Value) (*argmapper.Func, error) {
	anyType := reflect.TypeOf((*opaqueany.Any)(nil))
	protoMessageType := reflect.TypeOf((*proto.Message)(nil)).Elem()
	if !v.Type.Implements(protoMessageType) {
		return nil, nil
	}

	// We take this value as our input.
	inputSet, err := argmapper.NewValueSet([]argmapper.Value{v})
	if err != nil {
		return nil, err
	}

	// Generate an int with the subtype of the string value
	outputSet, err := argmapper.NewValueSet([]argmapper.Value{{
		Name:    v.Name,
		Type:    anyType,
		Subtype: string(proto.MessageName(reflect.Zero(v.Type).Interface().(proto.Message))),
	}})
	if err != nil {
		return nil, err
	}

	return argmapper.BuildFunc(inputSet, outputSet, func(in, out *argmapper.ValueSet) error {
		anyVal, err := opaqueany.New(inputSet.Typed(v.Type).Value.Interface().(proto.Message))
		if err != nil {
			return err
		}

		outputSet.Typed(anyType).Value = reflect.ValueOf(anyVal)
		return nil
	})
}

type protoToAny interface {
	Proto() *opaqueany.Any
}

func fromConvGen(v argmapper.Value) (*argmapper.Func, error) {
	anyType := reflect.TypeOf((*opaqueany.Any)(nil))
	protoMessageType := reflect.TypeOf((*protoToAny)(nil)).Elem()
	if !v.Type.Implements(protoMessageType) {
		return nil, nil
	}

	// We take this value as our input.
	inputSet, err := argmapper.NewValueSet([]argmapper.Value{v})
	if err != nil {
		return nil, err
	}

	// Generate an int with the subtype of the string value
	outputSet, err := argmapper.NewValueSet([]argmapper.Value{{
		Name:    v.Name,
		Type:    anyType,
		Subtype: string(proto.MessageName(reflect.Zero(v.Type).Interface().(proto.Message))),
	}})
	if err != nil {
		return nil, err
	}

	return argmapper.BuildFunc(inputSet, outputSet, func(in, out *argmapper.ValueSet) error {
		anyVal, err := opaqueany.New(inputSet.Typed(v.Type).Value.Interface().(proto.Message))
		if err != nil {
			return err
		}

		outputSet.Typed(anyType).Value = reflect.ValueOf(anyVal)
		return nil
	})
}
