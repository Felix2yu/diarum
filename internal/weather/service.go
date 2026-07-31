package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/songtianlun/diarum/internal/logger"
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

// SearchCities searches for cities by name.
// Priority: QWeather (if API key configured) → Open-Meteo.
// If query contains comma-separated coordinates (lat,lon), uses Nominatim reverse geocoding.
func SearchCities(query string) ([]CityInfo, error) {
	var lat, lon float64
	if n, err := fmt.Sscanf(query, "%f,%f", &lat, &lon); err == nil && n == 2 {
		cities, err := reverseGeocode(lat, lon)
		if err == nil {
			logger.Info("[WeatherCity] %s → Nominatim reverse: %d results", query, len(cities))
		}
		return cities, err
	}

	if qweatherAPIKey != "" {
		cities, err := searchQWeather(query)
		if err == nil {
			logger.Info("[WeatherCity] %s → QWeather: %d results", query, len(cities))
			return cities, nil
		}
		logger.Debug("[WeatherCity] %s → QWeather failed: %v, fallback to Open-Meteo", query, err)
	}

	cities, err := searchOpenMeteo(query)
	if err == nil {
		logger.Info("[WeatherCity] %s → Open-Meteo: %d results", query, len(cities))
	} else {
		logger.Debug("[WeatherCity] %s → all providers failed: %v", query, err)
	}
	return cities, err
}

// searchQWeather searches cities via QWeather city lookup API.
// Tries key param first, falls back to Bearer token auth.
func searchQWeather(query string) ([]CityInfo, error) {
	baseURL := "https://" + qweatherHost() + "/geo/v2/city/lookup?location=%s&number=10"
	encodedQuery := url.QueryEscape(query)

	cities, err := searchQwRequest(fmt.Sprintf(baseURL+"&key=%s", encodedQuery, qweatherAPIKey))
	if err == nil {
		return cities, nil
	}

	return searchQwRequestBearer(fmt.Sprintf(baseURL, encodedQuery), qweatherAPIKey)
}

func searchQwRequest(apiURL string) ([]CityInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("QWeather geo request failed: %w", err)
	}
	defer resp.Body.Close()
	return parseQwSearchResponse(resp)
}

func searchQwRequestBearer(apiURL, token string) ([]CityInfo, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("QWeather geo request failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("QWeather geo request (Bearer) failed: %w", err)
	}
	defer resp.Body.Close()
	return parseQwSearchResponse(resp)
}

func parseQwSearchResponse(resp *http.Response) ([]CityInfo, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("QWeather geo read failed: %w", err)
	}

	var data qwGeoResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf(
			"QWeather geo parse failed (status %d, body: %s): %w",
			resp.StatusCode, string(body), err,
		)
	}

	if data.Code != "200" {
		return nil, fmt.Errorf("QWeather geo api error (code: %s, status %d)", data.Code, resp.StatusCode)
	}

	cities := make([]CityInfo, 0, len(data.Location))
	for _, loc := range data.Location {
		var clat, clon float64
		fmt.Sscanf(loc.Lat, "%f", &clat)
		fmt.Sscanf(loc.Lon, "%f", &clon)
		cities = append(cities, CityInfo{
			Name:     loc.Name,
			Lat:      clat,
			Lon:      clon,
			Province: loc.Adm1,
			Country:  "中国",
		})
	}
	return cities, nil
}

// searchOpenMeteo searches cities via Open-Meteo geocoding API
func searchOpenMeteo(query string) ([]CityInfo, error) {
	apiURL := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&count=10&language=zh",
		url.QueryEscape(query),
	)

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", "Diarum/1.0 (diary-app)")

	resp, err := client.Do(req)
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

// NominatimError is returned when Nominatim API returns a non-200 status
type NominatimError struct {
	StatusCode int
}

func (e *NominatimError) Error() string {
	return fmt.Sprintf("Nominatim API returned status %d", e.StatusCode)
}

