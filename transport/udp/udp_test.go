package udp

import (
	"testing"
	"time"
)

func TestNewTransport(t *testing.T) {
	// Test server transport creation
	serverTransport := NewTransport(":8080", true)
	if serverTransport == nil {
		t.Fatal("Failed to create server transport")
	}

	// Verify defaults
	if serverTransport.maxPacketSize != DefaultMaxPacketSize {
		t.Errorf("Expected default packet size %d, got %d", DefaultMaxPacketSize, serverTransport.maxPacketSize)
	}

	if serverTransport.readTimeout != DefaultReadTimeout {
		t.Errorf("Expected default read timeout %v, got %v", DefaultReadTimeout, serverTransport.readTimeout)
	}

	// Test client transport creation
	clientTransport := NewTransport("localhost:8080", false)
	if clientTransport == nil {
		t.Fatal("Failed to create client transport")
	}

	// Test with options
	transport := NewTransport(":8081", true,
		WithMaxPacketSize(2048),
		WithReadTimeout(5*time.Second),
		WithWriteTimeout(3*time.Second),
		WithReliability(true),
	)

	// Verify custom options
	if transport.maxPacketSize != 2048 {
		t.Errorf("Expected custom packet size 2048, got %d", transport.maxPacketSize)
	}

	if transport.readTimeout != 5*time.Second {
		t.Errorf("Expected custom read timeout 5s, got %v", transport.readTimeout)
	}

	if transport.writeTimeout != 3*time.Second {
		t.Errorf("Expected custom write timeout 3s, got %v", transport.writeTimeout)
	}

	if !transport.reliabilityEnabled {
		t.Error("Expected reliability to be enabled")
	}
}

func TestPacketHeaderEncoding(t *testing.T) {
	// Create a sample header
	header := &PacketHeader{
		Magic:          [2]byte{MagicByte1, MagicByte2},
		Flags:          FlagReliable | FlagSingleFragment,
		MessageID:      12345,
		FragmentIndex:  0,
		TotalFragments: 1,
		Checksum:       987654321,
	}

	// Encode it
	encoded := encodeHeader(header)

	// Verify length
	if len(encoded) != HeaderSize {
		t.Errorf("Expected encoded header size %d, got %d", HeaderSize, len(encoded))
	}

	// Decode it back
	decoded, err := decodeHeader(encoded)
	if err != nil {
		t.Fatalf("Failed to decode header: %v", err)
	}

	// Verify decoded values
	if decoded.Magic[0] != MagicByte1 || decoded.Magic[1] != MagicByte2 {
		t.Errorf("Magic bytes mismatch: expected [%x,%x], got [%x,%x]",
			MagicByte1, MagicByte2, decoded.Magic[0], decoded.Magic[1])
	}

	if decoded.Flags != (FlagReliable | FlagSingleFragment) {
		t.Errorf("Flags mismatch: expected %x, got %x",
			(FlagReliable | FlagSingleFragment), decoded.Flags)
	}

	if decoded.MessageID != 12345 {
		t.Errorf("MessageID mismatch: expected %d, got %d", 12345, decoded.MessageID)
	}

	if decoded.FragmentIndex != 0 {
		t.Errorf("FragmentIndex mismatch: expected %d, got %d", 0, decoded.FragmentIndex)
	}

	if decoded.TotalFragments != 1 {
		t.Errorf("TotalFragments mismatch: expected %d, got %d", 1, decoded.TotalFragments)
	}

	if decoded.Checksum != 987654321 {
		t.Errorf("Checksum mismatch: expected %d, got %d", 987654321, decoded.Checksum)
	}
}

func TestInvalidPacketHeader(t *testing.T) {
	// Test with too small buffer
	_, err := decodeHeader(make([]byte, HeaderSize-1))
	if err == nil {
		t.Error("Expected error for too small buffer, got nil")
	}

	// Test with invalid magic bytes
	invalidMagic := make([]byte, HeaderSize)
	invalidMagic[0] = 0x00 // Invalid first byte
	invalidMagic[1] = 0x00 // Invalid second byte

	_, err = decodeHeader(invalidMagic)
	if err == nil {
		t.Error("Expected error for invalid magic bytes, got nil")
	}
}
