package test

import (
	"reflect"
	"testing"

	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/sse"
)

func TestAsSSE(t *testing.T) {
	s := server.NewServer("test")

	// Configure as SSE server with dynamic port
	address := ":0"
	s = s.AsSSE(address)

	// Get the underlying server
	serverImpl := s.GetServer()

	// Check that a transport was set
	if serverImpl.GetTransport() == nil {
		t.Fatal("Expected transport to be set, got nil")
	}

	// Check that the transport is an SSE transport
	_, ok := serverImpl.GetTransport().(*sse.Transport)
	if !ok {
		t.Errorf("Expected transport to be *sse.Transport, got %s", reflect.TypeOf(serverImpl.GetTransport()))
	}
}
