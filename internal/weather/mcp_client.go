package weather

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// MCPClient calls an external weather MCP server
type MCPClient struct {
	BaseURL    string
	HTTPClient *http.Client
}

// NewMCPClient creates a new MCP client
func NewMCPClient(baseURL string) *MCPClient {
	return &MCPClient{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// MCPRequest represents a JSON-RPC request
type MCPRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// MCPResponse represents a JSON-RPC response
type MCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *MCPError       `json:"error,omitempty"`
}

// MCPError represents an MCP error
type MCPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCPToolResult represents the result of an MCP tool call
type MCPToolResult struct {
	Content []MCPContent `json:"content"`
}

// MCPContent represents content in an MCP tool result
type MCPContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ForecastResponse represents the weather forecast from MCP
type ForecastResponse struct {
	City    string `json:"city"`
	Date    string `json:"date"`
	Weather string `json:"weather"`
	WMOCode int    `json:"wmo_code"`
	TempMin float64 `json:"temp_min"`
	TempMax float64 `json:"temp_max"`
}

// GetForecast calls the weather_get_forecast tool on the MCP server
func (c *MCPClient) GetForecast(city string) (*WeatherResult, error) {
	// First, initialize session
	sessionID, err := c.initialize()
	if err != nil {
		return nil, fmt.Errorf("MCP initialize failed: %w", err)
	}

	// Call the tool
	params := map[string]interface{}{
		"name": "weather_get_forecast",
		"arguments": map[string]string{
			"city": city,
		},
	}

	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      2,
		Method:  "tools/call",
		Params:  params,
	}

	resp, err := c.doRequest(req, sessionID)
	if err != nil {
		return nil, fmt.Errorf("MCP tool call failed: %w", err)
	}

	// Parse the result
	var toolResult MCPToolResult
	if err := json.Unmarshal(resp.Result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse MCP result: %w", err)
	}

	if len(toolResult.Content) == 0 {
		return nil, fmt.Errorf("MCP returned empty result")
	}

	// Try to parse the text content as forecast
	var forecast ForecastResponse
	if err := json.Unmarshal([]byte(toolResult.Content[0].Text), &forecast); err != nil {
		// If direct parse fails, return basic result
		return &WeatherResult{
			City: city,
			Date: time.Now().Format("2006-01-02"),
		}, nil
	}

	return &WeatherResult{
		City:    forecast.City,
		WMOCode: forecast.WMOCode,
		TempMin: forecast.TempMin,
		TempMax: forecast.TempMax,
		Date:    forecast.Date,
	}, nil
}

func (c *MCPClient) initialize() (string, error) {
	req := MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":   map[string]interface{}{},
			"clientInfo": map[string]string{
				"name":    "diarum",
				"version": "1.0.0",
			},
		},
	}

	httpReq, err := json.Marshal(req)
	if err != nil {
		return "", err
	}

	resp, err := c.HTTPClient.Post(
		c.BaseURL+"/mcp",
		"application/json",
		bytes.NewReader(httpReq),
	)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Extract session ID from response header
	sessionID := resp.Header.Get("Mcp-Session-Id")

	return sessionID, nil
}

func (c *MCPClient) doRequest(req MCPRequest, sessionID string) (*MCPResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.BaseURL+"/mcp", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if sessionID != "" {
		httpReq.Header.Set("Mcp-Session-Id", sessionID)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var mcpResp MCPResponse
	if err := json.Unmarshal(respBody, &mcpResp); err != nil {
		return nil, err
	}

	if mcpResp.Error != nil {
		return nil, fmt.Errorf("MCP error %d: %s", mcpResp.Error.Code, mcpResp.Error.Message)
	}

	return &mcpResp, nil
}
