# OpenAI Integration for GoMCP

This example demonstrates how to integrate OpenAI's API with GoMCP, allowing you to expose OpenAI capabilities as MCP tools.

## Features

- **Chat Completion**: Generate text responses using OpenAI's GPT models
- **Image Generation**: Create images using DALL-E models
- **Embeddings**: Generate embeddings for text using OpenAI's embedding models
- **Real-time Streaming**: Get responses in real-time via WebSockets

## Requirements

- Go 1.18 or later
- An OpenAI API key

## Setup

1. Install the OpenAI Go client:

```bash
go get github.com/sashabaranov/go-openai
```

2. Install WebSocket library:

```bash
go get github.com/gobwas/ws
```

3. Set your OpenAI API key as an environment variable:

```bash
export OPENAI_API_KEY="your-api-key-here"
```

## Running the Example

This example provides both a command-line client and a web-based client with WebSocket streaming.

### Server

The MCP server needs to be running before you can use either client:

```bash
# Start the server in a separate terminal window
cd examples/openai/server
go run main.go
```

### Command-line Client

To use the command-line client:

```bash
# Run the client in command-line mode
cd examples/openai/client
go run main.go
```

### Web Client with WebSockets

The web client provides a real-time chat interface using WebSockets for streaming responses:

```bash
# Run the web client
cd examples/openai/client
go run main.go -web
```

Then open your browser to http://localhost:3366 to interact with the chat interface.

## Technical Details

### WebSocket Implementation

The web client uses the `gobwas/ws` package to handle WebSocket connections. This provides several advantages:

1. **No External Dependencies**: unlike other WebSocket libraries, gobwas/ws has no external dependencies
2. **Low-level Control**: provides direct access to the underlying connection for better performance
3. **Streaming Capability**: allows for real-time streaming of responses

### Progress Notifications

The integration uses MCP's progress notifications to stream responses in real-time:

1. The server processes each chunk from OpenAI's streaming API
2. The server sends progress updates via the `ReportProgress` method
3. The client registers a progress handler to process these updates
4. The web UI displays the streaming response in real-time

### Protocol Flow

1. User sends a message via the WebSocket
2. Client calls the OpenAI MCP tool with streaming enabled
3. Server streams responses back to the client via progress notifications
4. Client forwards these chunks to the WebSocket client
5. WebSocket client displays the response in real-time

## Extending the Example

You can add more OpenAI features by:

1. Adding new tool handlers in `server/main.go`
2. Exposing these tools in the client interfaces
3. Updating the web UI to make use of the new capabilities

For example, you could add support for:

- Audio transcription with Whisper API
- Function calling with GPT models
- Fine-tuning API integration
