// Package client provides the client-side implementation of the MCP protocol.
package client

// Root represents a filesystem root exposed to the MCP server.
type Root struct {
	URI      string                 `json:"uri"`
	Name     string                 `json:"name,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// ClientCapabilities represents the capabilities supported by this client.
type ClientCapabilities struct {
	Roots        RootsCapability        `json:"roots,omitempty"`
	Sampling     map[string]interface{} `json:"sampling,omitempty"`
	Experimental map[string]interface{} `json:"experimental,omitempty"`
}

// RootsCapability represents the client's roots capability.
type RootsCapability struct {
	ListChanged bool `json:"listChanged"`
}
