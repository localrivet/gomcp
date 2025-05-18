package grpc

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	pb "github.com/localrivet/gomcp/transport/grpc/proto/gen"
	"google.golang.org/grpc"
)

// mcpServer implements the MCP gRPC server.
// It handles incoming client requests and streams messages
// between clients and the MCP server.
type mcpServer struct {
	pb.UnimplementedMCPServer
	transport *Transport
}

// startGRPCServer starts the gRPC server.
//
// This method creates a TCP listener, registers the MCP service,
// and starts the gRPC server in a background goroutine.
// It uses the transport's configured address and options.
func (t *Transport) startGRPCServer() error {
	// Check if we have a valid address
	if t.address == "" {
		t.address = fmt.Sprintf(":%d", DefaultPort)
	} else if !strings.Contains(t.address, ":") {
		t.address = fmt.Sprintf("%s:%d", t.address, DefaultPort)
	}

	// Create listener
	lis, err := net.Listen("tcp", t.address)
	if err != nil {
		return fmt.Errorf("failed to listen: %w", err)
	}

	// Create server options
	opts := t.getServerOptions()

	// Create server
	t.server = grpc.NewServer(opts...)

	// Register service
	pb.RegisterMCPServer(t.server, &mcpServer{transport: t})

	// Start server in a goroutine
	go func() {
		if err := t.server.Serve(lis); err != nil {
			select {
			case t.errCh <- fmt.Errorf("failed to serve: %w", err):
			case <-t.ctx.Done():
				// Context is done, server is shutting down
			}
		}
	}()

	return nil
}

// Initialize handles the Initialize RPC.
//
// This RPC is called by clients to establish a session with the server.
// It returns session information and server version details.
func (s *mcpServer) Initialize(ctx context.Context, req *pb.InitializeRequest) (*pb.InitializeResponse, error) {
	// TODO: Implement initialization logic
	resp := &pb.InitializeResponse{
		SessionId:     "session-1", // Generate real session ID
		ServerVersion: "1.0.0",     // Get from server
		Success:       true,
	}
	return resp, nil
}

// StreamMessages implements bidirectional streaming for message exchange.
//
// This method establishes a bidirectional stream between the client and server,
// allowing them to exchange messages in real-time. It handles both incoming
// messages from the client and outgoing messages from the server.
func (s *mcpServer) StreamMessages(stream pb.MCP_StreamMessagesServer) error {
	// Create done channel for this stream
	done := make(chan struct{})
	defer close(done)

	// Start a goroutine to send messages to the client
	go func() {
		defer func() {
			done <- struct{}{}
		}()

		for {
			select {
			case <-s.transport.ctx.Done():
				return
			case <-done:
				return
			case message := <-s.transport.sendCh:
				// Convert the message to gRPC format
				protoMsg := &pb.MCPMessage{
					Id: "msg-" + fmt.Sprintf("%d", time.Now().UnixNano()),
					Content: &pb.MCPMessage_TextContent{
						TextContent: string(message),
					},
					Timestamp: uint64(time.Now().UnixMilli()),
				}

				// Send the message to the client
				if err := stream.Send(protoMsg); err != nil {
					select {
					case s.transport.errCh <- fmt.Errorf("failed to send message: %w", err):
					default:
						// Error channel full, log and continue
					}
					return
				}
			}
		}
	}()

	// Receive messages from the client
	for {
		protoMsg, err := stream.Recv()
		if err != nil {
			// Check if the stream was closed by the client
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("failed to receive message: %w", err)
		}

		// Extract the message content
		var message []byte
		switch content := protoMsg.Content.(type) {
		case *pb.MCPMessage_TextContent:
			message = []byte(content.TextContent)
		case *pb.MCPMessage_BinaryContent:
			message = content.BinaryContent
		default:
			// Handle other message types (function calls, etc.)
			continue
		}

		// Process the message using the transport's handler
		response, err := s.transport.HandleMessage(message)
		if err != nil {
			select {
			case s.transport.errCh <- err:
			default:
				// Error channel full, log and continue
			}
			continue
		}

		// If there's a response, put it in the send channel
		if response != nil {
			select {
			case s.transport.sendCh <- response:
			case <-s.transport.ctx.Done():
				return fmt.Errorf("send canceled: %w", s.transport.ctx.Err())
			}
		} else {
			// If no response, just echo the message back
			select {
			case s.transport.recvCh <- message:
			case <-s.transport.ctx.Done():
				return fmt.Errorf("receive canceled: %w", s.transport.ctx.Err())
			}
		}
	}
}

// StreamEvents implements server-to-client event streaming.
func (s *mcpServer) StreamEvents(req *pb.EventStreamRequest, stream pb.MCP_StreamEventsServer) error {
	// TODO: Implement event streaming
	return fmt.Errorf("not implemented")
}

// ExecuteFunction executes a function and returns the result.
func (s *mcpServer) ExecuteFunction(ctx context.Context, req *pb.FunctionRequest) (*pb.FunctionResponse, error) {
	// TODO: Implement function execution
	return &pb.FunctionResponse{
		FunctionId: req.FunctionId,
		RequestId:  req.RequestId,
		IsFinal:    true,
	}, nil
}

// EndSession terminates an active MCP session.
func (s *mcpServer) EndSession(ctx context.Context, req *pb.EndSessionRequest) (*pb.EndSessionResponse, error) {
	// TODO: Implement session termination
	return &pb.EndSessionResponse{
		Success: true,
	}, nil
}
