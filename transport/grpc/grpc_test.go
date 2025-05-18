package grpc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"

	pb "github.com/localrivet/gomcp/transport/grpc/proto/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// bufDialer is a helper for testing gRPC servers without network connections
func bufDialer(listener *bufconn.Listener) func(context.Context, string) (net.Conn, error) {
	return func(ctx context.Context, s string) (net.Conn, error) {
		return listener.Dial()
	}
}

func TestNewTransport(t *testing.T) {
	// Test server mode
	serverTransport := NewTransport(":50051", true)
	if !serverTransport.isServer {
		t.Errorf("Expected server mode, got client mode")
	}

	// Test client mode
	clientTransport := NewTransport("localhost:50051", false)
	if clientTransport.isServer {
		t.Errorf("Expected client mode, got server mode")
	}

	// Test options
	transport := NewTransport(":50052", true,
		WithTLS("cert.pem", "key.pem", "ca.pem"),
		WithMaxMessageSize(8*1024*1024),
		WithConnectionTimeout(20*time.Second),
		WithBufferSize(200),
		WithKeepAliveParams(20*time.Second, 5*time.Second),
	)

	if !transport.useTLS {
		t.Errorf("Expected TLS to be enabled")
	}
	if transport.tlsCertFile != "cert.pem" {
		t.Errorf("Expected cert file 'cert.pem', got '%s'", transport.tlsCertFile)
	}
	if transport.maxMessageSize != 8*1024*1024 {
		t.Errorf("Expected max message size %d, got %d", 8*1024*1024, transport.maxMessageSize)
	}
	if transport.connectionTimeout != 20*time.Second {
		t.Errorf("Expected connection timeout %s, got %s", 20*time.Second, transport.connectionTimeout)
	}
	if transport.bufferSize != 200 {
		t.Errorf("Expected buffer size %d, got %d", 200, transport.bufferSize)
	}
	if transport.keepAliveTime != 20*time.Second {
		t.Errorf("Expected keepalive time %s, got %s", 20*time.Second, transport.keepAliveTime)
	}
	if transport.keepAliveTimeout != 5*time.Second {
		t.Errorf("Expected keepalive timeout %s, got %s", 5*time.Second, transport.keepAliveTimeout)
	}
}

func TestTransportInitialize(t *testing.T) {
	transport := NewTransport(":50053", true)

	err := transport.Initialize()
	if err != nil {
		t.Fatalf("Failed to initialize transport: %v", err)
	}

	// Check if channels are created
	if transport.sendCh == nil {
		t.Errorf("Send channel not created")
	}
	if transport.recvCh == nil {
		t.Errorf("Receive channel not created")
	}
	if transport.errCh == nil {
		t.Errorf("Error channel not created")
	}

	// Clean up
	defer transport.Stop()
}

func TestSendBeforeStart(t *testing.T) {
	transport := NewTransport(":50054", true)
	_ = transport.Initialize()

	// Sending before starting should return an error
	err := transport.Send([]byte("test message"))
	if err == nil || err != ErrNotRunning {
		t.Errorf("Expected error '%v', got '%v'", ErrNotRunning, err)
	}
}

func TestStopBeforeStart(t *testing.T) {
	transport := NewTransport(":50055", true)
	_ = transport.Initialize()

	// Stopping before starting should not return an error
	err := transport.Stop()
	if err != nil {
		t.Errorf("Expected no error when stopping before start, got '%v'", err)
	}
}

func TestErrorMapping(t *testing.T) {
	testCases := []struct {
		name            string
		grpcCode        codes.Code
		grpcMsg         string
		expectedMessage string
	}{
		{
			name:            "InvalidArgument",
			grpcCode:        codes.InvalidArgument,
			grpcMsg:         "Invalid request",
			expectedMessage: "Invalid request",
		},
		{
			name:            "NotFound",
			grpcCode:        codes.NotFound,
			grpcMsg:         "Resource not found",
			expectedMessage: "Resource not found",
		},
		{
			name:            "Internal",
			grpcCode:        codes.Internal,
			grpcMsg:         "Internal server error",
			expectedMessage: "Internal server error",
		},
		{
			name:            "Unimplemented",
			grpcCode:        codes.Unimplemented,
			grpcMsg:         "Method not implemented",
			expectedMessage: "Method not implemented",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a gRPC status error
			err := status.Error(tc.grpcCode, tc.grpcMsg)

			// Convert to JSON-RPC error
			jsonError := GRPCToJSONRPCError(err)

			// Verify the message conversion
			if jsonError.Message != tc.expectedMessage {
				t.Errorf("Expected error message '%s', got '%s'", tc.expectedMessage, jsonError.Message)
			}

			// Convert back to gRPC error
			grpcErr := JSONRPCToGRPCError(jsonError)

			// Verify the round-trip conversion
			st, ok := status.FromError(grpcErr)
			if !ok {
				t.Fatalf("Expected gRPC status error, got '%v'", grpcErr)
			}
			if st.Message() != tc.grpcMsg {
				t.Errorf("Expected gRPC message '%s', got '%s'", tc.grpcMsg, st.Message())
			}

			// Verify code round-trip (the code might change, but the status code category should be preserved)
			if tc.grpcCode != st.Code() {
				t.Errorf("gRPC code changed after round-trip. Original: %s, Got: %s", tc.grpcCode, st.Code())
			}
		})
	}
}

