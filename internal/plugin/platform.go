package plugin

import (
	"context"
	"encoding/json"
	"reflect"

	"github.com/davecgh/go-spew/spew"
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
	proto "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// PlatformPlugin implements plugin.Plugin (specifically GRPCPlugin) for
// the Platform component type.
type PlatformPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	Impl    component.Platform // Impl is the concrete implementation
	Mappers []*argmapper.Func  // Mappers
	Logger  hclog.Logger       // Logger
}

func (p *PlatformPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	base := &base{
		Mappers: p.Mappers,
		Logger:  p.Logger,
		Broker:  broker,
	}

	proto.RegisterPlatformServer(s, &platformServer{
		base: base,
		destroyerServer: &destroyerServer{
			base: base,
			Impl: p.Impl,
		},
		workspaceDestroyerServer: &workspaceDestroyerServer{
			base: base,
			Impl: p.Impl,
		},

		authenticatorServer: &authenticatorServer{
			base: base,
			Impl: p.Impl,
		},
		execerServer: &execerServer{
			base: base,
			Impl: p.Impl,
		},
		logPlatformServer: &logPlatformServer{
			base: base,
			Impl: p.Impl,
		},
		generationServer: &generationServer{
			base: base,
			Impl: p.Impl,
		},
		statusServer: &statusServer{
			base: base,
			Impl: p.Impl,
		},

		Impl: p.Impl,
	})

	return nil
}

func (p *PlatformPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	// Build our client to the platform service
	client := &platformClient{
		client:  proto.NewPlatformClient(c),
		logger:  p.Logger,
		broker:  broker,
		mappers: p.Mappers,
	}

	log := &logClient{
		Client:  client.client,
		Logger:  client.logger,
		Broker:  client.broker,
		Mappers: client.mappers,
	}
	if ok, err := log.Implements(ctx); err != nil {
		return nil, err
	} else if ok {
		p.Logger.Info("platform plugin capable of destroy")
	} else {
		log = nil
	}

	// Compose destroyer
	destroyer := &destroyerClient{
		Client:  client.client,
		Logger:  client.logger,
		Broker:  client.broker,
		Mappers: client.mappers,
	}
	if ok, err := destroyer.Implements(ctx); err != nil {
		return nil, err
	} else if ok {
		p.Logger.Info("platform plugin capable of destroy")
	} else {
		destroyer = nil
	}

	// Compose workspace destroyer
	wsDestroyer := &workspaceDestroyerClient{
		Client:  client.client,
		Logger:  client.logger,
		Broker:  client.broker,
		Mappers: client.mappers,
	}
	if ok, err := wsDestroyer.Implements(ctx); err != nil {
		return nil, err
	} else if ok {
		p.Logger.Info("platform plugin capable of destroy")
	} else {
		wsDestroyer = nil
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
		p.Logger.Info("platform plugin capable of auth")
	} else {
		authenticator = nil
	}

	execer := &execerClient{
		Client:  client.client,
		Logger:  client.logger,
		Broker:  client.broker,
		Mappers: client.mappers,
	}
	if ok, err := execer.Implements(ctx); err != nil {
		return nil, err
	} else if ok {
		p.Logger.Info("platform plugin capable of auth")
	} else {
		execer = nil
	}

	generation := &generationClient{
		Client:  client.client,
		Logger:  client.logger,
		Broker:  client.broker,
		Mappers: client.mappers,
	}
	if ok, err := generation.Implements(ctx); err != nil {
		return nil, err
	} else if ok {
		p.Logger.Info("platform plugin capable of generation ID creation")
	} else {
		generation = nil
	}

	status := &statusClient{
		Client:  client.client,
		Logger:  client.logger,
		Broker:  client.broker,
		Mappers: client.mappers,
	}
	if ok, err := status.Implements(ctx); err != nil {
		return nil, err
	} else if ok {
		p.Logger.Info("platform plugin capable of status")
	} else {
		status = nil
	}

	// Figure out what we're returning
	var result interface{} = client
	switch {
	case destroyer != nil:
		result = &mix_Platform_Destroy{
			Authenticator:      authenticator,
			ConfigurableNotify: client,
			Platform:           client,
			PlatformReleaser:   client,
			Destroyer:          destroyer,
			WorkspaceDestroyer: wsDestroyer,
			Documented:         client,
			Execer:             execer,
			LogPlatform:        log,
			Generation:         generation,
			Status:             status,
		}
	case execer != nil:
		result = &mix_Platform_Exec{
			Authenticator:      authenticator,
			ConfigurableNotify: client,
			Platform:           client,
			PlatformReleaser:   client,
			Execer:             execer,
			Documented:         client,
			LogPlatform:        log,
			Generation:         generation,
			Status:             status,
		}
	default:
		result = &mix_Platform_Authenticator{
			Authenticator:      authenticator,
			ConfigurableNotify: client,
			Platform:           client,
			PlatformReleaser:   client,
			WorkspaceDestroyer: wsDestroyer,
			Documented:         client,
			LogPlatform:        log,
			Generation:         generation,
			Status:             status,
		}
	}

	return result, nil
}

