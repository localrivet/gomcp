package server

import (
	"context" // Import context
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/url" // Import net/url
	"os"      // Import os
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"

	// Correct import path
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"

	"github.com/yosida95/uritemplate/v3"
)

// MessageHandler handles incoming and outgoing MCP messages.
type MessageHandler struct {
	server *Server // Reference back to the main Server struct

	activeRequests       map[string]chan *protocol.JSONRPCResponse // Use correct type
	requestMu            sync.Mutex
	notificationHandlers map[string]func(params json.RawMessage) // Use correct type for handler signature
	notificationMu       sync.Mutex
	activeCancels        map[string]context.CancelFunc // Map to store cancel functions for active requests
	cancelMu             sync.Mutex                    // Mutex for activeCancels map

	// Handlers for different message types
	lifecycleHandler *LifecycleHandler
	// TODO: Add other handlers (e.g., toolHandler, resourceHandler)
	// Adding a comment to trigger re-evaluation
}

// NewMessageHandler creates a new MessageHandler instance.
func NewMessageHandler(srv *Server) *MessageHandler {
	mh := &MessageHandler{
		server:               srv,
		activeRequests:       make(map[string]chan *protocol.JSONRPCResponse), // Use correct type
		notificationHandlers: make(map[string]func(params json.RawMessage)),
		activeCancels:        make(map[string]context.CancelFunc), // Initialize cancel map
		// Initialize handlers, passing dependencies
		lifecycleHandler: NewLifecycleHandler(srv), // Pass the server instance
	}
	return mh
}

// HandleMessage processes an incoming JSON-RPC message for a specific session.
// It unmarshals the message, dispatches it to the appropriate handler, and sends a response if necessary.
func (mh *MessageHandler) HandleMessage(session types.ClientSession, message []byte) error {
	// Use the server's logger
	mh.server.logger.Debug("Received raw message from session %s: %s", session.SessionID(), string(message))

	// Attempt to unmarshal into a structure that can identify the message type
	var msg struct {
		ID     json.RawMessage `json:"id"` // Can be string, number, or null
		Method string          `json:"method"`
		Params json.RawMessage `json:"params"`
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}

	if err := json.Unmarshal(message, &msg); err != nil {
		log.Printf("Error unmarshalling message structure from session %s: %v\n", session.SessionID(), err)
		// Send a parse error response. For parse errors, the ID is typically null.
		errorResponse := protocol.NewErrorResponse(nil, protocol.CodeParseError, fmt.Sprintf("Parse error: %v", err), nil)
		// Attempt to send the error response back to the session.
		// This might fail if the message was severely malformed, but we try.
		// Use the session to send the response.
		session.SendResponse(*errorResponse)
		return err // Return the error to potentially signal the transport to close the session
	}

	// Determine message type based on fields
	// A message is a Request if it has a method and a non-null ID.
	if msg.Method != "" && msg.ID != nil && string(msg.ID) != "null" {
		// It's a Request
		var req protocol.JSONRPCRequest
		if err := json.Unmarshal(message, &req); err != nil {
			log.Printf("Error unmarshalling request from session %s: %v\n", session.SessionID(), err)
			// Send an invalid request error response
			errorResponse := protocol.NewErrorResponse(msg.ID, protocol.CodeInvalidRequest, fmt.Sprintf("Invalid Request: %v", err), nil)
			// Use the session to send the response.
			session.SendResponse(*errorResponse)
			return err
		}
		// Pass session to handleRequest
		go mh.handleRequest(session, &req) // Handle in a goroutine
		// A message is a Notification if it has a method but no ID or a null ID.
	} else if msg.Method != "" && (msg.ID == nil || string(msg.ID) == "null") {
		// It's a Notification
		var notif protocol.JSONRPCNotification
		if err := json.Unmarshal(message, &notif); err != nil {
			log.Printf("Error unmarshalling notification from session %s: %v\n", session.SessionID(), err)
			// For notifications, we just log the error and continue
			return err
		}
		// Pass session to handleNotification
		go mh.handleNotification(session, &notif) // Handle in a goroutine
		// A message is a Response if it has a result or error field and a non-null ID.
	} else if (msg.Result != nil || msg.Error != nil) && msg.ID != nil && string(msg.ID) != "null" {
		// It's a Response
		var res protocol.JSONRPCResponse
		if err := json.Unmarshal(message, &res); err != nil {
			log.Printf("Error unmarshalling response from session %s: %v\n", session.SessionID(), err)
			// For responses, we just log the error and continue
			return err
		}
		// Handle response by matching to active requests
		mh.requestMu.Lock()
		// Convert ID to a comparable type (string or number)
		idStr := fmt.Sprintf("%v", res.ID) // Using Sprintf to handle different ID types
		if respChan, ok := mh.activeRequests[idStr]; ok {
			delete(mh.activeRequests, idStr) // Remove from active requests
			mh.requestMu.Unlock()
			respChan <- &res // Send the response to the waiting goroutine
			close(respChan)  // Close the channel
			log.Printf("Handled response for ID %v from session %s", res.ID, session.SessionID())
		} else {
			mh.requestMu.Unlock()
			log.Printf("Received unsolicited response for ID %v from session %s", res.ID, session.SessionID())
			// TODO: Potentially send an error response for unsolicited responses?
			// The JSON-RPC spec doesn't explicitly require this for unsolicited responses.
		}
		// If none of the above, it's an unknown message type.
	} else {
		// Unknown message type
		log.Printf("Received unknown message type from session %s: %s", session.SessionID(), string(message))
		// Send an invalid request error response
		errorResponse := protocol.NewErrorResponse(msg.ID, protocol.CodeInvalidRequest, "Unknown message type", nil)
		// Use the session to send the response.
		session.SendResponse(*errorResponse)
	}

	return nil
}

