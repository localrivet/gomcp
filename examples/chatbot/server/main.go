package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
)

// MCP server that provides useful tools for a chatbot
func main() {
	// Parse command line flags
	port := flag.Int("port", 4477, "Port to run the MCP server on")
	flag.Parse()

	// Create MCP server
	s := server.NewServer("Chatbot-Helper-Server")

	// Create a mock database connection
	// In a real implementation, this would be a real database connection
	db := NewMockWeatherDB()

	// Register weather as a resource
	s.Resource("weather://{location}/current",
		server.WithHandler(func(ctx *server.Context, location string) (WeatherResource, error) {
			// Get weather data from database
			record, err := getWeatherRecordFromDB(context.Background(), db, location)
			if err != nil {
				return WeatherResource{}, fmt.Errorf("error getting weather: %v", err)
			}

			// Convert database record to resource representation
			return WeatherResource{
				Location:    record.Location,
				Temperature: record.Temperature,
				Conditions:  record.Conditions,
				WindSpeed:   record.WindSpeed,
				LastUpdated: record.LastUpdated.Format(time.RFC3339),
			}, nil
		}),
		server.WithName("Weather Information"),
		server.WithDescription("Provides current weather information for a location"),
	)

	// Get available locations from the database
	locations, err := listLocationsFromDB(context.Background(), db)
	if err != nil {
		log.Fatalf("Failed to get locations: %v", err)
	}

	// Convert to location resources
	locationResources := make([]LocationResource, 0, len(locations))
	for _, loc := range locations {
		locationResources = append(locationResources, LocationResource{
			Name: loc,
		})
	}

	// Marshal to JSON for static content
	locationsJSON, err := json.Marshal(locationResources)
	if err != nil {
		log.Fatalf("Failed to marshal locations: %v", err)
	}

	// Register available locations as a static resource
	s.Resource("weather://locations",
		server.WithTextContent(string(locationsJSON)),
		server.WithName("Available Weather Locations"),
		server.WithDescription("Lists all locations with available weather data"),
		server.WithMimeType("application/json"),
	)

	// Register a facts tool that provides information on various topics
	s.Tool("fact", "Get a fact about a topic", func(ctx *server.Context, args struct {
		Topic string `json:"topic"`
	}) (interface{}, error) {
		if args.Topic == "" {
			return nil, fmt.Errorf("topic is required")
		}

		// In a real implementation, this might query a database or API
		fact := getFactAbout(args.Topic)

		return protocol.TextContent{
			Type: "text",
			Text: fact,
		}, nil
	})

	// Register a date tool that provides the current date and time
	s.Tool("datetime", "Get the current date and time", func(ctx *server.Context, args struct{}) (interface{}, error) {
		now := time.Now()
		return protocol.TextContent{
			Type: "text",
			Text: fmt.Sprintf("The current date and time is: %s", now.Format(time.RFC1123)),
		}, nil
	})

	// Configure SSE transport instead of WebSocket
	s.AsSSE("0.0.0.0:4477", "/mcp")

	// Start the server
	log.Printf("Starting Chatbot Helper MCP server on port %d", *port)
	if err := s.Run(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

// WeatherResource is the resource representation of weather data
type WeatherResource struct {
	Location    string  `json:"location"`
	Temperature float64 `json:"temperature"`
	Conditions  string  `json:"conditions"`
	WindSpeed   float64 `json:"wind_speed"`
	LastUpdated string  `json:"last_updated"`
}

// LocationResource is the resource representation of a location
type LocationResource struct {
	Name string `json:"name"`
}

// WeatherRecord represents a weather record in the database
type WeatherRecord struct {
	Location    string
	Temperature float64
	Conditions  string
	WindSpeed   float64
	LastUpdated time.Time
}

// WeatherDB is the interface for weather database operations
type WeatherDB interface {
	QueryWeather(ctx context.Context, location string) (WeatherRecord, error)
	ListLocations(ctx context.Context) ([]string, error)
}

// MockWeatherDB is a mock implementation of the WeatherDB interface
type MockWeatherDB struct {
	records map[string]WeatherRecord
}

// NewMockWeatherDB creates a new mock weather database
func NewMockWeatherDB() *MockWeatherDB {
	// Create mock database with sample weather data
	now := time.Now()
	return &MockWeatherDB{
		records: map[string]WeatherRecord{
			"new york": {
				Location:    "New York",
				Temperature: 22.5,
				Conditions:  "Partly Cloudy",
				WindSpeed:   8.3,
				LastUpdated: now,
			},
			"london": {
				Location:    "London",
				Temperature: 16.2,
				Conditions:  "Rainy",
				WindSpeed:   12.7,
				LastUpdated: now,
			},
			"tokyo": {
				Location:    "Tokyo",
				Temperature: 28.1,
				Conditions:  "Sunny",
				WindSpeed:   5.2,
				LastUpdated: now,
			},
			"sydney": {
				Location:    "Sydney",
				Temperature: 20.4,
				Conditions:  "Clear",
				WindSpeed:   9.8,
				LastUpdated: now,
			},
			"paris": {
				Location:    "Paris",
				Temperature: 18.7,
				Conditions:  "Cloudy",
				WindSpeed:   7.5,
				LastUpdated: now,
			},
			"berlin": {
				Location:    "Berlin",
				Temperature: 14.3,
				Conditions:  "Overcast",
				WindSpeed:   10.1,
				LastUpdated: now,
			},
			"san francisco": {
				Location:    "San Francisco",
				Temperature: 17.8,
				Conditions:  "Foggy",
				WindSpeed:   15.3,
				LastUpdated: now,
			},
		},
	}
}

// QueryWeather retrieves weather data for a location from the mock database
func (db *MockWeatherDB) QueryWeather(ctx context.Context, location string) (WeatherRecord, error) {
	// Simulate database query latency
	time.Sleep(100 * time.Millisecond)

	// Look up record in mock database
	record, exists := db.records[location]
	if !exists {
		return WeatherRecord{}, sql.ErrNoRows
	}

	return record, nil
}

// ListLocations returns a list of all available locations in the database
func (db *MockWeatherDB) ListLocations(ctx context.Context) ([]string, error) {
	// Simulate database query latency
	time.Sleep(50 * time.Millisecond)

	locations := make([]string, 0, len(db.records))
	for location := range db.records {
		locations = append(locations, location)
	}

	return locations, nil
}

// getWeatherRecordFromDB retrieves weather data from the database
func getWeatherRecordFromDB(ctx context.Context, db WeatherDB, location string) (WeatherRecord, error) {
	// Query the database for weather data
	record, err := db.QueryWeather(ctx, location)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return a more specific error for location not found
			return WeatherRecord{}, fmt.Errorf("location '%s' not found in database", location)
		}
		return WeatherRecord{}, fmt.Errorf("database error: %v", err)
	}

	return record, nil
}

