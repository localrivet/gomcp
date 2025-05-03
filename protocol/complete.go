package protocol

// MethodCompletionComplete defines the JSON-RPC method name for argument completion.
const MethodCompletionComplete = "completion/complete"

// ReferenceType defines the type of reference being completed.
type ReferenceType string

const (
	// RefTypePrompt indicates a reference to a prompt.
	RefTypePrompt ReferenceType = "ref/prompt"
	// RefTypeResource indicates a reference to a resource.
	RefTypeResource ReferenceType = "ref/resource"
)

// CompletionReference is a union type for prompt or resource references.
// Use Type to determine which field (Name or URI) is relevant.
type CompletionReference struct {
	Type ReferenceType `json:"type"`
	Name string        `json:"name,omitempty"` // Used for RefTypePrompt
	URI  string        `json:"uri,omitempty"`  // Used for RefTypeResource
}

// CompletionArgument holds the name and current value of the argument being completed.
type CompletionArgument struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// CompleteRequest defines the parameters for a completion/complete request.
type CompleteRequest struct {
	Ref      CompletionReference `json:"ref"`
	Argument CompletionArgument  `json:"argument"`
}

// Completion holds the results of an argument completion request.
type Completion struct {
	Values  []string `json:"values"`            // Array of completion suggestions (max 100).
	Total   *int     `json:"total,omitempty"`   // Optional total number of available matches.
	HasMore *bool    `json:"hasMore,omitempty"` // Optional flag indicating if more results exist beyond the returned Values.
}

// CompleteResult defines the structure of a successful completion/complete response.
type CompleteResult struct {
	Completion Completion `json:"completion"`
}