// handleRequest handles an incoming request message.
// connectionID is the unique identifier for the client connection.
func (mh *MessageHandler) handleRequest(session types.ClientSession, req *protocol.JSONRPCRequest) {
	requestIDStr := fmt.Sprintf("%v", req.ID) // Convert request ID to string
	mh.server.logger.Info("Handling request from session %s: %s (ID: %s)", session.SessionID(), req.Method, requestIDStr)

	// Create cancellable context for this request
	reqCtx, cancel := context.WithCancel(context.Background())
	defer cancel() // Ensure context is cancelled eventually if not already

	// Store the cancel function
	mh.cancelMu.Lock()
	mh.activeCancels[requestIDStr] = cancel
	mh.cancelMu.Unlock()

	// Ensure the cancel function is removed when the handler finishes
	defer func() {
		mh.cancelMu.Lock()
		delete(mh.activeCancels, requestIDStr)
		mh.cancelMu.Unlock()
		mh.server.logger.Debug("Removed cancel function for request ID: %s", requestIDStr)
	}()

	var result interface{}
	var err error

	// Dispatch the request based on its method.
	switch req.Method {
	case protocol.MethodInitialize:
		// Handle the 'initialize' request. This is the first message sent by the client
		// to establish a session and exchange capabilities.
		var params protocol.InitializeRequestParams
		if unmarshalErr := protocol.UnmarshalPayload(req.Params, &params); unmarshalErr != nil {
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeInvalidParams,
					Message: fmt.Sprintf("Invalid initialize parameters: %v", unmarshalErr),
				},
			}
		} else {
			// InitializeHandler returns result, caps, error. We only need result and error here.
			result, _, err = mh.lifecycleHandler.InitializeHandler(params)
		}

	case protocol.MethodShutdown:
		// Handle shutdown request
		// No parameters expected for this request
		err = mh.lifecycleHandler.ShutdownHandler()

	case protocol.MethodCallTool:
		// Handle the 'tools/call' request. Client executes a registered server tool.
		var params protocol.CallToolRequestParams // <-- Use correct request params struct
		if unmarshalErr := protocol.UnmarshalPayload(req.Params, &params); unmarshalErr != nil {
			err = protocol.NewInvalidParamsError(fmt.Sprintf("Invalid tools/call parameters: %v", unmarshalErr))
			break
		}

		// Validate required fields
		if params.ToolCall == nil || params.ToolCall.ToolName == "" || params.ToolCall.ID == "" {
			err = protocol.NewInvalidParamsError("Missing required fields in tools/call parameters (tool_call, tool_name, id)")
			break
		}

		toolName := params.ToolCall.ToolName
		toolInput := params.ToolCall.Input // Input is json.RawMessage
		toolCallID := params.ToolCall.ID
		progressToken := interface{}(nil)
		if params.Meta != nil {
			progressToken = params.Meta.ProgressToken // Extract progress token
		}

		mh.server.logger.Debug("Attempting to call tool '%s' with ID '%s' for request %s", toolName, toolCallID, requestIDStr)

		toolHandler, ok := mh.server.Registry.GetToolHandler(toolName)
		if !ok {
			mh.server.logger.Warn("Tool not found: %s", toolName)
			// Tool not found - return result with error according to 2025-03-26 schema
			result = protocol.CallToolResult{
				ToolCallID: toolCallID,
				Error: &protocol.ToolError{
					Code:    protocol.CodeMCPToolNotFound,
					Message: fmt.Sprintf("Tool not found: %s", toolName),
				},
			}
		} else {
			// Create the context for the tool handler, passing the progress token
			toolCtx := NewContext(reqCtx, requestIDStr, session, progressToken, mh.server)

			// Report start progress if token exists
			if progressToken != nil {
				startMessage := fmt.Sprintf("Executing tool: %s", toolName)
				mh.server.logger.Debug("Reporting start progress for token %v: %s", progressToken, startMessage)
				// Use dummy current/total for now, or extract from payload if defined
				toolCtx.ReportProgress(startMessage, 0, 100)
			}

			// Call the tool handler
			toolOutput, toolErr := toolHandler(toolCtx, toolInput) // Pass toolCtx and correct input

			// Report end progress if token exists
			if progressToken != nil {
				status := "completed"
				message := fmt.Sprintf("Tool execution finished: %s", toolName)
				current := 100 // Assume 100% on finish/fail for now
				total := 100
				if toolErr != nil {
					status = "failed"
					message = fmt.Sprintf("Tool execution failed: %s (%v)", toolName, toolErr)
				}
				mh.server.logger.Debug("Reporting end progress for token %v: %s (status: %s)", progressToken, message, status)
				// Use the message and dummy current/total
				toolCtx.ReportProgress(message, current, total)
			}

			// --- Determine and Construct Final Result ---
			negotiatedVersion := session.GetNegotiatedVersion()

			if toolErr != nil {
				// Handle TOOL EXECUTION Error
				mh.server.logger.Error("Tool '%s' execution failed: %v", toolName, toolErr)
				if negotiatedVersion == protocol.OldProtocolVersion {
					// --- V2024 Error Result ---
					content := []protocol.Content{protocol.TextContent{Type: "text", Text: toolErr.Error()}}
					result = protocol.CallToolResultV2024{
						ToolCallID: toolCallID,
						Content:    content,
						IsError:    true,
					}
				} else {
					// --- V2025 Error Result ---
					mcpErr, isMCPErr := toolErr.(*protocol.MCPError)
					var toolErrorPayload protocol.ToolError
					if isMCPErr {
						toolErrorPayload = protocol.ToolError{Code: mcpErr.Code, Message: mcpErr.Message, Data: mcpErr.Data}
					} else {
						toolErrorPayload = protocol.ToolError{Code: protocol.CodeMCPToolExecutionError, Message: fmt.Sprintf("Tool execution failed: %v", toolErr)}
					}
					var outputBytesOnError json.RawMessage
					if toolOutput != nil {
						marshalledOutput, marshalErr := json.Marshal(toolOutput)
						if marshalErr == nil {
							outputBytesOnError = json.RawMessage(marshalledOutput)
						}
					}
					result = protocol.CallToolResult{
						ToolCallID: toolCallID,
						Error:      &toolErrorPayload,
						Output:     outputBytesOnError,
					}
				}
			} else {
				// Handle TOOL EXECUTION Success
				mh.server.logger.Info("Tool '%s' executed successfully for request %s", toolName, requestIDStr)
				if negotiatedVersion == protocol.OldProtocolVersion {
					// --- V2024 Success Result ---
					content, conversionErr := convertToolOutputToContent(toolOutput)
					if conversionErr != nil {
						mh.server.logger.Error("Failed to convert tool output to V2024 content: %v", conversionErr)
						content = []protocol.Content{protocol.TextContent{Type: "text", Text: fmt.Sprintf("Error processing tool output: %v", conversionErr)}}
					}
					result = protocol.CallToolResultV2024{
						ToolCallID: toolCallID,
						Content:    content,
						IsError:    false,
					}
				} else {
					// --- V2025 Success Result ---
					outputBytes, marshalErr := json.Marshal(toolOutput)
					if marshalErr != nil {
						err = &protocol.MCPError{ // Assign to the main `err` variable for top-level error handling
							ErrorPayload: protocol.ErrorPayload{
								Code:    protocol.CodeInternalError,
								Message: fmt.Sprintf("Failed to marshal tool output: %v", marshalErr),
							},
						}
						break // Break out of the main switch if marshalling fails
					}
					result = protocol.CallToolResult{
						ToolCallID: toolCallID,
						Output:     json.RawMessage(outputBytes),
						Error:      nil,
					}
				}
			}
		}

	case protocol.MethodListResources:
		// Handle resources/list request
		// No parameters expected for this request according to protocol/resources.go
		// Get the list of registered resources from the registry
		resourceMap := mh.server.Registry.ResourceRegistry()

		// Convert the map to a slice of protocol.Resource
		resources := make([]protocol.Resource, 0, len(resourceMap))
		for _, res := range resourceMap {
			resources = append(resources, res)
		}

		// Format the result into a protocol.ListResourcesResult
		result = protocol.ListResourcesResult{
			Resources: resources,
			// NextCursor is optional, leaving empty for now
		}

	case protocol.MethodReadResource:
		// Handle the 'resources/read' request. This is sent by the client to get the content of a specific resource.
		var params protocol.ReadResourceRequestParams
		// Use UnmarshalPayload which handles map[string]interface{} or json.RawMessage
		if err := protocol.UnmarshalPayload(req.Params, &params); err != nil {
			mh.server.logger.Warn("Error unmarshalling resources/read params from session %s: %v", session.SessionID(), err)
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeMCPResourceNotFound,
					Message: fmt.Sprintf("Resource not found: %s", params.URI),
				},
			}
			break // Exit the switch case
		}

		// Get the resource from the registry
		resourceMap := mh.server.Registry.ResourceRegistry()
		resource, resourceFound := resourceMap[params.URI]

		// --- Logic for handling resource read ---
		var contents []protocol.ResourceContents
		var finalResource protocol.Resource // Store the resource/template info for the result
		var readErr error                   // Use a separate error variable for reading/template execution

		if resourceFound {
			// --- Handle Static Resource ---
			mh.server.logger.Debug("Found static resource for URI: %s", params.URI)
			finalResource = resource // Use the found static resource
			var staticContents protocol.ResourceContents
			switch resource.Kind {
			case string(protocol.ResourceKindFile):
				// ... (existing file reading logic) ...
				// Assign to staticContents, set readErr if needed
				u, err := url.Parse(resource.URI)
				if err != nil {
					readErr = &protocol.MCPError{ /* ... */ }
					break
				}
				if u.Scheme != "file" {
					readErr = &protocol.MCPError{ /* ... */ }
					break
				}
				filePath := u.Path
				data, err := os.ReadFile(filePath)
				if err != nil {
					readErr = &protocol.MCPError{ /* ... */ }
					break
				}
				contentType := "text/plain"
				staticContents = protocol.TextResourceContents{ContentType: contentType, Content: string(data)}

			case string(protocol.ResourceKindBlob):
				// ... (existing blob reading logic) ...
				// Assign to staticContents, set readErr if needed
				u, err := url.Parse(resource.URI)
				if err != nil {
					readErr = &protocol.MCPError{ /* ... */ }
					break
				}
				if u.Scheme != "file" {
					readErr = &protocol.MCPError{ /* ... */ }
					break
				}
				filePath := u.Path
				data, err := os.ReadFile(filePath)
				if err != nil {
					readErr = &protocol.MCPError{ /* ... */ }
					break
				}
				contentType := "application/octet-stream"
				encodedContent := base64.StdEncoding.EncodeToString(data)
				staticContents = protocol.BlobResourceContents{ContentType: contentType, Blob: encodedContent}

			case string(protocol.ResourceKindAudio):
				// ... (existing audio reading logic) ...
				// Assign to staticContents, set readErr if needed
				u, err := url.Parse(resource.URI)
				if err != nil {
					readErr = &protocol.MCPError{ /* ... */ }
					break
				}
				if u.Scheme != "file" {
					readErr = &protocol.MCPError{ /* ... */ }
					break
				}
				filePath := u.Path
				data, err := os.ReadFile(filePath)
				if err != nil {
					readErr = &protocol.MCPError{ /* ... */ }
					break
				}
				contentType := "audio/unknown"
				encodedContent := base64.StdEncoding.EncodeToString(data)
				staticContents = protocol.AudioResourceContents{ContentType: contentType, Audio: encodedContent}

			default:
				readErr = &protocol.MCPError{
					ErrorPayload: protocol.ErrorPayload{
						Code:    protocol.CodeMCPOperationFailed,
						Message: fmt.Sprintf("Unsupported static resource kind for reading: %s", resource.Kind),
					},
				}
				log.Printf("Unsupported static resource kind for reading for session %s: %v", session.SessionID(), readErr)
			}
			if readErr == nil && staticContents != nil {
				contents = []protocol.ResourceContents{staticContents}
			}
		} else {
			// --- Try Matching Resource Template ---
			mh.server.logger.Debug("Static resource not found, attempting template match for URI: %s", params.URI)
			templateRegistry := mh.server.Registry.TemplateRegistry()
			var matched bool
			for pattern, templateInfo := range templateRegistry {

				// TODO: Replace placeholder matching with actual library/logic
				matchResult, ok := matchURITemplate(pattern, params.URI)
				if ok {
					mh.server.logger.Debug("Matched URI %s to template pattern %s", params.URI, pattern)
					matched = true
					finalResource = protocol.Resource{URI: params.URI, Kind: "dynamic"} // Use requested URI, kind is dynamic

					// Create context for handler
					handlerCtx := NewContext(reqCtx, requestIDStr, session, nil, mh.server) // Pass request context

					// Prepare arguments for handler call
					handerArgs, prepErr := prepareHandlerArgs(handlerCtx, templateInfo, matchResult)
					if prepErr != nil {
						readErr = prepErr
						break // Exit template loop on prep error
					}

					// Call handler function via reflection
					handerVal := reflect.ValueOf(templateInfo.HandlerFn)
					returnValues := handerVal.Call(handerArgs)

					// Convert return values to protocol.Content
					contents, readErr = convertHandlerResultToResourceContents(returnValues)
					if readErr != nil {
						break // Exit template loop on conversion error
					}
				}
			}
			if !matched && readErr == nil { // If no static resource and no template matched
				mh.server.logger.Warn("Resource or template not found for URI: %s", params.URI)
				readErr = &protocol.MCPError{
					ErrorPayload: protocol.ErrorPayload{
						Code:    protocol.CodeMCPResourceNotFound,
						Message: fmt.Sprintf("Resource not found: %s", params.URI),
					},
				}
			}
		}

		if readErr != nil {
			err = readErr // Assign the read error to the main error variable
			break         // Exit the main switch case
		}

		// Format the result into a protocol.ReadResourceResult
		// Ensure finalResource is not zero-value if contents were found
		if len(contents) > 0 && finalResource.URI == "" {
			// This should ideally not happen if logic above is correct
			// If static resource was found, finalResource is set.
			// If template was found, finalResource is set.
			mh.server.logger.Error("Internal inconsistency: found resource contents but no final resource metadata for URI %s", params.URI)
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeInternalError,
					Message: "Internal server error processing resource read",
				},
			}
			break // Exit the main switch case
		}

		result = protocol.ReadResourceResult{
			Resource: finalResource, // Use the determined resource (static or dynamic placeholder)
			Contents: contents,      // Use the generated/read contents
		}

	case protocol.MethodSubscribeResource:
		// Handle resources/subscribe request
		var params protocol.SubscribeResourceParams
		rawParams, ok := req.Params.(json.RawMessage)
		if !ok {
			mh.server.logger.Warn("Invalid params type for resources/subscribe from session %s: expected json.RawMessage", session.SessionID())
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeInvalidParams,
					Message: "Invalid params type for resources/subscribe",
				},
			}
			break // Exit the switch case
		}
		if err := json.Unmarshal(rawParams, &params); err != nil {
			log.Printf("Error unmarshalling resources/subscribe params from session %s: %v", session.SessionID(), err)
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeInvalidParams,
					Message: fmt.Sprintf("Invalid params for resources/subscribe: %v", err),
				},
			}
			break // Exit the switch case
		}

		log.Printf("Received resources/subscribe request from session %s for URIs: %v", session.SessionID(), params.URIs)
		// Use the SubscriptionManager to subscribe the session ID
		for _, uri := range params.URIs {
			mh.server.SubscriptionManager.Subscribe(uri, session.SessionID())
		}

		// The result for subscribe is currently empty according to the protocol spec.
		result = protocol.SubscribeResourceResult{}

	case protocol.MethodUnsubscribeResource:
		// Handle resources/unsubscribe request
		var params protocol.UnsubscribeResourceParams
		rawParams, ok := req.Params.(json.RawMessage)
		if !ok {
			mh.server.logger.Warn("Invalid params type for resources/unsubscribe from session %s: expected json.RawMessage", session.SessionID())
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeInvalidParams,
					Message: "Invalid params type for resources/unsubscribe",
				},
			}
			break // Exit the switch case
		}
		if err := json.Unmarshal(rawParams, &params); err != nil {
			log.Printf("Error unmarshalling resources/unsubscribe params from session %s: %v", session.SessionID(), err)
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeInvalidParams,
					Message: fmt.Sprintf("Invalid params for resources/unsubscribe: %v", err),
				},
			}
			break // Exit the switch case
		}

		log.Printf("Received resources/unsubscribe request from session %s for URIs: %v", session.SessionID(), params.URIs)
		// Use the SubscriptionManager to unsubscribe the session ID
		for _, uri := range params.URIs {
			mh.server.SubscriptionManager.Unsubscribe(uri, session.SessionID())
		}

		// The result for unsubscribe is currently empty according to the protocol spec.
		result = protocol.UnsubscribeResourceResult{}

	case protocol.MethodPing:
		// Handle ping request (no parameters expected, result is empty object)
		log.Printf("Received ping request from session %s", session.SessionID())
		result = struct{}{} // Empty struct marshals to {}
		err = nil

	case protocol.MethodListPrompts:
		// Handle prompts/list request
		// No parameters expected for this request according to protocol/prompts.go
		// Get the list of registered prompts from the registry
		prompts := mh.server.Registry.GetPrompts()

		// Format the result into a protocol.ListPromptsResult
		result = protocol.ListPromptsResult{
			Prompts: prompts,
		}

	case protocol.MethodResourcesListTemplates:
		// Handle resources/list_templates request
		// No parameters expected
		log.Printf("Received resources/list_templates request from session %s", session.SessionID())
		// TODO: Implement actual template retrieval from registry when that's added
		templates := make([]protocol.ResourceTemplate, 0) // Return empty list for now
		result = protocol.ListResourceTemplatesResult{
			Templates: templates,
		}
		err = nil

	case protocol.MethodGetPrompt:
		// Handle prompts/get request
		var params protocol.GetPromptRequestParams
		if unmarshalErr := protocol.UnmarshalPayload(req.Params, &params); unmarshalErr != nil {
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeInvalidParams,
					Message: fmt.Sprintf("Invalid prompts/get parameters: %v", unmarshalErr),
				},
			}
			break
		}

		log.Printf("Received prompts/get request from session %s for URI: %s", session.SessionID(), params.URI)
		prompt, ok := mh.server.Registry.GetPrompt(params.URI)
		if !ok {
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeInternalError,
					Message: fmt.Sprintf("Prompt not found: %s", params.URI),
				},
			}
			break
		}
		// TODO: Handle applying arguments from params.Arguments to the prompt messages if needed
		result = protocol.GetPromptResult{
			Prompt: prompt,
		}
		err = nil

	case protocol.MethodCompletionComplete: // Argument autocompletion
		var rawParams json.RawMessage
		var ok bool
		rawParams, ok = req.Params.(json.RawMessage)
		if !ok {
			// If it's not RawMessage, try marshalling it (handles map[string]interface{})
			marshalledBytes, marshalErr := json.Marshal(req.Params)
			if marshalErr != nil {
				// If marshalling fails, set the error and break immediately
				err = protocol.NewInvalidParamsError(fmt.Sprintf("completion/complete params are not valid JSON: %v", marshalErr))
				break // Exit the switch case
			}
			rawParams = marshalledBytes // Use the re-marshalled bytes
		}
		// Now rawParams is guaranteed to be valid json.RawMessage (or was nil initially)
		// Call the handler, assigning to outer result and err
		result, err = mh.handleCompletionComplete(reqCtx, rawParams)

	case protocol.MethodLoggingSetLevel: // Handle logging/set_level request
		var params protocol.SetLevelRequestParams
		if unmarshalErr := protocol.UnmarshalPayload(req.Params, &params); unmarshalErr != nil {
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeInvalidParams,
					Message: fmt.Sprintf("Invalid logging/set_level parameters: %v", unmarshalErr),
				},
			}
			break // Exit the switch case
		}

		log.Printf("Received logging/set_level request from session %s. Setting level to: %s", session.SessionID(), params.Level)
		// Store the requested level in the Server struct
		mh.server.loggingLevel = params.Level
		// Apply the level to the actual logger instance
		mh.server.logger.SetLevel(params.Level)
		log.Printf("Server logging level set to: %s", params.Level)

		// The result for logging/set_level is an empty object {}.
		result = struct{}{} // Use an empty struct for an empty JSON object result.
		err = nil           // Ensure error is nil for successful setting

	default:
		// Handle unknown methods by returning a MethodNotFound error.
		err = &protocol.MCPError{
			ErrorPayload: protocol.ErrorPayload{
				Code:    protocol.CodeMethodNotFound,
				Message: fmt.Sprintf("Method not found: %s", req.Method),
			},
		}
	}

	// Send response
	if err != nil {
		// Send error response
		log.Printf("Sending error response for request %v to session %s: %v", req.ID, session.SessionID(), err)
		// Check if the error is an MCPError to use its specific code and data
		mcpErr, isMCPErr := err.(*protocol.MCPError)
		if isMCPErr {
			errorResponse := protocol.NewErrorResponse(req.ID, mcpErr.Code, mcpErr.Message, mcpErr.Data)
			session.SendResponse(*errorResponse)
		} else {
			// Use a generic internal error code for unexpected errors
			errorResponse := protocol.NewErrorResponse(req.ID, protocol.CodeInternalError, fmt.Sprintf("Internal error: %v", err), nil)
			session.SendResponse(*errorResponse)
		}
	} else {
		// Check if context was cancelled before sending success response
		select {
		case <-reqCtx.Done():
			mh.server.logger.Info("Request %s cancelled before sending success response.", requestIDStr)
			// Do not send a success response if cancelled. An error response might have already been sent
			// by the handler if it detected cancellation, or the client might just see the request end.
			// JSON-RPC spec is a bit ambiguous here, but LSP sends ErrorCodeRequestCancelled.
			// For now, just don't send the success response.
			// Alternative: Send specific cancellation error response if no other error occurred.
			// errorResponse := protocol.NewErrorResponse(req.ID, protocol.ErrorCodeRequestCancelled, "Request cancelled", nil)
			// session.SendResponse(*errorResponse)
		default:
			// Context not cancelled, send success response
			mh.server.logger.Info("Sending success response for request %v to session %s", req.ID, session.SessionID())
			successResponse := protocol.NewSuccessResponse(req.ID, result)
			session.SendResponse(*successResponse)
		}
	}
}

