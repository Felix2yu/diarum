package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Service provides weather data from Open-Meteo API
type Service struct {
	cityCoords map[string][2]float64
	mu         sync.RWMutex
}

// geocodingResponse represents the Open-Meteo geocoding API response
type geocodingResponse struct {
	Results []geocodingResult `json:"results"`
}

type geocodingResult struct {
	Name      string  `json:"name"`
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Country   string  `json:"country"`
	Admin1    string  `json:"admin1"`
}

// CityInfo represents a city with coordinates
type CityInfo struct {
	Name      string  `json:"name"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Province  string  `json:"province"`
	Country   string  `json:"country"`
}

// SearchCities searches for cities by name using Open-Meteo geocoding API
func SearchCities(query string) ([]CityInfo, error) {
	url := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&count=10&language=zh",
		query,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to call geocoding API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("geocoding API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read geocoding response: %w", err)
	}

	var data geocodingResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse geocoding response: %w", err)
	}

	cities := make([]CityInfo, 0, len(data.Results))
	for _, r := range data.Results {
		cities = append(cities, CityInfo{
			Name:     r.Name,
			Lat:      r.Latitude,
			Lon:      r.Longitude,
			Province: r.Admin1,
			Country:  r.Country,
		})
	}

	return cities, nil
}

// NewService creates a new weather service
func NewService() *Service {
	s := &Service{
		cityCoords: make(map[string][2]float64),
	}
	return s
}

// getCoords gets coordinates for a city, first checking cache, then geocoding API
func (s *Service) getCoords(city string) (lat, lon float64, err error) {
	// Check cache first
	s.mu.RLock()
	coords, ok := s.cityCoords[city]
	s.mu.RUnlock()

	if ok {
		return coords[0], coords[1], nil
	}

	// Call geocoding API
	lat, lon, err = geocodeCity(city)
	if err != nil {
		return 0, 0, err
	}

	// Cache the result
	s.mu.Lock()
	s.cityCoords[city] = [2]float64{lat, lon}
	s.mu.Unlock()

	return lat, lon, nil
}

// geocodeCity calls Open-Meteo geocoding API to get coordinates for a city
func geocodeCity(city string) (lat, lon float64, err error) {
	url := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=zh",
		city,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to call geocoding API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, 0, fmt.Errorf("geocoding API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read geocoding response: %w", err)
	}

	var data geocodingResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, 0, fmt.Errorf("failed to parse geocoding response: %w", err)
	}

	if len(data.Results) == 0 {
		return 0, 0, fmt.Errorf("city %q not found", city)
	}

	result := data.Results[0]
	return result.Latitude, result.Longitude, nil
}

// GetWeather fetches weather for a city on a specific date
func (s *Service) GetWeather(city string, date string) (*WeatherResult, error) {
	lat, lon, err := s.getCoords(city)
	if err != nil {
		return nil, err
	}

	return FetchWeatherFromOpenMeteo(city, lat, lon, date)
}

// GetWeatherByCoords fetches weather by coordinates on a specific date
func (s *Service) GetWeatherByCoords(city string, lat, lon float64, date string) (*WeatherResult, error) {
	return FetchWeatherFromOpenMeteo(city, lat, lon, date)
}

// SetCityCoords adds or updates city coordinates
func (s *Service) SetCityCoords(city string, lat, lon float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cityCoords[city] = [2]float64{lat, lon}
}
