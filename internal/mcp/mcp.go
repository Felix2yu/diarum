package mcp

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/songtianlun/diarum/internal/aicore"
	"github.com/songtianlun/diarum/internal/backup"
	"github.com/songtianlun/diarum/internal/chat"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/embedding"
	"github.com/songtianlun/diarum/internal/logger"
	"github.com/songtianlun/diarum/internal/store"
	"github.com/songtianlun/diarum/internal/weather"
)

// Server wraps the MCP server with Diarum integration
type Server struct {
	mcpServer        *server.MCPServer
	store            *store.Store
	configService    *config.ConfigService
	embeddingService *embedding.EmbeddingService
	chatService      *chat.ChatService
	backupScheduler  *backup.Scheduler
	onDiaryChanged   func(string)
}

// New creates a new MCP server integrated with Diarum.
// configService, embeddingService, chatService, backupScheduler and onDiaryChanged
// are optional (nil-safe); they gate the correction, semantic search, chat and backup tools respectively.
func New(appStore *store.Store, configService *config.ConfigService, onDiaryChanged func(string), embeddingService *embedding.EmbeddingService, chatService *chat.ChatService, backupScheduler *backup.Scheduler) *Server {
	s := &Server{
		store:            appStore,
		configService:    configService,
		embeddingService: embeddingService,
		chatService:      chatService,
		backupScheduler:  backupScheduler,
		onDiaryChanged:   onDiaryChanged,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"diarum",
		"1.0.0",
		server.WithToolCapabilities(true),
		server.WithPromptCapabilities(false),
	)

	s.mcpServer = mcpServer

	// Register all tools
	s.registerDiaryTools()
	s.registerDiaryEditTools()
	s.registerSearchTools()
	s.registerStatsTools()
	s.registerWeatherTools()
	s.registerCorrectionTools()
	s.registerPeriodTools()
	s.registerEmbeddingTools()
	s.registerChatTools()
	s.registerSettingsTools()
	s.registerBackupTools()
	s.registerPrompts()

	return s
}

// notifyChanged triggers the embedding rebuild hook (if configured) after a write.
func (s *Server) notifyChanged(userID string) {
	if s.onDiaryChanged != nil {
		s.onDiaryChanged(userID)
	}
}

// addLoggedTool registers a tool on the MCP server with request-level logging.
// Each invocation logs tool name, user, duration, args (keys only), and result.
// Handler panics are recovered and converted to error results so the server
// doesn't crash on a single bad call.
func (s *Server) addLoggedTool(tool mcp.Tool, handler server.ToolHandlerFunc) {
	name := tool.Name
	s.mcpServer.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (result *mcp.CallToolResult, err error) {
		start := time.Now()
		userID := getUserID(ctx)
		argKeys := make([]string, 0)
		for k := range req.GetArguments() {
			argKeys = append(argKeys, k)
		}
		defer func() {
			elapsed := time.Since(start)
			if r := recover(); r != nil {
				logger.Error("[MCP] tool panic user=%s tool=%s args=%v duration=%v panic=%v",
					userID, name, argKeys, elapsed, r)
				err = fmt.Errorf("tool %q panicked: %v", name, r)
				result = mcp.NewToolResultError(err.Error())
				return
			}
			status := "ok"
			if err != nil {
				status = "err"
			} else if result != nil && result.IsError {
				status = "err"
			}
			if status == "ok" {
				logger.Info("[MCP] tool=%s user=%s args=%v status=%s duration=%v",
					name, userID, argKeys, status, elapsed)
			} else {
				logger.Warn("[MCP] tool=%s user=%s args=%v status=%s duration=%v err=%v",
					name, userID, argKeys, status, elapsed, err)
			}
		}()
		return handler(ctx, req)
	})
}

// handleCreateOrUpsertDiary is the shared handler for create_diary and upsert_diary.
func (s *Server) handleCreateOrUpsertDiary(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	userID := getUserID(ctx)
	if userID == "" {
		return mcp.NewToolResultError("Authentication required"), nil
	}

	date := req.GetString("date", "")
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return mcp.NewToolResultError("'date' must be in YYYY-MM-DD format"), nil
	}

	// content 为可选字段：未提供时保留该日期已有日记的正文，
	// 避免仅编辑心情/标签等元数据时误清空正文。
	var content string
	args := req.GetArguments()
	has := func(k string) bool { _, ok := args[k]; return ok }
	if has("content") {
		content = strings.ReplaceAll(req.GetString("content", ""), "\\n", "\n")
	} else if existing, err := s.store.GetDiaryByDate(userID, date+" 00:00:00.000Z", date+" 23:59:59.999Z"); err == nil && existing != nil {
		content = existing.Content
	}

	var mood *int
	if has("mood") {
		v := req.GetInt("mood", 0)
		if v < 1 || v > 5 {
			return mcp.NewToolResultError("'mood' must be between 1 and 5"), nil
		}
		mood = &v
	}
	var weather *string
	if has("weather") {
		v := req.GetString("weather", "")
		weather = &v
	}
	var city *string
	if has("city") {
		v := req.GetString("city", "")
		city = &v
	}
	var tempMin, tempMax *float64
	if has("temp_min") {
		v := req.GetFloat("temp_min", 0)
		tempMin = &v
	}
	if has("temp_max") {
		v := req.GetFloat("temp_max", 0)
		tempMax = &v
	}
	var moodStates, scenarios, tags *[]string
	if has("mood_states") {
		v := req.GetStringSlice("mood_states", nil)
		moodStates = &v
	}
	if has("scenarios") {
		v := req.GetStringSlice("scenarios", nil)
		scenarios = &v
	}
	if has("tags") {
		v := req.GetStringSlice("tags", nil)
		tags = &v
	}

	diary, created, err := s.store.UpsertDiary(userID, date, content, mood, moodStates, scenarios, tags, weather, city, tempMin, tempMax)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("Failed to save diary: %v", err)), nil
	}
	if s.onDiaryChanged != nil {
		s.onDiaryChanged(userID)
	}

	action := "updated"
	if created {
		action = "created"
	}

	result, _ := json.Marshal(map[string]interface{}{
		"action":  action,
		"diary":   diary,
		"message": fmt.Sprintf("Diary %s for %s", action, date),
	})

	return mcp.NewToolResultText(string(result)), nil
}

// aiChatConfig resolves the chat-completion provider config for a user.
func (s *Server) aiChatConfig(userID string) (aicore.AIConfig, bool) {
	if s.configService == nil {
		return aicore.AIConfig{}, false
	}
	enabled, _ := s.configService.GetBool(userID, "ai.enabled")
	apiKey, _ := s.configService.GetString(userID, "ai.api_key")
	baseURL, _ := s.configService.GetString(userID, "ai.base_url")
	model, _ := s.configService.GetString(userID, "ai.chat_model")
	return aicore.AIConfig{Enabled: enabled, APIKey: apiKey, BaseURL: baseURL, Model: model}, true
}

// speechConfig resolves the speech-to-text provider config for a user, falling
// back to the shared AI credentials when the dedicated speech values are empty.
func (s *Server) speechConfig(userID string) (aicore.AIConfig, bool) {
	if s.configService == nil {
		return aicore.AIConfig{}, false
	}
	provider, _ := s.configService.GetString(userID, "ai.speech.provider")
	if provider == "" || provider == "none" {
		return aicore.AIConfig{}, false
	}
	baseURL, _ := s.configService.GetString(userID, "ai.speech.base_url")
	apiKey, _ := s.configService.GetString(userID, "ai.speech.api_key")
	model, _ := s.configService.GetString(userID, "ai.speech.model")
	if baseURL == "" || apiKey == "" {
		fbBase, _ := s.configService.GetString(userID, "ai.base_url")
		fbKey, _ := s.configService.GetString(userID, "ai.api_key")
		if fbBase != "" && fbKey != "" {
			baseURL, apiKey = fbBase, fbKey
		}
	}
	return aicore.AIConfig{Enabled: true, APIKey: apiKey, BaseURL: baseURL, Model: model}, true
}

// GetStreamableHTTPServer returns a Streamable HTTP server
func (s *Server) GetStreamableHTTPServer() *server.StreamableHTTPServer {
	return server.NewStreamableHTTPServer(s.mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
		server.WithStreamableHTTPCORS(
			server.WithCORSAllowedOrigins("*"),
			server.WithCORSAllowedMethods("GET", "POST", "OPTIONS"),
			server.WithCORSAllowedHeaders("Content-Type", "Authorization", "Accept"),
		),
		server.WithProtectedResourceMetadata(server.ProtectedResourceMetadataConfig{
			Resource:              "/mcp",
			BearerMethodsSupported: []string{"header"},
		}),
	)
}

// ContextKey is a custom type for context keys to avoid collisions
type ContextKey string

// UserIDKey is the context key for the authenticated user ID
const UserIDKey ContextKey = "user_id"

// getUserID extracts user ID from context
func getUserID(ctx context.Context) string {
	if userID, ok := ctx.Value(UserIDKey).(string); ok {
		return userID
	}
	return ""
}