// handleNotification handles an incoming notification message.
// connectionID is the unique identifier for the client connection.
func (mh *MessageHandler) handleNotification(session types.ClientSession, notif *protocol.JSONRPCNotification) {
	mh.server.logger.Info("Handling notification from session %s: %s", session.SessionID(), notif.Method)

	// Unmarshal notification parameters if they exist.
	// Note: Not all notifications have parameters, so we don't strictly require it.
	// We pass the raw parameters to the handler.

	// Dispatch the notification to registered handlers.
	mh.notificationMu.Lock()
	handler, ok := mh.notificationHandlers[notif.Method]
	mh.notificationMu.Unlock()

	if ok {
		// Handle the notification in a goroutine to avoid blocking.
		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("Panic in notification handler for method %s: %v", notif.Method, r)
					// TODO: Potentially send a logging notification about the panic?
				}
			}()
			// Pass the raw parameters directly to the handler
			rawParams, ok := notif.Params.(json.RawMessage)
			if !ok {
				log.Printf("Invalid params type for notification %s: expected json.RawMessage", notif.Method)
				// Log and ignore the notification if params is not json.RawMessage
				return
			}
			handler(rawParams)
		}()
	} else {
		// Handle unknown notification methods.
		log.Printf("Received unknown notification method from session %s: %s", session.SessionID(), notif.Method)
		// For notifications, we typically don't send an error response for unknown methods.
		// We just log and ignore.
	}

	switch notif.Method {
	case protocol.MethodInitialized:
		// Handle the 'initialized' notification. This is sent by the client after
		// receiving the InitializeResult and indicates that the client is ready
		// to receive further requests and notifications.
		// No parameters expected for this notification.
		mh.server.logger.Info("Received 'initialized' notification from session %s", session.SessionID())
		// TODO: Perform any post-initialization setup here if needed.

	case protocol.MethodCancelled: // Handle $/cancelled notification
		// Handle the '$/cancelled' notification. This is sent by the client to
		// indicate that a previously sent request should be cancelled.
		var params protocol.CancelledParams
		if unmarshalErr := protocol.UnmarshalPayload(notif.Params, &params); unmarshalErr != nil {
			mh.server.logger.Warn("Error unmarshalling $/cancelled parameters from session %s: %v", session.SessionID(), unmarshalErr)
			// For notifications, we just log the error and continue.
			return
		}

		// Convert ID to string (can be number or string)
		requestIDToCancel := fmt.Sprintf("%v", params.ID)
		mh.server.logger.Info("Received $/cancelled notification from session %s for ID: %s", session.SessionID(), requestIDToCancel)

		// Find and call the cancel function
		mh.cancelMu.Lock()
		cancelFunc, ok := mh.activeCancels[requestIDToCancel]
		if ok {
			// Remove from map *before* calling cancel, to avoid race conditions
			// if the request finishes concurrently and tries to remove itself.
			delete(mh.activeCancels, requestIDToCancel)
			mh.cancelMu.Unlock() // Unlock before potentially long-running cancel call
			mh.server.logger.Info("Attempting to cancel request ID: %s", requestIDToCancel)
			cancelFunc() // Call the cancel function
			mh.server.logger.Info("Cancellation signal sent for request ID: %s", requestIDToCancel)
		} else {
			mh.cancelMu.Unlock()
			mh.server.logger.Warn("Could not cancel request ID %s: No active cancel function found (already completed or invalid ID?)", requestIDToCancel)
		}
		return // Return after handling cancellation

	case protocol.MethodNotificationMessage: // Handle notifications/message notification
		// Handle the 'notifications/message' notification. This is sent by the client
		// to send a log message to the server.
		var params protocol.LoggingMessageParams
		if unmarshalErr := protocol.UnmarshalPayload(notif.Params, &params); unmarshalErr != nil {
			mh.server.logger.Warn("Error unmarshalling notifications/message parameters from session %s: %v", session.SessionID(), unmarshalErr)
			return
		}

		// Log the received message using the server's logger, respecting the level
		logMessage := fmt.Sprintf("[ClientLog Session: %s] %s", session.SessionID(), params.Message)
		if params.Logger != nil {
			logMessage = fmt.Sprintf("[ClientLog Session: %s Logger: %s] %s", session.SessionID(), *params.Logger, params.Message)
		}
		// Add structured data if present (requires logger supporting structured logging)
		// For now, just log the message string.

		switch params.Level {
		case protocol.LogLevelError:
			mh.server.logger.Error(logMessage)
		case protocol.LogLevelWarn:
			mh.server.logger.Warn(logMessage)
		case protocol.LogLevelInfo:
			mh.server.logger.Info(logMessage)
		case protocol.LogLevelDebug:
			mh.server.logger.Debug(logMessage)
		default:
			mh.server.logger.Warn("Received client log message with unknown level '%s': %s", params.Level, logMessage)
		}
		// No return here, might fall through to default below if not handled

	case protocol.MethodExit:
		// Handle the 'exit' notification. Client indicates it's exiting.
		// No parameters expected.
		mh.server.logger.Info("Received 'exit' notification from session %s", session.SessionID())
		mh.lifecycleHandler.ExitHandler() // Call the handler to trigger server shutdown

	// TODO: Add cases for other notifications as they are implemented.

	default:
		// Unknown notifications are handled by the dispatch logic above.
		// No need for a default case here unless specific unknown notification
		// handling beyond logging is required.
	}
}

