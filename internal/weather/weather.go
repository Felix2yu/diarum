package weather

import "fmt"

// WeatherResult represents weather data for a location
type WeatherResult struct {
	City    string  `json:"city"`
	WMOCode int     `json:"wmo_code"`
	TempMin float64 `json:"temp_min"`
	TempMax float64 `json:"temp_max"`
	Date    string  `json:"date"`
}

// WMOCodeInfo maps WMO weather codes to Chinese labels and emoji icons
var WMOCodeInfo = map[int]struct {
	Label string
	Icon  string
}{
	0:  {Label: "晴", Icon: "☀️"},
	1:  {Label: "少云", Icon: "🌤️"},
	2:  {Label: "多云", Icon: "⛅"},
	3:  {Label: "阴天", Icon: "☁️"},
	45: {Label: "雾", Icon: "🌫️"},
	48: {Label: "冻雾", Icon: "🌫️"},
	51: {Label: "毛毛雨", Icon: "🌦️"},
	53: {Label: "毛毛雨", Icon: "🌦️"},
	55: {Label: "毛毛雨", Icon: "🌦️"},
	56: {Label: "冻毛毛雨", Icon: "🌧️"},
	57: {Label: "冻毛毛雨", Icon: "🌧️"},
	61: {Label: "小雨", Icon: "🌧️"},
	63: {Label: "中雨", Icon: "🌧️"},
	65: {Label: "大雨", Icon: "🌧️"},
	66: {Label: "冻雨", Icon: "🌧️"},
	67: {Label: "冻雨", Icon: "🌧️"},
	71: {Label: "小雪", Icon: "❄️"},
	73: {Label: "中雪", Icon: "❄️"},
	75: {Label: "大雪", Icon: "❄️"},
	77: {Label: "雪粒", Icon: "❄️"},
	80: {Label: "小阵雨", Icon: "🌦️"},
	81: {Label: "阵雨", Icon: "🌦️"},
	82: {Label: "强阵雨", Icon: "⛈️"},
	85: {Label: "小阵雪", Icon: "🌨️"},
	86: {Label: "阵雪", Icon: "🌨️"},
	95: {Label: "雷雨", Icon: "⛈️"},
	96: {Label: "雷雨冰雹", Icon: "⛈️"},
	99: {Label: "雷雨冰雹", Icon: "⛈️"},
}

// GetWMOInfo returns label and icon for a WMO code
func GetWMOInfo(code int) (label, icon string) {
	if info, ok := WMOCodeInfo[code]; ok {
		return info.Label, info.Icon
	}
	return "未知", "❓"
}

// FormatDisplay returns formatted weather string like "🌧️ 小雨 12°~18°"
func FormatDisplay(code int, tempMin, tempMax float64) string {
	label, icon := GetWMOInfo(code)
	return icon + " " + label + " " + formatTemp(tempMin) + "~" + formatTemp(tempMax)
}

func formatTemp(t float64) string {
	return formatFloat(t) + "°"
}

func formatFloat(f float64) string {
	return fmt.Sprintf("%.0f", f)
}