// registerDiaryTools registers diary CRUD tools
func (s *Server) registerDiaryTools() {
	// Create / upsert Diary (two tool names, same handler)
	createDiary := mcp.NewTool("create_diary",
		mcp.WithDescription("Create or update a diary entry for a specific date (upsert semantics: updates existing or creates new)"),
		mcp.WithString("date", mcp.Required(), mcp.Description("Date in YYYY-MM-DD format")),
		mcp.WithString("content", mcp.Description("Diary content (HTML or plain text)")),
		mcp.WithNumber("mood", mcp.Description("Mood rating 1-5 (1=bad, 5=great)")),
		mcp.WithArray("mood_states", mcp.WithStringItems(), mcp.Description("List of mood states")),
		mcp.WithArray("scenarios", mcp.WithStringItems(), mcp.Description("List of scenarios")),
		mcp.WithString("weather", mcp.Description("Weather description")),
		mcp.WithString("city", mcp.Description("City name")),
		mcp.WithNumber("temp_min", mcp.Description("Minimum temperature in °C")),
		mcp.WithNumber("temp_max", mcp.Description("Maximum temperature in °C")),
		mcp.WithArray("tags", mcp.WithStringItems(), mcp.Description("List of tags")),
	)

	upsertDiary := mcp.NewTool("upsert_diary",
		mcp.WithDescription("Alias of create_diary. Create or update a diary by date (upsert semantics)."),
		mcp.WithString("date", mcp.Required(), mcp.Description("Date in YYYY-MM-DD format")),
		mcp.WithString("content", mcp.Description("Diary content (HTML or plain text)")),
		mcp.WithNumber("mood", mcp.Description("Mood rating 1-5 (1=bad, 5=great)")),
		mcp.WithArray("mood_states", mcp.WithStringItems(), mcp.Description("List of mood states")),
		mcp.WithArray("scenarios", mcp.WithStringItems(), mcp.Description("List of scenarios")),
		mcp.WithString("weather", mcp.Description("Weather description")),
		mcp.WithString("city", mcp.Description("City name")),
		mcp.WithNumber("temp_min", mcp.Description("Minimum temperature in °C")),
		mcp.WithNumber("temp_max", mcp.Description("Maximum temperature in °C")),
		mcp.WithArray("tags", mcp.WithStringItems(), mcp.Description("List of tags")),
	)

	s.addLoggedTool(createDiary, s.handleCreateOrUpsertDiary)
	s.addLoggedTool(upsertDiary, s.handleCreateOrUpsertDiary)

	// Get Diary
	getDiary := mcp.NewTool("get_diary",
		mcp.WithDescription("Get a diary entry by date or ID"),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format")),
		mcp.WithString("id", mcp.Description("Diary ID")),
	)

	s.addLoggedTool(getDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		id := req.GetString("id", "")
		date := req.GetString("date", "")

		if id == "" && date == "" {
			return mcp.NewToolResultError("Either 'id' or 'date' must be provided"), nil
		}

		var diary *store.Diary
		var err error

		if id != "" {
			diary, err = s.store.GetDiaryByID(id)
		} else {
			diary, err = s.store.GetDiaryByDate(userID, date+" 00:00:00.000Z", date+" 23:59:59.999Z")
		}

		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get diary: %v", err)), nil
		}
		if diary == nil {
			return mcp.NewToolResultError("Diary not found"), nil
		}

		result, _ := json.Marshal(diary)
		return mcp.NewToolResultText(string(result)), nil
	})

	// Delete Diary
	deleteDiary := mcp.NewTool("delete_diary",
		mcp.WithDescription("Delete a diary entry by ID"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Diary ID")),
	)

	s.addLoggedTool(deleteDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		id := req.GetString("id", "")

		err := s.store.DeleteDiary(id, userID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete diary: %v", err)), nil
		}

		s.notifyChanged(userID)

		result, _ := json.Marshal(map[string]interface{}{
			"action":  "deleted",
			"message": fmt.Sprintf("Diary %s deleted", id),
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// List Recent Diaries
	listRecent := mcp.NewTool("list_recent_diaries",
		mcp.WithDescription("Get recent diary entries"),
		mcp.WithNumber("limit", mcp.Description("Number of entries to return (default 10)")),
	)

	s.addLoggedTool(listRecent, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		limit := req.GetInt("limit", 10)
		diaries, err := s.store.ListDiaries(userID, "", "", "-date", limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list diaries: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"diaries": diaries,
			"count":   len(diaries),
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// On This Day: diaries from the same month-day in other years
	onThisDay := mcp.NewTool("on_this_day",
		mcp.WithDescription("Get diary entries written on the same month-day in previous years (往年今日). Defaults to today if 'date' is omitted."),
		mcp.WithString("date", mcp.Description("Reference date in YYYY-MM-DD (default: today)")),
	)
	s.addLoggedTool(onThisDay, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		date := strings.TrimSpace(req.GetString("date", ""))
		if date == "" {
			date = time.Now().UTC().Format("2006-01-02")
		} else if _, err := time.Parse("2006-01-02", date); err != nil {
			return mcp.NewToolResultError("'date' must be in YYYY-MM-DD format"), nil
		}
		diaries, err := s.store.GetDiariesByMonthDay(userID, date)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to query on-this-day diaries: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]interface{}{
			"date":    date,
			"diaries": diaries,
			"count":   len(diaries),
		})
		return mcp.NewToolResultText(string(out)), nil
	})

	// Random Diary: pick one random diary with meaningful content
	randomDiary := mcp.NewTool("random_diary",
		mcp.WithDescription("Return a single random diary entry (随机穿越). Prefers entries with content/mood/weather/tags. Use exclude_date to avoid today's entry."),
		mcp.WithString("exclude_date", mcp.Description("Exclude this date (YYYY-MM-DD) from the random pick")),
	)
	s.addLoggedTool(randomDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		exclude := strings.TrimSpace(req.GetString("exclude_date", ""))
		diary, err := s.store.GetRandomDiary(userID, exclude)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return mcp.NewToolResultError("No diary found"), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to pick random diary: %v", err)), nil
		}
		out, _ := json.Marshal(diary)
		return mcp.NewToolResultText(string(out)), nil
	})

	// Get Diaries by IDs: batch read by ID list
	getDiariesByIDs := mcp.NewTool("get_diaries_by_ids",
		mcp.WithDescription("Fetch multiple diary entries by their IDs in a single call. Returns only entries owned by the authenticated user."),
		mcp.WithArray("ids", mcp.Required(), mcp.WithStringItems(), mcp.Description("List of diary IDs")),
	)
	s.addLoggedTool(getDiariesByIDs, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		ids := req.GetStringSlice("ids", nil)
		if len(ids) == 0 {
			return mcp.NewToolResultError("'ids' is required and must be non-empty"), nil
		}
		result := make([]*store.Diary, 0, len(ids))
		for _, id := range ids {
			diary, err := s.store.GetDiaryByID(id)
			if err != nil || diary == nil || diary.Owner != userID {
				continue
			}
			result = append(result, diary)
		}
		out, _ := json.Marshal(map[string]interface{}{
			"diaries": result,
			"count":   len(result),
		})
		return mcp.NewToolResultText(string(out)), nil
	})

	// has_diary_content: check whether a given date range has any diary content
	hasContent := mcp.NewTool("has_diary_content",
		mcp.WithDescription("检查某个日期（或日期范围）是否有日记内容。返回 has_content(bool) + count(int)。常用于在写日记前检查该日期是否已有记录。"),
		mcp.WithString("date", mcp.Description("单日 YYYY-MM-DD（与 start_date/end_date 二选一）")),
		mcp.WithString("start_date", mcp.Description("范围开始 YYYY-MM-DD（含）")),
		mcp.WithString("end_date", mcp.Description("范围结束 YYYY-MM-DD（含）")),
		mcp.WithBoolean("exclude_empty", mcp.Description("排除只有天气/心情但无正文的条目（默认 false）")),
	)
	s.addLoggedTool(hasContent, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		date := strings.TrimSpace(req.GetString("date", ""))
		startDate := strings.TrimSpace(req.GetString("start_date", ""))
		endDate := strings.TrimSpace(req.GetString("end_date", ""))
		excludeEmpty := req.GetBool("exclude_empty", false)

		var count int
		var hasContent any

		if date != "" {
			if _, err := time.Parse("2006-01-02", date); err != nil {
				return mcp.NewToolResultError("'date' must be YYYY-MM-DD"), nil
			}
			if excludeEmpty {
				content, err := s.store.HasDiaryContent(userID, date)
				if err != nil {
					return mcp.NewToolResultError(fmt.Sprintf("Failed to check diary content: %v", err)), nil
				}
				hasContent = content
				count = 0
				if content {
					count = 1
				}
			} else {
				exists := s.store.DiaryExistsByDate(userID, date)
				hasContent = exists
				if exists {
					count = 1
				}
			}
		} else {
			if _, err := time.Parse("2006-01-02", startDate); err != nil {
				return mcp.NewToolResultError("'start_date' must be YYYY-MM-DD"), nil
			}
			if endDate != "" {
				if _, err := time.Parse("2006-01-02", endDate); err != nil {
					return mcp.NewToolResultError("'end_date' must be YYYY-MM-DD"), nil
				}
			}
			diaries, err := s.store.ListDiaries(userID, startDate+" 00:00:00.000Z", endDate+" 23:59:59.999Z", "date asc", 0)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to list diaries: %v", err)), nil
			}
			if excludeEmpty {
				filtered := diaries[:0]
				for _, d := range diaries {
					if strings.TrimSpace(d.Content) != "" {
						filtered = append(filtered, d)
					}
				}
				diaries = filtered
			}
			count = len(diaries)
			hasContent = count > 0
		}
		out, _ := json.Marshal(map[string]any{
			"has_content": hasContent,
			"count":       count,
			"date":        date,
			"start_date":  startDate,
			"end_date":    endDate,
		})
		return mcp.NewToolResultText(string(out)), nil
	})

	// list_diaries_by_tag: explicit tag filter
	listByTag := mcp.NewTool("list_diaries_by_tag",
		mcp.WithDescription("按标签列出日记（显式暴露 tag 筛选）。返回所有匹配条目。"),
		mcp.WithString("tag", mcp.Required(), mcp.Description("标签名（精确匹配）")),
	)
	s.addLoggedTool(listByTag, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		tag := strings.TrimSpace(req.GetString("tag", ""))
		if tag == "" {
			return mcp.NewToolResultError("'tag' is required"), nil
		}
		diaries, err := s.store.ListDiariesByTag(userID, tag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list by tag: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]any{
			"diaries": diaries,
			"total":   len(diaries),
			"tag":     tag,
		})
		return mcp.NewToolResultText(string(out)), nil
	})
}

// registerSearchTools registers search and filter tools
func (s *Server) registerSearchTools() {
	// Search Diaries
	searchDiaries := mcp.NewTool("search_diaries",
		mcp.WithDescription("Search diary content. By default uses keyword (SQL LIKE) search; set semantic=true to use vector/embedding semantic search (requires AI enabled and vector index built)."),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithString("scenario", mcp.Description("Filter by scenario (keyword mode only)")),
		mcp.WithNumber("limit", mcp.Description("Max results (default 50)")),
		mcp.WithBoolean("semantic", mcp.Description("Use vector semantic search instead of keyword search (default false)")),
	)

	s.addLoggedTool(searchDiaries, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		query := req.GetString("query", "")
		scenario := req.GetString("scenario", "")
		limit := req.GetInt("limit", 50)
		semantic := req.GetBool("semantic", false)

		// 语义搜索：走向量索引（embedding），scenario 过滤不适用
		if semantic {
			if s.embeddingService == nil {
				return mcp.NewToolResultError("Semantic search is not available (vector index not initialized)"), nil
			}
			results, err := s.embeddingService.QuerySimilar(ctx, userID, query, limit)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Semantic search failed: %v", err)), nil
			}
			out, _ := json.Marshal(map[string]interface{}{
				"diaries": results,
				"count":   len(results),
				"query":   query,
				"mode":    "semantic",
			})
			return mcp.NewToolResultText(string(out)), nil
		}

		diaries, err := s.store.SearchDiaries(userID, query, scenario, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to search diaries: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"diaries": diaries,
			"count":   len(diaries),
			"query":   query,
			"mode":    "keyword",
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Get Tags
	getTags := mcp.NewTool("get_tags",
		mcp.WithDescription("Get all tags with usage counts"),
	)

	s.addLoggedTool(getTags, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		tags, err := s.store.ListTagCounts(userID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get tags: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"tags":  tags,
			"count": len(tags),
		})

		return mcp.NewToolResultText(string(result)), nil
	})
}

// registerStatsTools registers statistics tools
func (s *Server) registerStatsTools() {
	// Get Stats
	getStats := mcp.NewTool("get_stats",
		mcp.WithDescription("Get diary statistics including total count"),
	)

	s.addLoggedTool(getStats, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		total := s.store.CountDiaries(userID)

		// 计算当前连续写作天数（streak）：从今天/昨天往前数连续有日记的天数
		streak := 0
		now := time.Now().UTC()
		oneYearAgo := now.AddDate(-1, 0, 0).Format("2006-01-02")
		diaries, _ := s.store.ListDiaries(userID, oneYearAgo+" 00:00:00.000Z", "", "-date", 366)
		dateSet := make(map[string]bool, len(diaries))
		for _, d := range diaries {
			dateSet[store.DateOnly(d.Date)] = true
		}
		today := now.Format("2006-01-02")
		yesterday := now.AddDate(0, 0, -1).Format("2006-01-02")
		var checkDate time.Time
		if dateSet[today] {
			checkDate = now
		} else if dateSet[yesterday] {
			checkDate = now.AddDate(0, 0, -1)
		}
		for !checkDate.IsZero() {
			if !dateSet[checkDate.Format("2006-01-02")] {
				break
			}
			streak++
			checkDate = checkDate.AddDate(0, 0, -1)
		}

		result, _ := json.Marshal(map[string]interface{}{
			"total":   total,
			"streak":  streak,
			"message": fmt.Sprintf("Total diaries: %d, current streak: %d days", total, streak),
		})

		return mcp.NewToolResultText(string(result)), nil
	})
}