// listLocationsFromDB retrieves all available locations from the database
func listLocationsFromDB(ctx context.Context, db WeatherDB) ([]string, error) {
	// Query the database for available locations
	locations, err := db.ListLocations(ctx)
	if err != nil {
		return nil, fmt.Errorf("database error: %v", err)
	}

	return locations, nil
}

// getFactAbout simulates getting a fact about a topic
func getFactAbout(topic string) string {
	// In a real implementation, this might query a database or API
	facts := map[string]string{
		"earth":      "Earth is the third planet from the Sun and the only astronomical object known to harbor life.",
		"mars":       "Mars is the fourth planet from the Sun and the second-smallest planet in the Solar System, only being larger than Mercury.",
		"jupiter":    "Jupiter is the largest planet in the Solar System. It is the fifth planet from the Sun.",
		"python":     "Python is a high-level, general-purpose programming language. Its design philosophy emphasizes code readability.",
		"go":         "Go is a statically typed, compiled programming language designed at Google by Robert Griesemer, Rob Pike, and Ken Thompson.",
		"javascript": "JavaScript is a high-level, often just-in-time compiled language that conforms to the ECMAScript specification.",
		"coffee":     "Coffee is a brewed drink prepared from roasted coffee beans, the seeds of berries from certain Coffea species.",
		"tea":        "Tea is an aromatic beverage prepared by pouring hot or boiling water over cured or fresh leaves of Camellia sinensis.",
	}

	// Check if we have a fact for the provided topic
	if fact, ok := facts[topic]; ok {
		return fact
	}

	// Return a default response for unknown topics
	return fmt.Sprintf("I don't have specific facts about '%s'. Please try another topic or ask a more specific question.", topic)
}
