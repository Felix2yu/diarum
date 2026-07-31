package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// OpenMeteoResponse represents the Open-Meteo API response
type OpenMeteoResponse struct {
	Daily struct {
		WeatherCode    []int     `json:"weather_code"`
		Temperature2mMax []float64 `json:"temperature_2m_max"`
		Temperature2mMin []float64 `json:"temperature_2m_min"`
		Time           []string  `json:"time"`
	} `json:"daily"`
}

// FetchWeatherFromOpenMeteo fetches weather from Open-Meteo API for a specific date
func FetchWeatherFromOpenMeteo(city string, lat, lon float64, date string) (*WeatherResult, error) {
	targetDate := date
	if targetDate == "" {
		targetDate = time.Now().Format("2006-01-02")
	}

	// Determine if date is in the past (need archive API) or future (forecast API)
	today := time.Now().Format("2006-01-02")
	isPast := targetDate < today

	var url string
	if isPast {
		// Use archive API for historical data (available from 1940-01-01 to yesterday)
		url = fmt.Sprintf(
			"https://archive-api.open-meteo.com/v1/archive?latitude=%.4f&longitude=%.4f&start_date=%s&end_date=%s&daily=weather_code,temperature_2m_max,temperature_2m_min&timezone=Asia/Shanghai",
			lat, lon, targetDate, targetDate,
		)
	} else {
		// Use forecast API for today and future dates
		url = fmt.Sprintf(
			"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&daily=weather_code,temperature_2m_max,temperature_2m_min&timezone=Asia/Shanghai",
			lat, lon,
		)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch weather: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("open-meteo returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var data OpenMeteoResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(data.Daily.WeatherCode) == 0 {
		return nil, fmt.Errorf("no weather data available")
	}

	// Find the index for the requested date
	for i, d := range data.Daily.Time {
		if d == targetDate {
			return &WeatherResult{
				City:     city,
				WMOCode:  data.Daily.WeatherCode[i],
				TempMin:  data.Daily.Temperature2mMin[i],
				TempMax:  data.Daily.Temperature2mMax[i],
				Date:     targetDate,
				Lat:      lat,
				Lon:      lon,
				Provider: "openmeteo",
			}, nil
		}
	}

	// If date not found, return first available day
	return &WeatherResult{
		City:     city,
		WMOCode:  data.Daily.WeatherCode[0],
		TempMin:  data.Daily.Temperature2mMin[0],
		TempMax:  data.Daily.Temperature2mMax[0],
		Date:     data.Daily.Time[0],
		Lat:      lat,
		Lon:      lon,
		Provider: "openmeteo",
	}, nil
}