// registerWeatherTools registers weather-related tools
func (s *Server) registerWeatherTools() {
	// Get Weather
	getWeather := mcp.NewTool("get_weather",
		mcp.WithDescription("Get weather forecast for a Chinese city"),
		mcp.WithString("city", mcp.Required(), mcp.Description("City name in Chinese (e.g., 北京, 上海)")),
	)

	s.addLoggedTool(getWeather, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		city := req.GetString("city", "")
		if city == "" {
			return mcp.NewToolResultError("City name is required"), nil
		}

		svc := weather.NewService()
		result, err := svc.GetWeather(city, "")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get weather: %v", err)), nil
		}

		weatherResult, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(weatherResult)), nil
	})

	// upsert_diary_weather: update (or create) only the weather fields on a diary,
	// without touching the content. Useful when calling get_weather first then
	// persisting the result.
	upsertWeather := mcp.NewTool("upsert_diary_weather",
		mcp.WithDescription("单独更新某天日记的天气信息，不改动正文。若当天没有日记记录，则创建一条只有天气字段的记录。"),
		mcp.WithString("date", mcp.Required(), mcp.Description("目标日期 YYYY-MM-DD")),
		mcp.WithString("city", mcp.Required(), mcp.Description("城市名（中文，如 北京）")),
		mcp.WithString("weather", mcp.Description("天气描述（如 晴、多云转小雨）")),
		mcp.WithNumber("temp_min", mcp.Description("最低温度（摄氏度）")),
		mcp.WithNumber("temp_max", mcp.Description("最高温度（摄氏度）")),
	)
	s.addLoggedTool(upsertWeather, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		date := strings.TrimSpace(req.GetString("date", ""))
		city := strings.TrimSpace(req.GetString("city", ""))
		if date == "" || city == "" {
			return mcp.NewToolResultError("'date' and 'city' are required"), nil
		}
		if _, err := time.Parse("2006-01-02", date); err != nil {
			return mcp.NewToolResultError("'date' must be YYYY-MM-DD"), nil
		}
		weatherStr := strings.TrimSpace(req.GetString("weather", ""))
		tempMin := req.GetFloat("temp_min", 0)
		tempMax := req.GetFloat("temp_max", 0)
		if _, err := s.store.UpsertDiaryWeather(userID, date, city, weatherStr, tempMin, tempMax); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to upsert weather: %v", err)), nil
		}
		if s.onDiaryChanged != nil {
			s.onDiaryChanged(userID)
		}
		out, _ := json.Marshal(map[string]any{
			"status": "ok", "date": date, "city": city,
			"weather": weatherStr, "temp_min": tempMin, "temp_max": tempMax,
		})
		return mcp.NewToolResultText(string(out)), nil
	})

	// weather_backfill: backfill weather for historical diaries that are missing it.
	// Non-streaming, runs synchronously. Returns summary stats.
	backfillWeather := mcp.NewTool("weather_backfill",
		mcp.WithDescription("为历史日记补全天气数据。从 start_date（或所有日记）开始遍历，调用天气 API 填充缺失的城市/天气/温度。同步执行，返回 updated/failed/skipped 统计。需要先在设置里配置默认城市 weather.default_city。"),
		mcp.WithString("start_date", mcp.Description("从该日期（YYYY-MM-DD）开始补全；留空则处理全部日记")),
		mcp.WithBoolean("skip_empty", mcp.Description("跳过无正文的日记（默认 true）")),
	)
	s.addLoggedTool(backfillWeather, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		startDate := strings.TrimSpace(req.GetString("start_date", ""))
		if startDate != "" {
			if _, err := time.Parse("2006-01-02", startDate); err != nil {
				return mcp.NewToolResultError("'start_date' must be YYYY-MM-DD"), nil
			}
		}
		skipEmpty := req.GetBool("skip_empty", true)

		defaultCity := ""
		if s.configService != nil {
			defaultCity, _ = s.configService.GetString(userID, "weather.default_city")
		}
		if defaultCity == "" {
			return mcp.NewToolResultError("Please configure weather.default_city setting first"), nil
		}

		var diaries []*store.Diary
		var err error
		if startDate != "" {
			diaries, err = s.store.ListDiaries(userID, startDate+" 00:00:00.000Z", "", "-date", 0)
		} else {
			diaries, err = s.store.ListDiaries(userID, "", "", "-date", 0)
		}
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch diaries: %v", err)), nil
		}

		svc := weather.NewService()
		updated := 0
		skipped := 0
		failed := 0

		for _, diary := range diaries {
			if skipEmpty && strings.TrimSpace(diary.Content) == "" {
				skipped++
				continue
			}
			if diary.Weather != "" && diary.City != "" {
				skipped++
				continue
			}
			targetCity := diary.City
			if targetCity == "" {
				targetCity = defaultCity
			}
			result, err := svc.GetWeather(targetCity, diary.Date)
			if err != nil {
				failed++
				continue
			}
			wmo := weather.WMOToSimple(result.WMOCode)
			var wLabel string
			if l, ok := weather.SimpleCodeInfo[wmo]; ok {
				wLabel = l.Label
			} else {
				wLabel = fmt.Sprintf("wmo:%d", result.WMOCode)
			}
			if _, err := s.store.UpsertDiaryWeather(userID, store.DateOnly(diary.Date), targetCity, wLabel, result.TempMin, result.TempMax); err != nil {
				failed++
				continue
			}
			updated++
		}

		if s.onDiaryChanged != nil && updated > 0 {
			s.onDiaryChanged(userID)
		}

		out, _ := json.Marshal(map[string]any{
			"updated":  updated,
			"skipped":  skipped,
			"failed":   failed,
			"total":    len(diaries),
			"city_used": defaultCity,
		})
		return mcp.NewToolResultText(string(out)), nil
	})
}

// ---------------------------------------------------------------------------
// Diary editing tools: id-based partial update, batch update/delete, filtered list
// ---------------------------------------------------------------------------

// diaryPatchArgs is the shared set of editable diary fields. Pointer fields stay
// nil when absent, which the store interprets as "leave unchanged".
type diaryPatchArgs struct {
	Content       *string   `json:"content"`
	Mood          *int      `json:"mood"`
	MoodStates    *[]string `json:"mood_states"`
	Scenarios     *[]string `json:"scenarios"`
	Weather       *string   `json:"weather"`
	City          *string   `json:"city"`
	TempMin       *float64  `json:"temp_min"`
	TempMax       *float64  `json:"temp_max"`
	Tags          *[]string `json:"tags"`
	TagsOp        string    `json:"tags_op"`
	ContentFormat string    `json:"content_format"`
}

type updateDiaryArgs struct {
	ID string `json:"id"`
	diaryPatchArgs
}

type diaryTargetsArgs struct {
	IDs       []string `json:"ids"`
	DateStart string   `json:"date_start"`
	DateEnd   string   `json:"date_end"`
	Tag       string   `json:"tag"`
	Scenario  string   `json:"scenario"`
	Query     string   `json:"query"`
}

type batchOptsArgs struct {
	DryRun          bool   `json:"dry_run"`
	ContinueOnError bool   `json:"continue_on_error"`
	TagsOp          string `json:"tags_op"`
	ContentFormat   string `json:"content_format"`
}

type batchUpdateArgs struct {
	Targets diaryTargetsArgs `json:"targets"`
	Patch   diaryPatchArgs   `json:"patch"`
	Opts    batchOptsArgs    `json:"opts"`
}

type createItemArgs struct {
	Date          string    `json:"date"`
	Content       *string   `json:"content"`
	Mood          *int      `json:"mood"`
	MoodStates    *[]string `json:"mood_states"`
	Scenarios     *[]string `json:"scenarios"`
	Weather       *string   `json:"weather"`
	City          *string   `json:"city"`
	TempMin       *float64  `json:"temp_min"`
	TempMax       *float64  `json:"temp_max"`
	Tags          *[]string `json:"tags"`
	TagsOp        string    `json:"tags_op"`
	ContentFormat string    `json:"content_format"`
}

type batchCreateArgs struct {
	Items []createItemArgs `json:"items"`
	Opts  batchOptsArgs    `json:"opts"`
}

func stringProp(desc string) map[string]any {
	return map[string]any{"type": "string", "description": desc}
}

func arrayStringProp(desc string) map[string]any {
	return map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": desc}
}

func targetsSchema() map[string]any {
	return map[string]any{
		"ids":        arrayStringProp("按 ID 列表定位（优先级最高）"),
		"date_start": stringProp("开始日期 YYYY-MM-DD（含）"),
		"date_end":   stringProp("结束日期 YYYY-MM-DD（含）"),
		"tag":        stringProp("按标签定位"),
		"scenario":   stringProp("按情景定位"),
		"query":      stringProp("按关键词搜索定位"),
	}
}

func patchSchema() map[string]any {
	return map[string]any{
		"content":        stringProp("日记正文（纯文本或 HTML）"),
		"mood":           map[string]any{"type": "integer", "description": "心情评分 1-5"},
		"mood_states":    arrayStringProp("心情状态"),
		"scenarios":      arrayStringProp("情景"),
		"weather":        stringProp("天气"),
		"city":           stringProp("城市"),
		"temp_min":       map[string]any{"type": "number"},
		"temp_max":       map[string]any{"type": "number"},
		"tags":           arrayStringProp("标签"),
		"tags_op":        stringProp("replace（默认，覆盖）| merge（合并）| remove（移除）"),
		"content_format": stringProp("text（默认，原样）| html（纯文本转 HTML 段落）"),
	}
}

func optsSchema() map[string]any {
	return map[string]any{
		"dry_run":           map[string]any{"type": "boolean", "description": "仅预览不落库"},
		"continue_on_error": map[string]any{"type": "boolean", "description": "单条失败继续"},
		"tags_op":           stringProp("默认标签操作"),
		"content_format":    stringProp("默认内容格式"),
	}
}

func createItemSchema() map[string]any {
	return map[string]any{
		"date":           stringProp("日记日期 YYYY-MM-DD（必填）"),
		"content":        stringProp("日记正文（纯文本或 HTML）"),
		"mood":           map[string]any{"type": "integer", "description": "心情评分 1-5"},
		"mood_states":    arrayStringProp("心情状态"),
		"scenarios":      arrayStringProp("情景"),
		"weather":        stringProp("天气"),
		"city":           stringProp("城市"),
		"temp_min":       map[string]any{"type": "number"},
		"temp_max":       map[string]any{"type": "number"},
		"tags":           arrayStringProp("标签"),
		"tags_op":        stringProp("replace（默认，覆盖）| merge（合并）| remove（移除）"),
		"content_format": stringProp("content 解释方式：text（默认，原样）| html（纯文本转 HTML 段落）"),
	}
}

func validatePatch(a diaryPatchArgs) error {
	if a.Mood != nil && (*a.Mood < 1 || *a.Mood > 5) {
		return errors.New("mood must be between 1 and 5")
	}
	if a.ContentFormat != "" && a.ContentFormat != "text" && a.ContentFormat != "html" {
		return errors.New("content_format must be 'text' or 'html'")
	}
	if a.TagsOp != "" && a.TagsOp != "replace" && a.TagsOp != "merge" && a.TagsOp != "remove" {
		return errors.New("tags_op must be 'replace', 'merge' or 'remove'")
	}
	return nil
}

func patchFromArgs(a diaryPatchArgs) store.DiaryPatch {
	p := store.DiaryPatch{
		Content:       a.Content,
		Mood:          a.Mood,
		MoodStates:    a.MoodStates,
		Scenarios:     a.Scenarios,
		Weather:       a.Weather,
		City:          a.City,
		TempMin:       a.TempMin,
		TempMax:       a.TempMax,
		Tags:          a.Tags,
		TagsOp:        a.TagsOp,
		ContentFormat: a.ContentFormat,
	}
	if p.Content != nil {
		// 处理 AI 客户端可能对换行符进行的双重转义
		v := strings.ReplaceAll(*p.Content, "\\n", "\n")
		p.Content = &v
	}
	return p
}

