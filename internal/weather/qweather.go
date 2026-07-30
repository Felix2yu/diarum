package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

var qweatherAPIKey string

func init() {
	qweatherAPIKey = os.Getenv("QWEATHER_API_KEY")
}

type qwDailyResponse struct {
	Code    string `json:"code"`
	Daily   []qwDaily `json:"daily"`
}

type qwDaily struct {
	FxDate   string `json:"fxDate"`
	TempMax  string `json:"tempMax"`
	TempMin  string `json:"tempMin"`
	IconDay  string `json:"iconDay"`
	TextDay  string `json:"textDay"`
}

func FetchFromQWeather(city string, lat, lon float64, date string) (*WeatherResult, error) {
	if qweatherAPIKey == "" {
		return nil, fmt.Errorf("QWEATHER_API_KEY not set")
	}

	targetDate := date
	if targetDate == "" {
		targetDate = time.Now().Format("2006-01-02")
	}

	url := fmt.Sprintf(
		"https://devapi.qweather.com/v7/weather/3d?location=%.4f,%.4f&key=%s",
		lon, lat, qweatherAPIKey,
	)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("qweather request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("qweather returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("qweather read failed: %w", err)
	}

	var data qwDailyResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("qweather parse failed: %w", err)
	}

	if data.Code != "200" {
		return nil, fmt.Errorf("qweather api error code: %s", data.Code)
	}

	for _, d := range data.Daily {
		if d.FxDate == targetDate {
			iconCode := 0
			fmt.Sscanf(d.IconDay, "%d", &iconCode)

			tempMax := 0.0
			tempMin := 0.0
			fmt.Sscanf(d.TempMax, "%f", &tempMax)
			fmt.Sscanf(d.TempMin, "%f", &tempMin)

			return &WeatherResult{
				City:    city,
				WMOCode: QWToWMO(iconCode),
				TempMin: tempMin,
				TempMax: tempMax,
				Date:    targetDate,
			}, nil
		}
	}

	return nil, fmt.Errorf("qweather no data for date %s", targetDate)
}
