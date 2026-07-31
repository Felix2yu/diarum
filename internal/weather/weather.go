package weather

import "fmt"

const (
	WeatherClear    = 0
	WeatherCloudy   = 1
	WeatherOvercast = 2
	WeatherFoggy    = 3
	WeatherRain     = 4
	WeatherSnow     = 5
	WeatherSleet    = 6
	WeatherThunder  = 7
	WeatherWindy    = 8
	WeatherDust     = 9
)

var SimpleCodeInfo = map[int]struct {
	Label string
	Icon  string
}{
	WeatherClear:    {Label: "晴", Icon: "☀️"},
	WeatherCloudy:   {Label: "多云", Icon: "⛅"},
	WeatherOvercast: {Label: "阴", Icon: "☁️"},
	WeatherFoggy:    {Label: "雾/霾", Icon: "🌫️"},
	WeatherRain:     {Label: "雨", Icon: "🌧️"},
	WeatherSnow:     {Label: "雪", Icon: "❄️"},
	WeatherSleet:    {Label: "雨夹雪", Icon: "🌨️"},
	WeatherThunder:  {Label: "雷暴", Icon: "⛈️"},
	WeatherWindy:    {Label: "大风", Icon: "💨"},
	WeatherDust:     {Label: "沙尘", Icon: "🌪️"},
}

type WeatherResult struct {
	City     string  `json:"city"`
	WMOCode  int     `json:"wmo_code"`
	TempMin  float64 `json:"temp_min"`
	TempMax  float64 `json:"temp_max"`
	Date     string  `json:"date"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	Provider string  `json:"provider,omitempty"`
}

func WMOToSimple(wmo int) int {
	switch {
	case wmo == 0:
		return WeatherClear
	case wmo == 1 || wmo == 2:
		return WeatherCloudy
	case wmo == 3:
		return WeatherOvercast
	case wmo >= 4 && wmo <= 7, wmo >= 10 && wmo <= 12, wmo >= 40 && wmo <= 49, wmo == 27 || wmo == 28:
		return WeatherFoggy
	case wmo == 8 || wmo == 9, wmo >= 30 && wmo <= 35, wmo == 19:
		return WeatherDust
	case wmo == 18:
		return WeatherWindy
	case wmo >= 13 && wmo <= 17, wmo == 29, wmo >= 90 && wmo <= 99:
		return WeatherThunder
	case wmo == 68 || wmo == 69 || wmo == 83 || wmo == 84:
		return WeatherSleet
	case wmo >= 50 && wmo <= 69, wmo >= 80 && wmo <= 82:
		return WeatherRain
	case wmo >= 70 && wmo <= 79, wmo >= 85 && wmo <= 86:
		if wmo == 76 {
			return WeatherClear
		}
		return WeatherSnow
	case wmo >= 20 && wmo <= 26:
		return mapPastWMO(wmo)
	case wmo >= 36 && wmo <= 39:
		return WeatherSnow
	case wmo >= 87 && wmo <= 89:
		return WeatherRain
	default:
		return WeatherClear
	}
}

func mapPastWMO(wmo int) int {
	switch wmo {
	case 20, 24:
		return WeatherRain
	case 21:
		return WeatherRain
	case 22, 25:
		return WeatherSnow
	case 23:
		return WeatherSleet
	case 26:
		return WeatherRain
	default:
		return WeatherClear
	}
}

func QWToWMO(qwCode int) int {
	switch qwCode {
	case 100, 150:
		return 0
	case 101, 102, 103, 151, 152, 153:
		return 2
	case 104:
		return 3
	case 300, 350:
		return 80
	case 301, 351:
		return 81
	case 302, 303, 304:
		return 95
	case 305:
		return 61
	case 306, 314, 315:
		return 63
	case 307, 316:
		return 65
	case 308:
		return 65
	case 309:
		return 51
	case 310, 311, 312:
		return 82
	case 313:
		return 66
	case 317, 318:
		return 82
	case 399:
		return 61
	case 400, 408:
		return 71
	case 401, 409:
		return 73
	case 402, 403, 410:
		return 75
	case 404, 405, 406, 456:
		return 68
	case 407, 457:
		return 85
	case 499:
		return 71
	case 500, 501:
		return 45
	case 502, 511, 512, 513:
		return 5
	case 503, 504:
		return 6
	case 507, 508:
		return 8
	case 509, 510, 514, 515:
		return 49
	case 900, 901:
		return 0
	default:
		return 0
	}
}

func GetWMOInfo(code int) (label, icon string) {
	simple := WMOToSimple(code)
	if info, ok := SimpleCodeInfo[simple]; ok {
		return info.Label, info.Icon
	}
	return "未知", "❓"
}

func FormatDisplay(code int, tempMin, tempMax float64) string {
	label, icon := GetWMOInfo(code)
	return icon + " " + label + " " + formatTemp(tempMin) + "~" + formatTemp(tempMax)
}

func formatTemp(t float64) string {
	return fmt.Sprintf("%.0f", t) + "°"
}
