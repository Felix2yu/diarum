package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/songtianlun/diarum/mcp-server/client"
)

func RegisterDiaryTools(s *server.MCPServer, apiClient *client.Client) {
	// Create Diary
	createDiary := mcp.NewTool("create_diary",
		mcp.WithDescription("Create a new diary entry for a specific date. If a diary already exists for that date, it will be updated."),
		mcp.WithString("date",
			mcp.Required(),
			mcp.Description("Date in YYYY-MM-DD format"),
		),
		mcp.WithString("content",
			mcp.Description("Diary content (HTML or plain text)"),
		),
		mcp.WithNumber("mood",
			mcp.Description("Mood rating 1-5 (1=bad, 5=great)"),
		),
		mcp.WithArray("mood_states",
			mcp.Items("string"),
			mcp.Description("List of mood states (e.g., ['happy', 'energetic'])"),
		),
		mcp.WithArray("scenarios",
			mcp.Items("string"),
			mcp.Description("List of scenarios (e.g., ['work', 'social'])"),
		),
		mcp.WithString("weather",
			mcp.Description("Weather description"),
		),
		mcp.WithArray("tags",
			mcp.Items("string"),
			mcp.Description("List of tags"),
		),
	)

	s.AddTool(createDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		date := req.GetString("date", "")
		content := req.GetString("content", "")
		weather := req.GetString("weather", "")
		mood := req.GetInt("mood", 0)
		moodStates := req.GetStringSlice("mood_states", nil)
		scenarios := req.GetStringSlice("scenarios", nil)
		tags := req.GetStringSlice("tags", nil)

		var moodPtr *int
		if mood > 0 {
			moodPtr = &mood
		}

		diary, created, err := apiClient.UpsertDiary(client.UpsertDiaryRequest{
			Date:       date,
			Content:    content,
			Mood:       moodPtr,
			MoodStates: moodStates,
			Scenarios:  scenarios,
			Weather:    weather,
			Tags:       tags,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to create diary: %v", err)), nil
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

	// Update Diary
	updateDiary := mcp.NewTool("update_diary",
		mcp.WithDescription("Update an existing diary entry by ID"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Diary ID"),
		),
		mcp.WithString("content",
			mcp.Description("Diary content (HTML or plain text)"),
		),
		mcp.WithNumber("mood",
			mcp.Description("Mood rating 1-5 (1=bad, 5=great)"),
		),
		mcp.WithArray("mood_states",
			mcp.Items("string"),
			mcp.Description("List of mood states"),
		),
		mcp.WithArray("scenarios",
			mcp.Items("string"),
			mcp.Description("List of scenarios"),
		),
		mcp.WithString("weather",
			mcp.Description("Weather description"),
		),
		mcp.WithArray("tags",
			mcp.Items("string"),
			mcp.Description("List of tags"),
		),
	)

	s.AddTool(updateDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")

		// Get existing diary first
		existing, err := apiClient.GetDiaryByID(id)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get diary: %v", err)), nil
		}

		// Merge with provided values
		content := req.GetString("content", existing.Content)
		weather := req.GetString("weather", existing.Weather)
		mood := req.GetInt("mood", existing.Mood)
		moodStates := req.GetStringSlice("mood_states", existing.MoodStates)
		scenarios := req.GetStringSlice("scenarios", existing.Scenarios)
		tags := req.GetStringSlice("tags", existing.Tags)

		diary, _, err := apiClient.UpsertDiary(client.UpsertDiaryRequest{
			Date:       existing.Date,
			Content:    content,
			Mood:       &mood,
			MoodStates: moodStates,
			Scenarios:  scenarios,
			Weather:    weather,
			Tags:       tags,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to update diary: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"action":  "updated",
			"diary":   diary,
			"message": fmt.Sprintf("Diary updated for %s", existing.Date),
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Get Diary
	getDiary := mcp.NewTool("get_diary",
		mcp.WithDescription("Get a diary entry by date or ID"),
		mcp.WithString("date",
			mcp.Description("Date in YYYY-MM-DD format"),
		),
		mcp.WithString("id",
			mcp.Description("Diary ID"),
		),
	)

	s.AddTool(getDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")
		date := req.GetString("date", "")

		if id == "" && date == "" {
			return mcp.NewToolResultError("Either 'id' or 'date' must be provided"), nil
		}

		var diary *client.Diary
		var err error

		if id != "" {
			diary, err = apiClient.GetDiaryByID(id)
		} else {
			diary, err = apiClient.GetDiaryByDate(date)
		}

		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get diary: %v", err)), nil
		}

		result, _ := json.Marshal(diary)
		return mcp.NewToolResultText(string(result)), nil
	})

	// Delete Diary
	deleteDiary := mcp.NewTool("delete_diary",
		mcp.WithDescription("Delete a diary entry by ID"),
		mcp.WithString("id",
			mcp.Required(),
			mcp.Description("Diary ID"),
		),
	)

	s.AddTool(deleteDiary, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id := req.GetString("id", "")

		err := apiClient.DeleteDiary(id)
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
	listRecentDiaries := mcp.NewTool("list_recent_diaries",
		mcp.WithDescription("Get recent diary entries"),
		mcp.WithNumber("limit",
			mcp.Description("Number of entries to return (default 10)"),
		),
	)

	s.AddTool(listRecentDiaries, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := req.GetInt("limit", 10)

		diaries, err := apiClient.ListRecentDiaries(limit)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list recent diaries: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"diaries": diaries,
			"count":   len(diaries),
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Get On This Day
	getOnThisDay := mcp.NewTool("get_on_this_day",
		mcp.WithDescription("Get diary entries from this day in previous years"),
		mcp.WithString("date",
			mcp.Description("Date in YYYY-MM-DD format (default: today)"),
		),
	)

	s.AddTool(getOnThisDay, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		date := req.GetString("date", "")

		diaries, err := apiClient.GetOnThisDay(date)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get diaries: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"diaries": diaries,
			"count":   len(diaries),
		})

		return mcp.NewToolResultText(string(result)), nil
	})
}
