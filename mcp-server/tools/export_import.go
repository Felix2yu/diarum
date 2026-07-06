package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/songtianlun/diarum/mcp-server/client"
)

func RegisterExportImportTools(s *server.MCPServer, apiClient *client.Client) {
	// Export Diaries
	exportDiaries := mcp.NewTool("export_diaries",
		mcp.WithDescription("Export diary entries as ZIP or Markdown"),
		mcp.WithString("format",
			mcp.Description("Export format: 'zip' or 'markdown' (default: 'zip')"),
		),
		mcp.WithString("start",
			mcp.Description("Start date in YYYY-MM-DD format"),
		),
		mcp.WithString("end",
			mcp.Description("End date in YYYY-MM-DD format"),
		),
	)

	s.AddTool(exportDiaries, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		format := req.GetString("format", "zip")
		start := req.GetString("start", "")
		end := req.GetString("end", "")

		data, err := apiClient.ExportDiaries(client.ExportRequest{
			Format: format,
			Start:  start,
			End:    end,
		})
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to export diaries: %v", err)), nil
		}

		// For MCP, we return metadata about the export
		// The actual file would need to be saved server-side or returned as base64
		result, _ := json.Marshal(map[string]interface{}{
			"action":    "export_started",
			"format":    format,
			"start":     start,
			"end":       end,
			"data_size": len(data),
			"message":   fmt.Sprintf("Export completed. Data size: %d bytes", len(data)),
		})

		return mcp.NewToolResultText(string(result)), nil
	})
}
