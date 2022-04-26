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
	"github.com/hashicorp/waypoint-plugin-sdk/internal/plugincomponent"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// TaskLauncherPlugin implements plugin.Plugin (specifically GRPCPlugin) for
// the TaskLauncher component type.
type TaskLauncherPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	Impl    component.TaskLauncher // Impl is the concrete implementation
	Mappers []*argmapper.Func      // Mappers
	Logger  hclog.Logger           // Logger
}

func (p *TaskLauncherPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	base := &base{
		Mappers: p.Mappers,
		Logger:  p.Logger,
		Broker:  broker,
	}

	pb.RegisterTaskLauncherServer(s, &taskLauncherServer{
		base: base,
		Impl: p.Impl,

		authenticatorServer: &authenticatorServer{
			base: base,
			Impl: p.Impl,
		},
	})
	return nil
}

func (p *TaskLauncherPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	client := &taskLauncherClient{
		client:  pb.NewTaskLauncherClient(c),
		logger:  p.Logger,
		broker:  broker,
		mappers: p.Mappers,
	}

	result := &mix_TaskLauncher_Authenticator{
		ConfigurableNotify: client,
		TaskLauncher:       client,
		Documented:         client,
	}

	return result, nil
}

// taskLauncherClient is an implementation of component.TaskLauncher that
// communicates over gRPC.
type taskLauncherClient struct {
	client  pb.TaskLauncherClient
	logger  hclog.Logger
	broker  *plugin.GRPCBroker
	mappers []*argmapper.Func
}

func (c *taskLauncherClient) Config() (interface{}, error) {
	return configStructCall(context.Background(), c.client)
}

func (c *taskLauncherClient) ConfigSet(v interface{}) error {
	return configureCall(context.Background(), c.client, v)
}

func (c *taskLauncherClient) Documentation() (*docs.Documentation, error) {
	return documentationCall(context.Background(), c.client)
}

func (c *taskLauncherClient) StartTaskFunc() interface{} {
	// Get the build spec
	spec, err := c.client.StartSpec(context.Background(), &empty.Empty{})
	if err != nil {
		c.logger.Error("start-spec error", "error", err)
		return funcErr(err)
	}

	// We don't want to be a mapper
	spec.Result = nil

	return funcspec.Func(spec, c.start,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *taskLauncherClient) StopTaskFunc() interface{} {
	// Get the build spec
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

func (c *taskLauncherClient) RunTaskFunc() interface{} {
	// Get the build spec
	spec, err := c.client.RunSpec(context.Background(), &empty.Empty{})
	if err != nil {
		return funcErr(err)
	}

	return funcspec.Func(spec, c.run,
		argmapper.Logger(c.logger),
		argmapper.Typed(&pluginargs.Internal{
			Broker:  c.broker,
			Mappers: c.mappers,
			Cleanup: &pluginargs.Cleanup{},
		}),
	)
}

func (c *taskLauncherClient) start(
	ctx context.Context,
	args funcspec.Args,
) (component.RunningTask, error) {
	// Call our function
	resp, err := c.client.StartTask(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		c.logger.Error("error starting task", "error", err)
		return nil, err
	}

	c.logger.Info("start done", "value", resp.Result)

	return &plugincomponent.RunningTask{
		Any: resp.Result,
	}, nil
}

func (c *taskLauncherClient) stop(
	ctx context.Context,
	args funcspec.Args,
) error {
	// Call our function
	_, err := c.client.StopTask(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		return err
	}

	return nil
}

func (c *taskLauncherClient) run(
	ctx context.Context,
	args funcspec.Args,
	internal *pluginargs.Internal,
) (*component.TaskResult, error) {
	// Run the cleanup
	defer internal.Cleanup.Close()

	// Call our function
	resp, err := c.client.RunTask(ctx, &pb.FuncSpec_Args{Args: args})
	if err != nil {
		c.logger.Error("error starting task", "error", err)
		return nil, err
	}

	return &component.TaskResult{
		ExitCode: int(resp.ExitCode),
	}, nil
}

// taskLauncherServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type taskLauncherServer struct {
	*base
	*authenticatorServer

	pb.UnsafeTaskLauncherServer

	Impl component.TaskLauncher
}

func (s *taskLauncherServer) ConfigStruct(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_StructResp, error) {
	return configStruct(s.Impl)
}

func (s *taskLauncherServer) Configure(
	ctx context.Context,
	req *pb.Config_ConfigureRequest,
) (*empty.Empty, error) {
	return configure(s.Impl, req)
}

func (s *taskLauncherServer) Documentation(
	ctx context.Context,
	empty *empty.Empty,
) (*pb.Config_Documentation, error) {
	return documentation(s.Impl)
}

func (s *taskLauncherServer) StartSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: taskLauncher")
	}

	return funcspec.Spec(s.Impl.StartTaskFunc(),
		argmapper.Logger(s.Logger),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(s.internal()),
	)
}

func (s *taskLauncherServer) StartTask(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.TaskLaunch_Resp, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	encoded, encodedJson, _, err := callDynamicFuncAny2(s.Impl.StartTaskFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
	)
	if err != nil {
		return nil, err
	}

	result := &pb.TaskLaunch_Resp{Result: encoded, ResultJson: encodedJson}
	return result, nil
}

func (s *taskLauncherServer) StopSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: taskLauncher")
	}

	return funcspec.Spec(s.Impl.StopTaskFunc(),
		argmapper.Logger(s.Logger),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(s.internal()),
	)
}

func (s *taskLauncherServer) StopTask(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*empty.Empty, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	_, err := callDynamicFunc2(s.Impl.StopTaskFunc(), args.Args,
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

func (s *taskLauncherServer) RunSpec(
	ctx context.Context,
	args *empty.Empty,
) (*pb.FuncSpec, error) {
	if s.Impl == nil {
		return nil, status.Errorf(codes.Unimplemented, "plugin does not implement: taskLauncher")
	}

	return funcspec.Spec(s.Impl.RunTaskFunc(),
		argmapper.Logger(s.Logger),
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Typed(s.internal()),
		argmapper.FilterOutput(
			argmapper.FilterType(reflect.TypeOf((*component.TaskResult)(nil))),
		),
	)
}

func (s *taskLauncherServer) RunTask(
	ctx context.Context,
	args *pb.FuncSpec_Args,
) (*pb.TaskRun_Resp, error) {
	internal := s.internal()
	defer internal.Cleanup.Close()

	result, err := callDynamicFunc2(s.Impl.RunTaskFunc(), args.Args,
		argmapper.ConverterFunc(s.Mappers...),
		argmapper.Logger(s.Logger),
		argmapper.Typed(ctx),
		argmapper.Typed(internal),
	)
	if err != nil {
		return nil, err
	}

	ret := &pb.TaskRun_Resp{}
	if r, ok := result.(*component.TaskResult); ok {
		ret.ExitCode = int32(r.ExitCode)
	}

	return ret, nil
}

var (
	_ plugin.Plugin                = (*TaskLauncherPlugin)(nil)
	_ plugin.GRPCPlugin            = (*TaskLauncherPlugin)(nil)
	_ pb.TaskLauncherServer        = (*taskLauncherServer)(nil)
	_ component.TaskLauncher       = (*taskLauncherClient)(nil)
	_ component.Configurable       = (*taskLauncherClient)(nil)
	_ component.Documented         = (*taskLauncherClient)(nil)
	_ component.ConfigurableNotify = (*taskLauncherClient)(nil)
)
