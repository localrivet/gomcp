package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/localrivet/gomcp/examples/chatbot/server/internal/services"
	"github.com/localrivet/gomcp/server"
)

// RegisterWeatherResources registers weather-related resources with the MCP server
func RegisterWeatherResources(s *server.Server, weatherService *services.WeatherService) error {
	// Register current weather resource
	s.Resource("weather://{location}/current",
		server.WithHandler(func(ctx *server.Context, location string) (interface{}, error) {
			// Get weather data from service
			resource, err := weatherService.GetWeatherForLocation(context.Background(), location)
			if err != nil {
				return nil, fmt.Errorf("error getting weather: %v", err)
			}

			return resource, nil
		}),
		server.WithName("Weather Information"),
		server.WithDescription("Provides current weather information for a location"),
	)

	// Get available locations from the service
	locations, err := weatherService.ListLocations(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get locations: %v", err)
	}

	// Marshal to JSON for static content
	locationsJSON, err := json.Marshal(locations)
	if err != nil {
		return fmt.Errorf("failed to marshal locations: %v", err)
	}

	// Register available locations as a static resource
	s.Resource("weather://locations",
		server.WithTextContent(string(locationsJSON)),
		server.WithName("Available Weather Locations"),
		server.WithDescription("Lists all locations with available weather data"),
		server.WithMimeType("application/json"),
	)

	return nil
}
