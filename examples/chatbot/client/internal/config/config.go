package config

import (
	"os"
	"time"

	"github.com/localrivet/gomcp/client"
)

// Config holds application configuration
type Config struct {
	MCPConfigPath string
	OpenAIKey     string
	Port          int
}

// New creates a new Config with default values
func New() *Config {
	return &Config{
		MCPConfigPath: "chatgpt-config.json",
		Port:          3366,
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() {
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		c.OpenAIKey = key
	}

	if path := os.Getenv("MCP_CONFIG_PATH"); path != "" {
		c.MCPConfigPath = path
	}
}

// LoadMCPClient loads the MCP client configuration from file
func (c *Config) LoadMCPClient() (*client.MCP, error) {
	mcpConfig, err := client.LoadFromFile(c.MCPConfigPath, nil)
	if err != nil {
		return nil, err
	}

	return client.New(mcpConfig, client.WithTimeout(60*time.Second))
}
