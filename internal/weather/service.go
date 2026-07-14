package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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
	// Don't use zoom parameter - let Nominatim return full address
	// Then we extract the right level ourselves
	url := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/reverse?lat=%f&lon=%f&format=json&accept-language=zh",
		lat, lon,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
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

	// Extract city name by priority:
	// 1. address.city - check if it's prefecture-level (地级市/直辖市/特区)
	// 2. display_name parsing - extract prefecture-level city
	// 3. fallback to town/village
	cityName := data.Address.City

	// If address.city is district/county level (contains 区/县/旗), extract from display_name
	if cityName != "" && (strings.Contains(cityName, "区") || strings.Contains(cityName, "县") || strings.Contains(cityName, "旗")) {
		if data.DisplayName != "" {
			cityName = extractPrefectureCity(data.DisplayName, data.Address.State)
		}
	}

	// If still no valid city, try display_name
	if cityName == "" && data.DisplayName != "" {
		cityName = extractPrefectureCity(data.DisplayName, data.Address.State)
	}

	// If still no city, fallback to town/village
	if cityName == "" {
		cityName = data.Address.Town
	}
	if cityName == "" {
		cityName = data.Address.Village
	}

	if cityName == "" {
		return nil, fmt.Errorf("该位置无法确定城市，请手动选择")
	}

	// Check if result is still district level
	if strings.HasSuffix(cityName, "区") || strings.HasSuffix(cityName, "县") {
		return nil, fmt.Errorf("该位置为区县级行政区（%s），请手动选择城市", cityName)
	}

	// Remove administrative suffixes (苏州市 -> 苏州, 香港特别行政区 -> 香港)
	cityName = cleanCityName(cityName)

	return []CityInfo{{
		Name:     cityName,
		Lat:      lat,
		Lon:      lon,
		Province: data.Address.State,
		Country:  data.Address.Country,
	}}, nil
}

// extractPrefectureCity extracts prefecture-level city from display_name
// display_name format: "姑苏区, 苏州市, 江苏省, 中国" or "姑苏区，苏州市，江苏省，中国"
func extractPrefectureCity(displayName, state string) string {
	// Handle both English comma and Chinese comma
	displayName = strings.ReplaceAll(displayName, "，", ",")
	parts := strings.Split(displayName, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		// Look for city-level entities
		if strings.HasSuffix(part, "市") ||
			strings.Contains(part, "特别行政区") ||
			strings.Contains(part, "自治州") ||
			strings.Contains(part, "盟") || // 内蒙古盟级
			strings.Contains(part, "地区") {
			return part
		}
	}
	// If no city found, try to find something that's not the state/province
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != state && part != "" {
			return part
		}
	}
	return ""
}

// cleanCityName removes administrative suffixes
// 苏州市 -> 苏州, 香港特别行政区 -> 香港, 延边朝鲜族自治州 -> 延边
func cleanCityName(name string) string {
	suffixes := []string{
		"壮族自治区", "回族自治区", "维吾尔自治区", "自治区",
		"特别行政区",
		"朝鲜族自治州", "土家族苗族自治州", "藏族自治州", "蒙古自治州", "自治州",
		"省", "市", "地区",
	}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) && len(name) > len(suffix) {
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
