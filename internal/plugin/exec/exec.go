package terminal

import (
	"context"
	"io"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/hashicorp/go-argmapper"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"

	"github.com/hashicorp/waypoint-plugin-sdk/component"
	pb "github.com/hashicorp/waypoint-plugin-sdk/proto/gen"
)

// UIPlugin implements plugin.Plugin (specifically GRPCPlugin) for
// the terminal.UI interface.
type ExecPlugin struct {
	plugin.NetRPCUnsupportedPlugin

	Impl    *component.ExecSessionInfo // Impl is the concrete implementation
	Mappers []*argmapper.Func          // Mappers
	Logger  hclog.Logger               // Logger
}

func (p *ExecPlugin) GRPCServer(broker *plugin.GRPCBroker, s *grpc.Server) error {
	pb.RegisterExecSessionServiceServer(s, &execServer{
		Impl:    p.Impl,
		Mappers: p.Mappers,
		Logger:  p.Logger,
	})
	return nil
}

type ioWriter struct {
	ctx    context.Context
	stderr bool
	client pb.ExecSessionServiceClient
}

func (i *ioWriter) Write(p []byte) (n int, err error) {
	_, err = i.client.Output(i.ctx, &pb.ExecSession_OutputRequest{
		Data:   p,
		Stderr: i.stderr,
	})
	if err != nil {
		return 0, err
	}

	return len(p), nil
}

func (p *ExecPlugin) GRPCClient(
	ctx context.Context,
	broker *plugin.GRPCBroker,
	c *grpc.ClientConn,
) (interface{}, error) {
	client := pb.NewExecSessionServiceClient(c)

	input, err := client.Input(ctx, &empty.Empty{})
	if err != nil {
		return nil, err
	}

	stdinR, stdinW := io.Pipe()

	// 2 is picked randomly, just to try not to block.
	wsUpdates := make(chan component.WindowSize, 2)

	go func() {
		for {
			req, err := input.Recv()
			if err != nil {
				p.Logger.Debug("exec plugin input stream exitted", "error", err)
				return
			}

			switch v := req.Input.(type) {
			case *pb.ExecSession_InputRequest_Data:
				p.Logger.Debug("exec plugin client, input received", "input", v.Data)

				stdinW.Write(v.Data)
			case *pb.ExecSession_InputRequest_WindowSize:
				update := component.WindowSize{
					Height: int(v.WindowSize.Height),
					Width:  int(v.WindowSize.Width),
				}
				select {
				case <-ctx.Done():
					return
				case wsUpdates <- update:
					// ok
				}
			case *pb.ExecSession_InputRequest_InputClosed:
				stdinW.Close()
			}
		}
	}()

	esi := &component.ExecSessionInfo{
		Input: stdinR,
		Output: &ioWriter{
			ctx:    ctx,
			client: client,
		},
		Error: &ioWriter{
			ctx:    ctx,
			stderr: true,
			client: client,
		},
		WindowSizeUpdates: wsUpdates,
	}

	return esi, nil
}

// execServer is a gRPC server that the client talks to and calls a
// real implementation of the component.
type execServer struct {
	pb.ExecSessionServiceServer
	Impl    *component.ExecSessionInfo
	Mappers []*argmapper.Func
	Logger  hclog.Logger
}

func (s *execServer) Output(
	ctx context.Context,
	req *pb.ExecSession_OutputRequest,
) (*empty.Empty, error) {
	var err error

	if req.Stderr {
		_, err = s.Impl.Error.Write(req.Data)
	} else {
		_, err = s.Impl.Output.Write(req.Data)
	}

	return &empty.Empty{}, err
}

func (s *execServer) Input(_ *empty.Empty, stream pb.ExecSessionService_InputServer) error {
	s.Logger.Trace("starting exec server input")

	readCh := make(chan []byte)

	// rather than make this a chan struct{} and just close it to signal to the
	// select loop, we send a bool and leave tha channel open. The reason being
	// that the select loop will continue to run after input has closed, and if we
	// close this channel, the select loop will fire constantly on this channel.
	// So instead we just send a bool and orphan the channel, knowing that the select
	// loop will stop when the stream context is done.
	closedCh := make(chan bool, 1)

	go func() {
		for {
			buf := make([]byte, 1024)

			n, err := s.Impl.Input.Read(buf)
			if err != nil && n == 0 {
				closedCh <- true
				return
			}

			select {
			case <-stream.Context().Done():
				return
			case readCh <- buf[:n]:
				// ok
			}
		}
	}()

	for {
		select {
		case <-stream.Context().Done():
			return nil
		case buf := <-readCh:
			err := stream.Send(&pb.ExecSession_InputRequest{
				Input: &pb.ExecSession_InputRequest_Data{
					Data: buf,
				},
			})
			if err != nil {
				return err
			}
		case <-closedCh:
			err := stream.Send(&pb.ExecSession_InputRequest{
				Input: &pb.ExecSession_InputRequest_InputClosed{},
			})
			if err != nil {
				return err
			}
		case ws := <-s.Impl.WindowSizeUpdates:
			err := stream.Send(&pb.ExecSession_InputRequest{
				Input: &pb.ExecSession_InputRequest_WindowSize{
					WindowSize: &pb.WindowSize{
						Height: uint32(ws.Height),
						Width:  uint32(ws.Width),
					},
				},
			})
			if err != nil {
				return err
			}
		}
	}
}

var (
	_ plugin.Plugin               = (*ExecPlugin)(nil)
	_ plugin.GRPCPlugin           = (*ExecPlugin)(nil)
	_ pb.ExecSessionServiceServer = (*execServer)(nil)
)
