package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/songtianlun/diarum/internal/auth"
	"github.com/songtianlun/diarum/internal/chat"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/embedding"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
)

type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

func RegisterAIRoutes(e *echo.Echo, s *store.Store, authMiddleware echo.MiddlewareFunc, embeddingService *embedding.EmbeddingService) {
	configService := config.NewConfigService(s)
	chatService := chat.NewChatService(s, embeddingService)
	group := e.Group("/api/v1/ai", authMiddleware)

	group.GET("/settings", func(c echo.Context) error {
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
			"api_key":                 apiKey,
			"base_url":                baseUrl,
			"chat_model":              chatModel,
			"embedding_model":         embeddingModel,
			"analysis_system_prompt":  analysisSystemPrompt,
			"analysis_user_prefix":    analysisUserPrefix,
			"enabled":                 enabled,
			"speech_provider":         speechProvider,
			"speech_base_url":         speechBaseUrl,
			"speech_api_key":          speechAPIKey,
			"speech_model":            speechModel,
			"speech_language":         speechLanguage,
		})
	})

	group.PUT("/settings", func(c echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			APIKey                string `json:"api_key"`
			BaseURL               string `json:"base_url"`
			ChatModel             string `json:"chat_model"`
			EmbeddingModel        string `json:"embedding_model"`
			AnalysisSystemPrompt  string `json:"analysis_system_prompt"`
			AnalysisUserPrefix    string `json:"analysis_user_prefix"`
			Enabled               bool   `json:"enabled"`
			SpeechProvider        string `json:"speech_provider"`
			SpeechBaseURL         string `json:"speech_base_url"`
			SpeechAPIKey          string `json:"speech_api_key"`
			SpeechModel           string `json:"speech_model"`
			SpeechLanguage        string `json:"speech_language"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		if body.Enabled && (body.APIKey == "" || body.BaseURL == "" || body.ChatModel == "" || body.EmbeddingModel == "") {
			return badRequest("All AI settings must be configured before enabling AI features", nil)
		}
		settings := map[string]any{
			"ai.api_key":                 body.APIKey,
			"ai.base_url":                body.BaseURL,
			"ai.chat_model":              body.ChatModel,
			"ai.embedding_model":         body.EmbeddingModel,
			"ai.analysis_system_prompt":  body.AnalysisSystemPrompt,
			"ai.analysis_user_prefix":    body.AnalysisUserPrefix,
			"ai.enabled":                 body.Enabled,
			"ai.speech.provider":         body.SpeechProvider,
			"ai.speech.base_url":         body.SpeechBaseURL,
			"ai.speech.api_key":          body.SpeechAPIKey,
			"ai.speech.model":            body.SpeechModel,
			"ai.speech.language":         body.SpeechLanguage,
		}
		if err := configService.SetBatch(userId, settings); err != nil {
			return badRequest("Failed to save AI settings", err)
		}
		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	group.POST("/models", func(c echo.Context) error {
		var body struct {
			APIKey  string `json:"api_key"`
			BaseURL string `json:"base_url"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
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
	group.POST("/transcribe", func(c echo.Context) error {
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

		// Build multipart request body for the upstream provider
		var requestBody bytes.Buffer
		boundary := "diarum-boundary-" + fmt.Sprintf("%d", time.Now().UnixNano())
		writer := multipart.NewWriter(&requestBody)

		if err := writer.SetBoundary(boundary); err != nil {
			return serverError("Failed to build request", err)
		}

		if err := writer.WriteField("model", model); err != nil {
			return serverError("Failed to build request", err)
		}
		if language != "" {
			if err := writer.WriteField("language", language); err != nil {
				return serverError("Failed to build request", err)
			}
		}
		if prompt != "" {
			if err := writer.WriteField("prompt", prompt); err != nil {
				return serverError("Failed to build request", err)
			}
		}
		// Always request JSON text output
		if err := writer.WriteField("response_format", "json"); err != nil {
			return serverError("Failed to build request", err)
		}
		if err := writer.WriteField("temperature", "0"); err != nil {
			return serverError("Failed to build request", err)
		}

		// Derive a reasonable filename / content type for the upstream provider
		origName := file.Filename
		if origName == "" {
			origName = "audio.webm"
		}
		part, err := writer.CreateFormFile("file", origName)
		if err != nil {
			return serverError("Failed to build request", err)
		}
		if _, err := part.Write(audioBytes); err != nil {
			return serverError("Failed to build request", err)
		}
		if err := writer.Close(); err != nil {
			return serverError("Failed to build request", err)
		}

		// Build upstream URL
		baseURL = strings.TrimSuffix(baseURL, "/")
		transcribeURL := baseURL + "/v1/audio/transcriptions"

		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, transcribeURL, &requestBody)
		if err != nil {
			return serverError("Failed to create speech request", err)
		}
		req.Header.Set("Authorization", "Bearer "+apiKey)
		req.Header.Set("Content-Type", writer.FormDataContentType())

		client := &http.Client{Timeout: 5 * time.Minute}
		resp, err := client.Do(req)
		if err != nil {
			return serverError("Speech recognition request failed: "+err.Error(), nil)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			bodyText, _ := io.ReadAll(resp.Body)
			return serverError(fmt.Sprintf("Speech provider returned status %d: %s", resp.StatusCode, string(bodyText)), nil)
		}

		var transcript struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&transcript); err != nil {
			return serverError("Failed to decode speech response", err)
		}

		return c.JSON(http.StatusOK, map[string]any{
			"text": transcript.Text,
		})
	})

	group.POST("/vectors/build", func(c echo.Context) error {
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
	group.POST("/vectors/build-incremental", func(c echo.Context) error {
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

	group.GET("/vectors/stats", func(c echo.Context) error {
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

	group.GET("/conversations", func(c echo.Context) error {
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

	group.POST("/conversations", func(c echo.Context) error {
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

	group.GET("/conversations/:id", func(c echo.Context) error {
		userId := auth.CurrentUser(c).ID
		conv, err := s.GetConversation(c.PathParam("id"), userId)
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

	group.DELETE("/conversations/:id", func(c echo.Context) error {
		userId := auth.CurrentUser(c).ID
		if err := s.DeleteConversation(c.PathParam("id"), userId); err != nil {
			return notFound("Conversation not found")
		}
		return c.JSON(http.StatusOK, map[string]any{"success": true})
	})

	group.PUT("/conversations/:id", func(c echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			Title string `json:"title"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		conv, err := s.UpdateConversationTitle(c.PathParam("id"), userId, body.Title)
		if err != nil {
			return notFound("Conversation not found")
		}
		return c.JSON(http.StatusOK, map[string]any{"id": conv.ID, "title": conv.Title, "updated": conv.Updated})
	})

	group.POST("/chat", func(c echo.Context) error {
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

	// Analysis - retrieve saved result (week / month / custom)
	group.GET("/analysis", func(c echo.Context) error {
		userId := auth.CurrentUser(c).ID
		period := strings.ToLower(strings.TrimSpace(c.QueryParam("period")))
		start := strings.TrimSpace(c.QueryParam("start"))
		end := strings.TrimSpace(c.QueryParam("end"))
		keywords := strings.TrimSpace(c.QueryParam("keywords"))
		if period != "week" && period != "month" && period != "custom" {
			return badRequest("period must be 'week', 'month' or 'custom'", nil)
		}
		if start == "" || end == "" {
			return badRequest("start and end are required", nil)
		}
		a, err := s.GetPeriodAnalysis(userId, period, start, end, keywords)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return c.JSON(http.StatusOK, map[string]any{"found": false})
			}
			return serverError("Failed to load period analysis", err)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"found":         true,
			"id":            a.ID,
			"period":        a.Period,
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
	group.GET("/analyses", func(c echo.Context) error {
		userId := auth.CurrentUser(c).ID
		period := strings.ToLower(strings.TrimSpace(c.QueryParam("period")))
		if period != "" && period != "week" && period != "month" && period != "custom" && period != "all" {
			return badRequest("period must be 'week', 'month', 'custom' or 'all'", nil)
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

	// Analysis endpoint - generate and save. Supports custom date ranges and
	// keyword / content filtering so users can analyze only matching diary entries.
	group.POST("/analysis", func(c echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			Period       string `json:"period"`
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
		if period != "week" && period != "month" && period != "custom" {
			return badRequest("period must be 'week', 'month' or 'custom'", nil)
		}
		start := strings.TrimSpace(body.Start)
		end := strings.TrimSpace(body.End)
		if start == "" || end == "" {
			return badRequest("start and end are required", nil)
		}
		if _, err := time.Parse("2006-01-02", start); err != nil {
			return badRequest("start must be in YYYY-MM-DD format", err)
		}
		if _, err := time.Parse("2006-01-02", end); err != nil {
			return badRequest("end must be in YYYY-MM-DD format", err)
		}
		keywords := strings.TrimSpace(body.Keywords)

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
				"keywords": keywords,
				"count":    0,
				"summary":  emptySummary,
			})
		}

		// Load AI config
		cfgService := config.NewConfigService(s)
		apiKey, _ := cfgService.GetString(userId, "ai.api_key")
		baseURL, _ := cfgService.GetString(userId, "ai.base_url")
		model, _ := cfgService.GetString(userId, "ai.chat_model")
		enabled, _ := cfgService.GetBool(userId, "ai.enabled")
		if !enabled || apiKey == "" || baseURL == "" || model == "" {
			return badRequest("AI settings are not configured", nil)
		}

		// Resolve prompts: request override → saved config → default
		savedSystemPrompt, _ := cfgService.GetString(userId, "ai.analysis_system_prompt")
		savedUserPrefix, _ := cfgService.GetString(userId, "ai.analysis_user_prefix")
		defaultSystemPrompt := "你是一个贴心的日记分析助手，基于用户提供的日记内容进行深入分析。你需要：\n1) 归纳总结日记的主要内容；\n2) 留意每篇日记的日期，分析情绪变化、生活模式在时间线上的演变；\n3) 找出亮点和值得改进的地方；\n4) 给出具体、可操作的建议。\n请用温暖、鼓励且理性的语气，分段输出，便于阅读。使用中文回答。"

		var periodLabel string
		switch period {
		case "month":
			periodLabel = "本月"
		case "week":
			periodLabel = "本周"
		default:
			periodLabel = "所选时间段"
		}
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
			if d.Mood != "" {
				sb.WriteString(fmt.Sprintf("心情：%s\n", d.Mood))
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
		saved, saveErr := s.SavePeriodAnalysis(userId, period, start, end, len(diaries), summary, systemPrompt, userPrefix, keywords)

		response := map[string]any{
			"start":         start,
			"end":           end,
			"period":        period,
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

	// Text polishing endpoint - three built-in modes plus custom prompt
	group.POST("/polish", func(c echo.Context) error {
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
		cfgService := config.NewConfigService(s)
		apiKey, _ := cfgService.GetString(userId, "ai.api_key")
		baseURL, _ := cfgService.GetString(userId, "ai.base_url")
		model, _ := cfgService.GetString(userId, "ai.chat_model")
		enabled, _ := cfgService.GetBool(userId, "ai.enabled")
		if !enabled || apiKey == "" || baseURL == "" || model == "" {
			return badRequest("AI settings are not configured", nil)
		}

		var systemPrompt string
		switch mode {
		case "medium":
			systemPrompt = "你是一个日记文本整理助手。请对用户提供的日记文本进行以下处理：\n1) 去除口语化的语气词（如「哈哈」、「呃」、「嗯」、「嘛」、「吧」的冗余使用）、多余的标点和感叹；\n2) 纠正明显的错别字、语法错误和语病；\n3) 根据内容含义自动分段，使段落结构清晰、阅读流畅；\n4) 保留原文的核心事实、情感表达和个人口吻，不要增加新的事件或情节；\n5) 输出只返回整理后的文本本身，不要包含解释、说明或额外文字。"
		case "strong":
			systemPrompt = "你是一个日记文本改写助手。请对用户提供的日记文本进行深度重组和精简：\n1) 主动重组句子结构，使其更通顺、逻辑更清晰；\n2) 去除一切冗余、重复、流水账式的描述，保留最有意义的内容；\n3) 让表达更书面、更精炼，但仍保持自然和个人的语气；\n4) 适当补充过渡语句，使段落之间衔接自然；\n5) 不要虚构新的事件或情节；\n6) 输出只返回改写后的文本本身，不要包含解释、说明或额外文字。"
		case "custom":
			systemPrompt = strings.TrimSpace(body.Prompt)
			if systemPrompt == "" {
				return badRequest("prompt is required for custom mode", nil)
			}
		}

		ctx, cancel := context.WithTimeout(c.Request().Context(), 5*time.Minute)
		defer cancel()

		baseURL = strings.TrimSuffix(baseURL, "/")
		url := baseURL + "/v1/chat/completions"

		reqBody := map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": content},
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

		polished := ""
		if len(aiResp.Choices) > 0 {
			polished = strings.TrimSpace(aiResp.Choices[0].Message.Content)
		}

		return c.JSON(http.StatusOK, map[string]any{
			"content": polished,
			"mode":    mode,
		})
	})
}

func fetchModels(baseURL, apiKey string) ([]ModelInfo, error) {
	baseURL = strings.TrimSuffix(baseURL, "/")
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
	var modelsResp ModelsResponse
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
