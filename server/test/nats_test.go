package test

import (
	"testing"
	"time"

	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/nats"
	"github.com/stretchr/testify/assert"
)

func TestNATSServer(t *testing.T) {
	// Skip in normal test runs since it requires a running NATS server
	t.Skip("NATS integration test requires a running NATS server - enable manually")

	// This test requires a running NATS server on localhost:4222
	serverURL := "nats://localhost:4222"

	// Create server with NATS transport
	srv := server.NewServer("test-server")
	srv.AsNATS(serverURL,
		nats.WithClientID("test-server-1"),
		nats.WithSubjectPrefix("mcp-test"),
	)

	// Register a simple echo tool
	srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (map[string]interface{}, error) {
		return map[string]interface{}{
			"message": args.Message,
		}, nil
	})

	// Run the server in a goroutine
	errCh := make(chan error, 1)
	go func() {
		err := srv.Run()
		errCh <- err
	}()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Check if server is running without error
	select {
	case err := <-errCh:
		assert.NoError(t, err)
	default:
		// Server is still running, as expected
	}

	// We can't easily test actual client-server communication in a unit test
	// because it would require running a real NATS server.
	// For actual integration tests, see the examples.
}

func TestNATSServerWithClientID(t *testing.T) {
	// Skip in normal test runs since it requires a running NATS server
	t.Skip("NATS integration test requires a running NATS server - enable manually")

	// This test requires a running NATS server on localhost:4222
	serverURL := "nats://localhost:4222"

	// Create server with NATS transport using the ClientID helper
	srv := server.NewServer("test-server")
	srv.AsNATSWithClientID(serverURL, "custom-server-id",
		nats.WithSubjectPrefix("mcp-test"),
	)

	// Verify server configuration
	serverImpl := srv.GetServer()
	assert.NotNil(t, serverImpl.GetTransport())
}

func TestNATSServerWithToken(t *testing.T) {
	// Skip in normal test runs since it requires a running secure NATS server
	t.Skip("NATS token integration test requires a running secure NATS broker - enable manually")

	// This test requires a running NATS server with token authentication
	serverURL := "nats://localhost:4222"

	// Create server with NATS transport using token authentication
	srv := server.NewServer("test-server")
	srv.AsNATSWithToken(serverURL, "secret-token",
		nats.WithSubjectPrefix("mcp-test"),
	)

	// Verify server configuration
	serverImpl := srv.GetServer()
	assert.NotNil(t, serverImpl.GetTransport())
}
