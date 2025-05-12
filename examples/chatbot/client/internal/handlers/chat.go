package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/localrivet/gomcp/examples/chatbot/client/internal/mcp"
	"github.com/localrivet/gomcp/examples/chatbot/client/internal/models"
	"github.com/localrivet/gomcp/protocol"
	"github.com/sashabaranov/go-openai"
)

// ChatHandler handles chat-related HTTP requests
type ChatHandler struct {
	mcpClient    *mcp.Client
	openaiClient *openai.Client
	chatHistory  []models.ChatMessage
	chatMutex    sync.Mutex
	templatePath string
}

// NewChatHandler creates a new chat handler
func NewChatHandler(mcpClient *mcp.Client, openaiClient *openai.Client, templatePath string) *ChatHandler {
	return &ChatHandler{
		mcpClient:    mcpClient,
		openaiClient: openaiClient,
		chatHistory:  []models.ChatMessage{},
		templatePath: templatePath,
	}
}

// IndexHandler handles requests to the home page
func (h *ChatHandler) IndexHandler(w http.ResponseWriter, r *http.Request) {
	h.chatMutex.Lock()
	history := make([]models.ChatMessage, len(h.chatHistory))
	copy(history, h.chatHistory)
	h.chatMutex.Unlock()

	tmpl, err := template.ParseFiles(h.templatePath)
	if err != nil {
		http.Error(w, "Error loading template", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, map[string]interface{}{
		"History": history,
	})
}

// ChatHandler handles chat form submissions
func (h *ChatHandler) ChatHandler(w http.ResponseWriter, r *http.Request) {
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
	h.chatMutex.Lock()
	h.chatHistory = append(h.chatHistory, models.ChatMessage{
		Role:      "user",
		Content:   userMessage,
		Timestamp: time.Now(),
	})
	h.chatMutex.Unlock()

	// Get available tools
	tools := h.mcpClient.GetTools()
	if len(tools) == 0 {
		// No tools available yet
		h.chatMutex.Lock()
		h.chatHistory = append(h.chatHistory, models.ChatMessage{
			Role:      "assistant",
			Content:   "I'm still initializing and discovering tools. Please try again in a moment.",
			Timestamp: time.Now(),
		})
		h.chatMutex.Unlock()

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Convert chat history to OpenAI message format
	h.chatMutex.Lock()
	messages := h.formatChatHistoryForOpenAI()
	h.chatMutex.Unlock()

	// Add tool information to system message
	systemMessage := h.createSystemMessageWithTools(tools)
	messages = append([]openai.ChatCompletionMessage{systemMessage}, messages...)

	// Create OpenAI chat completion request with tools
	chatRequest := openai.ChatCompletionRequest{
		Model:    "gpt-4.1-nano",
		Messages: messages,
		Tools:    h.createOpenAITools(tools),
	}

	// Call OpenAI API
	resp, err := h.openaiClient.CreateChatCompletion(context.Background(), chatRequest)
	if err != nil {
		log.Printf("Error calling OpenAI API: %v", err)

		h.chatMutex.Lock()
		h.chatHistory = append(h.chatHistory, models.ChatMessage{
			Role:      "assistant",
			Content:   "Sorry, I encountered an error while processing your message. Please try again.",
			Timestamp: time.Now(),
		})
		h.chatMutex.Unlock()

		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	// Process the response
	aiMessage := resp.Choices[0].Message

	// Add AI response to history
	h.chatMutex.Lock()
	h.chatHistory = append(h.chatHistory, models.ChatMessage{
		Role:      "assistant",
		Content:   aiMessage.Content,
		Timestamp: time.Now(),
	})
	h.chatMutex.Unlock()

	// Check if the model wants to use tools
	if len(aiMessage.ToolCalls) > 0 {
		h.handleToolCalls(aiMessage, messages, tools)
	}

	// Redirect back to the main page to show the updated chat
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// handleToolCalls processes tool calls from the AI
func (h *ChatHandler) handleToolCalls(
	aiMessage openai.ChatCompletionMessage,
	messages []openai.ChatCompletionMessage,
	tools map[string][]protocol.Tool,
) {
	// Add the assistant's message with tool_calls to the messages for the follow-up
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

		result, err := h.mcpClient.CallTool(serverName, toolName, args)

		// Format the tool response
		var toolResponse string
		if err != nil {
			toolResponse = fmt.Sprintf("Error calling tool %s: %v", toolName, err)
			log.Println(toolResponse)
		} else {
			toolResponse = h.extractToolResponse(result)
		}

		// Add tool call to history
		h.chatMutex.Lock()
		h.chatHistory = append(h.chatHistory, models.ChatMessage{
			Role:      "tool",
			Content:   fmt.Sprintf("Tool: %s\nArgs: %v\nResult: %s", toolName, args, toolResponse),
			Timestamp: time.Now(),
		})
		h.chatMutex.Unlock()

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
		Tools:    h.createOpenAITools(tools),
	}

	followupResp, err := h.openaiClient.CreateChatCompletion(context.Background(), followupRequest)
	if err != nil {
		log.Printf("Error getting follow-up response: %v", err)

		// Add error message to chat history
		h.chatMutex.Lock()
		h.chatHistory = append(h.chatHistory, models.ChatMessage{
			Role:      "system",
			Content:   fmt.Sprintf("Error getting AI interpretation of tool results: %v", err),
			Timestamp: time.Now(),
		})
		h.chatMutex.Unlock()
	} else {
		// Add follow-up response to history
		followupMessage := followupResp.Choices[0].Message.Content

		// Clear formatting to make it obvious this is the AI's interpretation of tool results
		if followupMessage == "" {
			followupMessage = "I processed the tool results but have no additional information to add."
		}

		log.Printf("AI interpretation of tool results: %s", followupMessage)

		h.chatMutex.Lock()
		h.chatHistory = append(h.chatHistory, models.ChatMessage{
			Role:      "assistant",
			Content:   followupMessage,
			Timestamp: time.Now(),
		})
		h.chatMutex.Unlock()

		// Check if there are more tool calls in the follow-up response
		if len(followupResp.Choices[0].Message.ToolCalls) > 0 {
			log.Printf("AI wants to call more tools - this is not currently supported in a single interaction")

			h.chatMutex.Lock()
			h.chatHistory = append(h.chatHistory, models.ChatMessage{
				Role:      "system",
				Content:   "The AI requested additional tool calls. Please submit a new message to continue the conversation.",
				Timestamp: time.Now(),
			})
			h.chatMutex.Unlock()
		}
	}
}

// formatChatHistoryForOpenAI converts chat history to OpenAI message format
func (h *ChatHandler) formatChatHistoryForOpenAI() []openai.ChatCompletionMessage {
	var messages []openai.ChatCompletionMessage

	for _, msg := range h.chatHistory {
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

// createSystemMessageWithTools creates a system message with tool information
func (h *ChatHandler) createSystemMessageWithTools(toolsByServer map[string][]protocol.Tool) openai.ChatCompletionMessage {
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

// createOpenAITools converts MCP tools to OpenAI tool format
func (h *ChatHandler) createOpenAITools(toolsByServer map[string][]protocol.Tool) []openai.Tool {
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

// extractToolResponse extracts formatted response from MCP tool result
func (h *ChatHandler) extractToolResponse(response []protocol.Content) string {
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
