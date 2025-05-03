---
title: Prompts
weight: 40
---

Prompts define reusable templates or interaction patterns for the client (often an LLM). Use `server.AddPrompt`.

```go
package main

import (
	"fmt"
	"net/url"

	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/gomcp/server"
	// ... other imports
)

// Prompt handler
func handleSummarizePrompt(uri *url.URL, args map[string]interface{}) (protocol.Content, error) {
	text, _ := args["text"].(string) // Basic argument handling
	prompt := fmt.Sprintf("Please summarize the following text concisely:

%s", text)
	return server.Text(prompt), nil
}

func registerPrompts(srv *server.Server) error {
	err := srv.AddPrompt(protocol.PromptDefinition{
		URI:         "prompt://tasks/summarize",
		Description: "Generate a prompt asking the LLM to summarize text.",
		Arguments: &protocol.JSONSchema{ // Define expected arguments
			Type: "object",
			Properties: map[string]*protocol.JSONSchema{
				"text": {Type: "string", Description: "The text to summarize."},
			},
			Required: []string{"text"},
		},
		Handler: handleSummarizePrompt,
	})
	return err
}
```

- **Method:** `"notifications/prompts/list_changed"`
- **Parameters:** `protocol.PromptsListChangedParams` (currently empty)

```go
type PromptsListChangedParams struct{} // Currently empty
```

This notification does not include the updated list itself, only signals that a change has occurred. Clients must send a `prompts/list` request to get the new list.

## Retrieving Prompt Content (`prompts/get`)

Clients can retrieve the full definition of a registered prompt, with arguments potentially filled in, using the `prompts/get` request. The client provides the prompt's URI and a map of argument values. The server is expected to substitute these values into the prompt template's message content.

**Note:** In the current version of the `gomcp` library, the server-side implementation for handling the `prompts/get` request is a stub and does not yet perform argument substitution or return the full prompt definition with substituted values. This functionality is planned for future development.

Currently, you can register prompt metadata, and clients can discover these prompts using `prompts/list`, but they will receive a "Prompt not found" error if they attempt to retrieve the filled-in content using `prompts/get`.

Implementing the `prompts/get` handler will involve:

1.  Receiving the `prompts/get` request with the prompt URI and argument values.
2.  Looking up the prompt template based on the URI.
3.  Performing argument substitution within the message content of the prompt template.
4.  Returning the updated `protocol.Prompt` struct in a `protocol.GetPromptResult`.

## Prompt Updates (`notifications/prompts/list_changed`)

If a prompt's definition changes after it has been registered (e.g., you update the template or add/remove arguments), the server can notify clients by calling `server.SendPromptsListChanged()`. This sends a `notifications/prompts/list_changed` notification to all sessions, indicating that the list of available prompts may have changed and clients should re-list using `prompts/list` if they need the latest definitions.

```go
// Assume srv is your initialized *server.Server instance
// Assume updatedPrompt is the protocol.Prompt with the new definition

func updatePromptAndNotify(srv *server.Server, updatedPrompt protocol.Prompt) {
    // First, update the prompt's definition in the server's registry.
    // RegisterPrompt also updates if a prompt with the same URI already exists.
    err := srv.RegisterPrompt(updatedPrompt)
    if err != nil {
        log.Printf("Failed to update prompt '%s': %v", updatedPrompt.URI, err)
        return
    }
    log.Printf("Updated prompt definition: %s", updatedPrompt.URI)

    // Then, notify clients that the list of prompts has changed.
    // Clients will need to re-list prompts to get the updated definition.
    err = srv.SendPromptsListChanged()
    if err != nil {
        log.Printf("Failed to send prompts/list_changed notification: %v", err)
    } else {
        log.Printf("Sent prompts/list_changed notification.")
    }
}
```

## Next Steps

- Implement the server-side logic to handle `prompts/get` requests and perform argument substitution. This is a key step to make your prompt templates fully functional.
- Consider how to manage and load prompt definitions in your server application, perhaps from files or a database.