func (mh *MessageHandler) RegisterNotificationHandler(method string, handler func(params json.RawMessage)) {
	mh.notificationMu.Lock()
	defer mh.notificationMu.Unlock()
	mh.notificationHandlers[method] = handler
}

// sendNotification sends a JSON-RPC notification over the transport to a specific session.
func (mh *MessageHandler) sendNotification(sessionID string, notif *protocol.JSONRPCNotification) {
	// Marshal the notification to JSON
	// data, err := json.Marshal(notif) // Marshalling happens within session.SendNotification
	// if err != nil {
	// 	log.Printf("Error marshalling notification for session %s: %v", sessionID, err)
	// 	return
	// }

	// Get the session from the TransportManager
	session, _, ok := mh.server.TransportManager.GetSession(sessionID) // Capture caps with _
	if !ok {
		mh.server.logger.Warn("Error sending notification: session not found for ID %s", sessionID)
		// TODO: Handle session not found error - REMOVED (logging is sufficient)
		return
	}

	// Send the notification using the session's method
	if err := session.SendNotification(*notif); err != nil {
		mh.server.logger.Error("Error sending notification via session %s: %v", sessionID, err)
		// TODO: Handle send error (e.g., connection closed) - REMOVED (logging is sufficient)
	}
}

// SendNotificationToConnections sends a JSON-RPC notification to multiple connection IDs.
func (mh *MessageHandler) SendNotificationToConnections(connectionIDs []string, notif *protocol.JSONRPCNotification) {
	// Marshal the notification once - No longer needed, marshaling happens in session.SendNotification
	// data, err := json.Marshal(notif)
	// if err != nil {
	// 	mh.server.logger.Error("Error marshalling notification for multiple connections: %v", err)
	// 	// TODO: Handle marshalling error - REMOVED (obsolete)
	// 	return
	// }

	// Send the message to each connection
	for _, connectionID := range connectionIDs {
		// Get the session
		session, _, ok := mh.server.TransportManager.GetSession(connectionID) // Capture caps with _
		if !ok {
			mh.server.logger.Warn("Error sending notification to connections: session not found for ID %s", connectionID)
			continue // Skip this ID and try the next one
		}
		// Send via the session
		if err := session.SendNotification(*notif); err != nil {
			mh.server.logger.Error("Error sending notification to connection %s via session: %v", connectionID, err)
			// TODO: Handle send error (e.g., connection closed) - REMOVED (logging is sufficient)
		}
	}
}