// platformClient is an implementation of component.Platform over gRPC.
type platformClient struct {
	client  proto.PlatformClient
	logger  hclog.Logger
	broker  *plugin.GRPCBroker
	mappers []*argmapper.Func
}

func (c *platformClient) Config() (interface{}, error) {
	return configStructCall(context.Background(), c.client)
}

func (c *platformClient) ConfigSet(v interface{}) error {
	return configureCall(context.Background(), c.client, v)
}

func (c *platformClient) Documentation() (*docs.Documentation, error) {
	return documentationCall(context.Background(), c.client)
}

func (c *platformClient) DeployFunc() interface{} {
	// Get the spec
	spec, err := c.client.DeploySpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	// We don't want to be a mapper
	spec.Result = nil

	return funcspec.Func(spec, c.deploy,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *platformClient) deploy(
	ctx context.Context,
	args funcspec.Args,
	internal *pluginargs.Internal,
	declaredResourcesResp *component.DeclaredResourcesResp,
) (component.Deployment, error) {
	// Run the cleanup
	defer internal.Cleanup.Close()

	// Call our function
	resp, err := c.client.Deploy(ctx, &proto.FuncSpec_Args{Args: args})
	if err != nil {
		return nil, err
	}

	var tplData map[string]interface{}
	if len(resp.TemplateData) > 0 {
		if err := json.Unmarshal(resp.TemplateData, &tplData); err != nil {
			return nil, err
		}
	}

	// Add declared resources to our outparameter so the caller can access them
	declaredResourcesResp.DeclaredResources = resp.DeclaredResources.Resources

	return &plugincomponent.Deployment{
		Any:         resp.Result,
		Deployment:  resp.Deployment,
		TemplateVal: tplData,
	}, nil
}

func (c *platformClient) DefaultReleaserFunc() interface{} {
	// Get the spec. If it is unimplemented thats no big deal we can just
	// return nil and the caller will handle this properly.
	spec, err := c.client.DefaultReleaserSpec(context.Background(), &empty.Empty{})
	if status.Code(err) == codes.Unimplemented {
		return nil
	}
	if err != nil {
		return funcErr(err)
	}

	// We don't want to be a mapper
	spec.Result = nil

	return funcspec.Func(spec, c.defaultReleaser,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *platformClient) defaultReleaser(
	ctx context.Context,
	args funcspec.Args,
	internal *pluginargs.Internal,
) (component.ReleaseManager, error) {
	// Run the cleanup
	defer internal.Cleanup.Close()

	// Call our function
	resp, err := c.client.DefaultReleaser(ctx, &proto.FuncSpec_Args{Args: args})
	if err != nil {
		return nil, err
	}

	// Get the stream ID and connect to it
	conn, err := c.broker.Dial(resp.StreamId)
	if err != nil {
		return nil, err
	}

	return &releaseManagerClient{
		client:  proto.NewReleaseManagerClient(conn),
		logger:  c.logger.Named("releaser"),
		broker:  c.broker,
		mappers: c.mappers,
	}, nil
}

// platformServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type platformServer struct {
	*base
	*destroyerServer
	*workspaceDestroyerServer
	*authenticatorServer
	*execerServer
	*logPlatformServer
	*generationServer
	*statusServer

	Impl component.Platform
}

func (s *platformServer) IsLogPlatform(
	ctx context.Context,
	empty *empty.Empty,
) (*proto.ImplementsResp, error) {
	_, ok := s.Impl.(component.LogPlatform)
	return &proto.ImplementsResp{Implements: ok}, nil
}

func (s *platformServer) ConfigStruct(
	ctx context.Context,
	empty *empty.Empty,
) (*proto.Config_StructResp, error) {
	return configStruct(s.Impl)
}

func (s *platformServer) Configure(
	ctx context.Context,
	req *proto.Config_ConfigureRequest,
) (*empty.Empty, error) {
	return configure(s.Impl, req)
}

func (s *platformServer) Documentation(
	ctx context.Context,
	empty *empty.Empty,
) (*proto.Config_Documentation, error) {
	docs, err := documentation(s.Impl)

	if docs != nil {
		s.Logger.Debug("docs", "docs", spew.Sdump(docs))
	}

	return docs, err
}

func (s *platformServer) DeploySpec(
	ctx context.Context,
	args *empty.Empty,
) (*proto.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: platform")
	}

	return funcspec.Spec(s.Impl.DeployFunc(),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(s.internal()),
	)
}

