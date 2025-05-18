package server

import (
	"github.com/localrivet/gomcp/transport/udp"
)

// AsUDP configures the server to use UDP for communication
// with optional configuration options.
//
// UDP provides low-latency communication with minimal overhead,
// suitable for high-throughput scenarios where occasional packet
// loss is acceptable.
//
// Parameters:
//   - address: The UDP address in the format "host:port"
//   - options: Optional configuration settings (buffering, timeouts, etc.)
//
// Example:
//
//	server.AsUDP(":8080")
//	// With options:
//	server.AsUDP(":8080",
//	    udp.WithMaxPacketSize(2048),
//	    udp.WithReadTimeout(5*time.Second),
//	    udp.WithReliability(true))
//
// Returns:
//   - The server instance for method chaining
func (s *serverImpl) AsUDP(address string, options ...udp.UDPOption) Server {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create UDP transport in server mode
	udpTransport := udp.NewTransport(address, true, options...)

	// Configure the message handler
	udpTransport.SetMessageHandler(s.handleMessage)

	// Set as the server's transport
	s.transport = udpTransport

	s.logger.Info("server configured with UDP transport",
		"address", address)
	return s
}