// SendProgress sends a $/progress notification to a specific connection.
// token is the progress token from the request's _meta field.
// value is the progress data payload.
func (mh *MessageHandler) SendProgress(connectionID string, token interface{}, value interface{}) {
	if token == nil {
		// Cannot send progress without a token
		return
	}

	// Assert the token to the expected type (string or number).
	// For simplicity, we'll assume string for now. A more robust implementation
	// would handle both string and number and convert numbers to strings.
	tokenStr, ok := token.(string)
	if !ok {
		log.Printf("Error sending progress: progress token is not a string (type: %T)", token)
		return // Cannot send progress without a valid string token
	}

	params := protocol.ProgressParams{
		Token: tokenStr, // Use the asserted string token
		Value: value,
		// Message can be added here if needed for more detailed progress
	}

	notification := protocol.NewNotification(protocol.MethodProgress, params)

	mh.sendNotification(connectionID, notification)
	log.Printf("Sent progress notification for token %v to connection %s", token, connectionID)
}

// SendLoggingMessage sends a notifications/message notification to all connected clients.
// This is used by the server to send log messages to the client.
// TODO: Implement filtering based on client logging level preferences.
func (mh *MessageHandler) SendLoggingMessage(level protocol.LoggingLevel, message string, logger *string, data interface{}) {
	params := protocol.LoggingMessageParams{
		Level:   level,
		Message: message, // For 2024-11-05 compatibility
		Logger:  logger,  // For 2025-03-26
		Data:    data,    // For 2025-03-26
	}

	notification := protocol.NewNotification(protocol.MethodNotificationMessage, params)

	// Get all connected client IDs and send the notification
	connectionIDs := mh.server.TransportManager.GetAllSessionIDs()
	if len(connectionIDs) > 0 {
		mh.SendNotificationToConnections(connectionIDs, notification)
		log.Printf("Sent logging message notification (level: %s) to %d connections", level, len(connectionIDs))
	}
}

