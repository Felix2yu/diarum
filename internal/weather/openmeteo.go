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

// FetchWeatherFromOpenMeteo fetches weather from Open-Meteo API
func FetchWeatherFromOpenMeteo(city string, lat, lon float64) (*WeatherResult, error) {
	url := fmt.Sprintf(
		"https://api.open-meteo.com/v1/forecast?latitude=%.4f&longitude=%.4f&daily=weather_code,temperature_2m_max,temperature_2m_min&timezone=Asia/Shanghai",
		lat, lon,
	)

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

	// Get today's data (index 0)
	today := time.Now().Format("2006-01-02")

	return &WeatherResult{
		City:    city,
		WMOCode: data.Daily.WeatherCode[0],
		TempMin: data.Daily.Temperature2mMin[0],
		TempMax: data.Daily.Temperature2mMax[0],
		Date:    today,
	}, nil
}
