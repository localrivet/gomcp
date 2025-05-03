---
title: Resources
weight: 30
---

Resources expose data to clients. They are primarily for providing information without significant computation or side effects (like GET requests). Use `server.AddResource`. Dynamic URIs with `{placeholders}` are supported.

```go
package main

import (
	"fmt"
	"net/url"
	"time"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// ... other imports
)

// Static resource handler
func handleAppVersion(uri *url.URL) (protocol.Content, error) {
	return server.Text("1.2.3"), nil
}

// Dynamic resource handler (extracting part from URI)
func handleUserData(uri *url.URL) (protocol.Content, error) {
	// Example URI: user://data/123/profile
	// We need to parse the user ID from the path
	userID := "" // Extract from uri.Path, e.g., using strings.Split
	// Fetch user data...
	userData := map[string]interface{}{
		"id":        userID,
		"email":     fmt.Sprintf("user%s@example.com", userID),
		"lastLogin": time.Now().Format(time.RFC3339),
	}
	return server.JSON(userData) // Helper for JSON responses
}


func registerResources(srv *server.Server) error {
	// Register a static resource
	err := srv.AddResource(protocol.ResourceDefinition{
		URI:         "app://info/version",
		Description: "Get the application version.",
		Handler:     handleAppVersion,
	})
	if err != nil { return err }

	// Register a dynamic resource template
	err = srv.AddResource(protocol.ResourceDefinition{
		URI:         "user://data/{userID}/profile", // Template URI
		Description: "Get user profile data.",
		IsTemplate:  true, // Mark as template
		Handler:     handleUserData,
	})
	return err
}
```

- **Method:** `"resources/subscribe"`
- **Parameters:** `protocol.SubscribeResourceParams`
- **Result:** `protocol.SubscribeResourceResult` (currently empty)

```go
type SubscribeResourceParams struct {
	URIs []string `json:"uris"` // List of resource URIs to subscribe to
}

type SubscribeResourceResult struct{} // Currently empty
```

Clients provide a list of resource URIs they want to subscribe to. The server tracks these subscriptions per session.

### `resources/unsubscribe` Request

Clients can unsubscribe from resource updates.

- **Method:** `"resources/unsubscribe"`
- **Parameters:** `protocol.UnsubscribeResourceParams`
- **Result:** `protocol.UnsubscribeResourceResult` (currently empty)

```go
type UnsubscribeResourceParams struct {
	URIs []string `json:"uris"` // List of resource URIs to unsubscribe from
}

type UnsubscribeResourceResult struct{} // Currently empty
```

Clients provide a list of resource URIs they no longer wish to receive updates for.

### `notifications/resources/updated` Notification

Servers send the `notifications/resources/updated` notification to inform subscribed clients that a specific resource has been updated.

- **Method:** `"notifications/resources/updated"`
- **Parameters:** `protocol.ResourceUpdatedParams`

```go
type ResourceUpdatedParams struct {
	Resource Resource `json:"resource"` // The updated resource metadata
}
```

The server includes the updated `protocol.Resource` metadata in the notification. Clients can then decide whether to re-read the resource content using `resources/read`.

### `notifications/resources/list_changed` Notification

Servers can send the `notifications/resources/list_changed` notification to inform clients that the list of available resources has changed (resources added or removed).

- **Method:** `"notifications/resources/list_changed"`
- **Parameters:** `protocol.ResourcesListChangedParams` (currently empty)

```go
type ResourcesListChangedParams struct{} // Currently empty
```

This notification does not include the updated list itself, only signals that a change has occurred. Clients must send a `resources/list` request to get the new list.

## Resource Updates and Subscriptions

Clients can subscribe to resource updates using the `resources/subscribe` request. This allows them to receive notifications when a resource's content or metadata changes without constantly polling the server.

The server tracks which sessions have subscribed to which resource URIs. When a resource is updated in your application, you can notify subscribed clients by calling `server.NotifyResourceUpdated(updatedResource)`.

```go
// Assume srv is your initialized *server.Server instance
// Assume updatedResource is the protocol.Resource with the new metadata/version

func updateResourceAndNotify(srv *server.Server, updatedResource protocol.Resource) {
    // First, update the resource's metadata in the server's registry.
    // RegisterResource also updates if a resource with the same URI already exists.
    err := srv.RegisterResource(updatedResource)
    if err != nil {
        log.Printf("Failed to update resource '%s': %v", updatedResource.URI, err)
        return
    }
    log.Printf("Updated resource metadata: %s", updatedResource.URI)

    // Then, notify subscribed clients about the update.
    // The server will automatically send a notifications/resources/updated
    // message to all sessions that subscribed to this resource's URI.
    srv.NotifyResourceUpdated(updatedResource)
    log.Printf("Sent resources/updated notification for: %s", updatedResource.URI)

    // Note: Clients receiving this notification will typically then send a
    // resources/read request to get the new content if they need it.
}
```

Clients can unsubscribe from resource updates using the `resources/unsubscribe` request.

## Next Steps

- Implement the server-side logic to handle `resources/read` requests and provide resource content. This is a key step to make your resources fully functional.
- Explore the `protocol` package for the different `ResourceContents` types you can use to represent various data formats.
- Integrate resource definition, registration, and update notification into your server application's data management logic.
