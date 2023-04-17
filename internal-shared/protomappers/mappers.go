// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package protomappers

import (
	"context"
	"io"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"github.com/mitchellh/mapstructure"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	"github.com/hashicorp/waypoint-plugin-sdk/datadir"
	pluginexec "github.com/hashicorp/waypoint-plugin-sdk/internal/plugin/exec"
	pluginlogs "github.com/hashicorp/waypoint-plugin-sdk/internal/plugin/logs"
	pluginterminal "github.com/hashicorp/waypoint-plugin-sdk/internal/plugin/terminal"
	"github.com/hashicorp/waypoint-plugin-sdk/internal/pluginargs"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
	"github.com/hashicorp/waypoint-plugin-sdk/terminal"
)

// All is the list of all mappers as raw function pointers.
var All = []interface{}{
	Source,
	SourceProto,
	JobInfo,
	JobInfoProto,
	DeploymentConfig,
	DeploymentConfigProto,
	DatadirProject,
	DatadirApp,
	DatadirComponent,
	DatadirProjectProto,
	DatadirAppProto,
	DatadirComponentProto,
	DeclaredResourcesComponent,
	DeclaredResourcesComponentProto,
	Logger,
	LoggerProto,
	TerminalUI,
	TerminalUIProto,
	LabelSet,
	LabelSetProto,
	ExecSessionInfo,
	ExecSessionInfoProto,
	LogViewer,
	LogViewerProto,
	TaskLaunchInfo,
	TaskLaunchInfoProto,
}

// Source maps Args.Source to component.Source.
func Source(input *pb.Args_Source) (*component.Source, error) {
	var result component.Source
	return &result, mapstructure.Decode(input, &result)
}

// SourceProto
func SourceProto(input *component.Source) (*pb.Args_Source, error) {
	var result pb.Args_Source
	return &result, mapstructure.Decode(input, &result)
}

// JobInfo maps Args.JobInfo to component.JobInfo.
func JobInfo(input *pb.Args_JobInfo) (*component.JobInfo, error) {
	var result component.JobInfo
	return &result, mapstructure.Decode(input, &result)
}

// JobInfoProto
func JobInfoProto(input *component.JobInfo) (*pb.Args_JobInfo, error) {
	var result pb.Args_JobInfo
	return &result, mapstructure.Decode(input, &result)
}

// TaskLaunchInfo maps Args.Args_TaskLaunchInfo to component.TaskLaunchInfo.
func TaskLaunchInfo(input *pb.Args_TaskLaunchInfo) (*component.TaskLaunchInfo, error) {
	var result component.TaskLaunchInfo
	return &result, mapstructure.Decode(input, &result)
}

// TaskLaunchInfoProto
func TaskLaunchInfoProto(input *component.TaskLaunchInfo) (*pb.Args_TaskLaunchInfo, error) {
	var result pb.Args_TaskLaunchInfo
	return &result, mapstructure.Decode(input, &result)
}

// DeploymentConfig
func DeploymentConfig(input *pb.Args_DeploymentConfig) (*component.DeploymentConfig, error) {
	var result component.DeploymentConfig
	return &result, mapstructure.Decode(input, &result)
}

func DeploymentConfigProto(input *component.DeploymentConfig) (*pb.Args_DeploymentConfig, error) {
	var result pb.Args_DeploymentConfig
	return &result, mapstructure.Decode(input, &result)
}

// DatadirProject maps *pb.Args_DataDir_Project to *datadir.Project
func DatadirProject(input *pb.Args_DataDir_Project) *datadir.Project {
	dir := datadir.NewBasicDir(input.CacheDir, input.DataDir)
	return &datadir.Project{Dir: dir}
}

func DatadirProjectProto(input *datadir.Project) *pb.Args_DataDir_Project {
	return &pb.Args_DataDir_Project{
		CacheDir: input.CacheDir(),
		DataDir:  input.DataDir(),
	}
}

