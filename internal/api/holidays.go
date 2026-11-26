package api

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/holidays"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
)

const maxBatchHolidayYears = 20

// RegisterHolidayRoutes 注册中国法定节假日查询与实例级管理接口。
// 当前 fork 没有管理员角色，所有已登录用户均可访问；自托管场景下
// 这些设置即实例所有者自己的设置。
func RegisterHolidayRoutes(e *echo.Echo, svc *holidays.Service, authMiddleware echo.MiddlewareFunc) {
	if svc == nil {
		return
	}
	group := e.Group("/api/v1/holidays", authMiddleware)

	// GET /api/v1/holidays/cn/status — 功能是否已在实例中启用。
	group.GET("/cn/status", func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]any{
			"region":  holidays.RegionCN,
			"enabled": svc.Enabled(),
		})
	})

	// GET /api/v1/holidays/cn?year=2026 或 ?years=2025,2026
	group.GET("/cn", func(c *echo.Context) error {
		yearsParam := strings.TrimSpace(c.QueryParam("years"))
		if !svc.Enabled() {
			if yearsParam != "" {
				return c.JSON(http.StatusOK, map[string]any{
					"region":  holidays.RegionCN,
					"enabled": false,
					"years":   map[string]any{},
				})
			}
			return c.JSON(http.StatusOK, map[string]any{
				"region":    holidays.RegionCN,
				"enabled":   false,
				"published": false,
				"days":      []holidays.Day{},
				"papers":    []string{},
			})
		}

		if yearsParam != "" {
			parts := strings.Split(yearsParam, ",")
			if len(parts) > maxBatchHolidayYears {
				return badRequest("Too many years requested (max 20)", nil)
			}
			seen := make(map[int]struct{}, len(parts))
			results := make(map[string]*holidays.YearResponse, len(parts))
			for _, raw := range parts {
				raw = strings.TrimSpace(raw)
				if raw == "" {
					continue
				}
				year, err := strconv.Atoi(raw)
				if err != nil || year < holidays.MinYear || year > holidays.MaxYear {
					return badRequest("Invalid year: "+raw, nil)
				}
				if _, ok := seen[year]; ok {
					continue
				}
				seen[year] = struct{}{}
				res, err := svc.GetYear(c.Request().Context(), year)
				if err != nil {
					return serverError("Failed to load holiday data", err)
				}
				results[strconv.Itoa(year)] = res
			}
			return c.JSON(http.StatusOK, map[string]any{
				"region":  holidays.RegionCN,
				"enabled": true,
				"years":   results,
			})
		}

		year := time.Now().Year()
		if raw := strings.TrimSpace(c.QueryParam("year")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < holidays.MinYear || parsed > holidays.MaxYear {
				return badRequest("Invalid year: "+raw, nil)
			}
			year = parsed
		}

		res, err := svc.GetYear(c.Request().Context(), year)
		if err != nil {
			return serverError("Failed to load holiday data", err)
		}
		return c.JSON(http.StatusOK, res)
	})

	// GET /api/v1/holidays/cn/settings — 实例级开关、同步状态与手工覆盖。
	group.GET("/cn/settings", func(c *echo.Context) error {
		status, err := svc.Status()
		if err != nil {
			return serverError("Failed to read holiday status", err)
		}
		overrides, err := svc.ListOverrides(0)
		if err != nil {
			return serverError("Failed to list holiday overrides", err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"region":          status.Region,
			"enabled":         status.Enabled,
			"years":           status.Years,
			"last_fetched_at": status.LastFetchedAt,
			"overrides":       overrides,
		})
	})

	// GET /api/v1/holidays/cn/year?year=2026 — 合并官方数据与手工覆盖后的有效日程。
	group.GET("/cn/year", func(c *echo.Context) error {
		year := time.Now().Year()
		if raw := strings.TrimSpace(c.QueryParam("year")); raw != "" {
			parsed, err := strconv.Atoi(raw)
			if err != nil || parsed < holidays.MinYear || parsed > holidays.MaxYear {
				return badRequest("Invalid year: "+raw, nil)
			}
			year = parsed
		}
		res, err := svc.GetYear(c.Request().Context(), year)
		if err != nil {
			return serverError("Failed to load holiday year", err)
		}
		return c.JSON(http.StatusOK, res)
	})

	// PUT /api/v1/holidays/cn/settings — 启用/停用节假日功能。
	group.PUT("/cn/settings", func(c *echo.Context) error {
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		if err := svc.SetEnabled(body.Enabled); err != nil {
			return serverError("Failed to save holiday settings", err)
		}
		logger.Info("[Holidays] feature toggled: enabled=%v", body.Enabled)
		return c.JSON(http.StatusOK, map[string]any{
			"region":  holidays.RegionCN,
			"enabled": body.Enabled,
		})
	})

	// POST /api/v1/holidays/cn/refresh — 从官方镜像同步当年与次年数据。
	group.POST("/cn/refresh", func(c *echo.Context) error {
		ctx, cancel := context.WithTimeout(c.Request().Context(), 25*time.Second)
		defer cancel()
		status, err := svc.RefreshPolicyYears(ctx)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, "Failed to refresh holiday data: "+err.Error())
		}
		overrides, err := svc.ListOverrides(0)
		if err != nil {
			return serverError("Failed to list holiday overrides", err)
		}
		logger.Info("[Holidays] manual policy refresh completed")
		return c.JSON(http.StatusOK, map[string]any{
			"region":          status.Region,
			"enabled":         status.Enabled,
			"years":           status.Years,
			"last_fetched_at": status.LastFetchedAt,
			"overrides":       overrides,
		})
	})

	// POST /api/v1/holidays/cn/import — 离线导入 holiday-cn 年度 JSON。
	group.POST("/cn/import", func(c *echo.Context) error {
		const maxImportBytes = 512 * 1024
		var raw []byte

		contentType := strings.ToLower(c.Request().Header.Get("Content-Type"))
		if strings.HasPrefix(contentType, "multipart/form-data") {
			fileHeader, err := c.FormFile("file")
			if err != nil {
				return badRequest("Missing JSON file", err)
			}
			f, err := fileHeader.Open()
			if err != nil {
				return badRequest("Failed to open uploaded file", err)
			}
			defer f.Close()
			raw, err = io.ReadAll(io.LimitReader(f, maxImportBytes+1))
			if err != nil {
				return badRequest("Failed to read uploaded file", err)
			}
		} else {
			var err error
			raw, err = io.ReadAll(io.LimitReader(c.Request().Body, maxImportBytes+1))
			if err != nil {
				return badRequest("Failed to read request body", err)
			}
		}
		if len(raw) > maxImportBytes {
			return badRequest("JSON file is too large (max 512 KB)", nil)
		}

		imported, err := svc.ImportYearJSON(raw)
		if err != nil {
			if errors.Is(err, holidays.ErrInvalidData) || errors.Is(err, holidays.ErrInvalidYear) {
				return badRequest(err.Error(), nil)
			}
			return serverError("Failed to import holiday JSON", err)
		}
		status, err := svc.Status()
		if err != nil {
			return serverError("Failed to read holiday status", err)
		}
		overrides, err := svc.ListOverrides(0)
		if err != nil {
			return serverError("Failed to list holiday overrides", err)
		}
		logger.Info("[Holidays] imported manual JSON: year=%d days=%d", imported.Year, len(imported.Days))
		return c.JSON(http.StatusOK, map[string]any{
			"imported": imported,
			"overview": map[string]any{
				"region":          status.Region,
				"enabled":         status.Enabled,
				"years":           status.Years,
				"last_fetched_at": status.LastFetchedAt,
				"overrides":       overrides,
			},
		})
	})

	// PUT /api/v1/holidays/cn/overrides — 新增或更新手工节假日/调休覆盖。
	group.PUT("/cn/overrides", func(c *echo.Context) error {
		var body struct {
			Date     string `json:"date"`
			Name     string `json:"name"`
			IsOffDay bool   `json:"isOffDay"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		day, err := svc.SetOverride(body.Date, body.Name, body.IsOffDay)
		if err != nil {
			if errors.Is(err, holidays.ErrInvalidDate) || errors.Is(err, holidays.ErrInvalidYear) || errors.Is(err, holidays.ErrInvalidName) {
				return badRequest(err.Error(), nil)
			}
			return serverError("Failed to save holiday override", err)
		}
		logger.Info("[Holidays] override saved: date=%s name=%s offDay=%v", day.Date, day.Name, day.IsOffDay)
		return c.JSON(http.StatusOK, day)
	})

	// DELETE /api/v1/holidays/cn/overrides/:date — 删除手工覆盖。
	group.DELETE("/cn/overrides/:date", func(c *echo.Context) error {
		date := c.Param("date")
		if err := svc.DeleteOverride(date); err != nil {
			if errors.Is(err, holidays.ErrInvalidDate) {
				return badRequest(err.Error(), nil)
			}
			if errors.Is(err, store.ErrNotFound) {
				return notFound("Holiday override not found")
			}
			return serverError("Failed to delete holiday override", err)
		}
		logger.Info("[Holidays] override deleted: date=%s", date)
		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})
}