func (s *Server) buildBatchTargets(a diaryTargetsArgs) store.BatchTargets {
	t := store.BatchTargets{
		Tag:      strings.TrimSpace(a.Tag),
		Scenario: strings.TrimSpace(a.Scenario),
		Query:    strings.TrimSpace(a.Query),
	}
	if len(a.IDs) > 0 {
		t.IDs = a.IDs
	}
	if strings.TrimSpace(a.DateStart) != "" && strings.TrimSpace(a.DateEnd) != "" {
		t.DateRange = &store.DateRange{Start: strings.TrimSpace(a.DateStart), End: strings.TrimSpace(a.DateEnd)}
	}
	return t
}

func (s *Server) registerDiaryEditTools() {
	// update_diary: partial update by ID
	updateDiary := mcp.NewTool("update_diary",
		mcp.WithDescription("按 ID 局部更新一篇日记（内容/心情/情景/标签等字段可选）。未传字段保持不变，不会清空。"),
		mcp.WithString("id", mcp.Required(), mcp.Description("Diary ID")),
		mcp.WithString("content", mcp.Description("日记正文（纯文本或 HTML）")),
		mcp.WithNumber("mood", mcp.Description("心情评分 1-5")),
		mcp.WithArray("mood_states", mcp.WithStringItems(), mcp.Description("心情状态列表")),
		mcp.WithArray("scenarios", mcp.WithStringItems(), mcp.Description("情景列表")),
		mcp.WithString("weather", mcp.Description("天气描述")),
		mcp.WithString("city", mcp.Description("城市")),
		mcp.WithNumber("temp_min", mcp.Description("最低温")),
		mcp.WithNumber("temp_max", mcp.Description("最高温")),
		mcp.WithArray("tags", mcp.WithStringItems(), mcp.Description("标签列表")),
		mcp.WithString("tags_op", mcp.Description("标签操作：replace（默认，覆盖）| merge（合并）| remove（移除）")),
		mcp.WithString("content_format", mcp.Description("content 解释方式：text（默认，原样）| html（纯文本转 HTML 段落）")),
	)
	s.addLoggedTool(updateDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		var a updateDiaryArgs
		if err := req.BindArguments(&a); err != nil {
			return mcp.NewToolResultError("Invalid arguments: " + err.Error()), nil
		}
		if strings.TrimSpace(a.ID) == "" {
			return mcp.NewToolResultError("'id' is required"), nil
		}
		if err := validatePatch(a.diaryPatchArgs); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		patch := patchFromArgs(a.diaryPatchArgs)
		diary, err := s.store.UpdateDiaryByID(a.ID, userID, patch)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return mcp.NewToolResultError("Diary not found or not owned by user"), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update diary: %v", err)), nil
		}
		s.notifyChanged(userID)
		result, _ := json.Marshal(diary)
		return mcp.NewToolResultText(string(result)), nil
	})

	// batch_update_diaries
	batchUpdate := mcp.NewTool("batch_update_diaries",
		mcp.WithDescription("批量局部更新日记。通过 targets 选择器（ids/日期范围/标签/情景/关键词）定位目标，patch 为要应用的字段，opts 控制预览与容错。"),
		mcp.WithObject("targets", mcp.Description("目标选择器"), mcp.Properties(targetsSchema())),
		mcp.WithObject("patch", mcp.Description("要应用的字段（同 update_diary）"), mcp.Properties(patchSchema())),
		mcp.WithObject("opts", mcp.Description("选项"), mcp.Properties(optsSchema())),
	)
	s.addLoggedTool(batchUpdate, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		var a batchUpdateArgs
		if err := req.BindArguments(&a); err != nil {
			return mcp.NewToolResultError("Invalid arguments: " + err.Error()), nil
		}
		if err := validatePatch(a.Patch); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		targets := s.buildBatchTargets(a.Targets)
		patch := patchFromArgs(a.Patch)
		opts := store.BatchOpts{
			DryRun:          a.Opts.DryRun,
			ContinueOnError: a.Opts.ContinueOnError,
			TagsOp:          a.Opts.TagsOp,
			ContentFormat:   a.Opts.ContentFormat,
		}
		results, err := s.store.BatchUpdateDiaries(userID, targets, patch, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Batch update failed: %v", err)), nil
		}
		if !opts.DryRun {
			s.notifyChanged(userID)
		}
		ok, failed := 0, 0
		for _, r := range results {
			if r.Status == "ok" {
				ok++
			} else if r.Status == "error" {
				failed++
			}
		}
		out, _ := json.Marshal(map[string]any{
			"results": results,
			"applied": ok,
			"failed":  failed,
			"total":   len(results),
			"dry_run": opts.DryRun,
		})
		return mcp.NewToolResultText(string(out)), nil
	})

	// batch_create_diaries: bulk create in a single transaction (import-style).
	// Create semantics: existing entries for the same date are NOT overwritten.
	batchCreate := mcp.NewTool("batch_create_diaries",
		mcp.WithDescription("批量新增日记（单事务多创建，适合导入）。items 每项 date 必填（YYYY-MM-DD），其余字段可选。同一天已有日记的项会被跳过（skipped，不覆盖；如需修改已有日记请用 batch_update_diaries）。opts.dry_run 可预览（含冲突检测）不落库，opts.continue_on_error 单条失败继续。"),
		mcp.WithArray("items", mcp.Required(), mcp.Description("要新增的日记列表"),
			mcp.Items(map[string]any{
				"type":       "object",
				"properties": createItemSchema(),
				"required":   []string{"date"},
			}),
		),
		mcp.WithObject("opts", mcp.Description("选项"), mcp.Properties(optsSchema())),
	)
	s.addLoggedTool(batchCreate, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		var a batchCreateArgs
		if err := req.BindArguments(&a); err != nil {
			return mcp.NewToolResultError("Invalid arguments: " + err.Error()), nil
		}
		if len(a.Items) == 0 {
			return mcp.NewToolResultError("'items' is required and must be non-empty"), nil
		}
		items := make([]store.DiaryCreateInput, 0, len(a.Items))
		for i, it := range a.Items {
			if strings.TrimSpace(it.Date) == "" {
				return mcp.NewToolResultError(fmt.Sprintf("items[%d].date is required", i)), nil
			}
			if _, err := time.Parse("2006-01-02", strings.TrimSpace(it.Date)); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("items[%d].date must be YYYY-MM-DD", i)), nil
			}
			if it.Mood != nil && (*it.Mood < 1 || *it.Mood > 5) {
				return mcp.NewToolResultError(fmt.Sprintf("items[%d].mood must be between 1 and 5", i)), nil
			}
			if it.TagsOp != "" && it.TagsOp != "replace" && it.TagsOp != "merge" && it.TagsOp != "remove" {
				return mcp.NewToolResultError(fmt.Sprintf("items[%d].tags_op must be 'replace', 'merge' or 'remove'", i)), nil
			}
			if it.ContentFormat != "" && it.ContentFormat != "text" && it.ContentFormat != "html" {
				return mcp.NewToolResultError(fmt.Sprintf("items[%d].content_format must be 'text' or 'html'", i)), nil
			}
			if it.Content != nil {
				// 处理 AI 客户端可能对换行符进行的双重转义
				v := strings.ReplaceAll(*it.Content, "\\n", "\n")
				it.Content = &v
			}
			items = append(items, store.DiaryCreateInput{
				Date:          it.Date,
				Content:       it.Content,
				Mood:          it.Mood,
				MoodStates:    it.MoodStates,
				Scenarios:     it.Scenarios,
				Weather:       it.Weather,
				City:          it.City,
				TempMin:       it.TempMin,
				TempMax:       it.TempMax,
				Tags:          it.Tags,
				TagsOp:        it.TagsOp,
				ContentFormat: it.ContentFormat,
			})
		}
		opts := store.BatchOpts{
			DryRun:          a.Opts.DryRun,
			ContinueOnError: a.Opts.ContinueOnError,
		}
		results, err := s.store.BatchCreateDiaries(userID, items, opts)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Batch create failed: %v", err)), nil
		}
		if !opts.DryRun {
			s.notifyChanged(userID)
		}
		created, failed := 0, 0
		for _, r := range results {
			switch r.Status {
			case "ok":
				created++
			case "error":
				failed++
			}
		}
		out, _ := json.Marshal(map[string]any{
			"results": results,
			"created": created,
			"failed":  failed,
			"total":   len(results),
			"dry_run": opts.DryRun,
		})
		return mcp.NewToolResultText(string(out)), nil
	})

	// batch_delete_diaries
	batchDelete := mcp.NewTool("batch_delete_diaries",
		mcp.WithDescription("批量删除日记（按 ID 列表）"),
		mcp.WithArray("ids", mcp.Required(), mcp.WithStringItems(), mcp.Description("要删除的日记 ID 列表")),
	)
	s.addLoggedTool(batchDelete, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		ids := req.GetStringSlice("ids", nil)
		if len(ids) == 0 {
			return mcp.NewToolResultError("'ids' is required and must be non-empty"), nil
		}
		results, err := s.store.BatchDeleteDiaries(userID, ids)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Batch delete failed: %v", err)), nil
		}
		s.notifyChanged(userID)
		out, _ := json.Marshal(map[string]any{"results": results, "total": len(results)})
		return mcp.NewToolResultText(string(out)), nil
	})

	// list_diaries: filterable + paginated replacement for list_recent_diaries
	listDiaries := mcp.NewTool("list_diaries",
		mcp.WithDescription("筛选并分页列出日记（支持日期范围/标签/情景/关键词/心情）。"),
		mcp.WithString("date_start", mcp.Description("开始日期 YYYY-MM-DD")),
		mcp.WithString("date_end", mcp.Description("结束日期 YYYY-MM-DD")),
		mcp.WithString("tag", mcp.Description("按标签筛选")),
		mcp.WithString("scenario", mcp.Description("按情景筛选")),
		mcp.WithString("query", mcp.Description("关键词（匹配正文或标签）")),
		mcp.WithNumber("mood", mcp.Description("按心情筛选 1-5")),
		mcp.WithNumber("limit", mcp.Description("返回条数（默认 50，最大 500）")),
		mcp.WithNumber("offset", mcp.Description("偏移量（分页）")),
		mcp.WithString("order", mcp.Description("排序：-date（默认，倒序）| date（正序）")),
	)
	s.addLoggedTool(listDiaries, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		f := store.DiaryFilter{
			Tag:      strings.TrimSpace(req.GetString("tag", "")),
			Scenario: strings.TrimSpace(req.GetString("scenario", "")),
			Query:    strings.TrimSpace(req.GetString("query", "")),
			Limit:    req.GetInt("limit", 50),
			Offset:   req.GetInt("offset", 0),
			Order:    req.GetString("order", "-date"),
		}
		if ds := strings.TrimSpace(req.GetString("date_start", "")); ds != "" {
			if _, err := time.Parse("2006-01-02", ds); err != nil {
				return mcp.NewToolResultError("date_start must be YYYY-MM-DD"), nil
			}
			f.DateStart = ds
		}
		if de := strings.TrimSpace(req.GetString("date_end", "")); de != "" {
			if _, err := time.Parse("2006-01-02", de); err != nil {
				return mcp.NewToolResultError("date_end must be YYYY-MM-DD"), nil
			}
			f.DateEnd = de
		}
		if m := req.GetInt("mood", 0); m > 0 {
			f.Mood = m
		}
		diaries, err := s.store.ListDiariesFiltered(userID, f)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list diaries: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]any{"diaries": diaries, "count": len(diaries)})
		return mcp.NewToolResultText(string(out)), nil
	})
}

