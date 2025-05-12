package server

import (
	"context" // Import context
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log" // Import net/url
	"net/http"
	"os" // Import os
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	// Correct import path
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/types"

	"github.com/localrivet/wilduri"
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
			// InitializeHandler returns result, caps, error
			var initResult protocol.InitializeResult
			var clientCaps *protocol.ClientCapabilities

			initResult, clientCaps, err = mh.lifecycleHandler.InitializeHandler(params)

			if err == nil {
				// Store the negotiated version in the session
				mh.server.logger.Info("Setting negotiated protocol version for session %s: %s",
					session.SessionID(), initResult.ProtocolVersion)
				session.SetNegotiatedVersion(initResult.ProtocolVersion)
				// Verify the version was set
				mh.server.logger.Debug("Verified negotiated protocol version for session %s: %s",
					session.SessionID(), session.GetNegotiatedVersion())

				// Also store the client capabilities
				if clientCaps != nil {
					session.StoreClientCapabilities(*clientCaps)
				}

				// Mark the session as initialized
				session.Initialize()
			}

			result = initResult
		}

	case protocol.MethodShutdown:
		// Handle shutdown request
		// No parameters expected for this request
		err = mh.lifecycleHandler.ShutdownHandler()

	case protocol.MethodListTools: // Use constant instead of hardcoded string
		// Handle the 'tools/list' request. Client requests a list of available tools.
		// Get the list of registered tools from the registry
		tools := mh.server.Registry.GetTools()

		// Format the result into a protocol.ListToolsResult
		result = &protocol.ListToolsResult{
			Tools: tools,
			// NextCursor is optional, leaving empty for now
		}
		mh.server.logger.Debug("Responding with %d tools for %s request", len(tools), protocol.MethodListTools)

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
			mh.server.logger.Debug("Using negotiated protocol version for session %s: %s",
				session.SessionID(), negotiatedVersion)

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

		// Get the resource from the registry using the single-item getter
		resource, resourceFound := mh.server.Registry.GetResource(params.URI)

		// --- Logic for handling resource read ---
		var contents []protocol.ResourceContents
		var finalResource protocol.Resource // Store the resource/template info for the result
		var readErr error                   // Use a separate error variable for reading/template execution

		if resourceFound {
			// --- Handle Static Resource ---
			mh.server.logger.Debug("Found static resource for URI: %s", params.URI)
			finalResource = resource // Use the found static resource

			// Retrieve the internal registered resource to access the config
			regResource, _ := mh.server.Registry.GetRegisteredResource(params.URI)
			if regResource == nil {
				// This shouldn't happen - internal inconsistency
				readErr = &protocol.MCPError{
					ErrorPayload: protocol.ErrorPayload{
						Code:    protocol.CodeInternalError,
						Message: "Internal error: Resource found but internal representation missing",
					},
				}
			} else {
				// Generate content based on the resource config
				config := regResource.config

				// Handle different content sources in order of priority
				switch {
				case config.Content != nil:
					// Handle direct content (text or binary)
					switch content := config.Content.(type) {
					case string:
						// Text content
						contentType := config.MimeType
						if contentType == "" {
							contentType = "text/plain; charset=utf-8"
						}
						contents = []protocol.ResourceContents{
							protocol.TextResourceContents{ContentType: contentType, Text: content, URI: params.URI},
						}
					case []byte:
						// Binary content
						contentType := config.MimeType
						if contentType == "" {
							contentType = "application/octet-stream"
						}
						encodedContent := base64.StdEncoding.EncodeToString(content)
						contents = []protocol.ResourceContents{
							protocol.BlobResourceContents{ContentType: contentType, Blob: encodedContent, URI: params.URI},
						}
					default:
						readErr = &protocol.MCPError{
							ErrorPayload: protocol.ErrorPayload{
								Code:    protocol.CodeInternalError,
								Message: fmt.Sprintf("Unsupported content type: %T", content),
							},
						}
					}

				case config.FilePath != "":
					// Handle file path content
					filePath := config.FilePath
					data, err := os.ReadFile(filePath)
					if err != nil {
						readErr = &protocol.MCPError{
							ErrorPayload: protocol.ErrorPayload{
								Code:    protocol.CodeMCPOperationFailed,
								Message: fmt.Sprintf("Failed to read file: %v", err),
							},
						}
						break
					}

					contentType := config.MimeType
					if contentType == "" {
						// Try to infer MIME type from file extension or content
						contentType = inferMimeTypeFromFile(filePath, data)
					}

					// Determine resource kind from file extension or MIME type
					isAudioFile := strings.HasSuffix(filePath, ".mp3") ||
						strings.HasSuffix(filePath, ".wav") ||
						strings.HasSuffix(filePath, ".ogg") ||
						strings.HasSuffix(filePath, ".audio") || // Special test extension
						strings.HasPrefix(contentType, "audio/")

					// Check for binary blob content type
					isBlobFile := strings.HasSuffix(filePath, ".blob") || // Special test extension
						contentType == "application/octet-stream" ||
						contentType == "application/binary"

					// Decide content type based on resource kind
					if isAudioFile {
						// Audio content
						encodedContent := base64.StdEncoding.EncodeToString(data)
						contents = []protocol.ResourceContents{
							protocol.AudioResourceContents{ContentType: contentType, Audio: encodedContent, URI: params.URI},
						}
					} else if isBlobFile {
						// Binary blob content
						encodedContent := base64.StdEncoding.EncodeToString(data)
						contents = []protocol.ResourceContents{
							protocol.BlobResourceContents{ContentType: contentType, Blob: encodedContent, URI: params.URI},
						}
					} else if strings.HasPrefix(contentType, "text/") ||
						contentType == "application/json" ||
						contentType == "application/xml" {
						// Treat as text
						contents = []protocol.ResourceContents{
							protocol.TextResourceContents{ContentType: contentType, Text: string(data), URI: params.URI},
						}
					} else {
						// Treat as binary
						encodedContent := base64.StdEncoding.EncodeToString(data)
						contents = []protocol.ResourceContents{
							protocol.BlobResourceContents{ContentType: contentType, Blob: encodedContent, URI: params.URI},
						}
					}

				case config.DirPath != "":
					// Handle directory listing
					dirPath := config.DirPath
					listing, err := generateDirectoryListing(dirPath)
					if err != nil {
						readErr = &protocol.MCPError{
							ErrorPayload: protocol.ErrorPayload{
								Code:    protocol.CodeMCPOperationFailed,
								Message: fmt.Sprintf("Failed to list directory: %v", err),
							},
						}
						break
					}

					// Convert to JSON and return as text resource
					listingJSON, err := json.Marshal(listing)
					if err != nil {
						readErr = &protocol.MCPError{
							ErrorPayload: protocol.ErrorPayload{
								Code:    protocol.CodeInternalError,
								Message: fmt.Sprintf("Failed to serialize directory listing: %v", err),
							},
						}
						break
					}

					contents = []protocol.ResourceContents{
						protocol.TextResourceContents{
							ContentType: "application/json; charset=utf-8",
							Text:        string(listingJSON),
							URI:         params.URI,
						},
					}

				case config.URL != "":
					// Handle URL content (external fetch)
					urlContent, err := fetchURLContent(config.URL)
					if err != nil {
						readErr = &protocol.MCPError{
							ErrorPayload: protocol.ErrorPayload{
								Code:    protocol.CodeMCPOperationFailed,
								Message: fmt.Sprintf("Failed to fetch URL content: %v", err),
							},
						}
						break
					}

					// Get content type from response or config
					contentType := urlContent.contentType
					if config.MimeType != "" {
						contentType = config.MimeType // Override with explicit config
					}

					// Determine if it's text or binary
					if isTextContent(contentType) {
						contents = []protocol.ResourceContents{
							protocol.TextResourceContents{
								ContentType: contentType,
								Text:        string(urlContent.data),
								URI:         params.URI,
							},
						}
					} else {
						encodedContent := base64.StdEncoding.EncodeToString(urlContent.data)
						contents = []protocol.ResourceContents{
							protocol.BlobResourceContents{
								ContentType: contentType,
								Blob:        encodedContent,
								URI:         params.URI,
							},
						}
					}

				default:
					// No content source specified
					readErr = &protocol.MCPError{
						ErrorPayload: protocol.ErrorPayload{
							Code:    protocol.CodeMCPOperationFailed,
							Message: "No content source defined for static resource",
						},
					}
				}
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
					handerArgs, prepErr := prepareHandlerArgs(handlerCtx, *templateInfo, matchResult)
					if prepErr != nil {
						readErr = prepErr
						break // Exit template loop on prep error
					}

					// Call handler function via reflection
					handerVal := reflect.ValueOf(templateInfo.HandlerFn)
					returnValues := handerVal.Call(handerArgs)

					// Convert return values to protocol.Content
					contents, readErr = convertHandlerResultToResourceContents(returnValues, params.URI)
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
		// Handle resources/templates/list request
		// No parameters expected
		log.Printf("Received resources/templates/list request from session %s", session.SessionID())
		// TODO: Implement actual template retrieval from registry when that's added
		templates := make([]protocol.ResourceTemplate, 0) // Return empty list for now
		result = protocol.ListResourceTemplatesResult{
			ResourceTemplates: templates,
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

		// Try to get by URI first
		uri := params.URI

		// If name is provided in params (from 2025 schema), use it as fallback
		if uri == "" && params.Name != "" {
			uri = params.Name
		}

		log.Printf("Received prompts/get request from session %s for URI: %s", session.SessionID(), uri)

		// Debug log all registered prompts to help diagnose issues
		prompts := mh.server.Registry.GetPrompts()
		log.Printf("DEBUG: Currently registered prompts (%d total):", len(prompts))
		for i, p := range prompts {
			log.Printf("DEBUG: Prompt %d - URI: '%s', Name: '%s'", i, p.URI, p.Name)
		}

		prompt, ok := mh.server.Registry.GetPrompt(uri)
		if !ok {
			err = &protocol.MCPError{
				ErrorPayload: protocol.ErrorPayload{
					Code:    protocol.CodeMCPResourceNotFound,
					Message: fmt.Sprintf("Prompt not found: %s", uri),
				},
			}
			break
		}

		log.Printf("Found prompt: URI='%s', Name='%s', MessageCount=%d", prompt.URI, prompt.Name, len(prompt.Messages))

		// Format the result in schema-compliant format
		result = protocol.GetPromptResult{
			Messages:    prompt.Messages,
			Description: prompt.Description,
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

// matchURITemplate attempts to match a URI against a URI template pattern
// using the wilduri library.
// Returns the extracted parameters and a boolean indicating if the URI matches the pattern.
func matchURITemplate(pattern, uri string) (map[string]string, bool) {
	// Use wilduri library pattern matching which already supports wildcards
	tmpl, err := wilduri.New(pattern)
	if err != nil {
		// Log the error, as an invalid pattern shouldn't be registered in the first place,
		// but handle defensively.
		log.Printf("Error parsing registered URI template pattern '%s': %v", pattern, err)
		return nil, false
	}

	// wilduri.Match returns Values (map[string]interface{})
	values, matched := tmpl.Match(uri)
	if !matched || values == nil {
		// No match
		return nil, false
	}

	// Convert wilduri.Values (map[string]interface{}) to map[string]string
	extractedParams := make(map[string]string)
	for _, varname := range tmpl.Varnames() { // Iterate through expected varnames from template
		if val, ok := values[varname]; ok && val != nil { // Check if the parameter was actually present in the matched URI
			// Convert to string - wilduri handles extraction of different types
			extractedParams[varname] = fmt.Sprintf("%v", val)
		}
	}

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

	// Create argument array for the handler function
	args := make([]reflect.Value, numArgs)
	args[0] = reflect.ValueOf(ctx) // First argument is always context

	// Get default parameter values from the config
	defaultValues := templateInfo.config.defaultParamValues
	if defaultValues == nil {
		defaultValues = make(map[string]interface{})
	}

	// Iterate through handler arguments (skip context)
	// Use templateInfo.Params which stores Name, HandlerIndex, HandlerType correctly from registration
	for _, paramInfo := range templateInfo.Params {
		argType := paramInfo.HandlerType       // Use type from stored info
		paramName := paramInfo.Name            // Use name from stored info
		handlerIndex := paramInfo.HandlerIndex // Use index from stored info

		// Get the parameter value, using either the extracted value or default
		uriValueStr, ok := extractedParams[paramName]
		var valueToConvert interface{}
		if !ok || uriValueStr == "" {
			// Parameter not found in URI, try to use default
			defaultValue, hasDefault := defaultValues[paramName]
			if !hasDefault {
				return nil, fmt.Errorf("handler for %s requires parameter '%s', but it was not found in the matched URI params and no default was provided", templateInfo.Pattern, paramName)
			}
			valueToConvert = defaultValue
		} else {
			valueToConvert = uriValueStr
		}

		// Convert the value to the required argument type
		convertedValue, err := convertParamValue(valueToConvert, argType)
		if err != nil {
			return nil, fmt.Errorf("error converting value '%v' for parameter '%s' (type %s) in handler %s: %w", valueToConvert, paramName, argType.String(), templateInfo.Pattern, err)
		}
		args[handlerIndex] = convertedValue
	}

	return args, nil
}

// convertParamValue converts a parameter value to the target reflect.Type.
// Supports string, int, bool, float, and slice conversions.
func convertParamValue(value interface{}, targetType reflect.Type) (reflect.Value, error) {
	// If value is already of target type or can be directly assigned, return it
	if value != nil && reflect.TypeOf(value).AssignableTo(targetType) {
		return reflect.ValueOf(value), nil
	}

	// For nil value or empty interface, return zero value of target type
	if value == nil {
		return reflect.Zero(targetType), nil
	}

	// Convert string value to target type
	if strValue, isStr := value.(string); isStr {
		switch targetType.Kind() {
		case reflect.String:
			return reflect.ValueOf(strValue), nil
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			intVal, err := strconv.ParseInt(strValue, 10, 64) // Parse as int64
			if err != nil {
				return reflect.Value{}, fmt.Errorf("invalid integer format: %w", err)
			}
			// Check for overflow before converting
			if targetType.OverflowInt(intVal) {
				return reflect.Value{}, fmt.Errorf("integer value '%s' overflows target type %s", strValue, targetType.String())
			}
			return reflect.ValueOf(intVal).Convert(targetType), nil // Convert to specific int type
		case reflect.Float32, reflect.Float64:
			floatVal, err := strconv.ParseFloat(strValue, 64)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("invalid float format: %w", err)
			}
			// Check for overflow
			if targetType.OverflowFloat(floatVal) {
				return reflect.Value{}, fmt.Errorf("float value '%s' overflows target type %s", strValue, targetType.String())
			}
			return reflect.ValueOf(floatVal).Convert(targetType), nil
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(strValue)
			if err != nil {
				return reflect.Value{}, fmt.Errorf("invalid boolean format: %w", err)
			}
			return reflect.ValueOf(boolVal), nil
		case reflect.Slice:
			// If target is []string, split comma-separated string
			if targetType.Elem().Kind() == reflect.String {
				parts := strings.Split(strValue, ",")
				sliceVal := reflect.MakeSlice(targetType, len(parts), len(parts))
				for i, part := range parts {
					sliceVal.Index(i).SetString(part)
				}
				return sliceVal, nil
			}
		}
	}

	// Try to convert between numeric types
	switch reflect.TypeOf(value).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		intVal := reflect.ValueOf(value).Int()
		switch targetType.Kind() {
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if targetType.OverflowInt(intVal) {
				return reflect.Value{}, fmt.Errorf("integer value %d overflows target type %s", intVal, targetType.String())
			}
			return reflect.ValueOf(intVal).Convert(targetType), nil
		case reflect.Float32, reflect.Float64:
			floatVal := float64(intVal)
			if targetType.OverflowFloat(floatVal) {
				return reflect.Value{}, fmt.Errorf("float value %f overflows target type %s", floatVal, targetType.String())
			}
			return reflect.ValueOf(floatVal).Convert(targetType), nil
		}
	case reflect.Float32, reflect.Float64:
		floatVal := reflect.ValueOf(value).Float()
		switch targetType.Kind() {
		case reflect.Float32, reflect.Float64:
			if targetType.OverflowFloat(floatVal) {
				return reflect.Value{}, fmt.Errorf("float value %f overflows target type %s", floatVal, targetType.String())
			}
			return reflect.ValueOf(floatVal).Convert(targetType), nil
		}
	}

	return reflect.Value{}, fmt.Errorf("unsupported conversion from %T to %s", value, targetType.String())
}

// convertHandlerResultToResourceContents converts the result(s) from a template handler
// into the []protocol.ResourceContents format.
// Assumes handler returns (result, error).
func convertHandlerResultToResourceContents(returnValues []reflect.Value, uri string) ([]protocol.ResourceContents, error) {
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
		// If any content is missing URI, add it
		for i, content := range contents {
			if textContent, ok := content.(protocol.TextResourceContents); ok {
				if textContent.URI == "" {
					textContent.URI = uri
					contents[i] = textContent
				}
			} else if blobContent, ok := content.(protocol.BlobResourceContents); ok {
				if blobContent.URI == "" {
					blobContent.URI = uri
					contents[i] = blobContent
				}
			} else if audioContent, ok := content.(protocol.AudioResourceContents); ok {
				if audioContent.URI == "" {
					audioContent.URI = uri
					contents[i] = audioContent
				}
			}
		}
		return contents, nil
	}

	// Handle string result -> TextResourceContents
	if str, ok := resultIf.(string); ok {
		// TODO: Determine ContentType more dynamically? Defaulting to text/plain.
		return []protocol.ResourceContents{protocol.TextResourceContents{
			ContentType: "text/plain",
			Text:        str,
			URI:         uri, // Use provided URI
		}}, nil
	}

	// Handle []byte result -> BlobResourceContents (Base64 encoded)
	if bytes, ok := resultIf.([]byte); ok {
		encoded := base64.StdEncoding.EncodeToString(bytes)
		// TODO: Determine ContentType more dynamically? Defaulting to application/octet-stream.
		return []protocol.ResourceContents{protocol.BlobResourceContents{
			ContentType: "application/octet-stream",
			Blob:        encoded,
			URI:         uri, // Use provided URI
		}}, nil
	}

	// Handle other types (structs, maps, slices etc.) -> JSON marshalled into TextResourceContents
	jsonBytes, err := json.MarshalIndent(resultIf, "", "  ")
	if err == nil {
		// TODO: Should JSON be its own ResourceContent type or always text?
		return []protocol.ResourceContents{protocol.TextResourceContents{
			ContentType: "application/json",
			Text:        string(jsonBytes),
			URI:         uri, // Use provided URI
		}}, nil
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
		// Suggest matching prompt names
		allPrompts := mh.server.Registry.GetPrompts()
		for _, p := range allPrompts {
			// Match against name (or URI if name is empty)
			matchTarget := p.Name
			if matchTarget == "" {
				matchTarget = p.URI // Fallback to URI if name empty
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

// Helper Functions for Resource Content Handling

// inferMimeTypeFromFile attempts to determine the MIME type of a file based on its extension and/or content
func inferMimeTypeFromFile(filePath string, data []byte) string {
	// Check file extension first
	ext := strings.ToLower(filepath.Ext(filePath))
	switch ext {
	case ".txt":
		return "text/plain; charset=utf-8"
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".pdf":
		return "application/pdf"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".mp3":
		return "audio/mpeg"
	case ".mp4":
		return "video/mp4"
	case ".wav":
		return "audio/wav"
	case ".zip":
		return "application/zip"
	case ".md":
		return "text/markdown; charset=utf-8"
	}

	// TODO: Consider content-based detection for more accurate type inference
	// For now, default to a safe type
	return "application/octet-stream"
}

// isTextContent determines if a MIME type likely represents text content
func isTextContent(mimeType string) bool {
	// Check for text-based MIME types
	if strings.HasPrefix(mimeType, "text/") {
		return true
	}

	// Check for specific text-based application types
	textApplicationTypes := []string{
		"application/json",
		"application/xml",
		"application/javascript",
		"application/xhtml+xml",
		"application/x-www-form-urlencoded",
	}

	for _, textType := range textApplicationTypes {
		if strings.HasPrefix(mimeType, textType) {
			return true
		}
	}

	return false
}

// directoryEntry represents a single item in a directory listing
type directoryEntry struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	IsDir   bool   `json:"is_dir"`
	Size    int64  `json:"size,omitempty"`
	ModTime string `json:"mod_time,omitempty"`
}

// generateDirectoryListing creates a structured listing of files in a directory
func generateDirectoryListing(dirPath string) ([]directoryEntry, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	listing := make([]directoryEntry, 0, len(entries))
	for _, entry := range entries {
		// Get file info for size and modification time
		info, err := entry.Info()
		if err != nil {
			// Skip entries we can't get info for
			continue
		}

		entryPath := filepath.Join(dirPath, entry.Name())
		relativePath, err := filepath.Rel(dirPath, entryPath)
		if err != nil {
			relativePath = entry.Name() // Fallback to just the name
		}

		listEntry := directoryEntry{
			Name:    entry.Name(),
			Path:    relativePath,
			IsDir:   entry.IsDir(),
			Size:    info.Size(),
			ModTime: info.ModTime().Format(time.RFC3339),
		}

		listing = append(listing, listEntry)
	}

	return listing, nil
}

// urlContent holds the result of a URL fetch operation
type urlContent struct {
	data        []byte
	contentType string
}

// fetchURLContent retrieves content from a URL
func fetchURLContent(urlStr string) (*urlContent, error) {
	// Use the http package to fetch the URL
	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check for successful response
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	// Read the response body
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Extract the content type from the response headers
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream" // Default if not specified
	}

	return &urlContent{
		data:        data,
		contentType: contentType,
	}, nil
}
