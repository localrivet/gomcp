package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
	"github.com/sashabaranov/go-openai"
)

// ChatMessage represents a message in the chat history
type ChatMessage struct {
	Role      string
	Content   string
	Timestamp time.Time
}

// ToolCall represents a tool call made by the LLM
type ToolCall struct {
	Name      string
	Arguments map[string]interface{}
	Response  string
}

// Global MCP client that can be accessed from HTTP handlers
var mcpClient *client.MCP
var mcpClientMutex sync.Mutex

func main() {
	// Load the MCP server configuration
	configPath := "chatgpt-config.json"

	// Set up the OpenAI client
	openaiClient := openai.NewClient(getOpenAIKey())

	// Initialize chat history and shared variables
	chatHistory := []ChatMessage{}
	var chatMutex sync.Mutex
	var availableTools = make(map[string][]protocol.Tool)
	var toolsMutex sync.Mutex
	var availableResources = make(map[string][]protocol.Resource)
	var resourcesMutex sync.Mutex

	// Set up web server handlers
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		chatMutex.Lock()
		history := make([]ChatMessage, len(chatHistory))
		copy(history, chatHistory)
		chatMutex.Unlock()

		tmpl := template.Must(template.ParseFiles("index.html"))
		tmpl.Execute(w, map[string]interface{}{
			"History": history,
		})
	})

	http.HandleFunc("/chat", func(w http.ResponseWriter, r *http.Request) {
		handleChatRequest(w, r, &chatHistory, &chatMutex, &availableTools, &toolsMutex, openaiClient)
	})

	// Start the web server
	port := 3366 // Using port 3366 as requested
	fmt.Printf("Starting chatbot server on http://localhost:%d\n", port)

	go func() {
		err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
		if err != nil {
			log.Fatalf("Error starting web server: %v", err)
		}
	}()

	// Start the MCP client in a separate goroutine
	go runMCPClient(configPath, &availableTools, &toolsMutex, &availableResources, &resourcesMutex)

	// Add a debugging goroutine to check connection status periodically
	go func() {
		mcpClientMutex.Lock()
		if mcpClient != nil {
			mcpClient.CheckConnectionStatus()
		} else {
			fmt.Println("MCP client not initialized yet")
		}
		mcpClientMutex.Unlock()
	}()

	// Keep the main goroutine running
	select {}
}

