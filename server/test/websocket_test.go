package test

import (
	"reflect"
	"testing"

	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/ws"
)

func TestAsWebsocket(t *testing.T) {
	s := server.NewServer("test")

	// Configure as WebSocket server with dynamic port
	address := ":0"
	s = s.AsWebsocket(address)

	// Get the underlying server
	serverImpl := s.GetServer()

	// Check that a transport was set
	if serverImpl.GetTransport() == nil {
		t.Fatal("Expected transport to be set, got nil")
	}

	// Check that the transport is a WebSocket transport
	_, ok := serverImpl.GetTransport().(*ws.Transport)
	if !ok {
		t.Errorf("Expected transport to be *ws.Transport, got %s", reflect.TypeOf(serverImpl.GetTransport()))
	}
}
