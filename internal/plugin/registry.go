package plugin

import (
	"context"
	"encoding/json"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/docs"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/funcspec"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/pluginargs"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/plugincomponent"
	"github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
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

	proto.RegisterRegistryServer(s, &registryServer{
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
		client:  proto.NewRegistryClient(c),
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
	}

	return result, nil
}

// registryClient is an implementation of component.Registry over gRPC.
type registryClient struct {
	client  proto.RegistryClient
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
	resp, err := c.client.Push(ctx, &proto.FuncSpec_Args{Args: args})
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
		TemplateVal: tplData,
	}, nil
}

// registryServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type registryServer struct {
	proto.UnimplementedRegistryServer
	*base
	*authenticatorServer

	Impl component.Registry
}

func (s *registryServer) ConfigStruct(
	ctx context.Context,
	empty *empty.Empty,
) (*proto.Config_StructResp, error) {
	return configStruct(s.Impl)
}

func (s *registryServer) Configure(
	ctx context.Context,
	req *proto.Config_ConfigureRequest,
) (*empty.Empty, error) {
	return configure(s.Impl, req)
}

func (s *registryServer) Documentation(
	ctx context.Context,
	empty *empty.Empty,
) (*proto.Config_Documentation, error) {
	return documentation(s.Impl)
}

func (s *registryServer) PushSpec(
	ctx context.Context,
	args *empty.Empty,
) (*proto.FuncSpec, error) {
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
	args *proto.FuncSpec_Args,
) (*proto.Push_Resp, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	encoded, raw, err := callDynamicFuncAny2(s.Impl.PushFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
	)
	if err != nil {
		return nil, err
	}

	result := &proto.Push_Resp{Result: encoded}
	result.TemplateData, err = templateData(raw)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *registryServer) IsAuthenticator(context.Context, *emptypb.Empty) (*proto.ImplementsResp, error) {
	return nil, status.Errorf(codes.Unimplemented, "method IsAuthenticator not implemented")
}
func (s *registryServer) Auth(context.Context, *proto.FuncSpec_Args) (*proto.Auth_AuthResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Auth not implemented")
}
func (s *registryServer) AuthSpec(context.Context, *emptypb.Empty) (*proto.FuncSpec, error) {
	return nil, status.Errorf(codes.Unimplemented, "method AuthSpec not implemented")
}
func (s *registryServer) ValidateAuth(context.Context, *proto.FuncSpec_Args) (*emptypb.Empty, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateAuth not implemented")
}
func (s *registryServer) ValidateAuthSpec(context.Context, *emptypb.Empty) (*proto.FuncSpec, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ValidateAuthSpec not implemented")
}

var (
	_ plugin.Plugin                = (*RegistryPlugin)(nil)
	_ plugin.GRPCPlugin            = (*RegistryPlugin)(nil)
	_ proto.RegistryServer         = (*registryServer)(nil)
	_ component.Registry           = (*registryClient)(nil)
	_ component.Configurable       = (*registryClient)(nil)
	_ component.Documented         = (*registryClient)(nil)
	_ component.ConfigurableNotify = (*registryClient)(nil)
)
