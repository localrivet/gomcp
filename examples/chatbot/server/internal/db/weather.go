package db

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/localrivet/gomcp/examples/chatbot/server/internal/models"
)

// WeatherDB is the interface for weather database operations
type WeatherDB interface {
	QueryWeather(ctx context.Context, location string) (models.WeatherRecord, error)
	ListLocations(ctx context.Context) ([]string, error)
}

// MockWeatherDB is a mock implementation of the WeatherDB interface
type MockWeatherDB struct {
	records map[string]models.WeatherRecord
}

// NewMockWeatherDB creates a new mock weather database
func NewMockWeatherDB() *MockWeatherDB {
	// Create mock database with sample weather data
	now := time.Now()
	return &MockWeatherDB{
		records: map[string]models.WeatherRecord{
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
func (db *MockWeatherDB) QueryWeather(ctx context.Context, location string) (models.WeatherRecord, error) {
	// Simulate database query latency
	time.Sleep(100 * time.Millisecond)

	// Look up record in mock database
	record, exists := db.records[location]
	if !exists {
		return models.WeatherRecord{}, sql.ErrNoRows
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

// GetWeatherRecordFromDB retrieves weather data from the database
func GetWeatherRecordFromDB(ctx context.Context, db WeatherDB, location string) (models.WeatherRecord, error) {
	// Query the database for weather data
	record, err := db.QueryWeather(ctx, location)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Return a more specific error for location not found
			return models.WeatherRecord{}, errors.New("location not found")
		}
		return models.WeatherRecord{}, err
	}

	return record, nil
}

// ListLocationsFromDB retrieves all available locations from the database
func ListLocationsFromDB(ctx context.Context, db WeatherDB) ([]string, error) {
	// Query the database for available locations
	return db.ListLocations(ctx)
}
