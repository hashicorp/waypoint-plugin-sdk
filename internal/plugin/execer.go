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

// execerClient implements component.Execer for a service that
// has the exec methods implemented.
type execerClient struct {
	Client  execerProtoClient
	Logger  hclog.Logger
	Broker  *plugin.GRPCBroker
	Mappers []*argmapper.Func
}

func (c *execerClient) Implements(ctx context.Context) (bool, error) {
	if c == nil {
		return false, nil
	}

	resp, err := c.Client.IsExecer(ctx, &empty.Empty{})
	if err != nil {
		// If the plugin doesn't implement IsExecer the RPC, then it definitely doesn't
		// implement it. If we return err here, it will blow up the whole usage of this
		// type so just say "sorry, not implemented" so the core can continue to run.
		if st, ok := status.FromError(err); ok && st.Code() == codes.Unavailable {
			return false, nil
		}
		return false, err
	}

	return resp.Implements, nil
}

func (c *execerClient) ExecFunc() interface{} {
	impl, err := c.Implements(context.Background())
	if err != nil {
		return funcErr(err)
	}
	if !impl {
		return nil
	}

	// Get the spec
	spec, err := c.Client.ExecSpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	return funcspec.Func(spec, c.exec,
		argmapper.Logger(c.Logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.Broker,
			Mappers: c.Mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *execerClient) exec(
	ctx context.Context,
	args funcspec.Args,
	internal *pluginargs.Internal,
) (*component.ExecResult, error) {
	// Run the cleanup
	defer internal.Cleanup.Close()

	// Call our function
	resp, err := c.Client.Exec(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		return nil, err
	}

	return &component.ExecResult{
		ExitCode: int(resp.ExitCode),
	}, nil
}

// execerServer implements the common Execer-related RPC calls.
// This should be embedded into the service implementation.
type execerServer struct {
	*base
	Impl interface{}
}

func (s *execerServer) IsExecer(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.ImplementsResp, error) {
	d, ok := s.Impl.(component.Execer)
	return &pb.ImplementsResp{
		Implements: ok && d.ExecFunc() != nil,
	}, nil
}

func (s *execerServer) ExecSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	return funcspec.Spec(s.Impl.(component.Execer).ExecFunc(),
		//argmapper.WithNoOutput(), // we only expect an error value so ignore the rest
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(s.internal()),
		argmapper.FilterOutput(
			argmapper.FilterType(reflect.TypeOf((*component.ExecResult)(nil))),
		),
	)
}

func (s *execerServer) Exec(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.ExecResult, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	result, err := callDynamicFunc2(s.Impl.(component.Execer).ExecFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(internal),
		argmapper.Typed(ctx),
	)
	if err != nil {
		return nil, err
	}

	ret := &pb.ExecResult{}

	if ec, ok := result.(*component.ExecResult); ok {
		ret.ExitCode = int32(ec.ExitCode)
	}

	return ret, nil
}

// execerProtoClient is the interface we expect any gRPC service that
// supports exec to implement.
type execerProtoClient interface {
	IsExecer(context.Context, *empty.Empty, ...grpc.CallOption) (*pb.ImplementsResp, error)
	ExecSpec(context.Context, *empty.Empty, ...grpc.CallOption) (*pb.FuncSpec, error)
	Exec(context.Context, *pb.FuncSpec_Args, ...grpc.CallOption) (*pb.ExecResult, error)
}

var (
	_ component.Execer = (*execerClient)(nil)
)
