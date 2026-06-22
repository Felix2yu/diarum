package api

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
)

// RegisterPublicRoutes registers public API endpoints that use API token authentication.
func RegisterPublicRoutes(e *echo.Echo, s *store.Store) {
	configService := config.NewConfigService(s)

	// Reads
	e.GET("/api/v1/diaries", func(c echo.Context) error {
		userId, err := authenticatePublicRequest(configService, c)
		if err != nil {
			return err
		}

		date := c.QueryParam("date")
		start := c.QueryParam("start")
		end := c.QueryParam("end")

		if date != "" {
			diary, err := s.GetDiaryByDate(userId, date+" 00:00:00.000Z", date+" 23:59:59.999Z")
			if err != nil {
				return c.JSON(http.StatusOK, map[string]any{"date": date, "content": "", "exists": false})
			}
			return c.JSON(http.StatusOK, diaryResponse(diary, date, true))
		}

		if start != "" && end != "" {
			diaries, err := s.ListDiaries(userId, start+" 00:00:00.000Z", end+" 23:59:59.999Z", "-date", 0)
			if err != nil {
				return serverError("Failed to query diaries", err)
			}
			results := make([]map[string]any, 0, len(diaries))
			for _, diary := range diaries {
				results = append(results, map[string]any{"id": diary.ID, "date": store.DateOnly(diary.Date), "content": diary.Content, "mood": diary.Mood, "scenarios": diary.Scenarios, "weather": diary.Weather})
			}
			return c.JSON(http.StatusOK, map[string]any{"diaries": results, "total": len(results)})
		}

		return badRequest("Either 'date' or both 'start' and 'end' query parameters are required", nil)
	})

	// Create or update a diary by date
	e.POST("/api/v1/diaries", func(c echo.Context) error {
		userId, err := authenticatePublicRequest(configService, c)
		if err != nil {
			return err
		}

		var body struct {
			Date       string   `json:"date"`
			Content    string   `json:"content"`
			Mood       int      `json:"mood"`
			MoodStates []string `json:"mood_states"`
			Scenarios  []string `json:"scenarios"`
			Weather    string   `json:"weather"`
			Tags       []string `json:"tags"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		if body.Date == "" {
			return badRequest("date is required", nil)
		}
		if body.Tags == nil {
			body.Tags = []string{}
		}

		diary, created, err := s.UpsertDiary(userId, body.Date, body.Content, body.Mood, body.MoodStates, body.Scenarios, body.Weather, body.Tags)
		if err != nil {
			return serverError("Failed to save diary", err)
		}
		status := http.StatusOK
		if created {
			status = http.StatusCreated
		}
		return c.JSON(status, diaryResponse(diary, body.Date, true))
	})

	// Update a diary by ID
	e.PUT("/api/v1/diaries/:id", func(c echo.Context) error {
		userId, err := authenticatePublicRequest(configService, c)
		if err != nil {
			return err
		}

		id := c.PathParam("id")
		existing, err := s.GetDiaryByID(id)
		if err != nil || existing.Owner != userId {
			return notFound("Diary not found")
		}

		var body struct {
			Content    string   `json:"content"`
			Mood       int      `json:"mood"`
			MoodStates []string `json:"mood_states"`
			Scenarios  []string `json:"scenarios"`
			Weather    string   `json:"weather"`
			Tags       []string `json:"tags"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}

		content := body.Content
		if content == "" {
			content = existing.Content
		}
		mood := body.Mood
		if mood == 0 {
			mood = existing.Mood
		}
		moodStates := body.MoodStates
		if moodStates == nil {
			moodStates = existing.MoodStates
		}
		scenarios := body.Scenarios
		if scenarios == nil {
			scenarios = existing.Scenarios
		}
		weather := body.Weather
		if weather == "" {
			weather = existing.Weather
		}
		tags := body.Tags
		if tags == nil {
			tags = existing.Tags
		}

		diary, _, err := s.UpsertDiary(userId, store.DateOnly(existing.Date), content, mood, moodStates, scenarios, weather, tags)
		if err != nil {
			return serverError("Failed to update diary", err)
		}
		return c.JSON(http.StatusOK, diaryResponse(diary, store.DateOnly(diary.Date), true))
	})

	// Delete a diary by date
	e.DELETE("/api/v1/diaries", func(c echo.Context) error {
		userId, err := authenticatePublicRequest(configService, c)
		if err != nil {
			return err
		}

		date := c.QueryParam("date")
		if date == "" {
			return badRequest("date query parameter is required", nil)
		}

		existing, err := s.GetDiaryByDate(userId, date+" 00:00:00.000Z", date+" 23:59:59.999Z")
		if err != nil {
			return notFound("Diary not found")
		}
		if err := s.DeleteDiary(existing.ID, userId); err != nil {
			return serverError("Failed to delete diary", err)
		}
		return c.JSON(http.StatusOK, map[string]any{"success": true, "date": date})
	})
}

// authenticatePublicRequest extracts and validates an API token from the request.
// It returns the authenticated user ID or an echo error response.
func authenticatePublicRequest(configService *config.ConfigService, c echo.Context) (string, error) {
	token := c.QueryParam("token")
	if token == "" {
		if bearer := c.Request().Header.Get("Authorization"); bearer != "" {
			const prefix = "Bearer "
			if len(bearer) > len(prefix) {
				token = bearer[len(prefix):]
			}
		}
	}
	if token == "" {
		return "", unauthorized("API token is required")
	}

	userId, err := configService.ValidateTokenAndGetUser(token)
	if err == config.ErrAPIDisabled {
		return "", unauthorized("API is disabled for this user")
	}
	if err != nil || userId == "" {
		return "", unauthorized("Invalid API token")
	}
	return userId, nil
}
