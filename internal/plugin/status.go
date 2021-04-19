package plugin

import (
	"context"
	"reflect"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/funcspec"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/pluginargs"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// statusClient implements component.Status for a service that
// has the status methods implemented.
type statusClient struct {
	Client  statusProtoClient
	Logger  hclog.Logger
	Broker  *plugin.GRPCBroker
	Mappers []*argmapper.Func
}

func (c *statusClient) Implements(ctx context.Context) (bool, error) {
	if c == nil {
		return false, nil
	}

	resp, err := c.Client.IsStatus(ctx, &empty.Empty{})
	if err != nil {
		return false, err
	}

	return resp.Implements, nil
}

func (c *statusClient) StatusFunc() interface{} {
	impl, err := c.Implements(context.Background())
	if err != nil {
		return funcErr(err)
	}
	if !impl {
		return nil
	}

	// Get the spec
	spec, err := c.Client.StatusSpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	spec.Result = nil

	return funcspec.Func(spec, c.status,
		argmapper.Logger(c.Logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.Broker,
			Mappers: c.Mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *statusClient) status(
	ctx context.Context,
	args funcspec.Args,
	internal *pluginargs.Internal,
) (*pb.StatusReport, error) {
	// Run the cleanup
	defer internal.Cleanup.Close()

	// Call our function
	resp, err := c.Client.Status(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		return nil, err
	}

	// just return `resp` here, pass it along
	return resp, nil
}

// statusServer implements the common Status-related RPC calls.
// This should be embedded into the service implementation.
type statusServer struct {
	*base
	Impl interface{}
}

func (s *statusServer) IsStatus(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.ImplementsResp, error) {
	d, ok := s.Impl.(component.Status)
	return &pb.ImplementsResp{
		Implements: ok && d.StatusFunc() != nil,
	}, nil
}

func (s *statusServer) StatusSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	return funcspec.Spec(s.Impl.(component.Status).StatusFunc(),
		//argmapper.WithNoOutput(), // we only expect an error value so ignore the rest
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(s.internal()),

		argmapper.FilterOutput(argmapper.FilterType(
			reflect.TypeOf((*pb.StatusReport)(nil))),
		),
	)
}

func (s *statusServer) Status(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.StatusReport, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	raw, err := callDynamicFunc2(s.Impl.(component.Status).StatusFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(internal),
		argmapper.Typed(ctx),
	)
	if err != nil {
		return nil, err
	}

	result, ok := raw.(*pb.StatusReport)
	if !ok {
		return nil, status.Errorf(codes.Aborted, "status result is not a *proto.StatusReport")
	}

	// validation could happen here
	// just return the proto instead
	return result, nil
}

// statusProtoClient is the interface we expect any gRPC service that
// supports status to implement.
type statusProtoClient interface {
	IsStatus(context.Context, *empty.Empty, ...grpc.CallOption) (*pb.ImplementsResp, error)
	StatusSpec(context.Context, *empty.Empty, ...grpc.CallOption) (*pb.FuncSpec, error)
	Status(context.Context, *pb.FuncSpec_Args, ...grpc.CallOption) (*pb.StatusReport, error)
}

var (
	_ component.Status = (*statusClient)(nil)
)
