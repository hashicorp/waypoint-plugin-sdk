package logs

import (
	"context"

	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// UIPlugin implements plugin.Plugin (specifically GRPCPlugin) for
// the terminal.UI interface.
type LogsPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	Impl    *component.LogViewer // Impl is the concrete implementation
	Mappers []*argmapper.Func    // Mappers
	Logger  hclog.Logger         // Logger
}

func (p *LogsPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterLogViewerServer(s, &logsServer{
		Impl:    p.Impl,
		Mappers: p.Mappers,
		Logger:  p.Logger,
	})
	return nil
}

func (p *LogsPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	p.Logger.Debug("starting logviewer client")

	client := pb.NewLogViewerClient(c)

	nlb, err := client.NextLogBatch(ctx)
	if err != nil {
		return nil, err
	}

	// TODO(evanphx) figure out the right backlog for this
	output := make(chan component.LogEvent, 10)

	/*
		go func() {
			for {
				req, err := stream.Recv()
				if err != nil {
					p.Logger.Debug("exec plugin input stream exitted", "error", err)
					return
				}

				p.Logger.Debug("processing logviewer events", "events", len(req.Events))

				for _, ev := range req.Events {

					out := component.LogEvent{
						Partition: ev.Partition,
						Timestamp: ev.Timestamp.AsTime(),
						Message:   ev.Contents,
					}
					select {
					case <-ctx.Done():
						return
					case output <- out:
						// ok
					}
				}
			}
		}()
	*/

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case cle, ok := <-output:
				if !ok {
					return
				}

				nlb.Send(&pb.Logs_NextBatchResp{
					Events: []*pb.Logs_Event{
						{
							Partition: cle.Partition,
							Timestamp: timestamppb.New(cle.Timestamp),
							Contents:  cle.Message,
						},
					},
				})
			}
		}
	}()

	lv := &component.LogViewer{
		Output: output,
	}

	return lv, nil
}

// logsServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type logsServer struct {
	Impl    *component.LogViewer
	Mappers []*argmapper.Func
	Logger  hclog.Logger
}

func (s *logsServer) NextLogBatch(lv pb.LogViewer_NextLogBatchServer) error {
	s.Logger.Debug("starting nextlogbatch rpc")
	defer s.Logger.Debug("ending nextlogbatch rpc")

	for {
		chunk, err := lv.Recv()
		if err != nil {
			return err
		}

		for _, ev := range chunk.Events {
			out := component.LogEvent{
				Partition: ev.Partition,
				Timestamp: ev.Timestamp.AsTime(),
				Message:   ev.Contents,
			}
			select {
			case <-lv.Context().Done():
				return nil
			case s.Impl.Output <- out:
				// ok
			}
		}
	}

	/*
		for {
			select {
			case <-lv.Context().Done():
				return lv.Context().Err()
			case log, ok := <-s.Impl.Output:
				if !ok {
					return io.EOF
				}

				s.Logger.Debug("sending log event")

				lv.Send(&pb.Logs_NextBatchResp{
					Events: []*pb.Logs_Event{
						{
							Partition: log.Partition,
							Timestamp: timestamppb.New(log.Timestamp),
							Contents:  log.Message,
						},
					},
				})
			}
		}
	*/
}

var (
	_ plugin.Plugin      = (*LogsPlugin)(nil)
	_ plugin.GRPCPlugin  = (*LogsPlugin)(nil)
	_ pb.LogViewerServer = (*logsServer)(nil)
)