// reverseGeocode uses Nominatim (OpenStreetMap) to get city info from coordinates
// zoom=8 returns prefecture-level city (地级市) for China
func reverseGeocode(lat, lon float64) ([]CityInfo, error) {
	url := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/reverse?lat=%f&lon=%f&format=json&accept-language=zh",
		lat, lon,
	)

	client := &http.Client{Timeout: 10 * time.Second}

	// Retry up to 3 times for rate limiting (429) and server errors (5xx)
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("User-Agent", "Diarum/1.0 (diary-app)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to call Nominatim API: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read Nominatim response: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = &NominatimError{StatusCode: resp.StatusCode}
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return nil, &NominatimError{StatusCode: resp.StatusCode}
		}

		var data nominatimResponse
		if err := json.Unmarshal(body, &data); err != nil {
			return nil, fmt.Errorf("failed to parse Nominatim response: %w", err)
		}

		return parseNominatimResult(data, lat, lon)
	}

	return nil, lastErr
}

func parseNominatimResult(data nominatimResponse, lat, lon float64) ([]CityInfo, error) {

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

// geocodeCity gets coordinates for a city.
// Priority: QWeather (if API key configured) → Open-Meteo → Nominatim.
func geocodeCity(city string) (lat, lon float64, err error) {
	if qweatherAPIKey != "" {
		lat, lon, err = geocodeWithQWeather(city)
		if err == nil {
			logger.Info("[WeatherGeo] %s → QWeather (%.4f, %.4f)", city, lat, lon)
			return lat, lon, nil
		}
		logger.Debug("[WeatherGeo] %s → QWeather failed: %v, fallback to Open-Meteo", city, err)
	}

	lat, lon, err = geocodeWithOpenMeteo(city)
	if err == nil {
		logger.Info("[WeatherGeo] %s → Open-Meteo (%.4f, %.4f)", city, lat, lon)
		return lat, lon, nil
	}
	logger.Debug("[WeatherGeo] %s → Open-Meteo failed: %v, fallback to Nominatim", city, err)

	lat, lon, err = geocodeWithNominatim(city)
	if err == nil {
		logger.Info("[WeatherGeo] %s → Nominatim (%.4f, %.4f)", city, lat, lon)
		return lat, lon, nil
	}

	return 0, 0, err
}

// qwGeoResponse represents the QWeather city lookup API response
type qwGeoResponse struct {
	Code     string        `json:"code"`
	Location []qwGeoResult `json:"location"`
}

type qwGeoResult struct {
	Name string `json:"name"`
	Lat  string `json:"lat"`
	Lon  string `json:"lon"`
	Adm1 string `json:"adm1"`
}

// geocodeWithQWeather uses QWeather city lookup API.
// Tries key param first, falls back to Bearer token auth.
func geocodeWithQWeather(city string) (lat, lon float64, err error) {
	baseURL := "https://" + qweatherHost() + "/geo/v2/city/lookup?location=%s&number=1"
	encodedCity := url.QueryEscape(city)

	// Try key param auth first
	lat, lon, err = qwGeoRequest(fmt.Sprintf(baseURL+"&key=%s", encodedCity, qweatherAPIKey))
	if err == nil {
		return lat, lon, nil
	}

	// Fall back to Bearer token auth
	return qwGeoRequestBearer(fmt.Sprintf(baseURL, encodedCity), qweatherAPIKey)
}

func qwGeoRequest(apiURL string) (lat, lon float64, err error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(apiURL)
	if err != nil {
		return 0, 0, fmt.Errorf("QWeather geo request failed: %w", err)
	}
	defer resp.Body.Close()
	return parseQwGeoResponse(resp)
}

func qwGeoRequestBearer(apiURL, token string) (lat, lon float64, err error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return 0, 0, fmt.Errorf("QWeather geo request failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, fmt.Errorf("QWeather geo request (Bearer) failed: %w", err)
	}
	defer resp.Body.Close()
	return parseQwGeoResponse(resp)
}

func parseQwGeoResponse(resp *http.Response) (lat, lon float64, err error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, 0, fmt.Errorf("QWeather geo read failed: %w", err)
	}

	var data qwGeoResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return 0, 0, fmt.Errorf(
			"QWeather geo parse failed (status %d, body: %s): %w",
			resp.StatusCode, string(body), err,
		)
	}

	if data.Code != "200" {
		return 0, 0, fmt.Errorf("QWeather geo api error (code: %s, status %d)", data.Code, resp.StatusCode)
	}

	if len(data.Location) == 0 {
		return 0, 0, fmt.Errorf("city not found via QWeather")
	}

	if _, err := fmt.Sscanf(data.Location[0].Lat, "%f", &lat); err != nil {
		return 0, 0, fmt.Errorf("QWeather geo parse lat failed: %w", err)
	}
	if _, err := fmt.Sscanf(data.Location[0].Lon, "%f", &lon); err != nil {
		return 0, 0, fmt.Errorf("QWeather geo parse lon failed: %w", err)
	}

	return lat, lon, nil
}

