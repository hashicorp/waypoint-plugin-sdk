package plugin

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/funcspec"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/pluginargs"
	proto "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// logClient is an implementation of component.LogPlatform over gRPC.
type logClient struct {
	Client  proto.PlatformClient
	Logger  hclog.Logger
	Broker  *plugin.GRPCBroker
	Mappers []*argmapper.Func
}

func (c *logClient) Implements(ctx context.Context) (bool, error) {
	if c == nil {
		return false, nil
	}

	resp, err := c.Client.IsLogPlatform(ctx, &empty.Empty{})
	if err != nil {
		return false, err
	}

	return resp.Implements, nil
}

func (c *logClient) LogsFunc() interface{} {
	// Get the spec
	spec, err := c.Client.LogsSpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	// We don't want to be a mapper
	spec.Result = nil

	return funcspec.Func(spec, c.logs,
		argmapper.Logger(c.Logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.Broker,
			Mappers: c.Mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *logClient) logs(
	ctx context.Context,
	args funcspec.Args,
	internal *pluginargs.Internal,
) error {
	// Run the cleanup
	defer internal.Cleanup.Close()

	// Call our function
	_, err := c.Client.Logs(ctx, &proto.FuncSpec_Args{Args: args})
	return err
}

// logPlatformServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type logPlatformServer struct {
	*base

	Impl interface{}
}

func (s *logPlatformServer) IsLogPlatform(
	ctx context.Context,
	empty *empty.Empty,
) (*proto.ImplementsResp, error) {
	d, ok := s.Impl.(component.LogPlatform)
	return &proto.ImplementsResp{
		Implements: ok && d.LogsFunc() != nil,
	}, nil
}

func (s *logPlatformServer) LogsSpec(
	ctx context.Context,
	args *empty.Empty,
) (*proto.FuncSpec, error) {
	return funcspec.Spec(s.Impl.(component.LogPlatform).LogsFunc(),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(s.internal()),
	)
}

func (s *logPlatformServer) Logs(
	ctx context.Context,
	args *proto.FuncSpec_Args,
) (*empty.Empty, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	_, err := callDynamicFunc2(s.Impl.(component.LogPlatform).LogsFunc(), args.Args,
		argmapper.Typed(ctx),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(internal),
	)
	return &empty.Empty{}, err
}

var (
	_ component.LogPlatform = (*logClient)(nil)
)
