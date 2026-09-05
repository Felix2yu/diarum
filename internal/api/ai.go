package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/chat"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/embedding"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/aicore"
	"github.com/songtianlun/diarum/internal/store"
)

type modelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type modelsResponse struct {
	Object string      `json:"object"`
	Data   []modelInfo `json:"data"`
}

// periodKeyRange derives the date range and Chinese label for a period key.
// Week keys follow ISO 8601 ("2026-W36", Monday-based, week 1 contains the
// first Thursday); month keys are "2026-09"; year keys are "2026".
func periodKeyRange(period, key string) (startDate, endDate, label string, err error) {
	switch period {
	case "week":
		var year, week int
		if n, _ := fmt.Sscanf(key, "%d-W%d", &year, &week); n != 2 || year < 1 || week < 1 || week > 53 || len(key) != 8 || key[4] != '-' || key[5] != 'W' {
			return "", "", "", fmt.Errorf("invalid week key, expected YYYY-Www (e.g. 2026-W36)")
		}
		// ISO week 1 starts on the Monday of the week containing Jan 4.
		jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.Local)
		offsetFromMonday := (int(jan4.Weekday()) + 6) % 7
		start := jan4.AddDate(0, 0, -offsetFromMonday+(week-1)*7)
		isoYear, isoWeek := start.ISOWeek()
		if isoYear != year || isoWeek != week {
			return "", "", "", fmt.Errorf("invalid week key %s for year %d", key, year)
		}
		end := start.AddDate(0, 0, 6)
		return start.Format("2006-01-02"), end.Format("2006-01-02"), fmt.Sprintf("%d年第%d周", year, week), nil
	case "month":
		var year, month int
		if n, _ := fmt.Sscanf(key, "%d-%d", &year, &month); n != 2 || year < 1 || month < 1 || month > 12 || len(key) != 7 {
			return "", "", "", fmt.Errorf("invalid month key, expected YYYY-MM (e.g. 2026-09)")
		}
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.Local)
		last := time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.Local)
		return start.Format("2006-01-02"), last.Format("2006-01-02"), fmt.Sprintf("%d年%d月", year, month), nil
	case "year":
		var year int
		if n, _ := fmt.Sscanf(key, "%d", &year); n != 1 || year < 1 || len(key) != 4 {
			return "", "", "", fmt.Errorf("invalid year key, expected YYYY (e.g. 2026)")
		}
		return fmt.Sprintf("%d-01-01", year), fmt.Sprintf("%d-12-31", year), fmt.Sprintf("%d年", year), nil
	default:
		return "", "", "", fmt.Errorf("period must be 'week', 'month' or 'year'")
	}
}

