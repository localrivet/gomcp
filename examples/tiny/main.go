package main

import (
	"fmt"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server" // Assuming helpers are in server package
	// "github.com/localrivet/gomcp/util/schema" // Removed unused import
)

func main() {
	mcp := server.NewServer("Demo 🚀")

	// Option A
	server.AddTool(mcp, "add", "Add two numbers", func(args struct {
		A int `json:"a"`
		B int `json:"b"`
	}) (protocol.Content, error) {
		return server.Text(fmt.Sprintf("%d", args.A+args.B)), nil
	})

	server.ServeStdio(mcp)
}
