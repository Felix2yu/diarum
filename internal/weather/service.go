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
// If query contains comma-separated coordinates (lat,lon), uses Nominatim reverse geocoding
func SearchCities(query string) ([]CityInfo, error) {
	// Check if query is coordinates (lat,lon format)
	var lat, lon float64
	if n, err := fmt.Sscanf(query, "%f,%f", &lat, &lon); err == nil && n == 2 {
		return reverseGeocode(lat, lon)
	}

	// Otherwise, use Open-Meteo forward geocoding
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

// nominatimResponse represents the Nominatim reverse geocoding API response
type nominatimResponse struct {
	Address  nominatimAddress `json:"address"`
	Lat      string           `json:"lat"`
	Lon      string           `json:"lon"`
	Name     string           `json:"name"`
	DisplayName string         `json:"display_name"`
}

type nominatimAddress struct {
	City        string `json:"city"`
	Town        string `json:"town"`
	Village     string `json:"village"`
	County      string `json:"county"`
	State       string `json:"state"`
	Country     string `json:"country"`
	CountryCode string `json:"country_code"`
}

// reverseGeocode uses Nominatim (OpenStreetMap) to get city info from coordinates
// zoom=8 returns prefecture-level city (地级市) for China
func reverseGeocode(lat, lon float64) ([]CityInfo, error) {
	url := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/reverse?lat=%f&lon=%f&format=json&accept-language=zh&zoom=6",
		lat, lon,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	// Nominatim requires a valid User-Agent
	req.Header.Set("User-Agent", "Diarum/1.0 (diary-app)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call Nominatim API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Nominatim API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read Nominatim response: %w", err)
	}

	var data nominatimResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse Nominatim response: %w", err)
	}

	// Get city name from address (prefer city > town > village)
	cityName := data.Address.City
	if cityName == "" {
		cityName = data.Address.Town
	}
	if cityName == "" {
		cityName = data.Address.Village
	}
	if cityName == "" {
		cityName = data.Name
	}

	if cityName == "" {
		return nil, fmt.Errorf("cannot determine city from coordinates")
	}

	// Remove suffixes that Open-Meteo doesn't support (e.g., "苏州市" -> "苏州")
	cityName = cleanCityName(cityName)

	return []CityInfo{{
		Name:     cityName,
		Lat:      lat,
		Lon:      lon,
		Province: data.Address.State,
		Country:  data.Address.Country,
	}}, nil
}

// cleanCityName removes administrative suffixes like 市、省、自治区 etc.
func cleanCityName(name string) string {
	suffixes := []string{"壮族自治区", "自治区", "特别行政区", "省", "市"}
	for _, suffix := range suffixes {
		if len(name) > len(suffix) && name[len(name)-len(suffix):] == suffix {
			return name[:len(name)-len(suffix)]
		}
	}
	return name
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
