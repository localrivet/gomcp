package main

import (
	"fmt"
	"log"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

func main() {
	mcp := server.NewServer("Demo 🚀")
	mcp.Prompt("Add two numbers", "Add two numbers",
		protocol.PromptMessage{
			Role:    "system",
			Content: []protocol.Content{protocol.TextContent{Type: "text", Text: "You are a helpful assistant that adds two numbers."}},
		},
		protocol.PromptMessage{
			Role:    "user",
			Content: []protocol.Content{protocol.TextContent{Type: "text", Text: "What is 2 + 2?"}},
		},
	)
	mcp.Tool("add", "Add two numbers", func(ctx *server.Context, args struct {
		A int `json:"a"`
		B int `json:"b"`
	}) (protocol.TextContent, error) {
		return protocol.TextContent{Type: "text", Text: fmt.Sprintf("%d", args.A+args.B)}, nil
	})

	if err := mcp.AsStdio().Run(); err != nil {
		log.Fatal(err)
	}
}
