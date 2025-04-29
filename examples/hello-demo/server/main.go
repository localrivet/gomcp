// Command hello-demo boots a tiny MCP server that showcases *all three* MCP
// primitives in under 60 lines of Go:
//
// Tools
// -----
//
//	hello   – returns a friendly greeting
//
// Prompts
// -------
//
//	greet   – sends an assistant welcome message (useful as a chat preamble)
//
// Resources
// ---------
//
//	icon.png – serves a fake PNG blob (Base64‑encoded string)
//
// Build & run:
//
//	go run .
//
// Example request via stdin:
//
//	{"method":"hello","params":{"name":"Alma"}}
//
// The sample focuses on *clarity* rather than business logic.
package main

import (
	"encoding/base64"
	"fmt"
	"log"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// HelloArgs is the request payload for the hello tool.
type HelloArgs struct {
	Name string `json:"name" description:"Name to greet" required:"true"`
}

func main() {
	// Create the MCP server instance.
	srv := server.NewServer("hello-demo")

	// ---------------------------------------------------------------------
	// Tool: hello
	// ---------------------------------------------------------------------
	if err := server.AddTool(
		srv,
		"hello",
		"Return a friendly greeting for the supplied name.",
		func(args HelloArgs) (protocol.Content, error) {
			greeting := fmt.Sprintf("Hello, %s!", args.Name)
			log.Printf("[hello] -> %q", greeting)
			return server.Text(greeting), nil
		},
	); err != nil {
		log.Fatalf("register hello tool: %v", err)
	}

	// ---------------------------------------------------------------------
	// Prompt: greet
	// ---------------------------------------------------------------------
	if err := server.AddPrompt(
		srv,
		"greet",
		"Assistant greeting prompt.",
		func(_ HelloArgs) (protocol.PromptMessage, error) {
			msg := "How can I help you today?"
			return server.Message("assistant", msg), nil
		},
	); err != nil {
		log.Fatalf("register greet prompt: %v", err)
	}

	// ---------------------------------------------------------------------
	// Resource: icon.png (placeholder data)
	// ---------------------------------------------------------------------
	if err := server.AddResource(
		srv,
		"icon.png",
		"image/png",
		"Server icon (placeholder)",
		"1.0",
		func() (protocol.ResourceContents, error) {
			fakeData := base64.StdEncoding.EncodeToString([]byte("pretend this is PNG data"))
			return protocol.BlobResourceContents{
				ContentType: "image/png",
				Blob:        fakeData,
			}, nil
		},
	); err != nil {
		log.Fatalf("register icon.png resource: %v", err)
	}

	// ---------------------------------------------------------------------
	// Start the server (stdio transport).
	// ---------------------------------------------------------------------
	log.Println("hello-demo ready (stdio mode)")
	if err := server.ServeStdio(srv); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
