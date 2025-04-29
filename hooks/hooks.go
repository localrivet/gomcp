// Package hooks defines types and interfaces for the gomcp hooks system,
// allowing users to inject custom logic at various points in the client
// and server lifecycles.
package hooks

import (
	"context"
	"encoding/json"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"
)

// --- Server Hooks ---

// ServerHookContext provides context available to most server-side hooks.
type ServerHookContext struct {
	Ctx            context.Context
	Session        types.ClientSession // Interface to interact with the specific client session
	MessageID      interface{}         // nil for notifications
	Method         string              // Method name if applicable
	ToolDefinition *protocol.Tool      // Populated for tool-related hooks
}

// BeforeHandleMessageHook: Runs after receiving raw bytes, before any JSON parsing.
// Return: Modified bytes (if changed), error to stop processing.
type BeforeHandleMessageHook func(ctx context.Context, session types.ClientSession, rawMessage []byte) (modifiedRawMessage []byte, err error)

// BeforeUnmarshalHook: Runs after basic JSON structure detected but before specific unmarshalling.
// Return: Error to stop processing.
type BeforeUnmarshalHook func(hookCtx ServerHookContext, rawParams json.RawMessage) error

// ServerBeforeHandleRequestHook: Runs before routing a parsed request to its specific handler on the server.
// Params: Parsed params (`any`).
// Return: Error to stop processing (e.g., return auth error response).
type ServerBeforeHandleRequestHook func(hookCtx ServerHookContext, params any) error

// ServerBeforeHandleNotificationHook: Runs before routing a parsed notification on the server.
// Params: Parsed params (`any`).
// Return: Error to stop processing.
type ServerBeforeHandleNotificationHook func(hookCtx ServerHookContext, params any) error

// FinalToolHandler defines the signature for the actual tool execution logic.
type FinalToolHandler func(ctx context.Context, progressToken interface{}, arguments any) (content []protocol.Content, isError bool)

// BeforeToolCallHook wraps the next handler in the chain, allowing modification before execution.
// It receives the next handler (which could be another hook wrapper or the final handler)
// and returns a new handler function that incorporates the hook's logic.
type BeforeToolCallHook func(next FinalToolHandler) FinalToolHandler

// AfterToolCallHook: Runs after a tool handler executes. Accesses ToolDefinition via hookCtx.
// Return: Modified content, isError flag, error.
type AfterToolCallHook func(hookCtx ServerHookContext, arguments any, content []protocol.Content, isError bool, toolErr error) (modifiedContent []protocol.Content, modifiedIsError bool, modifiedError error)

// BeforeSendResponseHook: Runs before a response is sent back to the client.
// Return: Modified response object, error to prevent sending.
type BeforeSendResponseHook func(hookCtx ServerHookContext, response *protocol.JSONRPCResponse) (modifiedResponse *protocol.JSONRPCResponse, err error)

// ServerBeforeSendNotificationHook: Runs before a server-initiated notification is sent.
// Return: Modified method/params, error to prevent sending.
type ServerBeforeSendNotificationHook func(hookCtx ServerHookContext, method string, params any) (modifiedMethod string, modifiedParams any, err error)

// OnSessionCreateHook: Runs after a new session is successfully registered.
type OnSessionCreateHook func(hookCtx ServerHookContext) error

// BeforeSessionDestroyHook: Runs just before a session is unregistered.
type BeforeSessionDestroyHook func(hookCtx ServerHookContext) error

// --- Client Hooks ---

// ClientHookContext provides context for client-side hooks.
type ClientHookContext struct {
	Ctx               context.Context
	ClientInfo        protocol.Implementation // Info about this client instance
	NegotiatedVersion string
	ServerInfo        protocol.Implementation // Info about the connected server
	ServerCaps        protocol.ServerCapabilities
	MessageID         interface{} // nil for notifications/incoming requests
	Method            string      // Can be empty for responses
}

// ClientBeforeSendRequestHook: Runs before marshalling and sending a client request.
// Return: Modified request object, error to prevent sending.
type ClientBeforeSendRequestHook func(hookCtx ClientHookContext, request *protocol.JSONRPCRequest) (modifiedRequest *protocol.JSONRPCRequest, err error)

// ClientBeforeSendNotificationHook: Runs before marshalling and sending a client notification.
// Return: Modified notification object, error to prevent sending.
type ClientBeforeSendNotificationHook func(hookCtx ClientHookContext, notification *protocol.JSONRPCNotification) (modifiedNotification *protocol.JSONRPCNotification, err error)

// OnReceiveRawMessageHook: Runs after receiving raw bytes from transport, before parsing.
// Return: Modified bytes, error to stop processing.
type OnReceiveRawMessageHook func(hookCtx ClientHookContext, rawMessage []byte) (modifiedRawMessage []byte, err error)

// ClientBeforeHandleResponseHook: Runs after parsing a response, before matching to pending request.
// Return: Modified response object, error to stop processing.
type ClientBeforeHandleResponseHook func(hookCtx ClientHookContext, response *protocol.JSONRPCResponse) (modifiedResponse *protocol.JSONRPCResponse, err error)

// ClientBeforeHandleNotificationHook: Runs after parsing an incoming notification on the client, before calling registered handler.
// Params: Parsed params (`any`).
// Return: Error to prevent calling handler.
type ClientBeforeHandleNotificationHook func(hookCtx ClientHookContext, method string, params any) error

// ClientBeforeHandleRequestHook: Runs after parsing an incoming server-to-client request on the client, before calling handler.
// Params: Parsed params (`any`).
// Return: Error to prevent calling handler (and send error response).
type ClientBeforeHandleRequestHook func(hookCtx ClientHookContext, id interface{}, method string, params any) error