// DatadirApp maps *pb.Args_DataDir_App to *datadir.App
func DatadirApp(input *pb.Args_DataDir_App) *datadir.App {
	dir := datadir.NewBasicDir(input.CacheDir, input.DataDir)
	return &datadir.App{Dir: dir}
}

func DatadirAppProto(input *datadir.App) *pb.Args_DataDir_App {
	return &pb.Args_DataDir_App{
		CacheDir: input.CacheDir(),
		DataDir:  input.DataDir(),
	}
}

// DatadirComponent maps *pb.Args_DataDir_Component to *datadir.Component
func DatadirComponent(input *pb.Args_DataDir_Component) *datadir.Component {
	dir := datadir.NewBasicDir(input.CacheDir, input.DataDir)
	return &datadir.Component{Dir: dir}
}

func DatadirComponentProto(input *datadir.Component) *pb.Args_DataDir_Component {
	return &pb.Args_DataDir_Component{
		CacheDir: input.CacheDir(),
		DataDir:  input.DataDir(),
	}
}

// DeclaredResourcesComponent maps *pb.DeclaredResources to *component.DeclaredResources
func DeclaredResourcesComponent(input *pb.DeclaredResources) (*component.DeclaredResources, error) {
	var result component.DeclaredResources
	return &result, mapstructure.Decode(input, &result)
}

func DeclaredResourcesComponentProto(input *component.DeclaredResources) (*pb.DeclaredResources, error) {
	var result pb.DeclaredResources
	return &result, mapstructure.Decode(input, &result)
}

// Logger maps *pb.Args_Logger to an hclog.Logger
func Logger(input *pb.Args_Logger) hclog.Logger {
	// We use the default logger as the base. Within a plugin we always set
	// it so we can confidently use this. This lets plugins potentially mess
	// with this but that's a risk we have to take.
	return hclog.L().ResetNamed(input.Name)
}

func LoggerProto(log hclog.Logger) *pb.Args_Logger {
	return &pb.Args_Logger{
		Name: log.Name(),
	}
}

// TerminalUI maps *pb.Args_TerminalUI to an hclog.TerminalUI
func TerminalUI(
	ctx context.Context,
	input *pb.Args_TerminalUI,
	log hclog.Logger,
	internal *pluginargs.Internal,
) (terminal.UI, error) {
	// Create our plugin
	p := &pluginterminal.UIPlugin{
		Mappers: internal.Mappers,
		Logger:  log,
	}

	conn, err := internal.Broker.Dial(input.StreamId)
	if err != nil {
		return nil, err
	}
	internal.Cleanup.Do(func() { conn.Close() })

	client, err := p.GRPCClient(ctx, internal.Broker, conn)
	if err != nil {
		return nil, err
	}

	// Our UI should implement close since we have to stop streams and
	// such but we gate it here in case we ever change the implementation.
	if closer, ok := client.(io.Closer); ok {
		internal.Cleanup.Do(func() { closer.Close() })
	}

	return client.(terminal.UI), nil
}

func TerminalUIProto(
	ui terminal.UI,
	log hclog.Logger,
	internal *pluginargs.Internal,
) *pb.Args_TerminalUI {
	// Create our plugin
	p := &pluginterminal.UIPlugin{
		Impl:    ui,
		Mappers: internal.Mappers,
		Logger:  log,
	}

	id := internal.Broker.NextId()

	// Serve it
	go internal.Broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		server := plugin.DefaultGRPCServer(opts)
		if err := p.GRPCServer(internal.Broker, server); err != nil {
			panic(err)
		}
		return server
	})

	return &pb.Args_TerminalUI{StreamId: id}
}

func LabelSet(input *pb.Args_LabelSet) *component.LabelSet {
	return &component.LabelSet{
		Labels: input.Labels,
	}
}

func LabelSetProto(labels *component.LabelSet) *pb.Args_LabelSet {
	return &pb.Args_LabelSet{Labels: labels.Labels}
}