// handleChatRequest processes incoming chat messages and interacts with the OpenAI and MCP clients.
func handleChatRequest(w http.ResponseWriter, r *http.Request, chatHistory *[]ChatMessage, chatMutex *sync.Mutex, availableTools *map[string][]protocol.Tool, toolsMutex *sync.Mutex, openaiClient *openai.Client) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Get user message
	userMessage := r.FormValue("message")
	if userMessage == "" {
		http.Error(w, "Empty message", http.StatusBadRequest)
		return
	}

	// Add user message to history
	chatMutex.Lock()
	*chatHistory = append(*chatHistory, ChatMessage{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})
	chatMutex.Unlock()

	// Check if tools are available
	toolsMutex.Lock()
	tools := make(map[string][]protocol.Tool)
	for k, v := range *availableTools {
		tools[k] = v
	}
	toolsMutex.Unlock()

	if len(tools) == 0 {
		// No tools available yet, add a message to the history
		chatMutex.Lock()
		*chatHistory = append(*chatHistory, ChatMessage{
			Role:      "assistant",
			Content:   "I'm still initializing and discovering tools. Please try again in a moment.",
			Timestamp: time.Now(),
		})
		chatMutex.Unlock()

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Convert chat history to OpenAI message format
	chatMutex.Lock()
	messages := formatChatHistoryForOpenAI(*chatHistory)
	chatMutex.Unlock()

	// Add tool information to system message
	systemMessage := createSystemMessageWithTools(tools)
	messages = append([]openai.ChatCompletionMessage{systemMessage}, messages...)

	// Create OpenAI chat completion request with tools
	chatRequest := openai.ChatCompletionRequest{
		Model:    "gpt-4.1-nano",
		Messages: messages,
		Tools:    createOpenAITools(tools),
	}

	// Call OpenAI API
	resp, err := openaiClient.CreateChatCompletion(context.Background(), chatRequest)
	if err != nil {
		log.Printf("Error calling OpenAI API: %v", err)

		chatMutex.Lock()
		*chatHistory = append(*chatHistory, ChatMessage{
			Role:      "assistant",
			Content:   "Sorry, I encountered an error while processing your message. Please try again.",
			Timestamp: time.Now(),
		})
		chatMutex.Unlock()

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Process the response
	aiMessage := resp.Choices[0].Message

	// Add AI response to history
	chatMutex.Lock()
	*chatHistory = append(*chatHistory, ChatMessage{
		Role:      "assistant",
		Content:   aiMessage.Content,
		Timestamp: time.Now(),
	})
	chatMutex.Unlock()

	// Check if the model wants to use tools
	if len(aiMessage.ToolCalls) > 0 {
		// Add the assistant's message with tool_calls to the messages for the follow-up
		// This is critical - tool responses must follow a message with tool_calls
		messages = append(messages, openai.ChatCompletionMessage{
			Role:      "assistant",
			Content:   aiMessage.Content,
			ToolCalls: aiMessage.ToolCalls,
		})

		// Process each tool call
		for _, toolCall := range aiMessage.ToolCalls {
			toolName := toolCall.Function.Name

			// Parse the arguments
			var args map[string]interface{}
			if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
				log.Printf("Error parsing tool arguments: %v", err)
				continue
			}

			// Find the server that has this tool
			var serverName string
			for name, serverTools := range tools {
				for _, tool := range serverTools {
					if tool.Name == toolName {
						serverName = name
						break
					}
				}
				if serverName != "" {
					break
				}
			}

			if serverName == "" {
				log.Printf("Tool %s not found in any MCP server", toolName)
				continue
			}

			// Call the tool using MCP
			log.Printf("Calling tool %s on server %s with args %v", toolName, serverName, args)

			// Access the global MCP client with lock protection
			mcpClientMutex.Lock()
			var result []protocol.Content
			var err error
			if mcpClient != nil {
				result, err = mcpClient.CallTool(serverName, toolName, args)
			} else {
				err = fmt.Errorf("MCP client not initialized")
			}
			mcpClientMutex.Unlock()

			// Format the tool response
			var toolResponse string
			if err != nil {
				toolResponse = fmt.Sprintf("Error calling tool %s: %v", toolName, err)
				log.Println(toolResponse)
			} else {
				toolResponse = extractToolResponse(result)
			}

			// Add tool call to history
			chatMutex.Lock()
			*chatHistory = append(*chatHistory, ChatMessage{
				Role:      "tool",
				Content:   fmt.Sprintf("Tool: %s\nArgs: %v\nResult: %s", toolName, args, toolResponse),
				Timestamp: time.Now(),
			})
			chatMutex.Unlock()

			// Add the tool response to OpenAI messages for follow-up
			messages = append(messages, openai.ChatCompletionMessage{
				Role:       "tool",
				Content:    toolResponse,
				ToolCallID: toolCall.ID,
			})
		}

		// Get follow-up response after tool usage
		followupRequest := openai.ChatCompletionRequest{
			Model:    "gpt-4.1-nano",
			Messages: messages,
			Tools:    createOpenAITools(tools),
		}

		followupResp, err := openaiClient.CreateChatCompletion(context.Background(), followupRequest)
		if err != nil {
			log.Printf("Error getting follow-up response: %v", err)

			// Add error message to chat history
			chatMutex.Lock()
			*chatHistory = append(*chatHistory, ChatMessage{
				Role:      "system",
				Content:   fmt.Sprintf("Error getting AI interpretation of tool results: %v", err),
				Timestamp: time.Now(),
			})
			chatMutex.Unlock()
		} else {
			// Add follow-up response to history
			followupMessage := followupResp.Choices[0].Message.Content

			// Clear formatting to make it obvious this is the AI's interpretation of tool results
			if followupMessage == "" {
				followupMessage = "I processed the tool results but have no additional information to add."
			}

			log.Printf("AI interpretation of tool results: %s", followupMessage)

			chatMutex.Lock()
			*chatHistory = append(*chatHistory, ChatMessage{
				Role:      "assistant",
				Content:   followupMessage,
				Timestamp: time.Now(),
			})
			chatMutex.Unlock()

			// Check if there are more tool calls in the follow-up response
			if len(followupResp.Choices[0].Message.ToolCalls) > 0 {
				log.Printf("AI wants to call more tools - this is not currently supported in a single interaction")

				chatMutex.Lock()
				*chatHistory = append(*chatHistory, ChatMessage{
					Role:      "system",
					Content:   "The AI requested additional tool calls. Please submit a new message to continue the conversation.",
					Timestamp: time.Now(),
				})
				chatMutex.Unlock()
			}
		}
	}

	// Redirect back to the main page to show the updated chat
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// runMCPClient initializes and connects to the MCP server in a background goroutine
func runMCPClient(configPath string, availableTools *map[string][]protocol.Tool, toolsMutex *sync.Mutex, availableResources *map[string][]protocol.Resource, resourcesMutex *sync.Mutex) {
	// Set up the MCP client for accessing tools
	log.Printf("Loading MCP config from %s", configPath)
	mcpConfig, err := client.LoadFromFile(configPath, nil)
	if err != nil {
		log.Printf("Failed to load MCP config: %v", err)
		return
	}

	fmt.Printf("mcpConfig: %+v\n", mcpConfig)

	// Create the client with custom options
	log.Printf("Creating MCP client...")
	mcp, err := client.New(mcpConfig,
		client.WithTimeout(60*time.Second), // Longer timeout for API calls
	)
	if err != nil {
		log.Printf("Failed to create MCP client: %v", err)
		return
	}

	// Register connection status handler to discover tools immediately when connected
	log.Printf("Setting up connection status handler for immediate tool discovery...")
	for serverName, serverClient := range mcp.Servers {
		// Create a closure to capture the server name
		currentServer := serverName
		serverClient.OnConnectionStatus(func(connected bool) error {
			if connected {
				log.Printf("Server %s connected! Discovering tools immediately...", currentServer)
				tools, err := serverClient.ListTools(context.Background())
				if err != nil {
					log.Printf("Error listing tools for server %s: %v", currentServer, err)
					return nil // Don't propagate the error to avoid disconnecting
				}

				if len(tools) > 0 {
					log.Printf("Server %s has %d tools:", currentServer, len(tools))

					toolsMutex.Lock()
					(*availableTools)[currentServer] = tools

					// Log each tool
					for _, tool := range tools {
						log.Printf("- %s: %s %+v", tool.Name, tool.Description, tool.InputSchema)

					}
					toolsMutex.Unlock()
				} else {
					log.Printf("No tools found for server %s", currentServer)
				}

				resources, err := serverClient.ListResources(context.Background())
				if err != nil {
					log.Printf("Error listing resources for server %s: %v", currentServer, err)
				} else {
					log.Printf("Server %s has %d resources:", currentServer, len(resources))
					(*availableResources)[currentServer] = resources
				}
			} else {
				log.Printf("Server %s disconnected", currentServer)
			}
			return nil
		})
	}

	// Store the MCP client in the global variable
	mcpClientMutex.Lock()
	mcpClient = mcp
	mcpClientMutex.Unlock()
	log.Printf("MCP client created and stored in global variable")

	// Start a goroutine to monitor connection errors
	go func() {
		log.Printf("Starting connection error monitor")
		for connErr := range mcp.ConnectionErrors() {
			log.Printf("Connection error from server %s: %v", connErr.ServerName, connErr.Err)
		}
		log.Printf("Connection error monitor exited")
	}()

	// Connect to MCP server non-blockingly
	log.Printf("Initiating connection to MCP server...")
	if err := mcp.Connect(); err != nil {
		log.Printf("Failed to initiate connection to MCP servers: %v", err)
	} else {
		log.Printf("Connection initiated successfully")
	}

	// Still maintain periodic discovery as a backup
	go func() {
		log.Printf("Starting regular tool discovery loop")
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			discoverAndRegisterTools(mcp, availableTools, toolsMutex)
		}
	}()

	// Keep this goroutine running
	select {}
}