// ---------------------------------------------------------------------------
// Correction tools: polish / transcribe / correct_voice
// ---------------------------------------------------------------------------

func (s *Server) registerCorrectionTools() {
	// polish_diary
	polish := mcp.NewTool("polish_diary",
		mcp.WithDescription("纠正/整理日记文本。mode: medium（去口语词/纠错/分段）| strong（深度改写）| voice（语音整理专用）| custom（自定义 prompt）。apply=true 时写回 target_diary_id。"),
		mcp.WithString("content", mcp.Required(), mcp.Description("待纠正的文本")),
		mcp.WithString("mode", mcp.Description("medium | strong | voice | custom（默认 medium）")),
		mcp.WithString("prompt", mcp.Description("custom 模式下的自定义指令")),
		mcp.WithBoolean("apply", mcp.Description("是否将结果写回日记（需 target_diary_id）")),
		mcp.WithString("target_diary_id", mcp.Description("apply=true 时写回的目标日记 ID")),
	)
	s.addLoggedTool(polish, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		content := strings.TrimSpace(req.GetString("content", ""))
		if content == "" {
			return mcp.NewToolResultError("'content' is required"), nil
		}
		mode := strings.ToLower(strings.TrimSpace(req.GetString("mode", "medium")))
		apply := req.GetBool("apply", false)
		targetID := strings.TrimSpace(req.GetString("target_diary_id", ""))
		if apply && targetID == "" {
			return mcp.NewToolResultError("apply=true requires 'target_diary_id'"), nil
		}
		cfg, ok := s.aiChatConfig(userID)
		if !ok {
			return mcp.NewToolResultError("AI service is not configured"), nil
		}
		systemPrompt, err := aicore.PolishSystemPrompt(mode, req.GetString("prompt", ""))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		corrected, err := aicore.ChatComplete(ctx, cfg, []aicore.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: content},
		}, false)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("AI request failed: %v", err)), nil
		}
		out := map[string]any{"original": content, "corrected": corrected, "mode": mode, "applied": false}
		if apply {
			patch := store.DiaryPatch{Content: &corrected}
			if _, err := s.store.UpdateDiaryByID(targetID, userID, patch); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to apply: %v", err)), nil
			}
			s.notifyChanged(userID)
			out["applied"] = true
			out["target_diary_id"] = targetID
		}
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	})

	// transcribe_audio
	transcribe := mcp.NewTool("transcribe_audio",
		mcp.WithDescription("将音频转写为文本（依赖配置的语音识别服务）。"),
		mcp.WithString("audio_base64", mcp.Required(), mcp.Description("音频文件 base64 编码")),
		mcp.WithString("filename", mcp.Description("文件名（含扩展名，如 audio.webm）")),
		mcp.WithString("language", mcp.Description("语言代码，如 zh")),
		mcp.WithString("model", mcp.Description("模型名（默认 whisper-1）")),
		mcp.WithString("prompt", mcp.Description("转写提示词")),
	)
	s.addLoggedTool(transcribe, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		b64 := strings.TrimSpace(req.GetString("audio_base64", ""))
		if b64 == "" {
			return mcp.NewToolResultError("'audio_base64' is required"), nil
		}
		audio, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			return mcp.NewToolResultError("invalid base64 audio: " + err.Error()), nil
		}
		cfg, ok := s.speechConfig(userID)
		if !ok {
			return mcp.NewToolResultError("Speech recognition is not configured"), nil
		}
		text, err := aicore.Transcribe(ctx, cfg, audio, req.GetString("filename", ""), "", req.GetString("language", ""), req.GetString("model", ""), req.GetString("prompt", ""))
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Transcription failed: %v", err)), nil
		}
		b, _ := json.Marshal(map[string]any{"text": text})
		return mcp.NewToolResultText(string(b)), nil
	})

	// correct_voice_diary: transcribe (or raw_text) -> voice preset polish -> optional apply
	correctVoice := mcp.NewTool("correct_voice_diary",
		mcp.WithDescription("语音日记修正流水线：音频转写（或直接给 raw_text）→ 语音整理预设纠正 → 可选格式化并写回 target_diary_id。apply=false 时仅返回预览。"),
		mcp.WithString("target_diary_id", mcp.Description("写回目标日记 ID（apply=true 时必填）")),
		mcp.WithString("audio_base64", mcp.Description("音频 base64（与 raw_text 二选一）")),
		mcp.WithString("raw_text", mcp.Description("已转写的原始文本（与 audio_base64 二选一）")),
		mcp.WithString("filename", mcp.Description("音频文件名")),
		mcp.WithString("language", mcp.Description("转写语言代码")),
		mcp.WithBoolean("apply", mcp.Description("是否写回日记")),
		mcp.WithString("content_format", mcp.Description("写回格式：html（默认，纯文本转 HTML 段落）| text")),
	)
	s.addLoggedTool(correctVoice, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		audioB64 := strings.TrimSpace(req.GetString("audio_base64", ""))
		rawText := strings.TrimSpace(req.GetString("raw_text", ""))
		var original string
		if audioB64 != "" {
			audio, err := base64.StdEncoding.DecodeString(audioB64)
			if err != nil {
				return mcp.NewToolResultError("invalid base64 audio: " + err.Error()), nil
			}
			cfg, ok := s.speechConfig(userID)
			if !ok {
				return mcp.NewToolResultError("Speech recognition is not configured"), nil
			}
			t, err := aicore.Transcribe(ctx, cfg, audio, req.GetString("filename", ""), "", req.GetString("language", ""), "", "")
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Transcription failed: %v", err)), nil
			}
			original = t
		} else if rawText != "" {
			original = rawText
		} else {
			return mcp.NewToolResultError("provide either 'audio_base64' or 'raw_text'"), nil
		}

		cfg, ok := s.aiChatConfig(userID)
		if !ok {
			return mcp.NewToolResultError("AI service is not configured"), nil
		}
		systemPrompt, err := aicore.PolishSystemPrompt(aicore.ModeVoice, "")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		corrected, err := aicore.ChatComplete(ctx, cfg, []aicore.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: original},
		}, false)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("AI request failed: %v", err)), nil
		}

		apply := req.GetBool("apply", false)
		targetID := strings.TrimSpace(req.GetString("target_diary_id", ""))
		if apply && targetID == "" {
			return mcp.NewToolResultError("apply=true requires 'target_diary_id'"), nil
		}
		out := map[string]any{
			"original": original,
			"corrected": corrected,
			"applied":  false,
			"note":     "语音转写 → 语音整理预设纠正 → 格式化",
		}
		if apply {
			cf := strings.ToLower(strings.TrimSpace(req.GetString("content_format", "html")))
			if cf == "" {
				cf = "html"
			}
			patch := store.DiaryPatch{Content: &corrected, ContentFormat: cf}
			if _, err := s.store.UpdateDiaryByID(targetID, userID, patch); err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to apply: %v", err)), nil
			}
			s.notifyChanged(userID)
			out["applied"] = true
			out["target_diary_id"] = targetID
		}
		b, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(b)), nil
	})
}

// ---------------------------------------------------------------------------
// Period analysis tools: read/save/list week-month-year analyses backed by
// the store's period_analyses subsystem (period_key like 2026-W36 / 2026-09 / 2026).
// ---------------------------------------------------------------------------

func validPeriod(p string) bool {
	switch p {
	case "week", "month", "year", "custom":
		return true
	}
	return false
}

// periodKeyExample returns the current period key for a calendar period, used
// in error messages to teach clients the expected format.
func periodKeyExample(period string) string {
	now := time.Now().UTC()
	switch period {
	case "week":
		y, w := now.ISOWeek()
		return fmt.Sprintf("%d-W%d", y, w)
	case "month":
		return now.Format("2006-01")
	case "year":
		return now.Format("2006")
	default:
		return ""
	}
}

// periodKeyLabel returns a human-readable Chinese label for a calendar period
// key (e.g. "2026年9月" for month "2026-09"). Used when building the default
// analysis user prefix.
func periodKeyLabel(period, periodKey string) string {
	switch period {
	case "week":
		var y, w int
		if _, err := fmt.Sscanf(periodKey, "%d-W%d", &y, &w); err == nil {
			return fmt.Sprintf("%d年第%d周", y, w)
		}
	case "month":
		var y, m int
		if _, err := fmt.Sscanf(periodKey, "%d-%d", &y, &m); err == nil {
			return fmt.Sprintf("%d年%d月", y, m)
		}
	case "year":
		var y int
		if _, err := fmt.Sscanf(periodKey, "%d", &y); err == nil {
			return fmt.Sprintf("%d年", y)
		}
	}
	return periodKey
}

// isoWeekStart returns the Monday of the given ISO week.
func isoWeekStart(year, week int) time.Time {
	jan4 := time.Date(year, 1, 4, 0, 0, 0, 0, time.UTC)
	wd := int(jan4.Weekday())
	if wd == 0 {
		wd = 7
	}
	return jan4.AddDate(0, 0, -(wd-1)).AddDate(0, 0, (week-1)*7)
}

// derivePeriodRange returns the canonical YYYY-MM-DD range for a calendar
// period key: week "2026-W36" -> Monday..Sunday, month "2026-09" -> 1st..last
// day, year "2026" -> Jan 1..Dec 31. Custom periods have no canonical range.
func derivePeriodRange(period, periodKey string) (string, string, error) {
	switch period {
	case "week":
		var year, week int
		if _, err := fmt.Sscanf(periodKey, "%d-W%d", &year, &week); err != nil || week < 1 || week > 53 {
			return "", "", fmt.Errorf("invalid week period_key %q, want e.g. 2026-W36", periodKey)
		}
		start := isoWeekStart(year, week)
		end := start.AddDate(0, 0, 6)
		return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
	case "month":
		var year, month int
		if _, err := fmt.Sscanf(periodKey, "%d-%d", &year, &month); err != nil || month < 1 || month > 12 {
			return "", "", fmt.Errorf("invalid month period_key %q, want e.g. 2026-09", periodKey)
		}
		start := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
		end := start.AddDate(0, 1, -1)
		return start.Format("2006-01-02"), end.Format("2006-01-02"), nil
	case "year":
		var year int
		if _, err := fmt.Sscanf(periodKey, "%d", &year); err != nil {
			return "", "", fmt.Errorf("invalid year period_key %q, want e.g. 2026", periodKey)
		}
		return fmt.Sprintf("%d-01-01", year), fmt.Sprintf("%d-12-31", year), nil
	default:
		return "", "", errors.New("custom period requires explicit date_start/date_end")
	}
}

// resolvePeriodRange validates period arguments and fills in the canonical
// date range for calendar periods from period_key when start/end are omitted.
// Custom periods must carry an explicit start/end pair.
func resolvePeriodRange(period, periodKey, start, end string) (string, string, error) {
	if !validPeriod(period) {
		return "", "", errors.New("period must be one of: week, month, year, custom")
	}
	if period == "custom" {
		if start == "" || end == "" {
			return "", "", errors.New("custom period requires date_start and date_end")
		}
		if _, err := time.Parse("2006-01-02", start); err != nil {
			return "", "", errors.New("date_start must be YYYY-MM-DD")
		}
		if _, err := time.Parse("2006-01-02", end); err != nil {
			return "", "", errors.New("date_end must be YYYY-MM-DD")
		}
		return start, end, nil
	}
	if periodKey == "" {
		return "", "", fmt.Errorf("period_key is required for %s (e.g. %s)", period, periodKeyExample(period))
	}
	ds, de, err := derivePeriodRange(period, periodKey)
	if err != nil {
		return "", "", err
	}
	if start == "" {
		start = ds
	}
	if end == "" {
		end = de
	}
	return start, end, nil
}

