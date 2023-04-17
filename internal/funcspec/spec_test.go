// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package funcspec

import (
	"reflect"
	"testing"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/stretchr/testify/require"
	empty "google.golang.org/protobuf/types/known/emptypb"

	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

func init() {
	hclog.L().SetLevel(hclog.Trace)
}

func TestSpec(t *testing.T) {
	t.Run("proto to proto", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(*empty.Empty) *empty.Empty { return nil })
		require.NoError(err)
		require.NotNil(spec)
		require.Len(spec.Args, 1)
		require.Empty(spec.Args[0].Name)
		require.Equal("google.protobuf.Empty", spec.Args[0].Type)
		require.Len(spec.Result, 1)
		require.Empty(spec.Result[0].Name)
		require.Equal("google.protobuf.Empty", spec.Result[0].Type)
	})

	t.Run("converted args to proto", func(t *testing.T) {
		require := require.New(t)

		type Foo struct{}

		spec, err := Spec(func(*Foo) *empty.Empty { return nil },
			argmapper.Converter(func(*empty.Empty) *Foo { return nil }),
		)
		require.NoError(err)
		require.NotNil(spec)
		require.Len(spec.Args, 1)
		require.Empty(spec.Args[0].Name)
		require.Equal("google.protobuf.Empty", spec.Args[0].Type)
		require.Len(spec.Result, 1)
		require.Empty(spec.Result[0].Name)
		require.Equal("google.protobuf.Empty", spec.Result[0].Type)
	})

	t.Run("primitive args", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(bool) *empty.Empty { return nil })
		require.NoError(err)
		require.NotNil(spec)
		require.Len(spec.Args, 1)
		require.Empty(spec.Args[0].Name)
		require.Equal("", spec.Args[0].Type)
		require.Equal(pb.FuncSpec_Value_BOOL, spec.Args[0].PrimitiveType)
		require.Len(spec.Result, 1)
		require.Empty(spec.Result[0].Name)
		require.Equal("google.protobuf.Empty", spec.Result[0].Type)
	})

	t.Run("named primitive args", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(struct {
			argmapper.Struct

			HasRegistry bool
		}) *empty.Empty {
			return nil
		})
		require.NoError(err)
		require.NotNil(spec)
		require.Len(spec.Args, 1)
		require.Equal("hasregistry", spec.Args[0].Name)
		require.Equal("", spec.Args[0].Type)
		require.Equal(pb.FuncSpec_Value_BOOL, spec.Args[0].PrimitiveType)
		require.Len(spec.Result, 1)
		require.Empty(spec.Result[0].Name)
		require.Equal("google.protobuf.Empty", spec.Result[0].Type)
	})

	t.Run("unsatisfied conversion", func(t *testing.T) {
		require := require.New(t)

		type Foo struct{}
		type Bar struct{}

		spec, err := Spec(func(*Foo) *empty.Empty { return nil },
			argmapper.Converter(func(*empty.Empty) *Bar { return nil }),
		)
		require.Error(err)
		require.Nil(spec)
	})

	t.Run("proto to int", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(*empty.Empty) int { return 0 })
		require.Error(err)
		require.Nil(spec)
	})

	t.Run("WithOutput proto to int", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(*empty.Empty) int { return 0 },
			argmapper.FilterOutput(argmapper.FilterType(reflect.TypeOf(int(0)))),
		)
		require.NoError(err)
		require.NotNil(spec)
		require.Len(spec.Args, 1)
		require.Empty(spec.Args[0].Name)
		require.Equal("google.protobuf.Empty", spec.Args[0].Type)
		require.Empty(spec.Result)
	})

	t.Run("WithOutput proto to interface, doesn't implement", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(*empty.Empty) struct{} { return struct{}{} },
			argmapper.FilterOutput(argmapper.FilterType(reflect.TypeOf((*testSpecInterface)(nil)).Elem())),
		)
		require.Error(err)
		require.Nil(spec)
	})

	t.Run("WithOutput proto to interface", func(t *testing.T) {
		require := require.New(t)

		spec, err := Spec(func(*empty.Empty) *testSpecInterfaceImpl { return nil },
			argmapper.FilterOutput(argmapper.FilterType(reflect.TypeOf((*testSpecInterface)(nil)).Elem())),
		)
		require.NoError(err)
		require.NotNil(spec)
		require.Len(spec.Args, 1)
		require.Empty(spec.Args[0].Name)
		require.Equal("google.protobuf.Empty", spec.Args[0].Type)
		require.Empty(spec.Result)
	})

	t.Run("args as extra values", func(t *testing.T) {
		require := require.New(t)

		type Foo struct{}
		type Bar struct{}

		spec, err := Spec(func(*Foo, *Bar) *empty.Empty { return nil },
			argmapper.Converter(func(*empty.Empty) *Foo { return nil }),
			argmapper.Typed(&Bar{}),
		)
		require.NoError(err)
		require.NotNil(spec)
		require.Len(spec.Args, 1)
		require.Empty(spec.Args[0].Name)
		require.Equal("google.protobuf.Empty", spec.Args[0].Type)
		require.Len(spec.Result, 1)
		require.Empty(spec.Result[0].Name)
		require.Equal("google.protobuf.Empty", spec.Result[0].Type)
	})
}

type testSpecInterface interface {
	hello()
}

type testSpecInterfaceImpl struct{}

func (testSpecInterfaceImpl) hello() {}
