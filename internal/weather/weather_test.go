package weather

import (
	"testing"
)

func TestGetWMOInfo(t *testing.T) {
	tests := []struct {
		code  int
		label string
		icon  string
	}{
		{0, "晴", "☀️"},
		{1, "少云", "🌤️"},
		{2, "多云", "⛅"},
		{3, "阴天", "☁️"},
		{45, "雾", "🌫️"},
		{51, "毛毛雨", "🌦️"},
		{61, "小雨", "🌧️"},
		{63, "中雨", "🌧️"},
		{65, "大雨", "🌧️"},
		{71, "小雪", "❄️"},
		{73, "中雪", "❄️"},
		{75, "大雪", "❄️"},
		{80, "小阵雨", "🌦️"},
		{95, "雷雨", "⛈️"},
		{999, "未知", "❓"},
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
	expected := "🌧️ 小雨 12°~18°"
	if result != expected {
		t.Errorf("FormatDisplay(61, 12.5, 18.3) = %q, want %q", result, expected)
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
	svc := NewService("http://localhost:8080", false)

	result, err := svc.GetWeather("北京", "")
	if err != nil {
		t.Fatalf("GetWeather failed: %v", err)
	}

	if result.City != "北京" {
		t.Errorf("City = %q, want 北京", result.City)
	}

	t.Logf("Weather for 北京: %s", FormatDisplay(result.WMOCode, result.TempMin, result.TempMax))
}
