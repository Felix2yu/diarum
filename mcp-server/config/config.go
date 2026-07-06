package config

import (
	"fmt"
	"os"
)

type Config struct {
	BaseURL  string
	APIToken string
	Host     string
	Port     string
}

func Load() (*Config, error) {
	baseURL := os.Getenv("DIARUM_BASE_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8090"
	}

	apiToken := os.Getenv("DIARUM_API_TOKEN")
	if apiToken == "" {
		return nil, fmt.Errorf("DIARUM_API_TOKEN environment variable is required")
	}

	host := os.Getenv("MCP_HOST")
	if host == "" {
		host = "0.0.0.0"
	}

	port := os.Getenv("MCP_PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		BaseURL:  baseURL,
		APIToken: apiToken,
		Host:     host,
		Port:     port,
	}, nil
}
