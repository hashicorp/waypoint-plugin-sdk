// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package funcspec

import (
	"reflect"
	"testing"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/opaqueany"
	"github.com/stretchr/testify/require"
	empty "google.golang.org/protobuf/types/known/emptypb"

	"github.com/hashicorp/go-hclog"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

func init() {
	hclog.L().SetLevel(hclog.Trace)
}

func TestFunc(t *testing.T) {
	t.Run("single any result", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(*empty.Empty) *empty.Empty { return &empty.Empty{} })
		require.NoError(err)
		require.NotNil(spec)

		f := Func(spec, func(args Args) (*opaqueany.Any, error) {
			require.Len(args, 1)
			require.NotNil(args[0])

			// At this point we'd normally RPC out.
			return opaqueany.New(&empty.Empty{})
		})

		msg, err := opaqueany.New(&empty.Empty{})
		require.NoError(err)

		name := string((&empty.Empty{}).ProtoReflect().Descriptor().FullName())
		result := f.Call(argmapper.TypedSubtype(msg, name))
		require.NoError(result.Err())
		require.Equal(reflect.Struct, reflect.ValueOf(result.Out(0)).Kind())
	})

	t.Run("single missing requirement", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(*empty.Empty) *empty.Empty { return &empty.Empty{} })
		require.NoError(err)
		require.NotNil(spec)

		f := Func(spec, func(args Args) (*opaqueany.Any, error) {
			require.Len(args, 1)
			require.NotNil(args[0])

			// At this point we'd normally RPC out.
			return opaqueany.New(&empty.Empty{})
		})

		// Create an argument with the wrong type
		msg, err := opaqueany.New(&empty.Empty{})
		require.NoError(err)
		name := string((&pb.FuncSpec{}).ProtoReflect().Descriptor().FullName())
		result := f.Call(argmapper.TypedSubtype(msg, name))

		// We should have an error
		require.Error(result.Err())
		require.IsType(result.Err(), &argmapper.ErrArgumentUnsatisfied{})
	})

	t.Run("match callback output if no results", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(*empty.Empty) *empty.Empty { return &empty.Empty{} })
		require.NoError(err)
		require.NotNil(spec)

		// No results
		spec.Result = nil

		// Build our func to return a primitive
		f := Func(spec, func(args Args) int {
			require.Len(args, 1)
			require.NotNil(args[0])
			return 42
		})

		// Call the function with the proto type we expect
		msg, err := opaqueany.New(&empty.Empty{})
		require.NoError(err)

		name := string((&empty.Empty{}).ProtoReflect().Descriptor().FullName())
		result := f.Call(argmapper.TypedSubtype(msg, name))

		// Should succeed and give us our primitive
		require.NoError(result.Err())
		require.Equal(42, result.Out(0))
	})

	t.Run("provide input arguments", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(*empty.Empty) *empty.Empty { return &empty.Empty{} })
		require.NoError(err)
		require.NotNil(spec)

		f := Func(spec, func(args Args, v int) (*opaqueany.Any, error) {
			require.Len(args, 2)
			require.NotNil(args[0])
			require.NotNil(args[1])
			require.Equal(42, v)

			// At this point we'd normally RPC out.
			return opaqueany.New(&empty.Empty{})
		}, argmapper.Typed(int(42)))

		msg, err := opaqueany.New(&empty.Empty{})
		require.NoError(err)

		name := string((&empty.Empty{}).ProtoReflect().Descriptor().FullName())
		result := f.Call(argmapper.TypedSubtype(msg, name))
		require.NoError(result.Err())
		require.Equal(reflect.Struct, reflect.ValueOf(result.Out(0)).Kind())
	})

	t.Run("primitive arguments", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(v bool) *empty.Empty { return &empty.Empty{} })
		require.NoError(err)
		require.NotNil(spec)

		f := Func(spec, func(args Args, v int) (*opaqueany.Any, error) {
			require.Len(args, 2)
			require.NotNil(args[0])
			require.NotNil(args[1])
			require.Equal(42, v)

			// At this point we'd normally RPC out.
			return opaqueany.New(&empty.Empty{})
		}, argmapper.Typed(int(42)))

		result := f.Call(argmapper.Typed(true))
		require.NoError(result.Err())
		require.Equal(reflect.Struct, reflect.ValueOf(result.Out(0)).Kind())
	})
}