// geocodeWithOpenMeteo calls Open-Meteo forward geocoding API
func geocodeWithOpenMeteo(city string) (lat, lon float64, err error) {
	apiURL := fmt.Sprintf(
		"https://geocoding-api.open-meteo.com/v1/search?name=%s&count=1&language=zh",
		url.QueryEscape(city),
	)

	client := &http.Client{Timeout: 10 * time.Second}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("User-Agent", "Diarum/1.0 (diary-app)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to call geocoding API: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read geocoding response: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("geocoding API returned status %d", resp.StatusCode)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return 0, 0, fmt.Errorf("geocoding API returned status %d", resp.StatusCode)
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

	return 0, 0, lastErr
}

// geocodeWithNominatim uses Nominatim (OpenStreetMap) search API as fallback
func geocodeWithNominatim(city string) (lat, lon float64, err error) {
	apiURL := fmt.Sprintf(
		"https://nominatim.openstreetmap.org/search?q=%s&format=json&limit=1&accept-language=zh",
		url.QueryEscape(city),
	)

	client := &http.Client{Timeout: 10 * time.Second}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * time.Second)
		}

		req, err := http.NewRequest("GET", apiURL, nil)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to create request: %w", err)
		}
		req.Header.Set("User-Agent", "Diarum/1.0 (diary-app)")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("failed to call Nominatim search API: %w", err)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = fmt.Errorf("failed to read Nominatim response: %w", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("Nominatim search API returned status %d", resp.StatusCode)
			continue
		}

		var results []struct {
			Lat string `json:"lat"`
			Lon string `json:"lon"`
		}
		if err := json.Unmarshal(body, &results); err != nil {
			return 0, 0, fmt.Errorf("failed to parse Nominatim response: %w", err)
		}

		if len(results) == 0 {
			return 0, 0, fmt.Errorf("city %q not found via Nominatim", city)
		}

		if _, err := fmt.Sscanf(results[0].Lat, "%f", &lat); err != nil {
			return 0, 0, fmt.Errorf("failed to parse latitude: %w", err)
		}
		if _, err := fmt.Sscanf(results[0].Lon, "%f", &lon); err != nil {
			return 0, 0, fmt.Errorf("failed to parse longitude: %w", err)
		}

		return lat, lon, nil
	}

	return 0, 0, lastErr
}

// GetWeather fetches weather for a city on a specific date
// Priority: QWeather first, Open-Meteo as fallback
func (s *Service) GetWeather(city string, date string) (*WeatherResult, error) {
	lat, lon, err := s.getCoords(city)
	if err != nil {
		return nil, err
	}

	return s.fetchWithFallback(city, lat, lon, date)
}

// GetWeatherByCoords fetches weather by coordinates on a specific date
func (s *Service) GetWeatherByCoords(city string, lat, lon float64, date string) (*WeatherResult, error) {
	return s.fetchWithFallback(city, lat, lon, date)
}

// fetchWithFallback tries QWeather first, falls back to Open-Meteo
func (s *Service) fetchWithFallback(city string, lat, lon float64, date string) (*WeatherResult, error) {
	result, err := FetchFromQWeather(city, lat, lon, date)
	if err == nil {
		logger.Info("[WeatherAPI] %s %s → QWeather", city, date)
		return result, nil
	}
	logger.Debug("[WeatherAPI] %s %s → QWeather failed: %v, fallback to Open-Meteo", city, date, err)

	result, err = FetchWeatherFromOpenMeteo(city, lat, lon, date)
	if err == nil {
		logger.Info("[WeatherAPI] %s %s → Open-Meteo", city, date)
		return result, nil
	}

	return nil, err
}

// SetCityCoords adds or updates city coordinates
func (s *Service) SetCityCoords(city string, lat, lon float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cityCoords[city] = [2]float64{lat, lon}
}
