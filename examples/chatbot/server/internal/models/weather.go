package models

import "time"

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
