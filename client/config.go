package client

// ServerConfig defines the configuration for launching a single MCP server process
// or connecting to an external one.
type ServerConfig struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
	// TODO: Add working directory, other options?
}

// MCPConfig defines the overall configuration for multiple MCP servers.
type MCPConfig struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}