func (s *Server) registerPeriodTools() {
	// get_period_analysis
	getAnalysis := mcp.NewTool("get_period_analysis",
		mcp.WithDescription("读取已保存的周/月/年分析报告。week/month/year 用 period_key 定位（如 2026-W36 / 2026-09 / 2026）；custom 用 date_start+date_end（可选 keywords）定位。"),
		mcp.WithString("period", mcp.Required(), mcp.Description("week | month | year | custom")),
		mcp.WithString("period_key", mcp.Description("周期键，week/month/year 必填（如 2026-W36、2026-09、2026）")),
		mcp.WithString("date_start", mcp.Description("开始日期 YYYY-MM-DD（custom 必填；week/month/year 可省略，自动由 period_key 推导）")),
		mcp.WithString("date_end", mcp.Description("结束日期 YYYY-MM-DD（同上）")),
		mcp.WithString("keywords", mcp.Description("自定义分析的关键词（custom 可选）")),
	)
	s.addLoggedTool(getAnalysis, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		period := strings.TrimSpace(req.GetString("period", ""))
		periodKey := strings.TrimSpace(req.GetString("period_key", ""))
		start := strings.TrimSpace(req.GetString("date_start", ""))
		end := strings.TrimSpace(req.GetString("date_end", ""))
		keywords := strings.TrimSpace(req.GetString("keywords", ""))
		start, end, err := resolvePeriodRange(period, periodKey, start, end)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		a, err := s.store.GetPeriodAnalysis(userID, period, periodKey, start, end, keywords)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return mcp.NewToolResultError("Analysis not found"), nil
			}
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get analysis: %v", err)), nil
		}
		out, _ := json.Marshal(a)
		return mcp.NewToolResultText(string(out)), nil
	})

	// list_period_analyses
	listAnalyses := mcp.NewTool("list_period_analyses",
		mcp.WithDescription("列出已保存的周期分析报告，可按 period 过滤（week | month | year | custom | all，默认 all）。"),
		mcp.WithString("period", mcp.Description("week | month | year | custom | all（默认 all）")),
		mcp.WithNumber("limit", mcp.Description("返回条数（默认 200）")),
	)
	s.addLoggedTool(listAnalyses, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		period := strings.TrimSpace(req.GetString("period", ""))
		if period != "" && period != "all" && !validPeriod(period) {
			return mcp.NewToolResultError("period must be one of: week, month, year, custom, all"), nil
		}
		limit := req.GetInt("limit", 200)
		analyses, err := s.store.ListSavedAnalyses(userID, period, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list analyses: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]any{"analyses": analyses, "count": len(analyses)})
		return mcp.NewToolResultText(string(out)), nil
	})

	// save_period_analysis
	saveAnalysis := mcp.NewTool("save_period_analysis",
		mcp.WithDescription("保存或更新一份周期分析报告（周/月/年/自定义区间）。同一定位（period+period_key 或 custom 日期区间）重复保存会覆盖更新。"),
		mcp.WithString("period", mcp.Required(), mcp.Description("week | month | year | custom")),
		mcp.WithString("period_key", mcp.Description("周期键，week/month/year 必填（如 2026-W36、2026-09、2026）")),
		mcp.WithString("date_start", mcp.Description("周期开始日期 YYYY-MM-DD（week/month/year 可省略，自动由 period_key 推导；custom 必填）")),
		mcp.WithString("date_end", mcp.Description("周期结束日期 YYYY-MM-DD（同上）")),
		mcp.WithNumber("diary_count", mcp.Description("该周期日记篇数（默认 0）")),
		mcp.WithString("summary", mcp.Required(), mcp.Description("分析摘要正文")),
		mcp.WithString("system_prompt", mcp.Description("生成该分析所用的 system prompt（可选，便于复现）")),
		mcp.WithString("user_prefix", mcp.Description("生成该分析所用的用户前缀指令（可选）")),
		mcp.WithString("keywords", mcp.Description("自定义分析关键词（可选）")),
	)
	s.addLoggedTool(saveAnalysis, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		period := strings.TrimSpace(req.GetString("period", ""))
		periodKey := strings.TrimSpace(req.GetString("period_key", ""))
		start := strings.TrimSpace(req.GetString("date_start", ""))
		end := strings.TrimSpace(req.GetString("date_end", ""))
		keywords := strings.TrimSpace(req.GetString("keywords", ""))
		summary := req.GetString("summary", "")
		start, end, err := resolvePeriodRange(period, periodKey, start, end)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if strings.TrimSpace(summary) == "" {
			return mcp.NewToolResultError("'summary' is required"), nil
		}
		a, err := s.store.SavePeriodAnalysis(userID, period, periodKey, start, end, req.GetInt("diary_count", 0), summary, req.GetString("system_prompt", ""), req.GetString("user_prefix", ""), keywords)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to save analysis: %v", err)), nil
		}
		out, _ := json.Marshal(a)
		return mcp.NewToolResultText(string(out)), nil
	})

	// generate_period_analysis: fetch diaries in range, call AI to generate a
	// summary, and persist it. Mirrors the REST POST /ai/analysis behaviour.
	generateAnalysis := mcp.NewTool("generate_period_analysis",
		mcp.WithDescription("生成周期分析报告：拉取指定时间段日记，调用 AI 生成总结并保存。week/month/year 用 period_key 定位；custom 用 date_start+date_end。返回生成的摘要。"),
		mcp.WithString("period", mcp.Required(), mcp.Description("week | month | year | custom")),
		mcp.WithString("period_key", mcp.Description("周期键，week/month/year 必填（如 2026-W36、2026-09、2026）")),
		mcp.WithString("date_start", mcp.Description("开始日期 YYYY-MM-DD（custom 必填；week/month/year 可省略，自动由 period_key 推导）")),
		mcp.WithString("date_end", mcp.Description("结束日期 YYYY-MM-DD（同上）")),
		mcp.WithString("keywords", mcp.Description("仅分析包含这些关键词的日记（可选，逗号分隔）")),
		mcp.WithString("system_prompt", mcp.Description("自定义 system prompt（可选，覆盖默认与已保存配置）")),
		mcp.WithString("user_prefix", mcp.Description("自定义用户前缀指令（可选）")),
		mcp.WithBoolean("save", mcp.Description("是否保存分析结果（默认 true）")),
	)
	s.addLoggedTool(generateAnalysis, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		period := strings.ToLower(strings.TrimSpace(req.GetString("period", "")))
		periodKey := strings.TrimSpace(req.GetString("period_key", ""))
		start := strings.TrimSpace(req.GetString("date_start", ""))
		end := strings.TrimSpace(req.GetString("date_end", ""))
		keywords := strings.TrimSpace(req.GetString("keywords", ""))
		save := req.GetBool("save", true)

		start, end, err := resolvePeriodRange(period, periodKey, start, end)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}

		// Fetch diaries in range
		diaries, err := s.store.ListDiaries(userID, start+" 00:00:00.000Z", end+" 23:59:59.999Z", "-date", 0)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to fetch diaries for analysis: %v", err)), nil
		}

		// Optional keyword filtering
		if keywords != "" {
			tokens := make([]string, 0)
			for _, t := range strings.FieldsFunc(keywords, func(r rune) bool {
				return r == ',' || r == '，' || r == ';' || r == '；' || r == '/' || r == '\\'
			}) {
				t = strings.TrimSpace(t)
				if t != "" {
					tokens = append(tokens, strings.ToLower(t))
				}
			}
			if len(tokens) > 0 {
				filtered := make([]*store.Diary, 0, len(diaries))
				for _, d := range diaries {
					haystack := strings.ToLower(d.Content)
					for _, tok := range tokens {
						if strings.Contains(haystack, tok) {
							filtered = append(filtered, d)
							break
						}
					}
				}
				diaries = filtered
			}
		}

		periodLabel := "所选时间段"
		if period == "week" || period == "month" || period == "year" {
			periodLabel = periodKeyLabel(period, periodKey)
		}

		// Empty: return a message without calling AI
		if len(diaries) == 0 {
			var emptySummary string
			if keywords != "" {
				emptySummary = fmt.Sprintf("在 %s 至 %s 的时间段内，没有找到包含关键词「%s」的日记记录，无法进行分析。建议调整时间范围、更换关键词，或先记录一些相关日常内容。", start, end, keywords)
			} else {
				emptySummary = fmt.Sprintf("在 %s 至 %s 的时间段内没有日记记录，无法进行分析。建议先记录一些日常内容，然后再尝试。", start, end)
			}
			out, _ := json.Marshal(map[string]any{
				"period": period, "period_key": periodKey, "date_start": start, "date_end": end,
				"keywords": keywords, "count": 0, "summary": emptySummary,
			})
			return mcp.NewToolResultText(string(out)), nil
		}

		// AI config
		cfg, ok := s.aiChatConfig(userID)
		if !ok || !cfg.Enabled || cfg.APIKey == "" || cfg.BaseURL == "" || cfg.Model == "" {
			return mcp.NewToolResultError("AI service is not configured"), nil
		}

		// Resolve prompts: request override → saved config → default
		systemPrompt := strings.TrimSpace(req.GetString("system_prompt", ""))
		if systemPrompt == "" && s.configService != nil {
			systemPrompt, _ = s.configService.GetString(userID, "ai.analysis_system_prompt")
			systemPrompt = strings.TrimSpace(systemPrompt)
		}
		if systemPrompt == "" {
			systemPrompt = "你是一个贴心的日记分析助手，基于用户提供的日记内容进行深入分析。你需要：\n1) 归纳总结日记的主要内容；\n2) 留意每篇日记的日期，分析情绪变化、生活模式在时间线上的演变；\n3) 找出亮点和值得改进的地方；\n4) 给出具体、可操作的建议。\n请用温暖、鼓励且理性的语气，分段输出，便于阅读。使用中文回答。"
		}

		userPrefix := strings.TrimSpace(req.GetString("user_prefix", ""))
		if userPrefix == "" && s.configService != nil {
			userPrefix, _ = s.configService.GetString(userID, "ai.analysis_user_prefix")
			userPrefix = strings.TrimSpace(userPrefix)
		}
		if userPrefix == "" {
			if keywords != "" {
				userPrefix = fmt.Sprintf("以下是%s（%s 至 %s）中包含关键词「%s」的日记记录，共 %d 篇。请根据这些内容进行重组、分析，并给出建议。\n\n", periodLabel, start, end, keywords, len(diaries))
			} else {
				userPrefix = fmt.Sprintf("以下是%s（%s 至 %s）的日记记录，共 %d 篇。请根据内容进行重组、分析，并给出建议。\n\n", periodLabel, start, end, len(diaries))
			}
		}

		// Build user content: diaries in ascending (old → new) order
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

		summary, err := aicore.ChatComplete(ctx, cfg, []aicore.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: sb.String()},
		}, false)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("AI request failed: %v", err)), nil
		}

		response := map[string]any{
			"period":        period,
			"period_key":    periodKey,
			"date_start":    start,
			"date_end":      end,
			"keywords":      keywords,
			"count":         len(diaries),
			"summary":       summary,
			"system_prompt": systemPrompt,
			"user_prefix":   userPrefix,
			"saved":         false,
		}

		if save {
			saved, saveErr := s.store.SavePeriodAnalysis(userID, period, periodKey, start, end, len(diaries), summary, systemPrompt, userPrefix, keywords)
			if saveErr == nil && saved != nil {
				response["saved"] = true
				response["id"] = saved.ID
			}
		}

		out, _ := json.Marshal(response)
		return mcp.NewToolResultText(string(out)), nil
	})
}