// --- Test Helpers ---
// These methods are intended for testing purposes only.

// AddCancelFuncForTest adds a cancel function to the active cancels map for testing.
// WARNING: Use only in tests. Does not reflect the actual request handling flow.
func (mh *MessageHandler) AddCancelFuncForTest(requestID string, cancel context.CancelFunc) {
	mh.cancelMu.Lock()
	defer mh.cancelMu.Unlock()
	mh.activeCancels[requestID] = cancel
}

// GetCancelFuncForTest retrieves a cancel function from the active cancels map for testing.
// WARNING: Use only in tests.
func (mh *MessageHandler) GetCancelFuncForTest(requestID string) (context.CancelFunc, bool) {
	mh.cancelMu.Lock()
	defer mh.cancelMu.Unlock()
	cancel, ok := mh.activeCancels[requestID]
	return cancel, ok
}

// convertToolOutputToContent attempts to convert arbitrary tool output to the []protocol.Content format used in V2024.
func convertToolOutputToContent(output interface{}) ([]protocol.Content, error) {
	if output == nil {
		return []protocol.Content{}, nil // Empty content for nil output
	}

	// If output is already []protocol.Content, return it directly
	if contentSlice, ok := output.([]protocol.Content); ok {
		return contentSlice, nil
	}

	// If output is a string, wrap it in TextContent
	if str, ok := output.(string); ok {
		return []protocol.Content{protocol.TextContent{Type: "text", Text: str}}, nil
	}

	// For other types (structs, maps, slices, etc.), attempt to JSON marshal and wrap in TextContent
	jsonBytes, err := json.Marshal(output)
	if err != nil {
		// If marshalling fails, return the error as text content
		errMsg := fmt.Sprintf("Error marshalling tool output: %v", err)
		return []protocol.Content{protocol.TextContent{Type: "text", Text: errMsg}}, fmt.Errorf("failed to marshal output to JSON: %w", err)
	}

	// Return marshalled JSON as text content
	return []protocol.Content{protocol.TextContent{Type: "text", Text: string(jsonBytes)}}, nil
}

// matchURITemplate attempts to match a URI against a URI template pattern (RFC 6570)
// using the yosida95/uritemplate library.
// Returns the extracted parameters and a boolean indicating if the URI matches the pattern.
func matchURITemplate(pattern, uri string) (map[string]string, bool) {
	tmpl, err := uritemplate.New(pattern)
	if err != nil {
		// Log the error, as an invalid pattern shouldn't be registered in the first place,
		// but handle defensively.
		log.Printf("Error parsing registered URI template pattern '%s': %v", pattern, err)
		return nil, false
	}

	values := tmpl.Match(uri)
	if values == nil {
		// No match
		return nil, false
	}

	// Convert uritemplate.Values (map[string]uritemplate.Value) to map[string]string
	extractedParams := make(map[string]string)
	for _, varname := range tmpl.Varnames() { // Iterate through expected varnames from template
		value := values.Get(varname)
		if value.Valid() { // Check if the parameter was actually present in the matched URI
			// For simplicity, we only handle string values here.
			// RFC 6570 supports list and map values, but our use case likely doesn't need them.
			if value.T == uritemplate.ValueTypeString {
				extractedParams[varname] = value.String()
			} else {
				log.Printf("Warning: Matched URI template parameter '%s' in '%s' is not a simple string (Type: %s). Skipping.", varname, uri, value.T.String())
			}
		}
	}

	// The library's Match() is successful if values is not nil.
	return extractedParams, true
}

