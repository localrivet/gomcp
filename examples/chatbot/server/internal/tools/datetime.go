package tools

import (
	"fmt"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// RegisterDatetimeTool registers the datetime tool with the MCP server
func RegisterDatetimeTool(s *server.Server) {
	s.Tool("datetime", "Get the current date and time", func(ctx *server.Context, args struct{}) (interface{}, error) {
		now := time.Now()
		return protocol.TextContent{
			Type: "text",
			Text: fmt.Sprintf("The current date and time is: %s", now.Format(time.RFC1123)),
		}, nil
	})
}