// ---------------------------------------------------------------------------
// Embedding / vector index tools: build (full or incremental) and stats
// ---------------------------------------------------------------------------

func (s *Server) registerEmbeddingTools() {
	// build_vectors: build (or incrementally update) the vector index
	buildVectors := mcp.NewTool("build_vectors",
		mcp.WithDescription("构建或增量更新日记的向量索引（用于语义搜索）。mode=incremental（默认）仅处理新增/变更的日记；mode=full 全量重建。返回成功/失败/总数。"),
		mcp.WithString("mode", mcp.Description("incremental（默认，增量）| full（全量重建）")),
	)
	s.addLoggedTool(buildVectors, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		if s.embeddingService == nil {
			return mcp.NewToolResultError("Vector index is not available (embedding service not initialized)"), nil
		}
		mode := strings.ToLower(strings.TrimSpace(req.GetString("mode", "incremental")))
		var result *embedding.BuildResult
		var err error
		if mode == "full" {
			result, err = s.embeddingService.BuildAllVectors(ctx, userID)
		} else if mode == "incremental" || mode == "" {
			result, err = s.embeddingService.BuildIncrementalVectors(ctx, userID)
		} else {
			return mcp.NewToolResultError("'mode' must be 'incremental' or 'full'"), nil
		}
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Build vectors failed: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]any{
			"success": result.Success,
			"failed":  result.Failed,
			"total":   result.Total,
			"errors":  result.Errors,
			"mode":    mode,
		})
		return mcp.NewToolResultText(string(out)), nil
	})

	// vector_stats: statistics about the vector index
	vectorStats := mcp.NewTool("vector_stats",
		mcp.WithDescription("查看向量索引统计：日记总数、已索引数、过期数、待处理数。"),
	)
	s.addLoggedTool(vectorStats, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		if s.embeddingService == nil {
			return mcp.NewToolResultError("Vector index is not available (embedding service not initialized)"), nil
		}
		stats, err := s.embeddingService.GetVectorStats(ctx, userID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get vector stats: %v", err)), nil
		}
		out, _ := json.Marshal(stats)
		return mcp.NewToolResultText(string(out)), nil
	})
}

// ---------------------------------------------------------------------------
// Chat / conversation tools: sessions, messages, and synchronous chat
// ---------------------------------------------------------------------------

// bufferWriter is a minimal chat.StreamWriter that just concatenates received
// chunks into a single string. Used to adapt StreamChat for MCP's sync request/response.
type bufferWriter struct {
	buf strings.Builder
}

func (w *bufferWriter) Write(b []byte) (int, error) {
	return w.buf.Write(b)
}

func (w *bufferWriter) Flush() {}
func (w *bufferWriter) Close() error { return nil }

func (s *Server) registerChatTools() {
	// list_conversations: list user's chat sessions, most recent first
	listConvs := mcp.NewTool("list_conversations",
		mcp.WithDescription("列出 AI 对话会话（最近修改的在前）。返回 id、标题、最后更新时间等。"),
		mcp.WithNumber("limit", mcp.Description("最多返回条数（默认 20）")),
	)
	s.addLoggedTool(listConvs, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		if s.chatService == nil {
			return mcp.NewToolResultError("Chat service not initialized"), nil
		}
		limit := int(req.GetFloat("limit", 20))
		if limit < 1 || limit > 100 {
			limit = 20
		}
		convs, err := s.store.ListConversations(userID, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list conversations: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]any{"conversations": convs, "count": len(convs)})
		return mcp.NewToolResultText(string(out)), nil
	})

	// get_conversation: single conversation detail
	getConv := mcp.NewTool("get_conversation",
		mcp.WithDescription("获取单个对话会话的详细信息（不含消息列表，消息请用 list_conversation_messages）。"),
		mcp.WithString("id", mcp.Required(), mcp.Description("对话 ID")),
	)
	s.addLoggedTool(getConv, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		id := strings.TrimSpace(req.GetString("id", ""))
		c, err := s.store.GetConversation(id, userID)
		if err != nil || c == nil {
			return mcp.NewToolResultError("Conversation not found"), nil
		}
		out, _ := json.Marshal(c)
		return mcp.NewToolResultText(string(out)), nil
	})

	// create_conversation: start a new session
	createConv := mcp.NewTool("create_conversation",
		mcp.WithDescription("创建一个新的 AI 对话会话。"),
		mcp.WithString("title", mcp.Description("可选标题")),
	)
	s.addLoggedTool(createConv, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		title := strings.TrimSpace(req.GetString("title", ""))
		c, err := s.store.CreateConversation(userID, title)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create conversation: %v", err)), nil
		}
		out, _ := json.Marshal(c)
		return mcp.NewToolResultText(string(out)), nil
	})

	// update_conversation: rename
	updateConv := mcp.NewTool("update_conversation",
		mcp.WithDescription("更新对话会话的标题。"),
		mcp.WithString("id", mcp.Required(), mcp.Description("对话 ID")),
		mcp.WithString("title", mcp.Required(), mcp.Description("新标题")),
	)
	s.addLoggedTool(updateConv, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		id := strings.TrimSpace(req.GetString("id", ""))
		title := strings.TrimSpace(req.GetString("title", ""))
		if title == "" {
			return mcp.NewToolResultError("'title' must not be empty"), nil
		}
		c, err := s.store.UpdateConversationTitle(id, userID, title)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update conversation: %v", err)), nil
		}
		out, _ := json.Marshal(c)
		return mcp.NewToolResultText(string(out)), nil
	})

	// delete_conversation: remove session + all its messages
	deleteConv := mcp.NewTool("delete_conversation",
		mcp.WithDescription("删除对话会话（同时删除所有消息）。"),
		mcp.WithString("id", mcp.Required(), mcp.Description("对话 ID")),
	)
	s.addLoggedTool(deleteConv, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		id := strings.TrimSpace(req.GetString("id", ""))
		if err := s.store.DeleteConversation(id, userID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete conversation: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]string{"status": "deleted"})
		return mcp.NewToolResultText(string(out)), nil
	})

	// list_conversation_messages: fetch messages in a conversation
	listMsgs := mcp.NewTool("list_conversation_messages",
		mcp.WithDescription("列出某个对话会话内的历史消息（按时间升序）。"),
		mcp.WithString("conversation_id", mcp.Required(), mcp.Description("对话 ID")),
		mcp.WithNumber("limit", mcp.Description("最多返回条数（默认 50，取最近 N 条）")),
	)
	s.addLoggedTool(listMsgs, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		convID := strings.TrimSpace(req.GetString("conversation_id", ""))
		limit := int(req.GetFloat("limit", 50))
		if limit < 1 || limit > 200 {
			limit = 50
		}
		// ownership check
		c, err := s.store.GetConversation(convID, userID)
		if err != nil || c == nil {
			return mcp.NewToolResultError("Conversation not found"), nil
		}
		msgs, err := s.store.ListMessages(convID, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list messages: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]any{"messages": msgs, "count": len(msgs)})
		return mcp.NewToolResultText(string(out)), nil
	})

	// chat: send a user message, get AI reply (synchronous, no streaming)
	chat := mcp.NewTool("chat",
		mcp.WithDescription("向 AI 发送消息并获得完整回复。内部使用同步适配 StreamChat。对话不存在时会自动创建。返回 AI 回复文本和引用的日记 ID 列表。"),
		mcp.WithString("message", mcp.Required(), mcp.Description("用户消息内容")),
		mcp.WithString("conversation_id", mcp.Description("已有对话 ID；留空则自动创建新对话")),
		mcp.WithString("conversation_title", mcp.Description("新建对话时使用的标题（仅 conversation_id 为空时生效）")),
	)
	s.addLoggedTool(chat, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		if s.chatService == nil {
			return mcp.NewToolResultError("Chat service not initialized"), nil
		}
		message := strings.TrimSpace(req.GetString("message", ""))
		if message == "" {
			return mcp.NewToolResultError("'message' is required"), nil
		}
		convID := strings.TrimSpace(req.GetString("conversation_id", ""))
		if convID == "" {
			title := strings.TrimSpace(req.GetString("conversation_title", ""))
			c, err := s.store.CreateConversation(userID, title)
			if err != nil {
				return mcp.NewToolResultError(fmt.Sprintf("Failed to create conversation: %v", err)), nil
			}
			convID = c.ID
		}

		buf := &bufferWriter{}
		fullText, refs, err := s.chatService.StreamChat(ctx, userID, convID, message, buf)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Chat failed: %v", err)), nil
		}
		// 若 fullText 空则使用 bufferWriter 累积的文本
		if fullText == "" {
			fullText = buf.buf.String()
		}
		out, _ := json.Marshal(map[string]any{
			"conversation_id": convID,
			"reply":           fullText,
			"referenced_diaries": refs,
		})
		return mcp.NewToolResultText(string(out)), nil
	})
}

// ---------------------------------------------------------------------------
// Settings tools: read / write / delete arbitrary user-scoped settings
// ---------------------------------------------------------------------------

func (s *Server) registerSettingsTools() {
	// get_settings: return all user settings (optionally filtered by prefix)
	getAll := mcp.NewTool("get_settings",
		mcp.WithDescription("读取用户设置。可按前缀过滤（如 ai、weather、backup）。返回所有匹配的 key→value 对。"),
		mcp.WithString("prefix", mcp.Description("只返回以此开头的 key（可选，如 ai、weather、backup）")),
	)
	s.addLoggedTool(getAll, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		if s.configService == nil {
			return mcp.NewToolResultError("Config service not initialized"), nil
		}
		prefix := strings.TrimSpace(req.GetString("prefix", ""))
		settingsRaw, err := s.configService.GetBatch(userID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to load settings: %v", err)), nil
		}
		settings := make(map[string]string, len(settingsRaw))
		for k, v := range settingsRaw {
			if prefix != "" && !strings.HasPrefix(k, prefix) {
				continue
			}
			settings[k] = fmt.Sprintf("%v", v)
		}
		out, _ := json.Marshal(map[string]any{"settings": settings, "count": len(settings)})
		return mcp.NewToolResultText(string(out)), nil
	})

	// get_setting: single key
	getOne := mcp.NewTool("get_setting",
		mcp.WithDescription("读取单个用户设置值。key 不存在时返回空字符串。"),
		mcp.WithString("key", mcp.Required(), mcp.Description("设置键名，如 ai.api_key、weather.default_city")),
	)
	s.addLoggedTool(getOne, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		if s.configService == nil {
			return mcp.NewToolResultError("Config service not initialized"), nil
		}
		key := strings.TrimSpace(req.GetString("key", ""))
		if key == "" {
			return mcp.NewToolResultError("'key' is required"), nil
		}
		val, _ := s.configService.GetString(userID, key)
		out, _ := json.Marshal(map[string]any{"key": key, "value": val})
		return mcp.NewToolResultText(string(out)), nil
	})

	// set_setting: set single key
	setOne := mcp.NewTool("set_setting",
		mcp.WithDescription("写入单个用户设置。value 必须是字符串；若写入数字/布尔请用 strconv 转字符串。覆盖已存在值。"),
		mcp.WithString("key", mcp.Required(), mcp.Description("设置键名，如 ai.api_key")),
		mcp.WithString("value", mcp.Required(), mcp.Description("设置值（字符串）")),
	)
	s.addLoggedTool(setOne, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		if s.configService == nil {
			return mcp.NewToolResultError("Config service not initialized"), nil
		}
		key := strings.TrimSpace(req.GetString("key", ""))
		value := strings.TrimSpace(req.GetString("value", ""))
		if key == "" {
			return mcp.NewToolResultError("'key' is required"), nil
		}
		if err := s.configService.Set(userID, key, value); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to set setting: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]any{"status": "ok", "key": key})
		return mcp.NewToolResultText(string(out)), nil
	})

	// delete_setting: remove a key
	deleteOne := mcp.NewTool("delete_setting",
		mcp.WithDescription("删除单个用户设置键。key 不存在时静默成功。"),
		mcp.WithString("key", mcp.Required(), mcp.Description("设置键名")),
	)
	s.addLoggedTool(deleteOne, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		if s.configService == nil {
			return mcp.NewToolResultError("Config service not initialized"), nil
		}
		key := strings.TrimSpace(req.GetString("key", ""))
		if key == "" {
			return mcp.NewToolResultError("'key' is required"), nil
		}
		if err := s.configService.Delete(userID, key); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete setting: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]any{"status": "deleted", "key": key})
		return mcp.NewToolResultText(string(out)), nil
	})
}