func (s *platformServer) Deploy(
	ctx context.Context,
	args *proto.FuncSpec_Args,
) (*proto.Deploy_Resp, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	// Inject our outparameter, so we can capture the response after invocation
	declaredResourcesResp := &component.DeclaredResourcesResp{}

	encoded, raw, err := callDynamicFuncAny2(s.Impl.DeployFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(internal),
		argmapper.Typed(ctx),
		argmapper.Typed(declaredResourcesResp),
	)
	if err != nil {
		return nil, err
	}

	result := &proto.Deploy_Resp{
		Result:     encoded,
		Deployment: &proto.Deploy{},
		DeclaredResources: &proto.DeclaredResources{
			Resources: declaredResourcesResp.DeclaredResources,
		},
	}

	deploymentWithUrl, ok := raw.(component.DeploymentWithUrl)
	if ok {
		result.Deployment.Url = deploymentWithUrl.URL()
	}

	result.TemplateData, err = templateData(raw)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *platformServer) DefaultReleaserSpec(
	ctx context.Context,
	args *empty.Empty,
) (*proto.FuncSpec, error) {
	var f interface{}
	if impl, ok := s.Impl.(component.PlatformReleaser); ok {
		f = impl.DefaultReleaserFunc()
	}

	// If there is no function, then we don't implement this.
	if f == nil {
		return nil, status.Errorf(codes.Unimplemented, "")
	}

	return funcspec.Spec(f,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(s.internal()),

		// We expect a component.LogViewer output type and not a proto.Message
		argmapper.FilterOutput(argmapper.FilterType(
			reflect.TypeOf((*component.ReleaseManager)(nil)).Elem()),
		),
	)
}

func (s *platformServer) DefaultReleaser(
	ctx context.Context,
	args *proto.FuncSpec_Args,
) (*proto.DefaultReleaser_Resp, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	impl, ok := s.Impl.(component.PlatformReleaser)
	if !ok {
		return nil, status.Errorf(codes.Unimplemented, "")
	}

	raw, err := callDynamicFunc2(impl.DefaultReleaserFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(internal),
		argmapper.Typed(ctx),
	)
	if err != nil {
		return nil, err
	}

	releaser, ok := raw.(component.ReleaseManager)
	if !ok || releaser == nil {
		return nil, status.Errorf(codes.FailedPrecondition,
			"plugin DefaultReleaser function should've returned a component.ReleaseManager, got %T",
			raw)
	}

	// Get the ID for the server we're going to start to run our viewer
	id := s.Broker.NextId()

	// Start our server
	go s.Broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		base := *s.base
		base.Logger = s.Logger.Named("releaser")

		server := plugin.DefaultGRPCServer(opts)
		proto.RegisterReleaseManagerServer(server, &releaseManagerServer{
			Impl: releaser,
			base: &base,
		})
		return server
	})

	return &proto.DefaultReleaser_Resp{StreamId: id}, nil
}

var (
	_ plugin.Plugin                = (*PlatformPlugin)(nil)
	_ plugin.GRPCPlugin            = (*PlatformPlugin)(nil)
	_ proto.PlatformServer         = (*platformServer)(nil)
	_ component.Platform           = (*platformClient)(nil)
	_ component.PlatformReleaser   = (*platformClient)(nil)
	_ component.Configurable       = (*platformClient)(nil)
	_ component.ConfigurableNotify = (*platformClient)(nil)
)