// prepareHandlerArgs prepares the arguments for invoking a resource template handler.
// It uses reflection to map extracted URI parameters (matched by name) to the handler function's arguments
// and performs basic type conversion (string, int).
func prepareHandlerArgs(ctx *Context, templateInfo resourceTemplateInfo, extractedParams map[string]string) ([]reflect.Value, error) {
	handlerVal := reflect.ValueOf(templateInfo.HandlerFn) // Corrected field name
	handlerType := handlerVal.Type()

	if handlerType.Kind() != reflect.Func {
		return nil, fmt.Errorf("handler for %s is not a function", templateInfo.Pattern) // Use Pattern field
	}

	numArgs := handlerType.NumIn()
	if numArgs == 0 {
		return nil, fmt.Errorf("handler for %s must accept at least *server.Context", templateInfo.Pattern) // Use Pattern field
	}

	// Check first argument type
	firstArgType := handlerType.In(0)
	contextType := reflect.TypeOf((*Context)(nil))                   // *server.Context
	stdContextType := reflect.TypeOf((*context.Context)(nil)).Elem() // context.Context
	if firstArgType != contextType && firstArgType != stdContextType && firstArgType.Kind() != reflect.Interface {
		return nil, fmt.Errorf("handler for %s must accept context.Context, *server.Context, or interface{} as the first argument, got %s", templateInfo.Pattern, firstArgType.String())
	}

	// Check number of parameters matches expected (excluding context)
	// TODO: Consider optional parameters or different matching logic?
	if (numArgs - 1) != len(extractedParams) {
		return nil, fmt.Errorf("handler for %s expects %d parameters, but URI matched %d", templateInfo.Pattern, numArgs-1, len(extractedParams)) // Use Pattern field
	}

	args := make([]reflect.Value, numArgs)
	args[0] = reflect.ValueOf(ctx) // First argument is always context

	// Get parameter names from function definition (requires specific naming convention)
	// funcParamNames := getFunctionParamNames(handlerType) // REMOVED: Cannot reliably get names
	/* // REMOVED old logic using funcParamNames
	if len(funcParamNames) != numArgs {
		// This might happen if param names couldn't be reliably extracted, fallback or error?
		return nil, fmt.Errorf("could not reliably determine parameter names for handler %s", templateInfo.Pattern) // Use Pattern field
	}
	*/

	// Iterate through handler arguments (skip context)
	// Use templateInfo.Params which stores Name, HandlerIndex, HandlerType correctly from registration
	for _, paramInfo := range templateInfo.Params {
		argType := paramInfo.HandlerType       // Use type from stored info
		paramName := paramInfo.Name            // Use name from stored info
		handlerIndex := paramInfo.HandlerIndex // Use index from stored info

		uriValueStr, ok := extractedParams[paramName]
		if !ok {
			return nil, fmt.Errorf("handler for %s requires parameter '%s', but it was not found in the matched URI params: %v", templateInfo.Pattern, paramName, extractedParams) // Use Pattern field
		}

		// Convert the string value from URI to the required argument type
		convertedValue, err := convertParamValue(uriValueStr, argType)
		if err != nil {
			return nil, fmt.Errorf("error converting value '%s' for parameter '%s' (type %s) in handler %s: %w", uriValueStr, paramName, argType.String(), templateInfo.Pattern, err) // Use Pattern field
		}
		args[handlerIndex] = convertedValue
	}

	return args, nil
}

// --- Helper functions for prepareHandlerArgs ---
// /* // This multi-line comment start is removed
// getFunctionParamNames attempts to extract parameter names from a function type.
// WARNING: Standard Go reflection *cannot* reliably get parameter names.
// This requires specific conventions or potentially parsing source code/debug info,
// which is complex and brittle. For this example, we'll assume a simple convention
// or return placeholders indicating the limitation.
// A common workaround is to expect a struct as the second argument instead of individual params.
func getFunctionParamNames(funcType reflect.Type) []string {
	numArgs := funcType.NumIn()
	names := make([]string, numArgs)
	// Placeholder implementation - Cannot get names reliably via reflection.
	// Returning placeholders. Real implementation might require parsing or conventions.
	for i := 0; i < numArgs; i++ {
		names[i] = fmt.Sprintf("param%d", i) // e.g., param0, param1
	}
	// PROBLEM: How do we map param0, param1 to {city}, {user} from the URI?
	// This highlights the difficulty. A better approach might be needed.
	log.Printf("WARNING: getFunctionParamNames uses placeholders (param0, param1...). Mapping URI params relies on *order*, not name.")
	return names

	// Alternative (if we enforce a struct param): If numArgs == 2 and In(1) is a struct,
	// we could iterate through struct fields using reflection.
}

// convertParamValue converts a string URI parameter value to the target reflect.Type.
// Supports string and basic int conversion.
func convertParamValue(valueStr string, targetType reflect.Type) (reflect.Value, error) {
	switch targetType.Kind() {
	case reflect.String:
		return reflect.ValueOf(valueStr), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal, err := strconv.ParseInt(valueStr, 10, 64) // Parse as int64
		if err != nil {
			return reflect.Value{}, fmt.Errorf("invalid integer format: %w", err)
		}
		// Check for overflow before converting
		if targetType.OverflowInt(intVal) {
			return reflect.Value{}, fmt.Errorf("integer value '%s' overflows target type %s", valueStr, targetType.String())
		}
		return reflect.ValueOf(intVal).Convert(targetType), nil // Convert to specific int type
	// TODO: Add conversions for other types (bool, float, etc.) as needed
	default:
		return reflect.Value{}, fmt.Errorf("unsupported parameter type for conversion: %s", targetType.String())
	}
}

// */ // This multi-line comment end is removed

