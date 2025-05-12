package tools

import (
	"fmt"

	"github.com/localrivet/gomcp/examples/chatbot/server/internal/services"
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// RegisterFactsTool registers the facts tool with the MCP server
func RegisterFactsTool(s *server.Server, factsService *services.FactsService) {
	s.Tool("fact", "Get a fact about a topic", func(ctx *server.Context, args struct {
		Topic string `json:"topic"`
	}) (interface{}, error) {
		if args.Topic == "" {
			return nil, fmt.Errorf("topic is required")
		}

		fact := factsService.GetFactAbout(args.Topic)

		return protocol.TextContent{
			Type: "text",
			Text: fact,
		}, nil
	})
}
