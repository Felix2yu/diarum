package weather

import (
	"fmt"
	"sync"
)

// Service provides weather data from Open-Meteo API
type Service struct {
	cityCoords map[string][2]float64
	mu         sync.RWMutex
}

// NewService creates a new weather service
func NewService() *Service {
	s := &Service{
		cityCoords: make(map[string][2]float64),
	}
	s.initCityCoords()
	return s
}

// initCityCoords initializes city coordinates for direct API calls
func (s *Service) initCityCoords() {
	cities := map[string][2]float64{
		"北京": {39.9042, 116.4074},
		"上海": {31.2304, 121.4737},
		"广州": {23.1291, 113.2644},
		"深圳": {22.5431, 114.0579},
		"成都": {30.5728, 104.0668},
		"重庆": {29.4316, 106.9123},
		"杭州": {30.2741, 120.1551},
		"武汉": {30.5928, 114.3055},
		"西安": {34.3416, 108.9398},
		"苏州": {31.2990, 120.5853},
		"南京": {32.0603, 118.7969},
		"天津": {39.3434, 117.3616},
		"长沙": {28.2282, 112.9388},
		"沈阳": {41.8057, 123.4315},
		"哈尔滨": {45.8038, 126.5350},
		"昆明": {25.0389, 102.7183},
		"大连": {38.9140, 121.6147},
		"厦门": {24.4798, 118.0894},
		"青岛": {36.0671, 120.3826},
		"郑州": {34.7466, 113.6253},
		"济南": {36.6512, 116.9972},
		"福州": {26.0745, 119.2965},
		"东莞": {23.0208, 113.7518},
		"无锡": {31.4912, 120.3119},
		"合肥": {31.8206, 117.2272},
		"佛山": {23.0218, 113.1218},
		"长春": {43.8171, 125.3235},
		"温州": {27.9939, 120.6993},
		"石家庄": {38.0428, 114.5149},
		"南宁": {22.8170, 108.3665},
		"常州": {31.8106, 119.9741},
		"泉州": {24.8741, 118.6759},
		"南昌": {28.6820, 115.8579},
		"贵阳": {26.6470, 106.6302},
		"太原": {37.8706, 112.5489},
		"烟台": {37.4638, 121.4479},
		"嘉兴": {30.7539, 120.7585},
		"南通": {32.0603, 120.8649},
		"金华": {29.0789, 119.6495},
		"惠州": {23.1116, 114.4160},
		"徐州": {34.2610, 117.1947},
		"中山": {22.5170, 113.3926},
		"台州": {28.6563, 121.4208},
		"兰州": {36.0611, 103.8343},
		"绍兴": {30.0300, 120.5800},
		"乌鲁木齐": {43.8256, 87.6168},
		"扬州": {32.3942, 119.4130},
		"廊坊": {39.5168, 116.7044},
		"洛阳": {34.6197, 112.4540},
		"汕头": {23.3541, 116.6819},
		"呼和浩特": {40.8424, 111.7490},
		"海口": {20.0174, 110.3492},
		"银川": {38.4872, 106.2309},
		"西宁": {36.6171, 101.7782},
		"拉萨": {29.6500, 91.1000},
		"澳门": {22.1987, 113.5439},
		"香港": {22.3193, 114.1694},
		"台北": {25.0330, 121.5654},
		"高雄": {22.6273, 120.3014},
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range cities {
		s.cityCoords[k] = v
	}
}

// GetWeather fetches weather for a city on a specific date
func (s *Service) GetWeather(city string, date string) (*WeatherResult, error) {
	s.mu.RLock()
	coords, ok := s.cityCoords[city]
	s.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("city %q not found", city)
	}

	return FetchWeatherFromOpenMeteo(city, coords[0], coords[1], date)
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