// ExecSessioInfo maps *pb.Args_ExecSessionInfo to a *component.ExecSessioInfo
func ExecSessionInfo(
	ctx context.Context,
	input *pb.Args_ExecSessionInfo,
	log hclog.Logger,
	internal *pluginargs.Internal,
) (*component.ExecSessionInfo, error) {
	// Create our plugin
	p := &pluginexec.ExecPlugin{
		Mappers: internal.Mappers,
		Logger:  log,
	}

	conn, err := internal.Broker.Dial(input.StreamId)
	if err != nil {
		return nil, err
	}
	internal.Cleanup.Do(func() { conn.Close() })

	v, err := p.GRPCClient(ctx, internal.Broker, conn)
	if err != nil {
		return nil, err
	}

	esi := v.(*component.ExecSessionInfo)
	esi.Arguments = input.Args
	esi.Environment = input.Env
	esi.IsTTY = input.IsTty
	esi.Term = input.TermType

	if input.InitialWindow != nil {
		esi.InitialWindowSize.Height = int(input.InitialWindow.Height)
		esi.InitialWindowSize.Width = int(input.InitialWindow.Width)
	}

	return esi, nil
}

// ExecSessionInfoProto maps a *component.ExecSessionInfo to a *pb.Args_ExecSessionInfo
func ExecSessionInfoProto(
	esi *component.ExecSessionInfo,
	log hclog.Logger,
	internal *pluginargs.Internal,
) *pb.Args_ExecSessionInfo {
	// Create our plugin
	p := &pluginexec.ExecPlugin{
		Impl:    esi,
		Mappers: internal.Mappers,
		Logger:  log,
	}

	id := internal.Broker.NextId()

	// Serve it
	go internal.Broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		server := plugin.DefaultGRPCServer(opts)
		if err := p.GRPCServer(internal.Broker, server); err != nil {
			panic(err)
		}
		return server
	})

	out := &pb.Args_ExecSessionInfo{
		StreamId: id,
		Args:     esi.Arguments,
		IsTty:    esi.IsTTY,
		Env:      esi.Environment,
	}

	if esi.IsTTY {
		out.IsTty = true
		out.InitialWindow = &pb.WindowSize{
			Height: uint32(esi.InitialWindowSize.Height),
			Width:  uint32(esi.InitialWindowSize.Width),
		}
		out.TermType = esi.Term
	}

	return out
}

// LogViewer maps *pb.Args_LogViewer to a *component.LogViewer
func LogViewer(
	ctx context.Context,
	input *pb.Args_LogViewer,
	log hclog.Logger,
	internal *pluginargs.Internal,
) (*component.LogViewer, error) {
	// Create our plugin
	p := &pluginlogs.LogsPlugin{
		Mappers: internal.Mappers,
		Logger:  log,
	}

	conn, err := internal.Broker.Dial(input.StreamId)
	if err != nil {
		return nil, err
	}
	internal.Cleanup.Do(func() { conn.Close() })

	v, err := p.GRPCClient(ctx, internal.Broker, conn)
	if err != nil {
		return nil, err
	}

	lv := v.(*component.LogViewer)
	lv.StartingAt = input.StartingAt.AsTime()
	lv.Limit = int(input.Limit)

	return lv, nil
}

// LogViewerProto maps a *component.LogViewer.Args_LogViewer
func LogViewerProto(
	lv *component.LogViewer,
	log hclog.Logger,
	internal *pluginargs.Internal,
) *pb.Args_LogViewer {
	// Create our plugin
	p := &pluginlogs.LogsPlugin{
		Impl:    lv,
		Mappers: internal.Mappers,
		Logger:  log,
	}

	id := internal.Broker.NextId()

	// Serve it
	go internal.Broker.AcceptAndServe(id, func(opts []grpc.ServerOption) *grpc.Server {
		server := plugin.DefaultGRPCServer(opts)
		if err := p.GRPCServer(internal.Broker, server); err != nil {
			panic(err)
		}
		return server
	})

	out := &pb.Args_LogViewer{
		StreamId:   id,
		StartingAt: timestamppb.New(lv.StartingAt),
		Limit:      uint32(lv.Limit),
	}

	return out
}
