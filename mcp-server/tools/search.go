package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/songtianlun/diarum/mcp-server/client"
)

func RegisterSearchTools(s *server.MCPServer, apiClient *client.Client) {
	// Search Diaries
	searchDiaries := mcp.NewTool("search_diaries",
		mcp.WithDescription("Search diary content by keyword"),
		mcp.WithString("query",
			mcp.Required(),
			mcp.Description("Search query"),
		),
		mcp.WithString("scenario",
			mcp.Description("Filter by scenario"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default 50)"),
		),
	)

	s.AddTool(searchDiaries, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		query := req.GetString("query", "")
		scenario := req.GetString("scenario", "")
		limit := req.GetInt("limit", 50)

		results, err := apiClient.SearchDiaries(query, scenario, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to search diaries: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"results": results,
			"count":   len(results),
			"query":   query,
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Filter Diaries
	filterDiaries := mcp.NewTool("filter_diaries",
		mcp.WithDescription("Filter diary entries by mood or scenario"),
		mcp.WithNumber("mood",
			mcp.Description("Filter by mood (1-5)"),
		),
		mcp.WithString("scenario",
			mcp.Description("Filter by scenario"),
		),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default 100)"),
		),
	)

	s.AddTool(filterDiaries, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		mood := req.GetInt("mood", 0)
		scenario := req.GetString("scenario", "")
		limit := req.GetInt("limit", 100)

		var moodPtr *int
		if mood > 0 {
			moodPtr = &mood
		}

		diaries, err := apiClient.FilterDiaries(moodPtr, scenario, limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to filter diaries: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"diaries": diaries,
			"count":   len(diaries),
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Get Tags
	getTags := mcp.NewTool("get_tags",
		mcp.WithDescription("Get all tags with usage counts"),
	)

	s.AddTool(getTags, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tags, err := apiClient.GetTags()
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get tags: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"tags":  tags,
			"count": len(tags),
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Get Diaries by Tag
	getDiariesByTag := mcp.NewTool("get_diaries_by_tag",
		mcp.WithDescription("Get all diary entries with a specific tag"),
		mcp.WithString("tag",
			mcp.Required(),
			mcp.Description("Tag name"),
		),
	)

	s.AddTool(getDiariesByTag, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		tag := req.GetString("tag", "")

		diaries, err := apiClient.GetDiariesByTag(tag)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get diaries by tag: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"diaries": diaries,
			"count":   len(diaries),
			"tag":     tag,
		})

		return mcp.NewToolResultText(string(result)), nil
	})
}
