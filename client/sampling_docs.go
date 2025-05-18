// Package client provides the client-side implementation of the MCP protocol.
package client

/*
# MCP Sampling Functionality

This file provides documentation and examples for the sampling functionality in the gomcp client package.

## Overview

Sampling in the Model Context Protocol (MCP) allows dynamic interaction with LLMs during execution.
Instead of providing all context upfront, the system can request specific information from an LLM
when needed during a task.

## Core Components

1. **SamplingMessage**: Represents messages in a sampling conversation with role and content.
2. **SamplingContentHandler**: Interface for different content types (text, image, audio).
3. **SamplingConfig**: Configuration options for sampling behavior, including protocol-specific settings.
4. **SamplingRequest/Response**: Structured representations of sampling operations.
5. **SamplingHandler**: Function type for processing sampling requests.

## Supported Content Types

| Content Type | Protocol Versions           | Description                          |
|--------------|----------------------------|--------------------------------------|
| text         | draft, 2024-11-05, 2025-03-26 | Text content for chat/completion    |
| image        | draft, 2024-11-05, 2025-03-26 | Image content (base64 encoded)      |
| audio        | draft, 2025-03-26          | Audio content (base64 encoded)       |

## Protocol Version Compatibility

### Draft Version
- Supports: text, image, audio
- Max tokens: 2048
- Max system prompt: 10000
- No streaming support

### 2024-11-05
- Supports: text, image
- Max tokens: 4096
- Max system prompt: 10000
- No streaming support

### 2025-03-26
- Supports: text, image, audio
- Max tokens: 8192
- Max system prompt: 16000
- Streaming support
- Chunk size range: 10-1000 (default: 100)

## Usage Examples

### Basic Text Completion

```go
// Create a client
client, err := client.NewClient("my-client", client.WithProtocolVersion("2025-03-26"))
if err != nil {
    // Handle error
    log.Fatalf("Failed to create client: %v", err)
}

// Register a sampling handler
client.WithSamplingHandler(func(params SamplingCreateMessageParams) (SamplingResponse, error) {
    // Process the request
    fmt.Println("Received sampling request with prompt:", params.Messages[0].Content.Text)

    // Return a response
    return SamplingResponse{
        Role: "assistant",
        Content: SamplingMessageContent{
            Type: "text",
            Text: "This is a response to your request.",
        },
    }, nil
})
```

### Text Chat Example

```go
// Create a request with multiple messages
messages := []SamplingMessage{
    CreateTextSamplingMessage("user", "Hello, how are you?"),
    CreateTextSamplingMessage("assistant", "I'm doing well! How can I help you today?"),
    CreateTextSamplingMessage("user", "Can you explain how MCP sampling works?"),
}

// Set model preferences
preferences := SamplingModelPreferences{
    Hints: []SamplingModelHint{
        {Name: "gpt-4"},
    },
    SpeedPriority: ptr(0.3),  // Lower number = higher priority
}

// Create a sampling handler that processes these requests
client.WithSamplingHandler(func(params SamplingCreateMessageParams) (SamplingResponse, error) {
    // In a real implementation, this would call an actual LLM API
    // and return the response

    return SamplingResponse{
        Role: "assistant",
        Content: SamplingMessageContent{
            Type: "text",
            Text: "MCP sampling allows servers to request information from LLMs during execution...",
        },
        Model: "gpt-4-turbo",
    }, nil
})

// Note: The actual handler would typically be called by the server, not invoked directly.
```

### Image Generation Example

```go
// Register an image-capable handler
client.WithSamplingHandler(func(params SamplingCreateMessageParams) (SamplingResponse, error) {
    // Check if this is an image generation request
    prompt := params.Messages[0].Content.Text
    if strings.Contains(prompt, "generate image") {
        // Generate image (pseudocode)
        imageData := callImageGenerationAPI(prompt)

        return SamplingResponse{
            Role: "assistant",
            Content: SamplingMessageContent{
                Type: "image",
                Data: imageData,
                MimeType: "image/png",
            },
        }, nil
    }

    // Handle other request types...
    return SamplingResponse{
        Role: "assistant",
        Content: SamplingMessageContent{
            Type: "text",
            Text: "I can only generate images when specifically asked.",
        },
    }, nil
})
```

### Streaming Example (2025-03-26 only)

```go
// Create a client with 2025-03-26 protocol version
client, err := client.NewClient("my-client", client.WithProtocolVersion("2025-03-26"))
if err != nil {
    // Handle error
    log.Fatalf("Failed to create client: %v", err)
}

// Register a streaming handler
client.WithStreamingSamplingHandler(func(params StreamingSamplingParams, sender StreamingSender) error {
    // Process streaming request
    prompt := params.Messages[0].Content.Text

    // Send chunks as they become available
    sender.SendChunk(&SamplingChunk{
        Content: SamplingMessageContent{
            Type: "text",
            Text: "I'm ",
        },
    })

    sender.SendChunk(&SamplingChunk{
        Content: SamplingMessageContent{
            Type: "text",
            Text: "processing ",
        },
    })

    sender.SendChunk(&SamplingChunk{
        Content: SamplingMessageContent{
            Type: "text",
            Text: "your request...",
        },
    })

    // Complete the stream
    sender.Complete(&SamplingCompletion{
        Model: "gpt-4-turbo-stream",
        StopReason: "complete",
    })

    return nil
})
```

## Performance Considerations

1. **Content Size Management**:
   - Large base64-encoded images can consume substantial memory
   - Consider downsampling images before encoding
   - For audio, use efficient codecs and appropriate sampling rates

2. **Caching Strategies**:
   - Consider caching commonly used sampling configurations
   - For frequently requested content, implement response caching
   - Use protocol-version-specific caching for optimal performance

3. **Resource Management**:
   - Implement timeouts appropriately for different content types
   - Use connection pooling for high-volume scenarios
   - Consider batching for multiple related sampling requests

4. **Streaming Optimization** (2025-03-26):
   - Adjust chunk sizes based on content type and network conditions
   - Implement backpressure handling for slow consumers
   - Consider adaptive streaming rates based on network conditions

## Error Handling

The sampling implementation includes comprehensive error handling:

1. **Protocol Validation**: Ensures all requests conform to the negotiated protocol version
2. **Content Validation**: Validates content types and formats before processing
3. **Retry Logic**: Configurable retry behavior for transient failures
4. **Graceful Degradation**: Options for handling failures gracefully

## Advanced Configuration

The `SamplingConfig` type provides extensive customization options:

```go
// Create a configuration optimized for image generation
config, err := client.NewSamplingConfig().
    ForVersion("2025-03-26").                 // Set protocol version
    WithMaxTokens(4000).                      // Set token limit
    WithRequestTimeout(45 * time.Second).     // Extended timeout for image generation
    OptimizeForImageGeneration()              // Apply image-specific optimizations
```

Each protocol version has its own optimized defaults, and the configuration system
provides both convenience methods for common scenarios and fine-grained control when needed.
*/

// This file contains no actual code, only documentation.
var _ = "documentation only"
