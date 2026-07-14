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
// Based on WMO Code Table 4677 (Present Weather)
var WMOCodeInfo = map[int]struct {
	Label string
	Icon  string
}{
	// 00-09: 无降水、无雾、无沙尘
	0:  {Label: "晴", Icon: "☀️"},
	1:  {Label: "晴转多云", Icon: "🌤️"},
	2:  {Label: "多云", Icon: "⛅"},
	3:  {Label: "阴天", Icon: "☁️"},
	4:  {Label: "烟雾", Icon: "🌫️"},
	5:  {Label: "霾", Icon: "🌫️"},
	6:  {Label: "浮尘", Icon: "🌫️"},
	7:  {Label: "扬沙", Icon: "🌫️"},
	8:  {Label: "沙尘暴", Icon: "🌪️"},
	9:  {Label: "强沙尘暴", Icon: "🌪️"},

	// 10-19: 特殊天气现象
	10: {Label: "薄雾", Icon: "🌫️"},
	11: {Label: "浅雾", Icon: "🌫️"},
	12: {Label: "雾凇", Icon: "🌫️"},
	13: {Label: "远处闪电", Icon: "⚡"},
	14: {Label: "远处降水", Icon: "🌦️"},
	15: {Label: "远处雷暴", Icon: "⛈️"},
	16: {Label: "近处雷暴", Icon: "⛈️"},
	17: {Label: "雷暴", Icon: "⛈️"},
	18: {Label: "飑", Icon: "🌬️"},
	19: {Label: "漏斗云", Icon: "🌪️"},

	// 20-29: 过去一小时曾出现但已停止
	20: {Label: "毛毛雨已停", Icon: "🌦️"},
	21: {Label: "小雨已停", Icon: "🌧️"},
	22: {Label: "雪已停", Icon: "❄️"},
	23: {Label: "雨夹雪已停", Icon: "🌨️"},
	24: {Label: "阵雨已停", Icon: "🌦️"},
	25: {Label: "阵雪已停", Icon: "🌨️"},
	26: {Label: "冰雹已停", Icon: "🧊"},
	27: {Label: "雾已停", Icon: "🌫️"},
	28: {Label: "雾凇已停", Icon: "🌫️"},
	29: {Label: "雷暴已停", Icon: "⛈️"},

	// 30-39: 沙尘暴与吹雪
	30: {Label: "弱沙尘暴", Icon: "🌪️"},
	31: {Label: "沙尘暴", Icon: "🌪️"},
	32: {Label: "强沙尘暴", Icon: "🌪️"},
	33: {Label: "特强沙尘暴", Icon: "🌪️"},
	34: {Label: "减弱沙尘暴", Icon: "🌪️"},
	35: {Label: "增强沙尘暴", Icon: "🌪️"},
	36: {Label: "低吹雪", Icon: "🌨️"},
	37: {Label: "强低吹雪", Icon: "🌨️"},
	38: {Label: "高吹雪", Icon: "❄️"},
	39: {Label: "强高吹雪", Icon: "❄️"},

	// 40-49: 雾与冰雾
	40: {Label: "近处有雾", Icon: "🌫️"},
	41: {Label: "雾", Icon: "🌫️"},
	42: {Label: "雾变薄", Icon: "🌫️"},
	43: {Label: "雾无变化", Icon: "🌫️"},
	44: {Label: "雾变厚", Icon: "🌫️"},
	45: {Label: "雾", Icon: "🌫️"},
	46: {Label: "冻雾", Icon: "🌫️"},
	47: {Label: "浓冻雾", Icon: "🌫️"},
	48: {Label: "雾凇", Icon: "🌫️"},
	49: {Label: "浓雾凇", Icon: "🌫️"},

	// 50-59: 毛毛雨
	50: {Label: "间歇毛毛雨", Icon: "🌦️"},
	51: {Label: "连续毛毛雨", Icon: "🌦️"},
	52: {Label: "中毛毛雨", Icon: "🌦️"},
	53: {Label: "中连续毛毛雨", Icon: "🌦️"},
	54: {Label: "重毛毛雨", Icon: "🌧️"},
	55: {Label: "重连续毛毛雨", Icon: "🌧️"},
	56: {Label: "冻毛毛雨", Icon: "🌧️"},
	57: {Label: "重冻毛毛雨", Icon: "🌧️"},
	58: {Label: "轻毛毛雨夹雨", Icon: "🌦️"},
	59: {Label: "重毛毛雨夹雨", Icon: "🌧️"},

	// 60-69: 雨
	60: {Label: "间歇小雨", Icon: "🌧️"},
	61: {Label: "连续小雨", Icon: "🌧️"},
	62: {Label: "中雨", Icon: "🌧️"},
	63: {Label: "中连续雨", Icon: "🌧️"},
	64: {Label: "大雨", Icon: "🌧️"},
	65: {Label: "重连续雨", Icon: "🌧️"},
	66: {Label: "冻雨", Icon: "🌧️"},
	67: {Label: "重冻雨", Icon: "🌧️"},
	68: {Label: "雨夹雪", Icon: "🌨️"},
	69: {Label: "重雨夹雪", Icon: "🌨️"},

	// 70-79: 固态降水（非阵性）
	70: {Label: "间歇小雪", Icon: "❄️"},
	71: {Label: "连续小雪", Icon: "❄️"},
	72: {Label: "中雪", Icon: "❄️"},
	73: {Label: "中连续雪", Icon: "❄️"},
	74: {Label: "大雪", Icon: "❄️"},
	75: {Label: "重连续雪", Icon: "❄️"},
	76: {Label: "钻石尘", Icon: "✨"},
	77: {Label: "雪粒", Icon: "❄️"},
	78: {Label: "星形雪晶", Icon: "❄️"},
	79: {Label: "冰粒", Icon: "🧊"},

	// 80-89: 阵性降水
	80: {Label: "小阵雨", Icon: "🌦️"},
	81: {Label: "阵雨", Icon: "🌦️"},
	82: {Label: "强阵雨", Icon: "⛈️"},
	83: {Label: "小阵雨夹雪", Icon: "🌨️"},
	84: {Label: "大阵雨夹雪", Icon: "🌨️"},
	85: {Label: "小阵雪", Icon: "🌨️"},
	86: {Label: "阵雪", Icon: "🌨️"},
	87: {Label: "小阵冰雹", Icon: "🧊"},
	88: {Label: "大阵冰雹", Icon: "🧊"},
	89: {Label: "小阵雨夹冰雹", Icon: "🧊"},

	// 90-99: 雷暴
	90: {Label: "大阵雨夹冰雹", Icon: "🧊"},
	91: {Label: "小雷暴有雨", Icon: "⛈️"},
	92: {Label: "大雷暴有雨", Icon: "⛈️"},
	93: {Label: "小雷暴有冰雹", Icon: "⛈️"},
	94: {Label: "大雷暴有冰雹", Icon: "⛈️"},
	95: {Label: "雷暴", Icon: "⛈️"},
	96: {Label: "雷暴有冰雹", Icon: "⛈️"},
	97: {Label: "重雷暴有冰雹", Icon: "⛈️"},
	98: {Label: "雷暴有沙尘", Icon: "⛈️"},
	99: {Label: "重雷暴有冰雹", Icon: "⛈️"},
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
