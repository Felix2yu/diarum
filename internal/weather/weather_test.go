package weather

import (
	"os"
	"testing"
)

func TestGetWMOInfo(t *testing.T) {
	tests := []struct {
		code  int
		label string
		icon  string
	}{
		{0, "晴", "☀️"},
		{2, "多云", "⛅"},
		{3, "阴", "☁️"},
		{45, "雾/霾", "🌫️"},
		{51, "雨", "🌧️"},
		{61, "雨", "🌧️"},
		{71, "雪", "❄️"},
		{95, "雷暴", "⛈️"},
		{999, "晴", "☀️"},
	}

	for _, tt := range tests {
		label, icon := GetWMOInfo(tt.code)
		if label != tt.label {
			t.Errorf("GetWMOInfo(%d) label = %q, want %q", tt.code, label, tt.label)
		}
		if icon != tt.icon {
			t.Errorf("GetWMOInfo(%d) icon = %q, want %q", tt.code, icon, tt.icon)
		}
	}
}

func TestFormatDisplay(t *testing.T) {
	result := FormatDisplay(61, 12.5, 18.3)
	expected := "🌧️ 雨 12°~18°"
	if result != expected {
		t.Errorf("FormatDisplay(61, 12.5, 18.3) = %q, want %q", result, expected)
	}
}

func TestWMOToSimple(t *testing.T) {
	tests := []struct {
		wmo    int
		simple int
	}{
		{0, WeatherClear},
		{1, WeatherCloudy},
		{2, WeatherCloudy},
		{3, WeatherOvercast},
		{45, WeatherFoggy},
		{51, WeatherRain},
		{61, WeatherRain},
		{95, WeatherThunder},
		{71, WeatherSnow},
		{18, WeatherWindy},
		{8, WeatherDust},
		{68, WeatherSleet},
	}

	for _, tt := range tests {
		got := WMOToSimple(tt.wmo)
		if got != tt.simple {
			t.Errorf("WMOToSimple(%d) = %d, want %d", tt.wmo, got, tt.simple)
		}
	}
}

func TestQWToWMO(t *testing.T) {
	tests := []struct {
		qw  int
		wmo int
	}{
		{100, 0},
		{104, 3},
		{305, 61},
		{302, 95},
		{400, 71},
		{404, 68},
		{501, 45},
		{507, 8},
	}

	for _, tt := range tests {
		got := QWToWMO(tt.qw)
		if got != tt.wmo {
			t.Errorf("QWToWMO(%d) = %d, want %d", tt.qw, got, tt.wmo)
		}
	}
}

func TestFetchWeatherFromOpenMeteo(t *testing.T) {
	result, err := FetchWeatherFromOpenMeteo("北京", 39.9042, 116.4074, "")
	if err != nil {
		t.Fatalf("FetchWeatherFromOpenMeteo failed: %v", err)
	}

	if result.City != "北京" {
		t.Errorf("City = %q, want 北京", result.City)
	}

	if result.WMOCode < 0 || result.WMOCode > 99 {
		t.Errorf("WMOCode = %d, want 0-99", result.WMOCode)
	}

	t.Logf("Weather: %s", FormatDisplay(result.WMOCode, result.TempMin, result.TempMax))
	t.Logf("Temp: %.1f°C ~ %.1f°C", result.TempMin, result.TempMax)
}

func TestServiceGetWeather(t *testing.T) {
	if _, ok := os.LookupEnv("QWEATHER_API_KEY"); !ok {
		t.Skip("QWEATHER_API_KEY not set, skipping integration test")
	}

	svc := NewService()

	result, err := svc.GetWeather("北京", "")
	if err != nil {
		t.Fatalf("GetWeather failed: %v", err)
	}

	if result.City != "北京" {
		t.Errorf("City = %q, want 北京", result.City)
	}

	t.Logf("Weather for 北京: %s", FormatDisplay(result.WMOCode, result.TempMin, result.TempMax))
}
