package plugin

import (
	"context"
	"reflect"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	empty "google.golang.org/protobuf/types/known/emptypb"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/docs"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/funcspec"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/pluginargs"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// ConfigSourcerPlugin implements plugin.Plugin (specifically GRPCPlugin) for
// the ConfigSourcer component type.
type ConfigSourcerPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	Impl    component.ConfigSourcer // Impl is the concrete implementation
	Mappers []*argmapper.Func       // Mappers
	Logger  hclog.Logger            // Logger
}

func (p *ConfigSourcerPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	base := &base{
		Mappers: p.Mappers,
		Logger:  p.Logger,
		Broker:  broker,
	}

	pb.RegisterConfigSourcerServer(s, &configSourcerServer{
		base: base,
		Impl: p.Impl,
	})
	return nil
}

func (p *ConfigSourcerPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	client := &configSourcerClient{
		client:  pb.NewConfigSourcerClient(c),
		logger:  p.Logger,
		broker:  broker,
		mappers: p.Mappers,
	}

	return client, nil
}

// configSourcerClient is an implementation of component.ConfigSourcer that
// communicates over gRPC.
type configSourcerClient struct {
	client  pb.ConfigSourcerClient
	logger  hclog.Logger
	broker  *plugin.GRPCBroker
	mappers []*argmapper.Func
}

func (c *configSourcerClient) Config() (interface{}, error) {
	return configStructCall(context.Background(), c.client)
}

func (c *configSourcerClient) ConfigSet(v interface{}) error {
	return configureCall(context.Background(), c.client, v)
}

func (c *configSourcerClient) Documentation() (*docs.Documentation, error) {
	return documentationCall(context.Background(), c.client)
}

func (c *configSourcerClient) ReadFunc() interface{} {
	// Get the spec
	spec, err := c.client.ReadSpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	// We don't want to be a mapper
	spec.Result = nil

	return funcspec.Func(spec, c.read,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *configSourcerClient) read(
	ctx context.Context,
	args funcspec.Args,
) ([]*pb.ConfigSource_Value, error) {
	// Call our function
	resp, err := c.client.Read(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		return nil, err
	}

	return resp.Values, nil
}

func (c *configSourcerClient) StopFunc() interface{} {
	// Get the spec
	spec, err := c.client.StopSpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	// We don't want to be a mapper
	spec.Result = nil

	return funcspec.Func(spec, c.stop,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *configSourcerClient) stop(
	ctx context.Context,
	args funcspec.Args,
) error {
	// Call our function
	_, err := c.client.Stop(ctx, &pb.FuncSpec_Args{Args: args})
	return err
}

// configSourcerServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type configSourcerServer struct {
	*base

	pb.UnimplementedConfigSourcerServer

	Impl component.ConfigSourcer
}

func (s *configSourcerServer) ConfigStruct(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_StructResp, error) {
	return configStruct(s.Impl)
}

func (s *configSourcerServer) Configure(
	ctx context.Context,
	req *pb.Config_ConfigureRequest,
) (*empty.Empty, error) {
	return configure(s.Impl, req)
}

func (s *configSourcerServer) Documentation(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_Documentation, error) {
	return documentation(s.Impl)
}

func (s *configSourcerServer) ReadSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: ConfigSourcer")
	}

	return funcspec.Spec(s.Impl.ReadFunc(),
		argmapper.Logger(s.Logger),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(s.internal()),

		argmapper.FilterOutput(argmapper.FilterType(
			reflect.TypeOf((*[]*pb.ConfigSource_Value)(nil)).Elem()),
		),
	)
}

func (s *configSourcerServer) Read(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.ConfigSource_ReadResponse, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	raw, err := callDynamicFunc2(s.Impl.ReadFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
	)
	if err != nil {
		return nil, err
	}

	values, ok := raw.([]*pb.ConfigSource_Value)
	if !ok {
		return nil, status.Errorf(codes.Aborted, "read result is not []*proto.ConfigSource_Value")
	}

	result := &pb.ConfigSource_ReadResponse{Values: values}
	return result, nil
}

func (s *configSourcerServer) StopSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: ConfigSourcer")
	}

	return funcspec.Spec(s.Impl.StopFunc(),
		argmapper.Logger(s.Logger),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(s.internal()),
	)
}

func (s *configSourcerServer) Stop(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*empty.Empty, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	_, err := callDynamicFunc2(s.Impl.StopFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
	)
	if err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}

var (
	_ plugin.Plugin                = (*ConfigSourcerPlugin)(nil)
	_ plugin.GRPCPlugin            = (*ConfigSourcerPlugin)(nil)
	_ pb.ConfigSourcerServer       = (*configSourcerServer)(nil)
	_ component.ConfigSourcer      = (*configSourcerClient)(nil)
	_ component.Configurable       = (*configSourcerClient)(nil)
	_ component.Documented         = (*configSourcerClient)(nil)
	_ component.ConfigurableNotify = (*configSourcerClient)(nil)
)
