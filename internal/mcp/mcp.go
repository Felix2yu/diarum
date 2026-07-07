package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/songtianlun/diarum/internal/config"
	"github.com/songtianlun/diarum/internal/store"
	"github.com/songtianlun/diarum/internal/weather"
)

// Server wraps the MCP server with Diarum integration
type Server struct {
	mcpServer *server.MCPServer
	store     *store.Store
}

// New creates a new MCP server integrated with Diarum
func New(appStore *store.Store) *Server {
	s := &Server{
		store: appStore,
	}

	// Create MCP server
	mcpServer := server.NewMCPServer(
		"diarum",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	s.mcpServer = mcpServer

	// Register all tools
	s.registerDiaryTools()
	s.registerSearchTools()
	s.registerStatsTools()
	s.registerWeatherTools()

	return s
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
	// Create/Update Diary
	createDiary := mcp.NewTool("create_diary",
		mcp.WithDescription("Create or update a diary entry for a specific date"),
		mcp.WithString("date", mcp.Required(), mcp.Description("Date in YYYY-MM-DD format")),
		mcp.WithString("content", mcp.Description("Diary content (HTML or plain text)")),
		mcp.WithNumber("mood", mcp.Description("Mood rating 1-5 (1=bad, 5=great)")),
		mcp.WithArray("mood_states", mcp.Items("string"), mcp.Description("List of mood states")),
		mcp.WithArray("scenarios", mcp.Items("string"), mcp.Description("List of scenarios")),
		mcp.WithString("weather", mcp.Description("Weather description")),
		mcp.WithArray("tags", mcp.Items("string"), mcp.Description("List of tags")),
	)

	s.mcpServer.AddTool(createDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		date := req.GetString("date", "")
		content := req.GetString("content", "")
		weather := req.GetString("weather", "")
		mood := req.GetInt("mood", 0)
		moodStates := req.GetStringSlice("mood_states", nil)
		scenarios := req.GetStringSlice("scenarios", nil)
		tags := req.GetStringSlice("tags", nil)

		diary, created, err := s.store.UpsertDiary(userID, date, content, mood, moodStates, scenarios, weather, tags)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to save diary: %v", err)), nil
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
	})

	// Get Diary
	getDiary := mcp.NewTool("get_diary",
		mcp.WithDescription("Get a diary entry by date or ID"),
		mcp.WithString("date", mcp.Description("Date in YYYY-MM-DD format")),
		mcp.WithString("id", mcp.Description("Diary ID")),
	)

	s.mcpServer.AddTool(getDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
			diaries, err := s.store.ListDiaries(userID, date+" 00:00:00.000Z", date+" 23:59:59.999Z", "date", 1)
			if err == nil && len(diaries) > 0 {
				diary = diaries[0]
			}
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

	s.mcpServer.AddTool(deleteDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		id := req.GetString("id", "")

		err := s.store.DeleteDiary(id, userID)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to delete diary: %v", err)), nil
		}

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

	s.mcpServer.AddTool(listRecent, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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
}

// registerSearchTools registers search and filter tools
func (s *Server) registerSearchTools() {
	// Search Diaries
	searchDiaries := mcp.NewTool("search_diaries",
		mcp.WithDescription("Search diary content by keyword"),
		mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
		mcp.WithString("scenario", mcp.Description("Filter by scenario")),
		mcp.WithNumber("limit", mcp.Description("Max results (default 50)")),
	)

	s.mcpServer.AddTool(searchDiaries, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		query := req.GetString("query", "")
		scenario := req.GetString("scenario", "")
		limit := req.GetInt("limit", 50)

		diaries, err := s.store.SearchDiaries(userID, query, scenario, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to search diaries: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"diaries": diaries,
			"count":   len(diaries),
			"query":   query,
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Get Tags
	getTags := mcp.NewTool("get_tags",
		mcp.WithDescription("Get all tags with usage counts"),
	)

	s.mcpServer.AddTool(getTags, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
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

	s.mcpServer.AddTool(getStats, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		total := s.store.CountDiaries(userID)

		result, _ := json.Marshal(map[string]interface{}{
			"total":   total,
			"message": fmt.Sprintf("Total diaries: %d", total),
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

	s.mcpServer.AddTool(getWeather, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		userID := getUserID(ctx)
		if userID == "" {
			return mcp.NewToolResultError("Authentication required"), nil
		}

		city := req.GetString("city", "")
		if city == "" {
			return mcp.NewToolResultError("City name is required"), nil
		}

		configService := config.NewConfigService(s.store)
		mcpURL, _ := configService.GetString(userID, "weather.mcp_url")
		if mcpURL == "" {
			mcpURL = "http://localhost:8080"
		}
		useMCP, _ := configService.GetBool(userID, "weather.use_mcp")

		svc := weather.NewService(mcpURL, useMCP)
		result, err := svc.GetWeather(city)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get weather: %v", err)), nil
		}

		weatherResult, _ := json.Marshal(result)
		return mcp.NewToolResultText(string(weatherResult)), nil
	})
}
