package client

import (
	"context"
	"time"

	"github.com/localrivet/gomcp/protocol"
)

// NotificationHandler processes incoming notifications
type NotificationHandler func(notification *protocol.JSONRPCNotification) error

// ProgressHandler processes progress updates
type ProgressHandler func(progress *protocol.ProgressParams) error

// ResourceUpdateHandler processes resource update notifications
type ResourceUpdateHandler func(uri string) error

// LogHandler processes log messages
type LogHandler func(level protocol.LoggingLevel, message string) error

// ConnectionStatusHandler processes connection status changes
type ConnectionStatusHandler func(connected bool) error

// ClientMiddleware intercepts client operations
type ClientMiddleware interface {
	BeforeSendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCRequest, error)
	AfterReceiveResponse(ctx context.Context, resp *protocol.JSONRPCResponse) (*protocol.JSONRPCResponse, error)
}

// BackoffStrategy defines how to handle retries and delays
type BackoffStrategy interface {
	NextDelay(attempt int) time.Duration
	MaxAttempts() int
}

// AuthProvider supplies authentication information
type AuthProvider interface {
	GetAuthHeaders() map[string]string
	GetAuthToken() string
}

// ProtocolHandler abstracts version-specific behavior
type ProtocolHandler interface {
	// Common operations with version-specific implementations
	FormatRequest(method string, params interface{}) (*protocol.JSONRPCRequest, error)
	ParseResponse(resp *protocol.JSONRPCResponse) (interface{}, error)
	FormatCallToolRequest(name string, args map[string]interface{}) (interface{}, error)
	ParseCallToolResult(result interface{}) ([]protocol.Content, error)
}
