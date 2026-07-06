package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/songtianlun/diarum/mcp-server/client"
)

func RegisterMediaTools(s *server.MCPServer, apiClient *client.Client) {
	// List Media
	listMedia := mcp.NewTool("list_media",
		mcp.WithDescription("List uploaded media files"),
		mcp.WithNumber("limit",
			mcp.Description("Max results (default 50)"),
		),
		mcp.WithNumber("offset",
			mcp.Description("Offset for pagination"),
		),
	)

	s.AddTool(listMedia, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		limit := req.GetInt("limit", 50)
		offset := req.GetInt("offset", 0)

		media, err := apiClient.ListMedia(limit, offset)
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("Failed to list media: %v", err)), nil
		}

		result, _ := json.Marshal(map[string]interface{}{
			"media":  media,
			"count":  len(media),
			"offset": offset,
			"limit":  limit,
		})

		return mcp.NewToolResultText(string(result)), nil
	})
}
