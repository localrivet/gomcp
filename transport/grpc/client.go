package grpc

import (
	"context"
	"fmt"
	"io"
	"time"

	pb "github.com/localrivet/gomcp/transport/grpc/proto/gen"
	"google.golang.org/grpc"
)

// startClient initializes and connects the gRPC client.
//
// This method establishes a connection to the gRPC server using
// the configured address and options. It then sets up a bidirectional
// stream for exchanging messages with the server.
func (t *Transport) startClient() error {
	// Set up a connection to the server
	ctx, cancel := context.WithTimeout(t.ctx, t.connectionTimeout)
	defer cancel()

	// Create dial options
	opts := t.getClientOptions()

	conn, err := grpc.DialContext(ctx, t.address, opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	t.clientConn = conn

	// Create gRPC client
	client := pb.NewMCPClient(conn)

	// Initialize the session
	_, err = client.Initialize(ctx, &pb.InitializeRequest{
		ClientId:      "client-1", // Should be configurable
		ClientVersion: "1.0.0",    // Should come from client config
	})
	if err != nil {
		return fmt.Errorf("failed to initialize session: %w", err)
	}

	// Start bidirectional streaming
	stream, err := client.StreamMessages(t.ctx)
	if err != nil {
		return fmt.Errorf("failed to create stream: %w", err)
	}

	// Start goroutine to receive messages from server
	go func() {
		for {
			message, err := stream.Recv()
			if err != nil {
				if err == io.EOF {
					// Stream closed by server
					select {
					case t.errCh <- fmt.Errorf("server closed connection"):
					case <-t.ctx.Done():
						// Transport is shutting down
					}
					return
				}
				// Other error
				select {
				case t.errCh <- fmt.Errorf("failed to receive message: %w", err):
				case <-t.ctx.Done():
					// Transport is shutting down
				}
				return
			}

			// Extract message content
			var content []byte
			switch c := message.Content.(type) {
			case *pb.MCPMessage_TextContent:
				content = []byte(c.TextContent)
			case *pb.MCPMessage_BinaryContent:
				content = c.BinaryContent
			default:
				// Unknown content type, skip
				continue
			}

			// Send to receive channel
			select {
			case t.recvCh <- content:
			case <-t.ctx.Done():
				return
			}
		}
	}()

	// Start goroutine to send messages to server
	go func() {
		for {
			select {
			case <-t.ctx.Done():
				return
			case message := <-t.sendCh:
				// Create message
				protoMsg := &pb.MCPMessage{
					Id: "msg-" + fmt.Sprintf("%d", time.Now().UnixNano()),
					Content: &pb.MCPMessage_TextContent{
						TextContent: string(message),
					},
					Timestamp: uint64(time.Now().UnixMilli()),
				}

				// Send message
				if err := stream.Send(protoMsg); err != nil {
					select {
					case t.errCh <- fmt.Errorf("failed to send message: %w", err):
					case <-t.ctx.Done():
						// Transport is shutting down
					}
					return
				}
			}
		}
	}()

	return nil
}
