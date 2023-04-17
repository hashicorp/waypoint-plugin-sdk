// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package plugin

import (
	"context"
	"encoding/json"

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
	"github.com/hashicorp/waypoint-plugin-sdk/internal/plugincomponent"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// ReleaseManagerPlugin implements plugin.Plugin (specifically GRPCPlugin) for
// the ReleaseManager component type.
type ReleaseManagerPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	Impl    component.ReleaseManager // Impl is the concrete implementation
	Mappers []*argmapper.Func        // Mappers
	Logger  hclog.Logger             // Logger
}

func (p *ReleaseManagerPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	base := &base{
		Mappers: p.Mappers,
		Logger:  p.Logger,
		Broker:  broker,
	}

	pb.RegisterReleaseManagerServer(s, &releaseManagerServer{
		base: base,
		Impl: p.Impl,

		authenticatorServer: &authenticatorServer{
			base: base,
			Impl: p.Impl,
		},

		destroyerServer: &destroyerServer{
			base: base,
			Impl: p.Impl,
		},

		workspaceDestroyerServer: &workspaceDestroyerServer{
			base: base,
			Impl: p.Impl,
		},

		statusServer: &statusServer{
			base: base,
			Impl: p.Impl,
		},
	})
	return nil
}

func (p *ReleaseManagerPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	client := &releaseManagerClient{
		client:  pb.NewReleaseManagerClient(c),
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
		p.Logger.Info("release plugin capable of auth")
	} else {
		authenticator = nil
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
		p.Logger.Info("release plugin capable of destroy")
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

	result := &mix_ReleaseManager_Authenticator{
		ConfigurableNotify: client,
		ReleaseManager:     client,
		Authenticator:      authenticator,
		Destroyer:          destroyer,
		WorkspaceDestroyer: wsDestroyer,
		Documented:         client,
		Status:             status,
	}

	return result, nil
}

// releaseManagerClient is an implementation of component.ReleaseManager that
// communicates over gRPC.
type releaseManagerClient struct {
	client  pb.ReleaseManagerClient
	logger  hclog.Logger
	broker  *plugin.GRPCBroker
	mappers []*argmapper.Func
}

func (c *releaseManagerClient) Config() (interface{}, error) {
	return configStructCall(context.Background(), c.client)
}

func (c *releaseManagerClient) ConfigSet(v interface{}) error {
	return configureCall(context.Background(), c.client, v)
}

func (c *releaseManagerClient) Documentation() (*docs.Documentation, error) {
	return documentationCall(context.Background(), c.client)
}

func (c *releaseManagerClient) ReleaseFunc() interface{} {
	if c == nil || c.client == nil {
		return nil
	}

	// Get the build spec
	spec, err := c.client.ReleaseSpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	// We don't want to be a mapper
	spec.Result = nil

	return funcspec.Func(spec, c.release,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *releaseManagerClient) release(
	ctx context.Context,
	args funcspec.Args,
	declaredResourcesResp *component.DeclaredResourcesResp,
) (component.Release, error) {
	// Call our function

	resp, err := c.client.Release(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		return nil, err
	}

	var tplData map[string]interface{}
	if len(resp.TemplateData) > 0 {
		if err := json.Unmarshal(resp.TemplateData, &tplData); err != nil {
			return nil, err
		}
	}

	declaredResourcesResp.DeclaredResources = resp.DeclaredResources.Resources

	return &plugincomponent.Release{
		Any:         resp.Result,
		Release:     resp.Release,
		TemplateVal: tplData,
	}, nil
}

// releaseManagerServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type releaseManagerServer struct {
	*base
	*authenticatorServer
	*destroyerServer
	*workspaceDestroyerServer
	*statusServer

	pb.UnsafeReleaseManagerServer

	Impl component.ReleaseManager
}

func (s *releaseManagerServer) ConfigStruct(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_StructResp, error) {
	return configStruct(s.Impl)
}

func (s *releaseManagerServer) Documentation(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_Documentation, error) {
	return documentation(s.Impl)
}

func (s *releaseManagerServer) Configure(
	ctx context.Context,
	req *pb.Config_ConfigureRequest,
) (*empty.Empty, error) {
	return configure(s.Impl, req)
}

func (s *releaseManagerServer) ReleaseSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: release manager")
	}

	return funcspec.Spec(s.Impl.ReleaseFunc(),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(s.internal()),
	)
}

func (s *releaseManagerServer) Release(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.Release_Resp, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	// Inject our outparameter, so we can capture the response after invocation
	declaredResourcesResp := &component.DeclaredResourcesResp{}

	raw, err := callDynamicFunc2(s.Impl.ReleaseFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
		argmapper.Typed(declaredResourcesResp),
	)
	if err != nil {
		return nil, err
	}
	encoded, err := component.ProtoAny(raw)
	if err != nil {
		return nil, err
	}

	release := raw.(component.Release)
	result := &pb.Release_Resp{
		Result: encoded,
		Release: &pb.Release{
			Url: release.URL(),
		},
		DeclaredResources: &pb.DeclaredResources{
			Resources: declaredResourcesResp.DeclaredResources,
		},
	}

	result.TemplateData, err = templateData(raw)
	if err != nil {
		return nil, err
	}

	return result, nil
}

var (
	_ plugin.Plugin                = (*ReleaseManagerPlugin)(nil)
	_ plugin.GRPCPlugin            = (*ReleaseManagerPlugin)(nil)
	_ pb.ReleaseManagerServer      = (*releaseManagerServer)(nil)
	_ component.ReleaseManager     = (*releaseManagerClient)(nil)
	_ component.Configurable       = (*releaseManagerClient)(nil)
	_ component.Documented         = (*releaseManagerClient)(nil)
	_ component.ConfigurableNotify = (*releaseManagerClient)(nil)
)
