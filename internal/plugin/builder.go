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

// BuilderPlugin implements plugin.Plugin (specifically GRPCPlugin) for
// the Builder component type.
type BuilderPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	Impl    component.Builder // Impl is the concrete implementation
	Mappers []*argmapper.Func // Mappers
	Logger  hclog.Logger      // Logger

	ODR *ODRSetting // Used to switch builder modes based on ondemand-runner in play
}

func (p *BuilderPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	base := &base{
		Mappers: p.Mappers,
		Logger:  p.Logger,
		Broker:  broker,
	}

	pb.RegisterBuilderServer(s, &builderServer{
		base: base,
		Impl: p.Impl,

		authenticatorServer: &authenticatorServer{
			base: base,
			Impl: p.Impl,
		},
	})
	return nil
}

func (p *BuilderPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	client := &builderClient{
		client:  pb.NewBuilderClient(c),
		logger:  p.Logger,
		broker:  broker,
		mappers: p.Mappers,
	}

	if p.ODR != nil && p.ODR.Enabled {
		client.odr = true
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
		p.Logger.Info("builder plugin capable of auth")
	} else {
		authenticator = nil
	}

	result := &mix_Builder_Authenticator{
		ConfigurableNotify: client,
		Builder:            client,
		Authenticator:      authenticator,
		Documented:         client,
	}

	return result, nil
}

// builderClient is an implementation of component.Builder that
// communicates over gRPC.
type builderClient struct {
	client  pb.BuilderClient
	logger  hclog.Logger
	broker  *plugin.GRPCBroker
	mappers []*argmapper.Func

	// indicates that the ODR version of the plugin should be used
	odr bool
}

func (c *builderClient) Config() (interface{}, error) {
	return configStructCall(context.Background(), c.client)
}

func (c *builderClient) ConfigSet(v interface{}) error {
	return configureCall(context.Background(), c.client, v)
}

func (c *builderClient) Documentation() (*docs.Documentation, error) {
	return documentationCall(context.Background(), c.client)
}

func (c *builderClient) BuildFunc() interface{} {
	if c.odr {
		c.logger.Debug("Running in ODR mode, attempting to retrieve ODR build spec")

		// Get the build spec
		spec, err := c.client.BuildSpecODR(context.Background(), &empty.Empty{})
		if err != nil {
			if status.Code(err) == codes.Unimplemented {
				// ok, this is an old plugin that doesn't support ODR mode, so just use
				// the basic mode.
				c.logger.Debug("plugin didn't implement BuildSpecODR, using Build")
				goto basic
			}

			c.logger.Error("error retrieving ODR build spec", "error", err)

			return funcErr(err)
		}

		// We don't want to be a mapper
		spec.Result = nil

		return funcspec.Func(spec, c.buildODR,
			argmapper.Logger(c.logger),
			argmapper.Typed(&pluginargs.Internal{
				Broker:  c.broker,
				Mappers: c.mappers,
				Cleanup: &pluginargs.Cleanup{},
			}),
		)
	} else {
		c.logger.Debug("Running in non-ODR mode, using Build")
	}

basic:
	// Get the build spec
	spec, err := c.client.BuildSpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	// We don't want to be a mapper
	spec.Result = nil

	return funcspec.Func(spec, c.build,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *builderClient) build(
	ctx context.Context,
	args funcspec.Args,
) (component.Artifact, error) {
	// Call our function
	resp, err := c.client.Build(ctx, &pb.FuncSpec_Args{Args: args})
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
		LabelsVal:   resp.Labels,
		TemplateVal: tplData,
	}, nil
}

func (c *builderClient) buildODR(
	ctx context.Context,
	args funcspec.Args,
) (component.Artifact, error) {
	// Call our function
	resp, err := c.client.BuildODR(ctx, &pb.FuncSpec_Args{Args: args})
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
		LabelsVal:   resp.Labels,
		TemplateVal: tplData,
	}, nil
}

// builderServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type builderServer struct {
	*base
	*authenticatorServer

	pb.UnsafeBuilderServer // to avoid having to copy stubs into here for authServer

	Impl component.Builder
}

func (s *builderServer) ConfigStruct(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_StructResp, error) {
	return configStruct(s.Impl)
}

func (s *builderServer) Configure(
	ctx context.Context,
	req *pb.Config_ConfigureRequest,
) (*empty.Empty, error) {
	return configure(s.Impl, req)
}

func (s *builderServer) Documentation(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_Documentation, error) {
	return documentation(s.Impl)
}

func (s *builderServer) BuildSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: builder")
	}

	return funcspec.Spec(s.Impl.BuildFunc(),
		argmapper.Logger(s.Logger),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(s.internal()),
	)
}

func (s *builderServer) BuildSpecODR(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: builder")
	}

	odr, ok := s.Impl.(component.BuilderODR)
	if !ok {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: builder")
	}

	return funcspec.Spec(odr.BuildODRFunc(),
		argmapper.Logger(s.Logger),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(s.internal()),
	)
}

func (s *builderServer) Build(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.Build_Resp, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	encoded, encodedJson, raw, err := callDynamicFuncAny2(s.Impl.BuildFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
	)
	if err != nil {
		return nil, err
	}

	result := &pb.Build_Resp{Result: encoded, ResultJson: encodedJson}
	if artifact, ok := raw.(component.Artifact); ok {
		result.Labels = artifact.Labels()
	}

	result.TemplateData, err = templateData(raw)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *builderServer) BuildODR(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.Build_Resp, error) {
	odr, ok := s.Impl.(component.BuilderODR)
	if !ok {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: builder")
	}

	internal := s.internal()
	defer internal.Cleanup.Close()

	encoded, encodedJson, raw, err := callDynamicFuncAny2(odr.BuildODRFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
	)
	if err != nil {
		return nil, err
	}

	result := &pb.Build_Resp{Result: encoded, ResultJson: encodedJson}
	if artifact, ok := raw.(component.Artifact); ok {
		result.Labels = artifact.Labels()
	}

	result.TemplateData, err = templateData(raw)
	if err != nil {
		return nil, err
	}

	return result, nil
}

var (
	_ plugin.Plugin                = (*BuilderPlugin)(nil)
	_ plugin.GRPCPlugin            = (*BuilderPlugin)(nil)
	_ pb.BuilderServer             = (*builderServer)(nil)
	_ component.Builder            = (*builderClient)(nil)
	_ component.Configurable       = (*builderClient)(nil)
	_ component.Documented         = (*builderClient)(nil)
	_ component.ConfigurableNotify = (*builderClient)(nil)
)
