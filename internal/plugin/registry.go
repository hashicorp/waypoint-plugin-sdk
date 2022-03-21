package plugin

import (
	"context"
	"encoding/json"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/docs"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/funcspec"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/pluginargs"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/plugincomponent"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	empty "google.golang.org/protobuf/types/known/emptypb"
)

// RegistryPlugin implements plugin.Plugin (specifically GRPCPlugin) for
// the Registry component type.
type RegistryPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	Impl    component.Registry // Impl is the concrete implementation
	Mappers []*argmapper.Func  // Mappers
	Logger  hclog.Logger       // Logger
}

func (p *RegistryPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	base := &base{
		Mappers: p.Mappers,
		Logger:  p.Logger,
		Broker:  broker,
	}

	pb.RegisterRegistryServer(s, &registryServer{
		base: base,
		Impl: p.Impl,

		authenticatorServer: &authenticatorServer{
			base: base,
			Impl: p.Impl,
		},
	})
	return nil
}

func (p *RegistryPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	client := &registryClient{
		client:  pb.NewRegistryClient(c),
		logger:  p.Logger,
		broker:  broker,
		mappers: p.Mappers,
	}

	authenticator := &authenticatorClient{
		Client:  client.client,
		Logger:  client.logger,
		Broker:  client.broker,
		Mappers: client.mappers,
	}
	if ok, err := authenticator.Implements(ctx); err != nil {
		return nil, err
	} else if ok {
		p.Logger.Info("registry plugin capable of auth")
	} else {
		authenticator = nil
	}

	result := &mix_Registry_Authenticator{
		ConfigurableNotify: client,
		Registry:           client,
		Authenticator:      authenticator,
		Documented:         client,
		RegistryAccess:     client,
	}

	return result, nil
}

// registryClient is an implementation of component.Registry over gRPC.
type registryClient struct {
	client  pb.RegistryClient
	logger  hclog.Logger
	broker  *plugin.GRPCBroker
	mappers []*argmapper.Func
}

func (c *registryClient) Config() (interface{}, error) {
	return configStructCall(context.Background(), c.client)
}

func (c *registryClient) ConfigSet(v interface{}) error {
	return configureCall(context.Background(), c.client, v)
}

func (c *registryClient) Documentation() (*docs.Documentation, error) {
	return documentationCall(context.Background(), c.client)
}

func (c *registryClient) PushFunc() interface{} {
	// Get the spec
	spec, err := c.client.PushSpec(context.Background(), &empty.Empty{})
	if err != nil {
		panic(err)
	}

	// We don't want to be a mapper
	spec.Result = nil

	return funcspec.Func(spec, c.push,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *registryClient) push(
	ctx context.Context,
	args funcspec.Args,
) (component.Artifact, error) {
	// Call our function
	resp, err := c.client.Push(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		return nil, err
	}

	var tplData map[string]interface{}
	if len(resp.TemplateData) > 0 {
		if err := json.Unmarshal(resp.TemplateData, &tplData); err != nil {
			return nil, err
		}
	}

	return &plugincomponent.Artifact{
		Any:         resp.Result,
		AnyJson:     resp.ResultJson,
		TemplateVal: tplData,
	}, nil
}

// AccessInfoFunc implements component.RegistryAccess. It returns nil if the
// remote plugin function doesn't actually implement the function, similiar
// to other optional interface functions.
func (c *registryClient) AccessInfoFunc() interface{} {
	// Get the spec
	spec, err := c.client.AccessSpec(context.Background(), &empty.Empty{})
	if err != nil {
		// Signal that this is not implemented.
		if status.Code(err) == codes.Unimplemented {
			return nil
		}

		panic(err)
	}

	// We don't want to be a mapper, WHICH MEANS that we get the real value instead
	// of an argmapper interval value.
	spec.Result = nil

	return funcspec.Func(spec, c.access,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *registryClient) access(
	ctx context.Context,
	args funcspec.Args,
) (component.AccessInfo, error) {
	// Call our function
	resp, err := c.client.Access(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		return nil, err
	}

	return &plugincomponent.AccessInfo{Any: resp.Result}, nil
}

// registryServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type registryServer struct {
	*base
	*authenticatorServer

	pb.UnsafeRegistryServer

	Impl component.Registry
}

func (s *registryServer) ConfigStruct(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_StructResp, error) {
	return configStruct(s.Impl)
}

func (s *registryServer) Configure(
	ctx context.Context,
	req *pb.Config_ConfigureRequest,
) (*empty.Empty, error) {
	return configure(s.Impl, req)
}

func (s *registryServer) Documentation(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_Documentation, error) {
	return documentation(s.Impl)
}

func (s *registryServer) PushSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: registry")
	}

	return funcspec.Spec(s.Impl.PushFunc(),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(s.internal()),
	)
}

func (s *registryServer) Push(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.Push_Resp, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	encoded, encodedJson, raw, err := callDynamicFuncAny2(s.Impl.PushFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
	)
	if err != nil {
		return nil, err
	}

	result := &pb.Push_Resp{Result: encoded, ResultJson: encodedJson}
	result.TemplateData, err = templateData(raw)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// AccessSpec returns the information about the plugins AccessInfoFunc function.
// If the plugin does not implement the function (as it is an optional interface)
// then a codes.Unimplemented is returned as an error.
func (s *registryServer) AccessSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: registry")
	}

	ra, ok := s.Impl.(component.RegistryAccess)
	if !ok {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: registry")
	}

	return funcspec.Spec(ra.AccessInfoFunc(),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(s.internal()),
	)
}

// Access calls the AccessInfoFunc on the plugin.
func (s *registryServer) Access(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.Access_Resp, error) {
	ra, ok := s.Impl.(component.RegistryAccess)
	if !ok {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: registry")
	}

	fn := ra.AccessInfoFunc()

	internal := s.internal()
	defer internal.Cleanup.Close()

	encoded, _, _, err := callDynamicFuncAny2(fn, args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
	)
	if err != nil {
		s.Logger.Error("error calling access info func", "error", err)
		return nil, err
	}

	result := &pb.Access_Resp{Result: encoded}

	return result, nil
}

var (
	_ plugin.Plugin                = (*RegistryPlugin)(nil)
	_ plugin.GRPCPlugin            = (*RegistryPlugin)(nil)
	_ pb.RegistryServer            = (*registryServer)(nil)
	_ component.Registry           = (*registryClient)(nil)
	_ component.Configurable       = (*registryClient)(nil)
	_ component.Documented         = (*registryClient)(nil)
	_ component.ConfigurableNotify = (*registryClient)(nil)
)
