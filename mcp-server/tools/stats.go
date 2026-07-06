package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/songtianlun/diarum/mcp-server/client"
)

func RegisterStatsTools(s *server.MCPServer, apiClient *client.Client) {
	// Get Stats
	getStats := mcp.NewTool("get_stats",
		mcp.WithDescription("Get diary statistics including total count and writing streak"),
		mcp.WithString("timezone",
			mcp.Description("Timezone for streak calculation (e.g., 'Asia/Shanghai')"),
		),
	)

	s.AddTool(getStats, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		timezone := req.GetString("timezone", "")

		stats, err := apiClient.GetStats(timezone)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get stats: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"total":   stats.Total,
			"streak":  stats.Streak,
			"message": fmt.Sprintf("Total diaries: %d, Current streak: %d days", stats.Total, stats.Streak),
		})

		return mcp.NewToolResultText(string(result)), nil
	})

	// Get Calendar Heatmap
	getCalendarHeatmap := mcp.NewTool("get_calendar_heatmap",
		mcp.WithDescription("Get diary existence data for calendar heatmap visualization"),
		mcp.WithString("start",
			mcp.Required(),
			mcp.Description("Start date in YYYY-MM-DD format"),
		),
		mcp.WithString("end",
			mcp.Required(),
			mcp.Description("End date in YYYY-MM-DD format"),
		),
	)

	s.AddTool(getCalendarHeatmap, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		start := req.GetString("start", "")
		end := req.GetString("end", "")

		entries, err := apiClient.GetCalendarHeatmap(start, end)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to get calendar heatmap: %v", err)), nil
		}

		// Count days with diaries
		daysWithDiaries := 0
		for _, e := range entries {
			if e.Exists {
				daysWithDiaries++
			}
		}

		result, _ := json.Marshal(map[string]interface{}{
			"entries":           entries,
			"total_days":        len(entries),
			"days_with_diaries": daysWithDiaries,
			"start":             start,
			"end":               end,
		})

		return mcp.NewToolResultText(string(result)), nil
	})
}
