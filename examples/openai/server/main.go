package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	"github.com/sashabaranov/go-openai"
)

func main() {
	// Parse command line flags
	port := flag.Int("port", 3000, "Port to run the MCP server on")
	flag.Parse()

	// Get OpenAI API key from environment
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		log.Fatal("OPENAI_API_KEY environment variable is required")
	}

	// Create OpenAI client
	openaiClient := openai.NewClient(apiKey)

	// Create MCP server
	s := server.NewServer("OpenAI-MCP-Server")

	// Register chat completion tool with streaming support
	s.Tool("chat_completion", "Generate a chat completion using OpenAI's API", func(ctx *server.Context, args struct {
		Model       string                         `json:"model"`
		Messages    []openai.ChatCompletionMessage `json:"messages"`
		Temperature float32                        `json:"temperature,omitempty"`
		MaxTokens   int                            `json:"max_tokens,omitempty"`
		Stream      bool                           `json:"stream,omitempty"`
	}) (interface{}, error) {
		// Check if streaming is requested
		if args.Stream {
			// Create completion request with stream enabled
			request := openai.ChatCompletionRequest{
				Model:       args.Model,
				Messages:    args.Messages,
				Temperature: args.Temperature,
				MaxTokens:   args.MaxTokens,
				Stream:      true,
			}

			// Create a stream
			stream, err := openaiClient.CreateChatCompletionStream(context.Background(), request)
			if err != nil {
				return nil, fmt.Errorf("OpenAI API error: %v", err)
			}
			defer stream.Close()

			// Initialize counters for progress reporting
			chunkCount := 0
			generatedText := ""

			// Process the stream in chunks
			for {
				response, err := stream.Recv()
				if err != nil {
					if err == io.EOF {
						// End of stream
						break
					}
					return nil, fmt.Errorf("stream error: %v", err)
				}

				// Increment chunk counter
				chunkCount++

				// Extract the content from the response
				content := ""
				if len(response.Choices) > 0 {
					content = response.Choices[0].Delta.Content
					generatedText += content
				}

				// Use the raw OpenAI response as the message, which will be stringified in the progress params
				progressMessage := fmt.Sprintf("chunk %d: %s", chunkCount, content)

				// Send progress report with the chunk
				// The client code expects:
				// - choices[0].delta.content format in the progressValue
				// For the UI we also track:
				// - current chunk number
				// - total count (arbitrary high number since we don't know the total)
				// - the content of the current chunk
				ctx.ReportProgress(progressMessage, chunkCount, 1000)
			}

			// Return the full generated text
			return protocol.TextContent{
				Type: "text",
				Text: generatedText,
			}, nil
		}

		// Non-streaming mode
		request := openai.ChatCompletionRequest{
			Model:       args.Model,
			Messages:    args.Messages,
			Temperature: args.Temperature,
			MaxTokens:   args.MaxTokens,
		}

		// Call OpenAI API
		response, err := openaiClient.CreateChatCompletion(context.Background(), request)
		if err != nil {
			return nil, fmt.Errorf("OpenAI API error: %v", err)
		}

		// Return the response as text
		if len(response.Choices) > 0 {
			return protocol.TextContent{
				Type: "text",
				Text: response.Choices[0].Message.Content,
			}, nil
		}

		return protocol.TextContent{
			Type: "text",
			Text: "No response from OpenAI API",
		}, nil
	})

	// Register image generation tool
	s.Tool("generate_image", "Generate an image using DALL-E", func(ctx *server.Context, args struct {
		Prompt  string `json:"prompt"`
		Size    string `json:"size,omitempty"`
		Quality string `json:"quality,omitempty"`
		N       int    `json:"n,omitempty"`
	}) (interface{}, error) {
		// Set defaults
		size := args.Size
		if size == "" {
			size = openai.CreateImageSize1024x1024
		}

		n := args.N
		if n <= 0 {
			n = 1
		}

		// Create image request
		request := openai.ImageRequest{
			Prompt:  args.Prompt,
			Size:    size,
			Quality: args.Quality,
			N:       n,
		}

		// Call OpenAI API
		response, err := openaiClient.CreateImage(context.Background(), request)
		if err != nil {
			return nil, fmt.Errorf("OpenAI API error: %v", err)
		}

		// Return image URLs as text
		if len(response.Data) > 0 {
			result := "Generated Images:\n"
			for i, image := range response.Data {
				result += fmt.Sprintf("%d. %s\n", i+1, image.URL)
			}
			return protocol.TextContent{
				Type: "text",
				Text: result,
			}, nil
		}

		return protocol.TextContent{
			Type: "text",
			Text: "No images generated",
		}, nil
	})

	// Register embeddings tool
	s.Tool("create_embeddings", "Generate embeddings for input text", func(ctx *server.Context, args struct {
		Input []string              `json:"input"`
		Model openai.EmbeddingModel `json:"model,omitempty"`
	}) (interface{}, error) {
		// Set default model if not provided
		model := args.Model
		if model == "" {
			model = openai.LargeEmbedding3
		}

		// Create embedding request
		request := openai.EmbeddingRequest{
			Input: args.Input,
			Model: model,
		}

		// Call OpenAI API
		response, err := openaiClient.CreateEmbeddings(context.Background(), request)
		if err != nil {
			return nil, fmt.Errorf("OpenAI API error: %v", err)
		}

		// Format the response
		result := fmt.Sprintf("Generated %d embeddings\n", len(response.Data))
		result += fmt.Sprintf("Model: %s\n", response.Model)
		result += fmt.Sprintf("Usage: %d tokens\n", response.Usage.TotalTokens)

		return protocol.TextContent{
			Type: "text",
			Text: result,
		}, nil
	})

	// Configure WebSocket transport
	s.AsWebsocket("0.0.0.0", fmt.Sprintf(":%d", *port))

	// Start the server
	log.Printf("Starting OpenAI MCP server on port %d", *port)
	if err := s.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
