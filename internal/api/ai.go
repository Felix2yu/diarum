package api

import (
	"bytes"
	"context"
	"encoding/json"
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
		enabled, _ := configService.GetBool(userId, "ai.enabled")
		return c.JSON(http.StatusOK, map[string]any{"api_key": apiKey, "base_url": baseUrl, "chat_model": chatModel, "embedding_model": embeddingModel, "enabled": enabled})
	})

	group.PUT("/settings", func(c echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			APIKey         string `json:"api_key"`
			BaseURL        string `json:"base_url"`
			ChatModel      string `json:"chat_model"`
			EmbeddingModel string `json:"embedding_model"`
			Enabled        bool   `json:"enabled"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		if body.Enabled && (body.APIKey == "" || body.BaseURL == "" || body.ChatModel == "" || body.EmbeddingModel == "") {
			return badRequest("All AI settings must be configured before enabling AI features", nil)
		}
		settings := map[string]any{"ai.api_key": body.APIKey, "ai.base_url": body.BaseURL, "ai.chat_model": body.ChatModel, "ai.embedding_model": body.EmbeddingModel, "ai.enabled": body.Enabled}
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

	// Week / Month analysis endpoint
	group.POST("/analysis", func(c echo.Context) error {
		userId := auth.CurrentUser(c).ID
		var body struct {
			Period string `json:"period"`
			Start  string `json:"start"`
			End    string `json:"end"`
		}
		if err := c.Bind(&body); err != nil {
			return badRequest("Invalid request body", err)
		}
		period := strings.ToLower(strings.TrimSpace(body.Period))
		if period != "week" && period != "month" {
			return badRequest("period must be 'week' or 'month'", nil)
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

		// Fetch diaries in range
		diaries, err := s.ListDiaries(userId, start+" 00:00:00.000Z", end+" 23:59:59.999Z", "-date", 0)
		if err != nil {
			return serverError("Failed to fetch diaries for analysis", err)
		}
		if len(diaries) == 0 {
			return c.JSON(http.StatusOK, map[string]any{
				"start":   start,
				"end":     end,
				"period":  period,
				"count":   0,
				"summary": "这个时间段内没有日记记录，无法进行分析。建议先记录一些日常内容，然后再尝试。",
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

		// Build prompt with diary content
		var sb strings.Builder
		periodLabel := "本周"
		if period == "month" {
			periodLabel = "本月"
		}
		sb.WriteString(fmt.Sprintf("以下是%s（%s 至 %s）的日记记录，共 %d 篇。请根据内容进行重组、分析，并给出建议。\n\n", periodLabel, start, end, len(diaries)))
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

		systemPrompt := "你是一个贴心的日记分析助手，基于用户提供的日记内容进行深入分析。你需要：\n1) 归纳总结日记的主要内容；\n2) 分析用户的情绪变化、生活模式；\n3) 找出亮点和值得改进的地方；\n4) 给出具体、可操作的建议。\n请用温暖、鼓励且理性的语气，分段输出，便于阅读。使用中文回答。"

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

		return c.JSON(http.StatusOK, map[string]any{
			"start":   start,
			"end":     end,
			"period":  period,
			"count":   len(diaries),
			"summary": summary,
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
