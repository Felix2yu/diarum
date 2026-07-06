package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/songtianlun/diarum/mcp-server/client"
)

func RegisterAITools(s *server.MCPServer, apiClient *client.Client) {
	// Analyze Period
	analyzePeriod := mcp.NewTool("analyze_period",
		mcp.WithDescription("Generate AI analysis for a time period (weekly, monthly, or custom)"),
		mcp.WithString("period",
			mcp.Required(),
			mcp.Description("Analysis period: 'week', 'month', or 'custom'"),
		),
		mcp.WithString("start_date",
			mcp.Required(),
			mcp.Description("Start date in YYYY-MM-DD format"),
		),
		mcp.WithString("end_date",
			mcp.Required(),
			mcp.Description("End date in YYYY-MM-DD format"),
		),
		mcp.WithArray("keywords",
			mcp.Items("string"),
			mcp.Description("Filter diaries by keywords"),
		),
		mcp.WithString("system_prompt",
			mcp.Description("Custom system prompt for analysis"),
		),
	)

	s.AddTool(analyzePeriod, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		period := req.GetString("period", "")
		startDate := req.GetString("start_date", "")
		endDate := req.GetString("end_date", "")
		systemPrompt := req.GetString("system_prompt", "")
		keywords := req.GetStringSlice("keywords", nil)

		analysis, err := apiClient.AnalyzePeriod(client.AnalyzePeriodRequest{
			Period:       period,
			StartDate:    startDate,
			EndDate:      endDate,
			Keywords:     keywords,
			SystemPrompt: systemPrompt,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to analyze period: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"analysis": analysis,
			"message":  fmt.Sprintf("Analysis generated for %s from %s to %s", period, startDate, endDate),
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// List Analyses
	listAnalyses := mcp.NewTool("list_analyses",
		mcp.WithDescription("List saved AI analyses"),
		mcp.WithString("period",
			mcp.Description("Filter by period: 'week', 'month', or 'custom'"),
		),
	)

	s.AddTool(listAnalyses, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		period := req.GetString("period", "")

		analyses, err := apiClient.ListAnalyses(period)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list analyses: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"analyses": analyses,
			"count":    len(analyses),
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// AI Chat
	aiChat := mcp.NewTool("ai_chat",
		mcp.WithDescription("Chat with AI about your diary entries. The AI can search and reference your diaries."),
		mcp.WithString("message",
			mcp.Required(),
			mcp.Description("Your message to the AI"),
		),
		mcp.WithString("conversation_id",
			mcp.Description("Continue an existing conversation (optional)"),
		),
	)

	s.AddTool(aiChat, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		message := req.GetString("message", "")
		conversationID := req.GetString("conversation_id", "")

		response, err := apiClient.Chat(client.ChatRequest{
			Message:        message,
			ConversationID: conversationID,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to chat: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"reply":              response.Reply,
			"conversation_id":    response.ConversationID,
			"referenced_diaries": response.ReferencedDiaries,
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Build Embeddings
	buildEmbeddings := mcp.NewTool("build_embeddings",
		mcp.WithDescription("Build or rebuild vector embeddings for semantic search. This may take a while."),
	)

	s.AddTool(buildEmbeddings, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		err := apiClient.BuildEmbeddings()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to build embeddings: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"action":  "build_started",
			"message": "Embedding build started. Check vector_stats for progress.",
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Get Vector Stats
	getVectorStats := mcp.NewTool("get_vector_stats",
		mcp.WithDescription("Get vector embedding index statistics"),
	)

	s.AddTool(getVectorStats, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		stats, err := apiClient.GetVectorStats()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get vector stats: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"diary_count":    stats.DiaryCount,
			"indexed_count":  stats.IndexedCount,
			"outdated_count": stats.OutdatedCount,
			"pending_count":  stats.PendingCount,
		})

		return mcp.NewToolResultText(string(result)), nil
	})
}
