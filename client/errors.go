package client

import (
	"errors"
	"fmt"
	"time"
)

// Standard error types that can be used with errors.Is()
var (
	ErrNotConnected     = errors.New("client is not connected")
	ErrAlreadyConnected = errors.New("client is already connected")
	ErrRequestTimeout   = errors.New("request timed out")
	ErrVersionMismatch  = errors.New("protocol version mismatch")
	ErrAuthFailure      = errors.New("authentication failed")
	ErrTransportFailure = errors.New("transport failure")
	ErrInvalidResponse  = errors.New("invalid response from server")
	ErrServerError      = errors.New("server reported error")
	ErrCancelled        = errors.New("operation was cancelled")
)

// ClientError is the base error type for client errors
type ClientError struct {
	Message string
	Code    int
	Cause   error
}

// Error implements the error interface
func (e *ClientError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s (code=%d): %v", e.Message, e.Code, e.Cause)
	}
	return fmt.Sprintf("%s (code=%d)", e.Message, e.Code)
}

// Unwrap returns the underlying cause
func (e *ClientError) Unwrap() error {
	return e.Cause
}

// TransportError indicates a problem with the transport layer
type TransportError struct {
	ClientError
	Transport string
}

// Error implements the error interface
func (e *TransportError) Error() string {
	return fmt.Sprintf("transport error (%s): %s", e.Transport, e.ClientError.Error())
}

// ConnectionError indicates a connection issue
type ConnectionError struct {
	ClientError
	Endpoint string
}

// Error implements the error interface
func (e *ConnectionError) Error() string {
	return fmt.Sprintf("connection error (%s): %s", e.Endpoint, e.ClientError.Error())
}

// TimeoutError indicates a timeout
type TimeoutError struct {
	ClientError
	Operation string
	Timeout   time.Duration
}

// Error implements the error interface
func (e *TimeoutError) Error() string {
	return fmt.Sprintf("timeout after %v during %s: %s", e.Timeout, e.Operation, e.ClientError.Error())
}

// ServerError represents an error returned by the server
type ServerError struct {
	ClientError
	Method   string
	ServerID string
}

// Error implements the error interface
func (e *ServerError) Error() string {
	return fmt.Sprintf("server error during %s: %s", e.Method, e.ClientError.Error())
}

// NewClientError creates a new ClientError
func NewClientError(message string, code int, cause error) error {
	return &ClientError{
		Message: message,
		Code:    code,
		Cause:   cause,
	}
}

// NewTransportError creates a new TransportError
func NewTransportError(transport, message string, cause error) error {
	return &TransportError{
		ClientError: ClientError{
			Message: message,
			Code:    0,
			Cause:   cause,
		},
		Transport: transport,
	}
}

// NewConnectionError creates a new ConnectionError
func NewConnectionError(endpoint, message string, cause error) error {
	return &ConnectionError{
		ClientError: ClientError{
			Message: message,
			Code:    0,
			Cause:   cause,
		},
		Endpoint: endpoint,
	}
}

// NewTimeoutError creates a new TimeoutError
func NewTimeoutError(operation string, timeout time.Duration, cause error) error {
	return &TimeoutError{
		ClientError: ClientError{
			Message: fmt.Sprintf("operation timed out after %v", timeout),
			Code:    0,
			Cause:   cause,
		},
		Operation: operation,
		Timeout:   timeout,
	}
}

// NewServerError creates a new ServerError
func NewServerError(method, serverID string, code int, message string, cause error) error {
	return &ServerError{
		ClientError: ClientError{
			Message: message,
			Code:    code,
			Cause:   cause,
		},
		Method:   method,
		ServerID: serverID,
	}
}

// IsTimeoutError checks if an error is a timeout error
func IsTimeoutError(err error) bool {
	var timeoutErr *TimeoutError
	return errors.As(err, &timeoutErr) || errors.Is(err, ErrRequestTimeout)
}

// IsTransportError checks if an error is a transport error
func IsTransportError(err error) bool {
	var transportErr *TransportError
	return errors.As(err, &transportErr) || errors.Is(err, ErrTransportFailure)
}

// IsConnectionError checks if an error is a connection error
func IsConnectionError(err error) bool {
	var connErr *ConnectionError
	return errors.As(err, &connErr) || errors.Is(err, ErrNotConnected) || errors.Is(err, ErrAlreadyConnected)
}

// IsServerError checks if an error is a server-reported error
func IsServerError(err error) bool {
	var serverErr *ServerError
	return errors.As(err, &serverErr) || errors.Is(err, ErrServerError)
}
