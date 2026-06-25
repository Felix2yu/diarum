package api

import (
	"net/http"
	"runtime"
	"time"

	"github.com/labstack/echo/v5"
)

var startTime = time.Now()

// MetricsResponse represents the application metrics
type MetricsResponse struct {
	Uptime    string            `json:"uptime"`
	StartedAt string            `json:"started_at"`
	GoVersion string            `json:"go_version"`
	GoOS      string            `json:"go_os"`
	GoArch    string            `json:"go_arch"`
	MemStats  MemoryStats       `json:"memory_stats"`
}

// MemoryStats represents memory usage statistics
type MemoryStats struct {
	Alloc      uint64 `json:"alloc"`
	TotalAlloc uint64 `json:"total_alloc"`
	Sys        uint64 `json:"sys"`
	NumGC      uint32 `json:"num_gc"`
	HeapAlloc  uint64 `json:"heap_alloc"`
	HeapSys    uint64 `json:"heap_sys"`
	HeapIdle   uint64 `json:"heap_idle"`
	HeapInuse  uint64 `json:"heap_inuse"`
	HeapReleased uint64 `json:"heap_released"`
	HeapObjects uint64 `json:"heap_objects"`
	StackInuse uint64 `json:"stack_inuse"`
	StackSys   uint64 `json:"stack_sys"`
	Mallocs    uint64 `json:"mallocs"`
	Frees      uint64 `json:"frees"`
}

// RegisterMetricsRoutes registers the metrics API endpoint
func RegisterMetricsRoutes(e *echo.Echo) {
	handler := func(c echo.Context) error {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		uptime := time.Since(startTime)

		metrics := MetricsResponse{
			Uptime:    uptime.String(),
			StartedAt: startTime.UTC().Format(time.RFC3339),
			GoVersion: runtime.Version(),
			GoOS:      runtime.GOOS,
			GoArch:    runtime.GOARCH,
			MemStats: MemoryStats{
				Alloc:        m.Alloc,
				TotalAlloc:   m.TotalAlloc,
				Sys:          m.Sys,
				NumGC:        m.NumGC,
				HeapAlloc:    m.HeapAlloc,
				HeapSys:      m.HeapSys,
				HeapIdle:     m.HeapIdle,
				HeapInuse:    m.HeapInuse,
				HeapReleased: m.HeapReleased,
				HeapObjects:  m.HeapObjects,
				StackInuse:   m.StackInuse,
				StackSys:     m.StackSys,
				Mallocs:      m.Mallocs,
				Frees:        m.Frees,
			},
		}

		return c.JSON(http.StatusOK, metrics)
	}

	e.GET("/api/v1/metrics", handler)
	e.GET("/api/metrics", handler)
}
