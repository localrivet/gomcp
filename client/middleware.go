package client

import (
	"context"
	"time"

	"github.com/localrivet/gomcp/logx"
	"github.com/localrivet/gomcp/protocol"
)

// Common middleware implementations

// loggingMiddleware logs requests and responses
type loggingMiddleware struct {
	logger logx.Logger
}

// NewLoggingMiddleware creates a new logging middleware
func NewLoggingMiddleware(logger logx.Logger) ClientMiddleware {
	return &loggingMiddleware{
		logger: logger,
	}
}

// BeforeSendRequest implements ClientMiddleware.BeforeSendRequest
func (m *loggingMiddleware) BeforeSendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCRequest, error) {
	m.logger.Debug("Sending request: %s (id=%v)", req.Method, req.ID)
	return req, nil
}

// AfterReceiveResponse implements ClientMiddleware.AfterReceiveResponse
func (m *loggingMiddleware) AfterReceiveResponse(ctx context.Context, resp *protocol.JSONRPCResponse) (*protocol.JSONRPCResponse, error) {
	if resp.Error != nil {
		m.logger.Error("Received error response: %s (id=%v)", resp.Error.Message, resp.ID)
	} else {
		m.logger.Debug("Received successful response (id=%v)", resp.ID)
	}
	return resp, nil
}

// authMiddleware adds authentication to requests
type authMiddleware struct {
	authProvider AuthProvider
}

// NewAuthMiddleware creates a new authentication middleware
func NewAuthMiddleware(authProvider AuthProvider) ClientMiddleware {
	return &authMiddleware{
		authProvider: authProvider,
	}
}

// BeforeSendRequest implements ClientMiddleware.BeforeSendRequest
func (m *authMiddleware) BeforeSendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCRequest, error) {
	// For now, there's no direct way to add headers to the request in the protocol
	// This will be handled by the transport layer using the authProvider
	return req, nil
}

// AfterReceiveResponse implements ClientMiddleware.AfterReceiveResponse
func (m *authMiddleware) AfterReceiveResponse(ctx context.Context, resp *protocol.JSONRPCResponse) (*protocol.JSONRPCResponse, error) {
	// Check for authentication errors
	if resp.Error != nil && resp.Error.Code == 401 {
		return resp, NewAuthError("Authentication failed", resp.Error.Message, nil)
	}
	return resp, nil
}

// retryMiddleware handles retrying failed requests
type retryMiddleware struct {
	backoff BackoffStrategy
}

// NewRetryMiddleware creates a new retry middleware
func NewRetryMiddleware(backoff BackoffStrategy) ClientMiddleware {
	return &retryMiddleware{
		backoff: backoff,
	}
}

// BeforeSendRequest implements ClientMiddleware.BeforeSendRequest
func (m *retryMiddleware) BeforeSendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCRequest, error) {
	// Nothing to do before sending
	return req, nil
}

// AfterReceiveResponse implements ClientMiddleware.AfterReceiveResponse
func (m *retryMiddleware) AfterReceiveResponse(ctx context.Context, resp *protocol.JSONRPCResponse) (*protocol.JSONRPCResponse, error) {
	// The actual retry logic is handled at a higher level, not in the middleware
	// This is because we need to re-send the entire request, not just transform the response
	return resp, nil
}

// timeoutMiddleware adds timeout handling to requests
type timeoutMiddleware struct {
	timeout time.Duration
}

// NewTimeoutMiddleware creates a new timeout middleware
func NewTimeoutMiddleware(timeout time.Duration) ClientMiddleware {
	return &timeoutMiddleware{
		timeout: timeout,
	}
}

// BeforeSendRequest implements ClientMiddleware.BeforeSendRequest
func (m *timeoutMiddleware) BeforeSendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCRequest, error) {
	// Create a new context with timeout if none exists
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, m.timeout)

		// Store the cancel function in the context so it can be called later
		ctx = context.WithValue(ctx, "cancel_func", cancel)
	}

	// The context is not directly passed to the request, but will be used
	// by the transport layer when sending the request
	return req, nil
}

// AfterReceiveResponse implements ClientMiddleware.AfterReceiveResponse
func (m *timeoutMiddleware) AfterReceiveResponse(ctx context.Context, resp *protocol.JSONRPCResponse) (*protocol.JSONRPCResponse, error) {
	// Check if the context has a cancel function and call it to release resources
	if cancel, ok := ctx.Value("cancel_func").(context.CancelFunc); ok {
		cancel()
	}

	return resp, nil
}

// NewAuthError creates a new authentication error
func NewAuthError(message, serverMessage string, cause error) error {
	return &ClientError{
		Message: message + ": " + serverMessage,
		Code:    401,
		Cause:   cause,
	}
}

// MiddlewareChain chains multiple middleware together
type MiddlewareChain struct {
	middleware []ClientMiddleware
}

// NewMiddlewareChain creates a new middleware chain
func NewMiddlewareChain(middleware ...ClientMiddleware) ClientMiddleware {
	return &MiddlewareChain{
		middleware: middleware,
	}
}

// BeforeSendRequest implements ClientMiddleware.BeforeSendRequest
func (c *MiddlewareChain) BeforeSendRequest(ctx context.Context, req *protocol.JSONRPCRequest) (*protocol.JSONRPCRequest, error) {
	var err error
	for _, m := range c.middleware {
		req, err = m.BeforeSendRequest(ctx, req)
		if err != nil {
			return req, err
		}
	}
	return req, nil
}

// AfterReceiveResponse implements ClientMiddleware.AfterReceiveResponse
func (c *MiddlewareChain) AfterReceiveResponse(ctx context.Context, resp *protocol.JSONRPCResponse) (*protocol.JSONRPCResponse, error) {
	var err error
	// Apply middleware in reverse order for responses
	for i := len(c.middleware) - 1; i >= 0; i-- {
		resp, err = c.middleware[i].AfterReceiveResponse(ctx, resp)
		if err != nil {
			return resp, err
		}
	}
	return resp, nil
}
