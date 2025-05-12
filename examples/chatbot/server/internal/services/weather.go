package services

import (
	"context"
	"time"

	"github.com/localrivet/gomcp/examples/chatbot/server/internal/db"
	"github.com/localrivet/gomcp/examples/chatbot/server/internal/models"
)

// WeatherService provides operations for weather data
type WeatherService struct {
	DB db.WeatherDB
}

// NewWeatherService creates a new weather service
func NewWeatherService(db db.WeatherDB) *WeatherService {
	return &WeatherService{
		DB: db,
	}
}

// GetWeatherForLocation retrieves weather data for a location and transforms it to a resource representation
func (s *WeatherService) GetWeatherForLocation(ctx context.Context, location string) (models.WeatherResource, error) {
	// Get weather data from database
	record, err := db.GetWeatherRecordFromDB(ctx, s.DB, location)
	if err != nil {
		return models.WeatherResource{}, err
	}

	// Convert database record to resource representation
	return models.WeatherResource{
		Location:    record.Location,
		Temperature: record.Temperature,
		Conditions:  record.Conditions,
		WindSpeed:   record.WindSpeed,
		LastUpdated: record.LastUpdated.Format(time.RFC3339),
	}, nil
}

// ListLocations retrieves all available locations and transforms them to resource representations
func (s *WeatherService) ListLocations(ctx context.Context) ([]models.LocationResource, error) {
	// Get locations from database
	locations, err := db.ListLocationsFromDB(ctx, s.DB)
	if err != nil {
		return nil, err
	}

	// Convert to location resources
	resources := make([]models.LocationResource, 0, len(locations))
	for _, loc := range locations {
		resources = append(resources, models.LocationResource{
			Name: loc,
		})
	}

	return resources, nil
}