// convertHandlerResultToResourceContents converts the result(s) from a template handler
// into the []protocol.ResourceContents format.
// Assumes handler returns (result, error).
func convertHandlerResultToResourceContents(returnValues []reflect.Value) ([]protocol.ResourceContents, error) {
	if len(returnValues) != 2 {
		// This check might be too strict if handlers can omit the error return.
		// Consider adjusting if handlers with only one return value are allowed.
		return nil, fmt.Errorf("internal error: template handler did not return exactly 2 values (result, error)")
	}

	// Check the error return value (second value)
	errVal := returnValues[1]
	if !errVal.IsNil() {
		if err, ok := errVal.Interface().(error); ok {
			return nil, err // Return the error returned by the handler
		} else {
			// Should not happen if handler signature is correct (returns error)
			return nil, fmt.Errorf("internal error: handler's second return value is not nil but not an error type")
		}
	}

	// Process the result value (first value)
	resultVal := returnValues[0]
	resultIf := resultVal.Interface() // Get the underlying interface{}

	// Handle nil result
	if resultIf == nil || (resultVal.Kind() == reflect.Ptr && resultVal.IsNil()) {
		return []protocol.ResourceContents{}, nil // Empty contents for nil result
	}

	// Handle if result is already []protocol.ResourceContents
	if contents, ok := resultIf.([]protocol.ResourceContents); ok {
		return contents, nil
	}

	// Handle string result -> TextResourceContents
	if str, ok := resultIf.(string); ok {
		// TODO: Determine ContentType more dynamically? Defaulting to text/plain.
		return []protocol.ResourceContents{protocol.TextResourceContents{ContentType: "text/plain", Content: str}}, nil
	}

	// Handle []byte result -> BlobResourceContents (Base64 encoded)
	if bytes, ok := resultIf.([]byte); ok {
		encoded := base64.StdEncoding.EncodeToString(bytes)
		// TODO: Determine ContentType more dynamically? Defaulting to application/octet-stream.
		return []protocol.ResourceContents{protocol.BlobResourceContents{ContentType: "application/octet-stream", Blob: encoded}}, nil
	}

	// Handle other types (structs, maps, slices etc.) -> JSON marshalled into TextResourceContents
	jsonBytes, err := json.Marshal(resultIf)
	if err == nil {
		// TODO: Should JSON be its own ResourceContent type or always text?
		return []protocol.ResourceContents{protocol.TextResourceContents{ContentType: "application/json", Content: string(jsonBytes)}}, nil
	}

	// If no conversion worked, return an error
	return nil, fmt.Errorf("failed to convert handler result type %T to ResourceContents: %w", resultIf, err)
}

// handleCompletionComplete handles the `completion/complete` request for argument autocompletion.
// It now returns the standard `error` interface type.
func (mh *MessageHandler) handleCompletionComplete(ctx context.Context, rawParams json.RawMessage) (interface{}, error) {
	mh.server.logger.Debug("Handling completion/complete request")

	var params protocol.CompleteRequest
	if err := json.Unmarshal(rawParams, &params); err != nil {
		mh.server.logger.Error("Failed to unmarshal completion/complete params: %v", err)
		// Return an MCPError wrapped as a standard error
		return nil, protocol.NewInvalidParamsError(fmt.Sprintf("failed to parse params: %s", err.Error()))
	}

	mh.server.logger.Debug("Completion requested for ref type '%s' (Name: '%s', URI: '%s'), argument '%s' with value '%s'",
		params.Ref.Type, params.Ref.Name, params.Ref.URI, params.Argument.Name, params.Argument.Value)

	// --- Actual Completion Logic ---
	suggestions := []string{}
	partialValue := params.Argument.Value

	switch params.Ref.Type {
	case protocol.RefTypePrompt:
		// Suggest matching prompt names/titles
		allPrompts := mh.server.Registry.GetPrompts()
		for _, p := range allPrompts {
			// Match against title (or URI if title is empty?)
			matchTarget := p.Title
			if matchTarget == "" {
				matchTarget = p.URI // Fallback to URI if title empty
			}
			if strings.HasPrefix(strings.ToLower(matchTarget), strings.ToLower(partialValue)) {
				suggestions = append(suggestions, matchTarget)
			}
		}

	case protocol.RefTypeResource:
		// Suggest matching resource URIs
		allResources := mh.server.Registry.ResourceRegistry()
		for uri := range allResources {
			if strings.HasPrefix(strings.ToLower(uri), strings.ToLower(partialValue)) {
				suggestions = append(suggestions, uri)
			}
		}
		// TODO: Potentially add matching for resource templates as well?

	default:
		mh.server.logger.Warn("Unsupported completion reference type: %s", params.Ref.Type)
		// Return empty results for unsupported types
	}

	// Sort suggestions alphabetically
	sort.Strings(suggestions)

	// Limit results and determine HasMore
	const maxResults = 100
	totalMatches := len(suggestions)
	hasMore := false
	if totalMatches > maxResults {
		suggestions = suggestions[:maxResults]
		hasMore = true
	}

	// --- Populate Result ---
	result := protocol.CompleteResult{
		Completion: protocol.Completion{
			Values:  suggestions,
			Total:   protocol.IntPtr(totalMatches),
			HasMore: protocol.BoolPtr(hasMore),
		},
	}

	mh.server.logger.Debug("completion/complete handled, returning %d suggestions (total: %d, hasMore: %t)", len(suggestions), totalMatches, hasMore)
	return result, nil // Return nil for the error interface
}

// // handleCompletionCompleteV2024 simulates handling the OLD `completion/complete` request (V2024 spec)
// // This was previously misinterpreted as LLM generation, but is now understood as argument completion.
// // Keeping this commented out for reference, but the unified handleCompletionComplete should be used.
// func (mh *MessageHandler) handleCompletionCompleteV2024(ctx context.Context, rawParams json.RawMessage) (interface{}, *protocol.MCPError) {
// 	mh.server.logger.Debug("Handling V2024 completion/complete request (Simulation)")

// 	// Check capability
// 	if !mh.server.ImplementsCompletions {
// 		return nil, protocol.NewMethodNotFoundError("completion/complete (V2024)")
// 	}

// 	// --- Simulate V2024 Completion --- // This logic is WRONG for arg completion
// 	// Parse params (V2024 CreateMessageRequestParams)
// 	var paramsV2024 protocol.CreateMessageRequestParams
// 	if err := json.Unmarshal(rawParams, &paramsV2024); err != nil {
// 		mh.server.logger.Error("Failed to unmarshal V2024 completion/complete params: %v", err)
// 		return nil, protocol.NewInvalidParamsError(fmt.Sprintf("failed to parse V2024 params: %s", err.Error()))
// 	}

// 	// Simulate a simple response
// 	simulatedContent := "Simulated V2024 completion response."
// if len(paramsV2024.Context) > 0 && len(paramsV2024.Context[0].Content) > 0 {
// 		if tc, ok := paramsV2024.Context[0].Content[0].(protocol.TextContent); ok {
// 			simulatedContent = fmt.Sprintf("Simulated V2024 response to: %s", tc.Text)
// 		}
// 	}

// 	// Construct V2024 result
// 	result := protocol.CreateMessageResult{
// 		Message: protocol.SamplingMessage{
// 			Role: "assistant",
// 			Content: []protocol.Content{
// 				protocol.TextContent{Type: "text", Text: simulatedContent},
// 			},
// 		},
// 		// Model: protocol.StringPtr("simulator-v2024"), // Optional
// 		// StopReason: protocol.StringPtr("simulated_stop"), // Optional
// 	}

// 	mh.server.logger.Debug("V2024 completion/complete handled successfully (simulation)")
// 	return result, nil
// }

// handleReadResource handles the `resources/read` request.
// ... existing code ...