func RegisterAIRoutes(e *echo.Echo, s *store.Store, authMiddleware echo.MiddlewareFunc, embeddingService *embedding.EmbeddingService) {
	configService := config.NewConfigService(s)
	chatService := chat.NewChatService(s, embeddingService)
	group := e.Group("/api/v1/ai", authMiddleware)

	group.GET("/settings", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		apiKey, _ := configService.GetString(userId, "ai.api_key")
		baseUrl, _ := configService.GetString(userId, "ai.base_url")
		chatModel, _ := configService.GetString(userId, "ai.chat_model")
		embeddingModel, _ := configService.GetString(userId, "ai.embedding_model")
		analysisSystemPrompt, _ := configService.GetString(userId, "ai.analysis_system_prompt")
		analysisUserPrefix, _ := configService.GetString(userId, "ai.analysis_user_prefix")
		enabled, _ := configService.GetBool(userId, "ai.enabled")
		speechProvider, _ := configService.GetString(userId, "ai.speech.provider")
		speechBaseUrl, _ := configService.GetString(userId, "ai.speech.base_url")
		speechAPIKey, _ := configService.GetString(userId, "ai.speech.api_key")
		speechModel, _ := configService.GetString(userId, "ai.speech.model")
		speechLanguage, _ := configService.GetString(userId, "ai.speech.language")
		return c.JSON(http.StatusOK, map[string]any{
			"api_key":                maskSecret(apiKey),
			"base_url":               baseUrl,
			"chat_model":             chatModel,
			"embedding_model":        embeddingModel,
			"analysis_system_prompt": analysisSystemPrompt,
			"analysis_user_prefix":   analysisUserPrefix,
			"enabled":                enabled,
			"speech_provider":        speechProvider,
			"speech_base_url":        speechBaseUrl,
			"speech_api_key":         maskSecret(speechAPIKey),
			"speech_model":           speechModel,
			"speech_language":        speechLanguage,
		})
	})

	group.PUT("/settings", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			APIKey               string `json:"api_key"`
			BaseURL              string `json:"base_url"`
			ChatModel            string `json:"chat_model"`
			EmbeddingModel       string `json:"embedding_model"`
			AnalysisSystemPrompt string `json:"analysis_system_prompt"`
			AnalysisUserPrefix   string `json:"analysis_user_prefix"`
			Enabled              bool   `json:"enabled"`
			SpeechProvider       string `json:"speech_provider"`
			SpeechBaseURL        string `json:"speech_base_url"`
			SpeechAPIKey         string `json:"speech_api_key"`
			SpeechModel          string `json:"speech_model"`
			SpeechLanguage       string `json:"speech_language"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		// 密钥字段为空或掩码值时保留原值，避免前端提交掩码/空串覆盖真实密钥
		apiKey := body.APIKey
		if isSecretPlaceholder(apiKey) {
			apiKey, _ = configService.GetString(userId, "ai.api_key")
		}
		speechAPIKey := body.SpeechAPIKey
		if isSecretPlaceholder(speechAPIKey) {
			speechAPIKey, _ = configService.GetString(userId, "ai.speech.api_key")
		}
		if body.Enabled && (apiKey == "" || body.BaseURL == "" || body.ChatModel == "" || body.EmbeddingModel == "") {
			return badRequest("All AI settings must be configured before enabling AI features", nil)
		}
		settings := map[string]any{
			"ai.api_key":                apiKey,
			"ai.base_url":               body.BaseURL,
			"ai.chat_model":             body.ChatModel,
			"ai.embedding_model":        body.EmbeddingModel,
			"ai.analysis_system_prompt": body.AnalysisSystemPrompt,
			"ai.analysis_user_prefix":   body.AnalysisUserPrefix,
			"ai.enabled":                body.Enabled,
			"ai.speech.provider":        body.SpeechProvider,
			"ai.speech.base_url":        body.SpeechBaseURL,
			"ai.speech.api_key":         speechAPIKey,
			"ai.speech.model":           body.SpeechModel,
			"ai.speech.language":        body.SpeechLanguage,
		}
		if err := configService.SetBatch(userId, settings); err != nil {
			return badRequest("Failed to save AI settings", err)
		}
		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	group.POST("/models", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			APIKey  string `json:"api_key"`
			BaseURL string `json:"base_url"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		// API key 为空或掩码时使用已保存的密钥
		if isSecretPlaceholder(body.APIKey) {
			body.APIKey, _ = configService.GetString(userId, "ai.api_key")
		}
		if body.APIKey == "" || body.BaseURL == "" {
			return badRequest("API key and base URL are required", nil)
		}
		models, err := fetchModels(body.BaseURL, body.APIKey)
		if err != nil {
			logger.Error("[POST /api/v1/ai/models] error fetching models: %v", err)
			return badRequest("Failed to fetch models: "+err.Error(), nil)
		}
		return c.JSON(http.StatusOK, map[string]any{"models": models})
	})

	// Speech transcription: accepts multipart/form-data with a `file` field and
	// calls an OpenAI-compatible /v1/audio/transcriptions endpoint using the
	// configured speech credentials. Also supports the optional `prompt`,
	// `language`, and `model` overrides in the request.
	group.POST("/transcribe", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID

		provider, _ := configService.GetString(userId, "ai.speech.provider")
		if provider == "" || provider == "none" {
			return badRequest("Speech recognition is not enabled. Please configure it in Settings.", nil)
		}

		// Load speech settings; fall back to shared AI settings if the dedicated
		// speech values are empty so users only have to fill in one API base.
		baseURL, _ := configService.GetString(userId, "ai.speech.base_url")
		apiKey, _ := configService.GetString(userId, "ai.speech.api_key")
		model, _ := configService.GetString(userId, "ai.speech.model")
		language, _ := configService.GetString(userId, "ai.speech.language")

		if baseURL == "" || apiKey == "" {
			fallbackBase, _ := configService.GetString(userId, "ai.base_url")
			fallbackKey, _ := configService.GetString(userId, "ai.api_key")
			if fallbackBase == "" || fallbackKey == "" {
				return badRequest("Speech recognition requires a base URL and API key.", nil)
			}
			baseURL = fallbackBase
			apiKey = fallbackKey
		}
		if model == "" {
			model = "whisper-1"
		}

		// File upload
		file, err := c.FormFile("file")
		if err != nil {
			return badRequest("Missing audio file", err)
		}
		if file == nil || file.Size == 0 {
			return badRequest("Invalid audio file", nil)
		}
		// 25MB safety limit (matches OpenAI)
		if file.Size > 25*1024*1024 {
			return badRequest("Audio file is too large (max 25MB)", nil)
		}
		src, err := file.Open()
		if err != nil {
			return badRequest("Failed to read audio file", err)
		}
		defer src.Close()

		// Read file content into memory for proxying (audio files are typically small)
		audioBytes, err := io.ReadAll(src)
		if err != nil {
			return badRequest("Failed to read audio file", err)
		}

		// Let the request override language / prompt if provided
		if overrideLang := strings.TrimSpace(c.FormValue("language")); overrideLang != "" {
			language = overrideLang
		}
		if overrideModel := strings.TrimSpace(c.FormValue("model")); overrideModel != "" {
			model = overrideModel
		}
		prompt := strings.TrimSpace(c.FormValue("prompt"))

		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
		defer cancel()

		cfg := aicore.AIConfig{Enabled: true, APIKey: apiKey, BaseURL: baseURL, Model: model}
		origName := file.Filename
		if origName == "" {
			origName = "audio.webm"
		}
		text, err := aicore.Transcribe(ctx, cfg, audioBytes, origName, file.Header.Get("Content-Type"), language, model, prompt)
		if err != nil {
			return serverError("Speech recognition failed: "+err.Error(), nil)
		}

		return c.JSON(http.StatusOK, map[string]any{
			"text": text,
		})
	})

	group.POST("/vectors/build", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		if embeddingService == nil {
			return badRequest("Embedding service not initialized", nil)
		}
		ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Minute)
		defer cancel()
		result, err := embeddingService.BuildAllVectors(ctx, userId)
		if err != nil {
			return badRequest("Failed to build vectors: "+err.Error(), nil)
		}
		return c.JSON(http.StatusOK, result)
	})
	group.POST("/vectors/build-incremental", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		if embeddingService == nil {
			return badRequest("Embedding service not initialized", nil)
		}
		ctx, cancel := context.WithTimeout(c.Request().Context(), 10*time.Minute)
		defer cancel()
		result, err := embeddingService.BuildIncrementalVectors(ctx, userId)
		if err != nil {
			return badRequest("Failed to build vectors: "+err.Error(), nil)
		}
		return c.JSON(http.StatusOK, result)
	})

	group.GET("/vectors/stats", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		if embeddingService == nil {
			return badRequest("Embedding service not initialized", nil)
		}
		stats, err := embeddingService.GetVectorStats(c.Request().Context(), userId)
		if err != nil {
			return badRequest("Failed to get vector stats: "+err.Error(), nil)
		}
		return c.JSON(http.StatusOK, stats)
	})

	group.GET("/conversations", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		conversations, err := s.ListConversations(userId, 100)
		if err != nil {
			return badRequest("Failed to fetch conversations", err)
		}
		result := make([]map[string]any, 0, len(conversations))
		for _, conv := range conversations {
			count, _ := chatService.GetConversationMessageCount(conv.ID)
			result = append(result, map[string]any{"id": conv.ID, "title": conv.Title, "created": conv.Created, "updated": conv.Updated, "message_count": count})
		}
		return c.JSON(http.StatusOK, result)
	})

	group.POST("/conversations", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			Title string `json:"title"`
		}
		_ = c.Bind(&body)
		conv, err := s.CreateConversation(userId, body.Title)
		if err != nil {
			return badRequest("Failed to create conversation", err)
		}
		return c.JSON(http.StatusOK, map[string]any{"id": conv.ID, "title": conv.Title, "created": conv.Created, "updated": conv.Updated})
	})

	group.GET("/conversations/:id", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		conv, err := s.GetConversation(c.Param("id"), userId)
		if err != nil {
			return notFound("Conversation not found")
		}
		messages, err := s.ListMessages(conv.ID, 100)
		if err != nil {
			return badRequest("Failed to fetch messages", err)
		}
		msgList := make([]map[string]any, 0, len(messages))
		for _, msg := range messages {
			msgList = append(msgList, map[string]any{"id": msg.ID, "role": msg.Role, "content": msg.Content, "referenced_diaries": msg.ReferencedDiaries, "created": msg.Created})
		}
		return c.JSON(http.StatusOK, map[string]any{"conversation": map[string]any{"id": conv.ID, "title": conv.Title, "created": conv.Created, "updated": conv.Updated}, "messages": msgList})
	})

	group.DELETE("/conversations/:id", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		if err := s.DeleteConversation(c.Param("id"), userId); err != nil {
			return notFound("Conversation not found")
		}
		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	group.PUT("/conversations/:id", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			Title string `json:"title"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		conv, err := s.UpdateConversationTitle(c.Param("id"), userId, body.Title)
		if err != nil {
			return notFound("Conversation not found")
		}
		return c.JSON(http.StatusOK, map[string]any{"id": conv.ID, "title": conv.Title, "updated": conv.Updated})
	})

	group.POST("/chat", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			ConversationID string `json:"conversation_id"`
			Content        string `json:"content"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		if body.ConversationID == "" || body.Content == "" {
			return badRequest("conversation_id and content are required", nil)
		}
		conv, err := s.GetConversation(body.ConversationID, userId)
		if err != nil {
			return notFound("Conversation not found")
		}
		messageCount, _ := chatService.GetConversationMessageCount(body.ConversationID)
		isFirstMessage := messageCount == 0
		currentTitle := conv.Title
		userMsg, err := chatService.SaveMessage(userId, body.ConversationID, "user", body.Content, nil)
		if err != nil {
			logger.Error("[POST /api/v1/ai/chat] failed to save user message: %v", err)
		} else {
			logger.Info("[POST /api/v1/ai/chat] saved user message: %s", userMsg.ID)
		}

		c.Response().Header().Set("Content-Type", "text/event-stream")
		c.Response().Header().Set("Cache-Control", "no-cache")
		c.Response().Header().Set("Connection", "keep-alive")
		c.Response().WriteHeader(http.StatusOK)
		writer := &sseWriter{w: c.Response()}
		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
		defer cancel()

		var newTitle string
		if isFirstMessage && currentTitle == "" {
			title, err := chatService.GenerateTitleFromUserMessage(ctx, userId, body.Content)
			if err == nil {
				newTitle = title
				if err := chatService.UpdateConversationTitle(body.ConversationID, title); err == nil {
					titleData, _ := json.Marshal(map[string]any{"title": newTitle})
					writer.Write([]byte("data: " + string(titleData) + "\n\n"))
					writer.Flush()
				}
			}
		}

		fullResponse, referencedDiaries, err := chatService.StreamChat(ctx, userId, body.ConversationID, body.Content, writer)
		if err != nil {
			logger.Error("[POST /api/v1/ai/chat] stream chat error: %v", err)
			errData, _ := json.Marshal(map[string]string{"error": err.Error()})
			writer.Write([]byte("data: " + string(errData) + "\n\n"))
			writer.Flush()
			return nil
		}
		assistantMsg, err := chatService.SaveMessage(userId, body.ConversationID, "assistant", fullResponse, referencedDiaries)
		if err != nil {
			logger.Error("[POST /api/v1/ai/chat] failed to save assistant message: %v", err)
		} else {
			logger.Info("[POST /api/v1/ai/chat] saved assistant message: %s", assistantMsg.ID)
		}
		doneData, _ := json.Marshal(map[string]any{"done": true, "referenced_diaries": referencedDiaries, "title": newTitle})
		writer.Write([]byte("data: " + string(doneData) + "\n\n"))
		writer.Flush()
		return nil
	})

	// Analysis - retrieve saved result (week / month / year by period key, or custom by date range)
	group.GET("/analysis", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		period := strings.ToLower(strings.TrimSpace(c.QueryParam("period")))
		key := strings.TrimSpace(c.QueryParam("key"))
		start := strings.TrimSpace(c.QueryParam("start"))
		end := strings.TrimSpace(c.QueryParam("end"))
		keywords := strings.TrimSpace(c.QueryParam("keywords"))
		var a *store.PeriodAnalysis
		switch period {
		case "week", "month", "year":
			if key == "" {
				return badRequest("key is required for week/month/year analyses", nil)
			}
			startDate, endDate, _, err := periodKeyRange(period, key)
			if err != nil {
				return badRequest(err.Error(), nil)
			}
			saved, err := s.GetPeriodAnalysis(userId, period, key, startDate, endDate, "")
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return c.JSON(http.StatusOK, map[string]any{"found": false})
				}
				return serverError("Failed to load period analysis", err)
			}
			a = saved
		case "custom":
			if start == "" || end == "" {
				return badRequest("start and end are required for custom analyses", nil)
			}
			saved, err := s.GetPeriodAnalysis(userId, period, "", start, end, keywords)
			if err != nil {
				if errors.Is(err, sql.ErrNoRows) {
					return c.JSON(http.StatusOK, map[string]any{"found": false})
				}
				return serverError("Failed to load period analysis", err)
			}
			a = saved
		default:
			return badRequest("period must be 'week', 'month', 'year' or 'custom'", nil)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"found":         true,
			"id":            a.ID,
			"period":        a.Period,
			"key":           a.PeriodKey,
			"start":         a.StartDate,
			"end":           a.EndDate,
			"count":         a.DiaryCount,
			"summary":       a.Summary,
			"system_prompt": a.SystemPrompt,
			"user_prefix":   a.UserPrefix,
			"keywords":      a.Keywords,
			"created":       a.Created,
			"updated":       a.Updated,
		})
	})

	// Analysis - list all saved analyses (optionally filtered by period)
	group.GET("/analyses", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		period := strings.ToLower(strings.TrimSpace(c.QueryParam("period")))
		if period != "" && period != "week" && period != "month" && period != "year" && period != "custom" && period != "all" {
			return badRequest("period must be 'week', 'month', 'year', 'custom' or 'all'", nil)
		}
		filter := period
		if period == "all" {
			filter = ""
		}
		items, err := s.ListSavedAnalyses(userId, filter, 200)
		if err != nil {
			return serverError("Failed to list period analyses", err)
		}
		out := make([]map[string]any, 0, len(items))
		for _, a := range items {
			out = append(out, map[string]any{
				"id":            a.ID,
				"period":        a.Period,
				"key":           a.PeriodKey,
				"start":         a.StartDate,
				"end":           a.EndDate,
				"count":         a.DiaryCount,
				"summary":       a.Summary,
				"system_prompt": a.SystemPrompt,
				"user_prefix":   a.UserPrefix,
				"keywords":      a.Keywords,
				"created":       a.Created,
				"updated":       a.Updated,
			})
		}
		return c.JSON(http.StatusOK, map[string]any{"items": out})
	})

	// Analysis endpoint - generate and save. Week/month/year reports are addressed
	// by a period key (e.g. 2026-W36 / 2026-09 / 2026) and the date range is derived
	// server-side; custom analyses keep explicit date ranges and keyword filtering
	// so users can analyze only matching diary entries.
	group.POST("/analysis", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			Period       string `json:"period"`
			Key          string `json:"key"`
			Start        string `json:"start"`
			End          string `json:"end"`
			Keywords     string `json:"keywords"`
			SystemPrompt string `json:"system_prompt"`
			UserPrefix   string `json:"user_prefix"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		period := strings.ToLower(strings.TrimSpace(body.Period))
		if period == "" {
			period = "custom"
		}
		key := strings.TrimSpace(body.Key)
		keywords := strings.TrimSpace(body.Keywords)
		var start, end, periodLabel string
		switch period {
		case "week", "month", "year":
			if key == "" {
				return badRequest("key is required for week/month/year analyses", nil)
			}
			startDate, endDate, label, err := periodKeyRange(period, key)
			if err != nil {
				return badRequest(err.Error(), nil)
			}
			start, end, periodLabel = startDate, endDate, label
		case "custom":
			start = strings.TrimSpace(body.Start)
			end = strings.TrimSpace(body.End)
			if start == "" || end == "" {
				return badRequest("start and end are required for custom analyses", nil)
			}
			if _, err := time.Parse("2006-01-02", start); err != nil {
				return badRequest("start must be in YYYY-MM-DD format", err)
			}
			if _, err := time.Parse("2006-01-02", end); err != nil {
				return badRequest("end must be in YYYY-MM-DD format", err)
			}
			periodLabel = "所选时间段"
		default:
			return badRequest("period must be 'week', 'month', 'year' or 'custom'", nil)
		}

		// Fetch diaries in range
		diaries, err := s.ListDiaries(userId, start+" 00:00:00.000Z", end+" 23:59:59.999Z", "-date", 0)
		if err != nil {
			return serverError("Failed to fetch diaries for analysis", err)
		}

		// Optional keyword/content filtering
		if keywords != "" {
			// Split on common separators (comma, space) and build a lowercase token list
			rawTokens := strings.FieldsFunc(keywords, func(r rune) bool {
				return r == ',' || r == '，' || r == ';' || r == '；' || r == '/' || r == '\\'
			})
			tokens := make([]string, 0, len(rawTokens))
			for _, t := range rawTokens {
				t = strings.TrimSpace(t)
				if t == "" {
					continue
				}
				tokens = append(tokens, strings.ToLower(t))
			}
			if len(tokens) > 0 {
				filtered := make([]*store.Diary, 0, len(diaries))
				for _, d := range diaries {
					haystack := strings.ToLower(d.Content)
					matched := false
					for _, tok := range tokens {
						if strings.Contains(haystack, tok) {
							matched = true
							break
						}
					}
					if matched {
						filtered = append(filtered, d)
					}
				}
				diaries = filtered
			}
		}

		if len(diaries) == 0 {
			var emptySummary string
			if keywords != "" {
				emptySummary = fmt.Sprintf("在 %s 至 %s 的时间段内，没有找到包含关键词「%s」的日记记录，无法进行分析。建议调整时间范围、更换关键词，或先记录一些相关日常内容。", start, end, keywords)
			} else {
				emptySummary = fmt.Sprintf("在 %s 至 %s 的时间段内没有日记记录，无法进行分析。建议先记录一些日常内容，然后再尝试。", start, end)
			}
			return c.JSON(http.StatusOK, map[string]any{
				"start":    start,
				"end":      end,
				"period":   period,
				"key":      key,
				"keywords": keywords,
				"count":    0,
				"summary":  emptySummary,
			})
		}

		// Load AI config
		apiKey, _ := configService.GetString(userId, "ai.api_key")
		baseURL, _ := configService.GetString(userId, "ai.base_url")
		model, _ := configService.GetString(userId, "ai.chat_model")
		enabled, _ := configService.GetBool(userId, "ai.enabled")
		if !enabled || apiKey == "" || baseURL == "" || model == "" {
			return serviceUnavailable("AI service is not configured", nil)
		}

		// Resolve prompts: request override → saved config → default
		savedSystemPrompt, _ := configService.GetString(userId, "ai.analysis_system_prompt")
		savedUserPrefix, _ := configService.GetString(userId, "ai.analysis_user_prefix")
		defaultSystemPrompt := "你是一个贴心的日记分析助手，基于用户提供的日记内容进行深入分析。你需要：\n1) 归纳总结日记的主要内容；\n2) 留意每篇日记的日期，分析情绪变化、生活模式在时间线上的演变；\n3) 找出亮点和值得改进的地方；\n4) 给出具体、可操作的建议。\n请用温暖、鼓励且理性的语气，分段输出，便于阅读。使用中文回答。"

		defaultUserPrefix := ""
		if keywords != "" {
			defaultUserPrefix = fmt.Sprintf("以下是%s（%s 至 %s）中包含关键词「%s」的日记记录，共 %d 篇。请根据这些内容进行重组、分析，并给出建议。\n\n", periodLabel, start, end, keywords, len(diaries))
		} else {
			defaultUserPrefix = fmt.Sprintf("以下是%s（%s 至 %s）的日记记录，共 %d 篇。请根据内容进行重组、分析，并给出建议。\n\n", periodLabel, start, end, len(diaries))
		}

		systemPrompt := strings.TrimSpace(body.SystemPrompt)
		if systemPrompt == "" {
			systemPrompt = strings.TrimSpace(savedSystemPrompt)
		}
		if systemPrompt == "" {
			systemPrompt = defaultSystemPrompt
		}

		userPrefix := strings.TrimSpace(body.UserPrefix)
		if userPrefix == "" {
			userPrefix = strings.TrimSpace(savedUserPrefix)
		}
		if userPrefix == "" {
			userPrefix = defaultUserPrefix
		}

		// Build user content with diary entries.
		// Reverse the diaries so they are presented to the AI in ascending (old → new)
		// chronological order — this makes it easier for the AI to detect trends and
		// mood progression across the analyzed period.
		for i, j := 0, len(diaries)-1; i < j; i, j = i+1, j-1 {
			diaries[i], diaries[j] = diaries[j], diaries[i]
		}
		var sb strings.Builder
		sb.WriteString(userPrefix)
		if !strings.HasSuffix(userPrefix, "\n") {
			sb.WriteString("\n\n")
		}
		for i, d := range diaries {
			sb.WriteString(fmt.Sprintf("--- 第 %d 篇 - %s ---\n", i+1, store.DateOnly(d.Date)))
			if d.Mood != 0 {
				sb.WriteString(fmt.Sprintf("心情：%s\n", store.MoodToEmoji(d.Mood)))
			}
			if len(d.MoodStates) > 0 {
				sb.WriteString(fmt.Sprintf("心情状态：%s\n", strings.Join(d.MoodStates, ", ")))
			}
			if len(d.Scenarios) > 0 {
				sb.WriteString(fmt.Sprintf("情景：%s\n", strings.Join(d.Scenarios, ", ")))
			}
			if d.Weather != "" {
				sb.WriteString(fmt.Sprintf("天气：%s\n", d.Weather))
			}
			sb.WriteString(fmt.Sprintf("内容：\n%s\n\n", d.Content))
		}

		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
		defer cancel()

		baseURL = strings.TrimSuffix(baseURL, "/")
		url := baseURL + "/v1/chat/completions"

		reqBody := map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": sb.String()},
			},
			"stream": false,
		}

		jsonBytes, err := json.Marshal(reqBody)
		if err != nil {
			return serverError("Failed to build AI request", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBytes))
		if err != nil {
			return serverError("Failed to create AI request", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", "application/json")

		client := &http.Client{Timeout: 5 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			return serverError("AI request failed: "+err.Error(), nil)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyText, _ := io.ReadAll(resp.Body)
			return serverError(fmt.Sprintf("AI returned status %d: %s", resp.StatusCode, string(bodyText)), nil)
		}

		var aiResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&aiResp); err != nil {
			return serverError("Failed to decode AI response", err)
		}

		summary := ""
		if len(aiResp.Choices) > 0 {
			summary = aiResp.Choices[0].Message.Content
		}

		// Persist the analysis for later retrieval
		saved, saveErr := s.SavePeriodAnalysis(userId, period, key, start, end, len(diaries), summary, systemPrompt, userPrefix, keywords)

		response := map[string]any{
			"start":         start,
			"end":           end,
			"period":        period,
			"key":           key,
			"keywords":      keywords,
			"count":         len(diaries),
			"summary":       summary,
			"system_prompt": systemPrompt,
			"user_prefix":   userPrefix,
		}
		if saveErr == nil && saved != nil {
			response["id"] = saved.ID
			response["created"] = saved.Created
			response["updated"] = saved.Updated
		} else if saveErr != nil {
			logger.Warn("[POST /api/v1/ai/analysis] failed to persist analysis: %v", saveErr)
		}
		return c.JSON(http.StatusOK, response)
	})

	// Analysis endpoint - manually save/edit a weekly / monthly / yearly report.
	// Unlike the POST endpoint this skips AI generation entirely: the user fills in
	// the report content ("周日记分析 / 月日记分析 / 年日记分析") and it is persisted
	// for later retrieval under the same period key.
	group.PUT("/analysis", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			Period  string `json:"period"`
			Key     string `json:"key"`
			Summary string `json:"summary"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		period := strings.ToLower(strings.TrimSpace(body.Period))
		key := strings.TrimSpace(body.Key)
		if period != "week" && period != "month" && period != "year" {
			return badRequest("period must be 'week', 'month' or 'year'", nil)
		}
		if key == "" {
			return badRequest("key is required", nil)
		}
		start, end, _, err := periodKeyRange(period, key)
		if err != nil {
			return badRequest(err.Error(), nil)
		}
		summary := strings.TrimSpace(body.Summary)
		if summary == "" {
			return badRequest("summary is required", nil)
		}
		diaries, err := s.ListDiaries(userId, start+" 00:00:00.000Z", end+" 23:59:59.999Z", "-date", 0)
		if err != nil {
			return serverError("Failed to fetch diaries for analysis", err)
		}
		saved, err := s.SavePeriodAnalysis(userId, period, key, start, end, len(diaries), summary, "", "", "")
		if err != nil {
			return serverError("Failed to save period analysis", err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"period":  period,
			"key":     key,
			"start":   start,
			"end":     end,
			"count":   len(diaries),
			"summary": summary,
			"id":      saved.ID,
			"created": saved.Created,
			"updated": saved.Updated,
		})
	})

	// Text polishing endpoint - three built-in modes plus custom prompt
	group.POST("/polish", func(c *echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			Content string `json:"content"`
			Mode    string `json:"mode"` // "medium" | "strong" | "custom"
			Prompt  string `json:"prompt"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		content := strings.TrimSpace(body.Content)
		if content == "" {
			return badRequest("content is required", nil)
		}
		mode := strings.ToLower(strings.TrimSpace(body.Mode))
		if mode == "" {
			mode = "medium"
		}
		if mode != "medium" && mode != "strong" && mode != "custom" {
			return badRequest("mode must be 'medium', 'strong' or 'custom'", nil)
		}

		// Load AI config
		apiKey, _ := configService.GetString(userId, "ai.api_key")
		baseURL, _ := configService.GetString(userId, "ai.base_url")
		model, _ := configService.GetString(userId, "ai.chat_model")
		enabled, _ := configService.GetBool(userId, "ai.enabled")
		if !enabled || apiKey == "" || baseURL == "" || model == "" {
			return serviceUnavailable("AI service is not configured", nil)
		}

		systemPrompt, err := aicore.PolishSystemPrompt(mode, body.Prompt)
		if err != nil {
			return badRequest(err.Error(), nil)
		}

		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
		defer cancel()

		cfg := aicore.AIConfig{Enabled: enabled, APIKey: apiKey, BaseURL: baseURL, Model: model}
		polished, err := aicore.ChatComplete(ctx, cfg, []aicore.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		}, false)
		if err != nil {
			return serverError("AI request failed: "+err.Error(), nil)
		}

		return c.JSON(http.StatusOK, map[string]any{
			"content": polished,
			"mode":    mode,
		})
	})
}

func fetchModels(baseURL, apiKey string) ([]modelInfo, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
	baseURL = strings.TrimSuffix(baseURL, "/v1")
	url := baseURL + "/v1/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}
	var modelsResp modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	return modelsResp.Data, nil
}

type sseWriter struct{ w http.ResponseWriter }

func (s *sseWriter) Write(p []byte) (int, error) { return s.w.Write(p) }
func (s *sseWriter) Flush() {
	if f, ok := s.w.(http.Flusher); ok {
		f.Flush()
	}
}
