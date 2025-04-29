// Package protocol defines the structures and constants for the Model Context Protocol (MCP).
package protocol

const (
	// CurrentProtocolVersion defines the MCP version this library implementation supports.
	CurrentProtocolVersion = "2025-03-26" // The primary version this server implements
	OldProtocolVersion     = "2024-11-05" // An older version accepted for compatibility

	// --- Message Type (Method Name) Constants ---
	// These align with the JSON-RPC 'method' field names from the spec.

	// Initialization
	MethodInitialize  = "initialize"
	MethodInitialized = "initialized" // Notification

	// Tools
	MethodListTools              = "tools/list"
	MethodCallTool               = "tools/call"
	MethodNotifyToolsListChanged = "notifications/tools/list_changed" // Notification

	// Resources
	MethodListResources              = "resources/list"
	MethodReadResource               = "resources/read"
	MethodSubscribeResource          = "resources/subscribe"                  // Request
	MethodUnsubscribeResource        = "resources/unsubscribe"                // Request (Optional, can use subscribe with empty list)
	MethodNotifyResourcesListChanged = "notifications/resources/list_changed" // Notification
	MethodNotifyResourceUpdated      = "notifications/resources/updated"      // Notification (Renamed from changed)

	// Prompts
	MethodListPrompts              = "prompts/list"
	MethodGetPrompt                = "prompts/get"
	MethodNotifyPromptsListChanged = "notifications/prompts/list_changed" // Notification

	// Logging
	MethodLoggingSetLevel     = "logging/set_level"
	MethodNotificationMessage = "notifications/message" // Note: This is a notification

	// Sampling
	MethodSamplingCreateMessage = "sampling/create_message" // Note: 2025 spec name
	MethodSamplingRequest       = "sampling/request"        // Generic name used in client handler

	// Roots
	MethodRootsList              = "roots/list"
	MethodNotifyRootsListChanged = "notifications/roots/list_changed" // Notification

	// Ping
	MethodPing = "ping"

	// Cancellation & Progress (Notifications)
	MethodCancelled = "$/cancelled"
	MethodProgress  = "$/progress"

	// MessageTypeError identifies an Error message (conceptually).
	MessageTypeError = "Error" // This might become irrelevant
)