// discoverAndRegisterTools checks for available tools and registers them in the shared map
func discoverAndRegisterTools(mcp *client.MCP, availableTools *map[string][]protocol.Tool, toolsMutex *sync.Mutex) {
	if !mcp.IsConnected() {
		log.Printf("MCP not connected yet, waiting before trying to list tools...")
		mcp.CheckConnectionStatus()
		return
	}

	log.Printf("MCP connected, discovering tools from all servers...")
	toolMap := mcp.ListTools()

	// Process the results
	if len(toolMap) > 0 {
		log.Printf("Found tools from %d servers", len(toolMap))

		// Store the tools in our shared map
		toolsMutex.Lock()
		for serverName, serverTools := range toolMap {
			log.Printf("Server %s has %d tools:", serverName, len(serverTools))
			(*availableTools)[serverName] = serverTools
		}
		toolsMutex.Unlock()
	} else {
		log.Printf("No tools found, will retry later")
	}
}

// Convert chat history to OpenAI message format
func formatChatHistoryForOpenAI(history []ChatMessage) []openai.ChatCompletionMessage {
	var messages []openai.ChatCompletionMessage

	for _, msg := range history {
		// Skip tool messages in the conversion
		if msg.Role == "tool" {
			continue
		}

		role := msg.Role
		if role == "tool" {
			role = "function" // OpenAI API uses "function" instead of "tool"
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    role,
			Content: msg.Content,
		})
	}

	return messages
}

