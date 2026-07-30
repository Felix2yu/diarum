package weather

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
)

// citySetting represents the weather.default_city stored as JSON with coordinates.
type citySetting struct {
	Name string  `json:"name"`
	Lat  float64 `json:"lat"`
	Lon  float64 `json:"lon"`
}

type Scheduler struct {
	store         *store.Store
	configService *config.ConfigService
	weatherSvc    *Service
	mu            sync.Mutex
	userTimers    map[string]*time.Timer
}

func NewScheduler(s *store.Store, cfg *config.ConfigService, svc *Service) *Scheduler {
	return &Scheduler{
		store:         s,
		configService: cfg,
		weatherSvc:    svc,
		userTimers:    make(map[string]*time.Timer),
	}
}

func (sc *Scheduler) Start() {
	users, err := sc.store.ListUsers()
	if err != nil {
		logger.Error("[WeatherAuto] failed to list users: %v", err)
		return
	}
	for _, u := range users {
		sc.Refresh(u.ID)
	}
	logger.Info("[WeatherAuto] scheduler started for %d users", len(users))
}

func (sc *Scheduler) Stop() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	for uid, t := range sc.userTimers {
		t.Stop()
		delete(sc.userTimers, uid)
	}
}

func (sc *Scheduler) Refresh(userID string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	if t, ok := sc.userTimers[userID]; ok {
		t.Stop()
		delete(sc.userTimers, userID)
	}

	enabled, _ := sc.configService.GetBool(userID, "weather.auto_fetch")
	if !enabled {
		return
	}

	next := sc.nextFetchTime(userID)
	if next.IsZero() {
		return
	}

	delay := time.Until(next)
	if delay < 0 {
		delay = 0
	}

	sc.userTimers[userID] = time.AfterFunc(delay, func() {
		sc.execute(userID)
	})
	logger.Debug("[WeatherAuto] user %s: next fetch at %s (in %s)", userID, next.Format(time.RFC3339), delay)
}

func (sc *Scheduler) RunNow(userID string) error {
	return sc.execute(userID)
}

func (sc *Scheduler) execute(userID string) error {
	cityRaw, _ := sc.configService.GetString(userID, "weather.default_city")
	if cityRaw == "" {
		logger.Debug("[WeatherAuto] user %s: no default city, skipping", userID)
		sc.Refresh(userID)
		return nil
	}

	// Try to parse default_city as structured JSON with coordinates
	var (
		city = cityRaw
		lat  float64
		lon  float64
	)
	var cs citySetting
	if err := json.Unmarshal([]byte(cityRaw), &cs); err == nil && cs.Name != "" {
		city = cs.Name
		lat, lon = cs.Lat, cs.Lon
	}

	today := time.Now().Format("2006-01-02")

	var result *WeatherResult
	var err error
	if lat != 0 && lon != 0 {
		result, err = sc.weatherSvc.GetWeatherByCoords(city, lat, lon, today)
	} else {
		result, err = sc.weatherSvc.GetWeather(city, today)
		// Cache coordinates for next run
		if err == nil {
			sc.weatherSvc.SetCityCoords(city, result.Lat, result.Lon)
		}
	}
	if err != nil {
		logger.Error("[WeatherAuto] user %s: fetch failed for %s: %v", userID, city, err)
		sc.Refresh(userID)
		return err
	}

	_, err = sc.store.UpsertDiaryWeather(userID, today, fmt.Sprintf("%d", result.WMOCode), result.City, result.TempMin, result.TempMax)
	if err != nil {
		logger.Error("[WeatherAuto] user %s: upsert failed: %v", userID, err)
		sc.Refresh(userID)
		return err
	}

	logger.Info("[WeatherAuto] user %s: saved weather for %s: %s", userID, today, FormatDisplay(result.WMOCode, result.TempMin, result.TempMax))
	sc.Refresh(userID)
	return nil
}

func (sc *Scheduler) nextFetchTime(userID string) time.Time {
	timeStr, _ := sc.configService.GetString(userID, "weather.auto_fetch_time")
	if timeStr == "" {
		timeStr = "20:00"
	}

	parts := strings.Split(timeStr, ":")
	hour := 20
	minute := 0
	if len(parts) >= 1 {
		hour, _ = strconv.Atoi(parts[0])
	}
	if len(parts) >= 2 {
		minute, _ = strconv.Atoi(parts[1])
	}

	now := time.Now()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if !next.After(now) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}
