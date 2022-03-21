package plugin

import (
	"context"
	"reflect"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	empty "google.golang.org/protobuf/types/known/emptypb"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/funcspec"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/pluginargs"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// generationClient implements component.Generation for a service that
// has the generation ID methods implemented.
type generationClient struct {
	Client  generationProtoClient
	Logger  hclog.Logger
	Broker  *plugin.GRPCBroker
	Mappers []*argmapper.Func
}

func (c *generationClient) Implements(ctx context.Context) (bool, error) {
	if c == nil {
		return false, nil
	}

	resp, err := c.Client.IsGeneration(ctx, &empty.Empty{})
	if err != nil {
		return false, err
	}

	return resp.Implements, nil
}

func (c *generationClient) GenerationFunc() interface{} {
	impl, err := c.Implements(context.Background())
	if err != nil {
		return funcErr(err)
	}
	if !impl {
		return nil
	}

	// Get the spec
	spec, err := c.Client.GenerationSpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	return funcspec.Func(spec, c.generation,
		argmapper.Logger(c.Logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.Broker,
			Mappers: c.Mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *generationClient) generation(
	ctx context.Context,
	args funcspec.Args,
	internal *pluginargs.Internal,
) ([]byte, error) {
	// Run the cleanup
	defer internal.Cleanup.Close()

	// Call our function
	resp, err := c.Client.Generation(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		return nil, err
	}

	return resp.Id, nil
}

// generationServer implements the common Generation-related RPC calls.
// This should be embedded into the service implementation.
type generationServer struct {
	*base
	Impl interface{}
}

func (s *generationServer) IsGeneration(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.ImplementsResp, error) {
	d, ok := s.Impl.(component.Generation)
	return &pb.ImplementsResp{
		Implements: ok && d.GenerationFunc() != nil,
	}, nil
}

func (s *generationServer) GenerationSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	return funcspec.Spec(s.Impl.(component.Generation).GenerationFunc(),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(s.internal()),
		argmapper.FilterOutput(
			argmapper.FilterType(reflect.TypeOf([]byte(nil))),
		),
	)
}

func (s *generationServer) Generation(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.Generation_Resp, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	resp, err := callDynamicFunc2(s.Impl.(component.Generation).GenerationFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(internal),
		argmapper.Typed(ctx),
	)
	if err != nil {
		return nil, err
	}

	return &pb.Generation_Resp{
		Id: resp.([]byte),
	}, nil
}

// generationProtoClient is the interface we expect any gRPC service that
// supports component.Generation to implement.
type generationProtoClient interface {
	IsGeneration(context.Context, *empty.Empty, ...grpc.CallOption) (*pb.ImplementsResp, error)
	GenerationSpec(context.Context, *empty.Empty, ...grpc.CallOption) (*pb.FuncSpec, error)
	Generation(context.Context, *pb.FuncSpec_Args, ...grpc.CallOption) (*pb.Generation_Resp, error)
}

var (
	_ component.Generation = (*generationClient)(nil)
)
