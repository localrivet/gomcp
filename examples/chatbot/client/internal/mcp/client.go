package mcp

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/protocol"
)

// Client wraps the MCP client with additional functionality
type Client struct {
	mcp                *client.MCP
	mutex              sync.Mutex
	availableTools     map[string][]protocol.Tool
	toolsMutex         sync.Mutex
	availableResources map[string][]protocol.Resource
	resourcesMutex     sync.Mutex
}

// New creates a new MCP client wrapper
func New(mcp *client.MCP) *Client {
	return &Client{
		mcp:                mcp,
		availableTools:     make(map[string][]protocol.Tool),
		availableResources: make(map[string][]protocol.Resource),
	}
}

// Connect establishes connections to MCP servers and starts discovery
func (c *Client) Connect() error {
	// Set up connection status handlers for each server
	for serverName, serverClient := range c.mcp.Servers {
		// Create a closure to capture the server name
		currentServer := serverName
		serverClient.OnConnectionStatus(func(connected bool) error {
			return c.handleConnectionStatus(currentServer, connected)
		})
	}

	// Start connection error monitor
	go c.monitorConnectionErrors()

	// Connect to MCP servers
	if err := c.mcp.Connect(); err != nil {
		return err
	}

	// Start periodic tool discovery
	go c.periodicDiscovery()

	return nil
}

// handleConnectionStatus handles server connection status changes
func (c *Client) handleConnectionStatus(serverName string, connected bool) error {
	if connected {
		log.Printf("Server %s connected! Discovering tools immediately...", serverName)

		// List tools for this server
		tools, err := c.mcp.Servers[serverName].ListTools(context.Background())
		if err != nil {
			log.Printf("Error listing tools for server %s: %v", serverName, err)
		} else if len(tools) > 0 {
			log.Printf("Server %s has %d tools", serverName, len(tools))

			c.toolsMutex.Lock()
			c.availableTools[serverName] = tools
			c.toolsMutex.Unlock()
		}

		// List resources for this server
		resources, err := c.mcp.Servers[serverName].ListResources(context.Background())
		if err != nil {
			log.Printf("Error listing resources for server %s: %v", serverName, err)
		} else if len(resources) > 0 {
			log.Printf("Server %s has %d resources", serverName, len(resources))

			c.resourcesMutex.Lock()
			c.availableResources[serverName] = resources
			c.resourcesMutex.Unlock()
		}
	} else {
		log.Printf("Server %s disconnected", serverName)
	}

	return nil
}

// monitorConnectionErrors monitors connection errors from the MCP client
func (c *Client) monitorConnectionErrors() {
	log.Printf("Starting connection error monitor")
	for connErr := range c.mcp.ConnectionErrors() {
		log.Printf("Connection error from server %s: %v", connErr.ServerName, connErr.Err)
	}
	log.Printf("Connection error monitor exited")
}

// periodicDiscovery periodically discovers and registers tools
func (c *Client) periodicDiscovery() {
	log.Printf("Starting regular tool discovery loop")
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		if !c.mcp.IsConnected() {
			log.Printf("MCP not connected, waiting before trying to list tools...")
			c.mcp.CheckConnectionStatus()
			continue
		}

		toolMap := c.mcp.ListTools()
		if len(toolMap) > 0 {
			c.toolsMutex.Lock()
			for serverName, serverTools := range toolMap {
				c.availableTools[serverName] = serverTools
			}
			c.toolsMutex.Unlock()
		}
	}
}

// GetTools returns a copy of all available tools
func (c *Client) GetTools() map[string][]protocol.Tool {
	c.toolsMutex.Lock()
	defer c.toolsMutex.Unlock()

	result := make(map[string][]protocol.Tool)
	for server, tools := range c.availableTools {
		result[server] = tools
	}

	return result
}

// CallTool calls a tool on an MCP server
func (c *Client) CallTool(serverName, toolName string, args map[string]interface{}) ([]protocol.Content, error) {
	return c.mcp.CallTool(serverName, toolName, args)
}
