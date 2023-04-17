// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package component

import (
	"reflect"

	"github.com/hashicorp/opaqueany"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

// ProtoMarshaler is the interface required by objects that must support
// protobuf marshaling. This expects the object to go to a proto.Message
// which is converted to a proto Any value[1]. The plugin is expected to
// register a proto type that can decode this Any value.
//
// This enables the project to encode intermediate objects (such as artifacts)
// and store them in a database.
//
// [1]: https://developers.google.com/protocol-buffers/docs/proto3#any
type ProtoMarshaler interface {
	// Proto returns a proto.Message of this structure. This may also return
	// a proto Any value and it will not be re-wrapped with Any.
	Proto() proto.Message
}

// Proto returns the proto.Message for a given value that is either already
// a proto.Message or implements ProtoMarshaler. If the value implements neither,
// then this returns (nil, nil).
func Proto(m interface{}) (proto.Message, error) {
	msg, ok := m.(proto.Message)
	if ok {
		return msg, nil
	}

	// If it isn't a message directly, we accept marshalers
	pm, ok := m.(ProtoMarshaler)
	if !ok {
		return nil, nil
	}

	return pm.Proto(), nil
}

// ProtoAny returns an *opaqueany.Any for the given ProtoMarshaler object. This
// will return nil if the given message is nil.
func ProtoAny(m interface{}) (*opaqueany.Any, error) {
	msg, err := Proto(m)
	if err != nil {
		return nil, err
	}

	// If the message is already an opaqueany.Any, then we're done
	if result, ok := msg.(*opaqueany.Any); ok {
		return result, nil
	}

	// If the message is an any.Any, then convert it to an opaqueany.Any
	if result, ok := msg.(*anypb.Any); ok {
		return &opaqueany.Any{
			TypeUrl: result.TypeUrl,
			Value:   result.Value,
		}, nil
	}

	// If we have a nil message, we don't marshal anything.
	if msg == nil {
		return nil, nil
	}

	return opaqueany.New(msg)
}

// ProtoAny returns []*opaqueany.Any for the given input slice by encoding
// each result into a proto value.
func ProtoAnySlice(m interface{}) ([]*opaqueany.Any, error) {
	val := reflect.ValueOf(m)
	result := make([]*opaqueany.Any, val.Len())
	for i := 0; i < val.Len(); i++ {
		var err error
		result[i], err = ProtoAny(val.Index(i).Interface())
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

// ProtoAnyUnmarshal attempts to unmarshal a ProtoMarshler implementation
// to another type. This can be used to get more concrete data out of a
// generic component.
func ProtoAnyUnmarshal(m interface{}, out proto.Message) error {
	msg, ok := m.(proto.Message)

	// If it isn't a message directly, we accept marshalers
	if !ok {
		pm, ok := m.(ProtoMarshaler)
		if !ok {
			return status.Errorf(codes.FailedPrecondition,
				"expected value to be a proto message, got %T",
				m)
		}

		msg = pm.Proto()
	}

	result, ok := msg.(*opaqueany.Any)
	if !ok {
		return status.Errorf(codes.FailedPrecondition, "expected *opaqueany.Any, got %T", msg)
	}

	// Unmarshal
	return result.UnmarshalTo(out)
}
