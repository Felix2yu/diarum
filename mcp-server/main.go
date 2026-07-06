package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/mark3labs/mcp-go/server"
	"github.com/songtianlun/diarum/mcp-server/client"
	"github.com/songtianlun/diarum/mcp-server/config"
	"github.com/songtianlun/diarum/mcp-server/tools"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		os.Exit(1)
	}

	// Create API client
	apiClient := client.New(cfg.BaseURL, cfg.APIToken)

	// Create MCP server
	s := server.NewMCPServer(
		"diarum",
		"1.0.0",
		server.WithToolCapabilities(true),
	)

	// Register all tools
	tools.RegisterDiaryTools(s, apiClient)
	tools.RegisterSearchTools(s, apiClient)
	tools.RegisterStatsTools(s, apiClient)
	tools.RegisterAITools(s, apiClient)
	tools.RegisterMediaTools(s, apiClient)
	tools.RegisterExportImportTools(s, apiClient)

	// Start server with Streamable HTTP transport
	addr := cfg.Host + ":" + cfg.Port
	fmt.Fprintf(os.Stderr, "Starting Diarum MCP Server...\n")
	fmt.Fprintf(os.Stderr, "API URL: %s\n", cfg.BaseURL)
	fmt.Fprintf(os.Stderr, "MCP Server listening on: http://%s/mcp\n", addr)

	// Create Streamable HTTP server
	streamableServer := server.NewStreamableHTTPServer(s,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)

	// Create HTTP server
	httpServer := &http.Server{
		Addr:    addr,
		Handler: streamableServer,
	}

	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