// Create a system message informing the LLM about available tools
func createSystemMessageWithTools(toolsByServer map[string][]protocol.Tool) openai.ChatCompletionMessage {
	var builder strings.Builder

	builder.WriteString("You are a helpful assistant with access to the following tools:\n\n")

	for serverName, tools := range toolsByServer {
		builder.WriteString(fmt.Sprintf("Server: %s\n", serverName))

		for _, tool := range tools {
			builder.WriteString(fmt.Sprintf("- %s: %s\n", tool.Name, tool.Description))
		}

		builder.WriteString("\n")
	}

	builder.WriteString("Use these tools when needed to provide accurate and helpful responses. " +
		"You can call them by outputting a tool call in your response.")

	return openai.ChatCompletionMessage{
		Role:    "system",
		Content: builder.String(),
	}
}

// Convert MCP tools to OpenAI tool format
func createOpenAITools(toolsByServer map[string][]protocol.Tool) []openai.Tool {
	var openaiTools []openai.Tool

	for _, tools := range toolsByServer {
		for _, tool := range tools {
			// Create default parameters object for when schema is missing or invalid
			params := map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}

			// Check if the schema has any properties
			hasProperties := len(tool.InputSchema.Properties) > 0

			if hasProperties {
				// Try to convert the schema to a map
				schemaJSON, err := json.Marshal(tool.InputSchema)
				if err == nil {
					var parsedParams map[string]interface{}
					err = json.Unmarshal(schemaJSON, &parsedParams)
					if err == nil && parsedParams != nil {
						params = parsedParams
					}
				}
			}

			// Create the OpenAI tool definition
			toolDef := openai.Tool{
				Type: openai.ToolTypeFunction,
			}

			// Initialize the Function field with a pointer to a FunctionDefinition
			toolDef.Function = &openai.FunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  params,
			}

			openaiTools = append(openaiTools, toolDef)
		}
	}

	return openaiTools
}

// Extract formatted response from MCP tool result
func extractToolResponse(response []protocol.Content) string {
	if len(response) == 0 {
		return "No response from tool"
	}

	var result strings.Builder

	for _, item := range response {
		switch item.GetType() {
		case "text":
			if textContent, ok := item.(protocol.TextContent); ok {
				result.WriteString(textContent.Text)
				result.WriteString("\n")
			}
		case "image":
			result.WriteString("[Image content]\n")
		default:
			result.WriteString(fmt.Sprintf("[Content of type %s]\n", item.GetType()))
		}
	}

	return result.String()
}

// Get OpenAI API key from environment variable
func getOpenAIKey() string {
	// key := "dummy-key" // Replace with actual implementation
	// In a real application, you would get this from an environment variable
	key := os.Getenv("OPENAI_API_KEY")
	return key
}
