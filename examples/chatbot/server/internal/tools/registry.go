package tools

import (
	"github.com/localrivet/gomcp/examples/chatbot/server/internal/services"
	"github.com/localrivet/gomcp/server"
)

// RegisterAllTools registers all tools with the MCP server
func RegisterAllTools(s *server.Server, weatherService *services.WeatherService, factsService *services.FactsService) error {
	// Register the datetime tool
	RegisterDatetimeTool(s)

	// Register the facts tool
	RegisterFactsTool(s, factsService)

	// Register weather resources
	if err := RegisterWeatherResources(s, weatherService); err != nil {
		return err
	}

	return nil
}