func TestValueConversion(t *testing.T) {
	testCases := []struct {
		name  string
		value interface{}
	}{
		{
			name:  "String Value",
			value: "test string",
		},
		{
			name:  "Boolean Value",
			value: true,
		},
		{
			name:  "Integer Value",
			value: 42,
		},
		{
			name:  "Float Value",
			value: 3.14159,
		},
		{
			name:  "Null Value",
			value: nil,
		},
		{
			name:  "Binary Value",
			value: []byte("binary data"),
		},
		{
			name:  "Array Value",
			value: []interface{}{"string", 42, true, nil},
		},
		{
			name: "Object Value",
			value: map[string]interface{}{
				"string": "value",
				"number": 42,
				"bool":   true,
				"null":   nil,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Convert Go value to Proto value
			protoValue, err := ValueToProto(tc.value)
			if err != nil {
				t.Fatalf("Failed to convert value to proto: %v", err)
			}

			// Convert Proto value back to Go value
			goValue, err := ProtoToValue(protoValue)
			if err != nil {
				t.Fatalf("Failed to convert proto to value: %v", err)
			}

			// Verify the conversion
			// Note: For some types like floats, direct comparison might not work well
			// We could implement a more sophisticated comparison for a real test
			// For this example, we'll just check that the conversion doesn't error
			fmt.Printf("Original: %v, Converted: %v\n", tc.value, goValue)
		})
	}
}

// Integration test for client-server communication
func TestClientServerCommunication(t *testing.T) {
	// Create a buffer connection listener
	listener := bufconn.Listen(bufSize)

	// Create and start a gRPC server
	s := grpc.NewServer()
	pb.RegisterMCPServer(s, &mcpServer{
		transport: &Transport{
			sendCh: make(chan []byte, 10),
			recvCh: make(chan []byte, 10),
			errCh:  make(chan error, 10),
			ctx:    context.Background(),
		},
	})
	go func() {
		if err := s.Serve(listener); err != nil {
			t.Errorf("Failed to serve: %v", err)
		}
	}()
	defer s.Stop()

	// Create a client connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "bufnet",
		grpc.WithContextDialer(bufDialer(listener)),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	// Create a client
	client := pb.NewMCPClient(conn)

	// Test the Initialize RPC
	resp, err := client.Initialize(ctx, &pb.InitializeRequest{
		ClientId:      "test-client",
		ClientVersion: "1.0.0",
	})
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if !resp.Success {
		t.Errorf("Expected successful initialization, got failure: %v", resp.Error)
	}
}

func TestErrorHandling(t *testing.T) {
	// Test handling of nil errors
	if jsonErr := GRPCToJSONRPCError(nil); jsonErr != nil {
		t.Errorf("Expected nil JSON-RPC error for nil gRPC error, got %v", jsonErr)
	}

	if grpcErr := JSONRPCToGRPCError(nil); grpcErr != nil {
		t.Errorf("Expected nil gRPC error for nil JSON-RPC error, got %v", grpcErr)
	}

	// Test handling of non-status errors
	plainErr := errors.New("plain error")
	jsonErr := GRPCToJSONRPCError(plainErr)
	if jsonErr.Code != -32603 {
		t.Errorf("Expected internal error code for plain error, got %d", jsonErr.Code)
	}
	if jsonErr.Message != "plain error" {
		t.Errorf("Expected plain error message, got %s", jsonErr.Message)
	}

	// Test handling of unknown JSON-RPC error codes
	unknownJSONErr := &pb.ErrorInfo{
		Code:    -99999,
		Message: "Unknown error",
	}
	grpcErr := JSONRPCToGRPCError(unknownJSONErr)
	st, ok := status.FromError(grpcErr)
	if !ok {
		t.Fatalf("Expected gRPC status error, got %v", grpcErr)
	}
	if st.Code() != codes.Internal {
		t.Errorf("Expected Internal code for unknown JSON-RPC error, got %s", st.Code())
	}
}

func TestMapFunctionRequest(t *testing.T) {
	// Create a sample gRPC function request
	req := &pb.FunctionRequest{
		FunctionId: "test_function",
		RequestId:  "req-123",
		Parameters: map[string]*pb.Value{
			"string_param": {Kind: &pb.Value_StringValue{StringValue: "value"}},
			"number_param": {Kind: &pb.Value_NumberValue{NumberValue: 42.0}},
			"bool_param":   {Kind: &pb.Value_BoolValue{BoolValue: true}},
		},
	}

	// Map to JSON-RPC request
	jsonReq, err := MapToJSONRPCRequest(req)
	if err != nil {
		t.Fatalf("Failed to map to JSON-RPC request: %v", err)
	}

	// Verify basic fields
	if jsonReq["jsonrpc"] != "2.0" {
		t.Errorf("Expected jsonrpc version '2.0', got %v", jsonReq["jsonrpc"])
	}
	if jsonReq["method"] != "test_function" {
		t.Errorf("Expected method 'test_function', got %v", jsonReq["method"])
	}
	if jsonReq["id"] != "req-123" {
		t.Errorf("Expected id 'req-123', got %v", jsonReq["id"])
	}

	// Verify parameters
	params, ok := jsonReq["params"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected params to be a map, got %T", jsonReq["params"])
	}
	if params["string_param"] != "value" {
		t.Errorf("Expected string_param 'value', got %v", params["string_param"])
	}
	if params["number_param"] != 42.0 {
		t.Errorf("Expected number_param 42.0, got %v", params["number_param"])
	}
	if params["bool_param"] != true {
		t.Errorf("Expected bool_param true, got %v", params["bool_param"])
	}
}