// ---------------------------------------------------------------------------
// Backup tools: list / get / delete backups, trigger manual backup
// ---------------------------------------------------------------------------

func (s *Server) registerBackupTools() {
	// list_backups
	listBk := mcp.NewTool("list_backups",
		mcp.WithDescription("列出用户的备份记录（最近的在前）。"),
		mcp.WithNumber("limit", mcp.Description("最多返回条数（默认 20）")),
	)
	s.addLoggedTool(listBk, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		limit := int(req.GetFloat("limit", 20))
		if limit < 1 || limit > 100 {
			limit = 20
		}
		// Store.ListBackups 分页，这里取 page=1 的前 limit 条
		bks, total, err := s.store.ListBackups(userID, 1, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list backups: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]any{
			"backups": bks, "total": total, "count": len(bks),
		})
		return mcp.NewToolResultText(string(out)), nil
	})

	// get_backup
	getBk := mcp.NewTool("get_backup",
		mcp.WithDescription("获取单个备份记录的详细信息。"),
		mcp.WithString("id", mcp.Required(), mcp.Description("备份记录 ID")),
	)
	s.addLoggedTool(getBk, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		id := strings.TrimSpace(req.GetString("id", ""))
		b, err := s.store.GetBackupByID(id)
		if err != nil || b == nil || b.Owner != userID {
			return mcp.NewToolResultError("Backup not found"), nil
		}
		out, _ := json.Marshal(b)
		return mcp.NewToolResultText(string(out)), nil
	})

	// trigger_backup: manually run backup
	triggerBk := mcp.NewTool("trigger_backup",
		mcp.WithDescription("手动触发一次备份。备份完成后会在返回里确认状态。"),
	)
	s.addLoggedTool(triggerBk, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		if s.backupScheduler == nil {
			return mcp.NewToolResultError("Backup scheduler not initialized"), nil
		}
		if err := s.backupScheduler.RunNow(userID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Backup failed: %v", err)), nil
		}
		// Fetch latest entry to confirm
		bks, _, _ := s.store.ListBackups(userID, 1, 1)
		out := map[string]any{"status": "completed"}
		if len(bks) > 0 {
			out["backup"] = bks[0]
		}
		outBytes, _ := json.Marshal(out)
		return mcp.NewToolResultText(string(outBytes)), nil
	})

	// delete_backup: remove record and associated file
	deleteBk := mcp.NewTool("delete_backup",
		mcp.WithDescription("删除单个备份记录（同时删除磁盘文件）。"),
		mcp.WithString("id", mcp.Required(), mcp.Description("备份记录 ID")),
	)
	s.addLoggedTool(deleteBk, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}
		id := strings.TrimSpace(req.GetString("id", ""))
		b, err := s.store.GetBackupByID(id)
		if err != nil || b == nil || b.Owner != userID {
			return mcp.NewToolResultError("Backup not found"), nil
		}
		// Remove disk file
		if b.Filepath != "" {
			_ = os.Remove(b.Filepath)
		}
		if err := s.store.DeleteBackup(id, userID); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete backup: %v", err)), nil
		}
		out, _ := json.Marshal(map[string]string{"status": "deleted"})
		return mcp.NewToolResultText(string(out)), nil
	})
}

// ---------------------------------------------------------------------------
// MCP Prompts: preset message templates the LLM client can use as starting
// points for common workflows. Each prompt returns a list of PromptMessages
// (system + user) with variable substitution applied. The caller then feeds
// these into its own tool-calling loop — MCP prompts are thin templates, they
// never invoke tools directly.
// ---------------------------------------------------------------------------

// registerPrompts registers all Diarum preset prompts.

// ---------------------------------------------------------------------------
// MCP Prompts: preset message templates the LLM client can use as starting
// points for common workflows. Each prompt returns a list of PromptMessages
// (user / assistant, per MCP spec) with variable substitution applied. The
// caller then feeds these into its own tool-calling loop — prompts are thin
// templates, they never invoke tools directly.
// ---------------------------------------------------------------------------

// registerPrompts registers all Diarum preset prompts.
func (s *Server) registerPrompts() {

	// -- write_diary ---------------------------------------------------------
	writeDiary := mcp.NewPrompt("write_diary",
		mcp.WithPromptDescription("帮我写一篇日记：先查天气 + 往年今日，再生成结构化、有情感温度的日记。写完用 create_diary 保存。"),
		mcp.WithArgument("topic", mcp.ArgumentDescription("今天想写的主题或发生的事（留空则由 Agent 自行构思）")),
		mcp.WithArgument("date", mcp.ArgumentDescription("目标日期 YYYY-MM-DD（默认今天）")),
		mcp.WithArgument("city", mcp.ArgumentDescription("查询天气的城市（中文，如 北京）")),
	)
	s.mcpServer.AddPrompt(writeDiary, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		a := req.Params.Arguments
		topic := a["topic"]
		date := a["date"]
		city := a["city"]
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		var buf strings.Builder
		fmt.Fprintf(&buf, "请帮我写一篇 %s 的日记。", date)
		if topic != "" {
			fmt.Fprintf(&buf, "主题/事件：%s。", topic)
		} else {
			buf.WriteString("如果我没有给出具体主题，请先通过工具了解今天的天气、往年今日的日记内容，再结合这些素材自行构思一篇自然、真诚的日记。")
		}
		buf.WriteString("\n\n写作要求：\n- 先调用 get_weather 查天气；调用 on_this_day 看看往年今天写过什么\n- 结构建议：日期 / 天气 / 今日见闻 / 心情 / 思考或待办\n- 语气自然、像和自己对话，不要太书面\n- 写完一定用 create_diary 保存，date=" + date + "，别忘了 mood、tags 等字段")
		if city != "" {
			buf.WriteString("\n\n提示：查询天气时用 city=" + city)
		}
		return &mcp.GetPromptResult{
			Description: "帮写日记（" + date + "）",
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(buf.String())),
			},
		}, nil
	})

	// -- analyze_period ------------------------------------------------------
	analyzePeriod := mcp.NewPrompt("analyze_period",
		mcp.WithPromptDescription("对某段时间（week/month/year/custom）的日记做 AI 周期分析，输出结构化总结并保存。"),
		mcp.WithArgument("period", mcp.ArgumentDescription("week / month / year / custom"), mcp.RequiredArgument()),
		mcp.WithArgument("period_key", mcp.ArgumentDescription("周期键（如 2026-W36 / 2026-09 / 2026）")),
		mcp.WithArgument("keywords", mcp.ArgumentDescription("只分析含这些关键词的日记（逗号分隔）")),
	)
	s.mcpServer.AddPrompt(analyzePeriod, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		a := req.Params.Arguments
		period := a["period"]
		periodKey := a["period_key"]
		keywords := a["keywords"]
		var buf strings.Builder
		fmt.Fprintf(&buf, "请帮我生成 %s 周期的日记分析报告。", period)
		if periodKey != "" {
			fmt.Fprintf(&buf, "period_key=%s。", periodKey)
		}
		if keywords != "" {
			fmt.Fprintf(&buf, "只关心包含关键词：%s。", keywords)
		}
		buf.WriteString("\n\n步骤：\n1) 调用 generate_period_analysis 让 AI 自动生成总结（save=true）\n2) 再用 get_period_analysis 确认已保存\n3) 把分析展示给我，重点：情绪趋势、高频场景、亮点、可操作建议")
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(buf.String())),
			},
		}, nil
	})

	// -- chat_about_diary ----------------------------------------------------
	chatAboutDiary := mcp.NewPrompt("chat_about_diary",
		mcp.WithPromptDescription("用自然语言向我的日记提问。AI 会自动检索相关日记、结合上下文回答。"),
		mcp.WithArgument("question", mcp.ArgumentDescription("你想问日记的问题"), mcp.RequiredArgument()),
		mcp.WithArgument("conversation_id", mcp.ArgumentDescription("已有对话 ID（留空开启新对话）")),
	)
	s.mcpServer.AddPrompt(chatAboutDiary, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		a := req.Params.Arguments
		question := a["question"]
		convID := a["conversation_id"]
		var buf strings.Builder
		fmt.Fprintf(&buf, "我想向我的日记问一个问题：「%s」\n\n", question)
		buf.WriteString("请调用 chat 工具帮我解答（它内部会自动检索相关日记、做语义搜索）。")
		if convID != "" {
			fmt.Fprintf(&buf, "用这个已有对话继续：conversation_id=%s", convID)
		} else {
			buf.WriteString("开启一个新对话即可。")
		}
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(buf.String())),
			},
		}, nil
	})

	// -- mood_review ---------------------------------------------------------
	moodReview := mcp.NewPrompt("mood_review",
		mcp.WithPromptDescription("回顾最近的情绪变化：统计 mood 分布、识别共同场景、给出自我觉察建议。"),
		mcp.WithArgument("days", mcp.ArgumentDescription("回顾多少天（默认 30）")),
	)
	s.mcpServer.AddPrompt(moodReview, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		a := req.Params.Arguments
		days := a["days"]
		if days == "" {
			days = "30"
		}
		buf := fmt.Sprintf("请帮我回顾最近 %s 天的情绪变化趋势。\n\n步骤：\n1) get_stats 看写作频率和连续天数\n2) list_diaries 拉最近 %s 天（包含 mood 字段）\n3) 统计各 mood 等级出现次数，情绪好/差的日子分别有哪些共同场景\n4) 最后给：情绪曲线描述、可能触发因素、一个小小建议", days, days)
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(buf)),
			},
		}, nil
	})

	// -- today_summary -------------------------------------------------------
	todaySummary := mcp.NewPrompt("today_summary",
		mcp.WithPromptDescription("今日回顾：拉取当天日记，生成简短总结 + 明日待办。"),
		mcp.WithArgument("date", mcp.ArgumentDescription("要回顾的日期 YYYY-MM-DD（默认今天）")),
	)
	s.mcpServer.AddPrompt(todaySummary, func(ctx context.Context, req mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		a := req.Params.Arguments
		date := a["date"]
		if date == "" {
			date = time.Now().Format("2006-01-02")
		}
		buf := fmt.Sprintf("请帮我回顾 %s 这一天。\n\n步骤：\n1) has_diary_content 先检查当天是否有日记\n2) list_diaries 拉当天所有条目\n3) get_weather 查当天天气（如果日记里没写）\n4) 给简短总结（3-5 句），再列 3-5 条明日待办（从今天的思考里提炼）", date)
		return &mcp.GetPromptResult{
			Messages: []mcp.PromptMessage{
				mcp.NewPromptMessage(mcp.RoleUser, mcp.NewTextContent(buf)),
			},
		}, nil
	})
}
