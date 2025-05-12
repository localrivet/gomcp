package server

import (
	"github.com/localrivet/gomcp/protocol"
	"github.com/localrivet/wilduri"
)

// ResourceOption is a function type that modifies a resourceConfig.
type ResourceOption func(*resourceConfig)

// resourceConfig holds the configuration for a resource or resource template.
type resourceConfig struct {
	HandlerFn   any    // For dynamic resources/templates (used with server.Resource)
	Content     any    // For static resources (e.g., string, []byte) (used with server.Resource or helpers)
	FilePath    string // For file-based static resources (used with server.Resource or AddFileResource)
	DirPath     string // For directory listing resources (used with server.Resource or AddDirectoryListing)
	URL         string // For HTTP(S) resources (used with server.Resource or AddURLContent)
	ContentType string // Explicit content type if needed

	// Metadata (inspired by FastMCP)
	Name        string
	Description string
	MimeType    string // Overrides inferred or default MIME type
	Tags        []string
	Annotations map[string]any // Additional annotations

	// Internal fields for Registry use
	template *wilduri.Template // Compiled wilduri template for resource templates

	// New fields for Phase 3 features
	wildcardParams     map[string]bool
	defaultParamValues map[string]interface{}
	additionalURIs     []string
	customKey          string
	duplicateHandling  DuplicateHandling
	returnConverter    func(interface{}) ([]protocol.ResourceContents, error)
	inferContentType   bool
	async              bool
	progressTemplate   string

	// Note: Resources will use the server's logger automatically.
	// Having a separate logger field is unnecessary.
}

// WithHandler specifies the handler function for a dynamic resource or resource template.
func WithHandler(handlerFn any) ResourceOption {
	return func(c *resourceConfig) {
		c.HandlerFn = handlerFn
	}
}

// WithTextContent specifies static text content for a resource.
func WithTextContent(text string) ResourceOption {
	return func(c *resourceConfig) {
		c.Content = text
	}
}

// WithBinaryContent specifies static binary content for a resource.
func WithBinaryContent(data []byte) ResourceOption {
	return func(c *resourceConfig) {
		c.Content = data
	}
}

// WithFileContent specifies that the resource content should be read from a local file path when requested.
func WithFileContent(filePath string) ResourceOption {
	return func(c *resourceConfig) {
		c.FilePath = filePath
	}
}

// WithDirectoryListing specifies that the resource should provide a listing of files in a local directory.
func WithDirectoryListing(dirPath string) ResourceOption {
	return func(c *resourceConfig) {
		c.DirPath = dirPath
	}
}

// WithURLContent specifies that the resource content should be fetched from an HTTP(S) URL when requested.
func WithURLContent(url string) ResourceOption {
	return func(c *resourceConfig) {
		c.URL = url
	}
}

// WithName sets a custom human-readable name for the resource/template.
func WithName(name string) ResourceOption {
	return func(c *resourceConfig) {
		c.Name = name
	}
}

// WithDescription sets a custom description for the resource/template.
func WithDescription(description string) ResourceOption {
	return func(c *resourceConfig) {
		c.Description = description
	}
}

// WithMimeType explicitly sets the MIME type for the resource/template.
func WithMimeType(mimeType string) ResourceOption {
	return func(c *resourceConfig) {
		c.MimeType = mimeType
	}
}

// WithTags adds categorization tags to the resource/template.
func WithTags(tags ...string) ResourceOption {
	return func(c *resourceConfig) {
		c.Tags = append(c.Tags, tags...)
	}
}

// WithAnnotations adds custom annotations to the resource/template metadata.
func WithAnnotations(annotations map[string]any) ResourceOption {
	return func(c *resourceConfig) {
		if c.Annotations == nil {
			c.Annotations = make(map[string]any)
		}
		for k, v := range annotations {
			c.Annotations[k] = v
		}
	}
}

// WithWildcardParam enables a wildcard parameter in a resource template.
// This allows a parameter to match multiple path segments like `/docs/{path*}/edit`
// which would match `/docs/a/b/c/edit` and extract path="a/b/c".
// Note: The wildcard syntax is already supported in the wilduri library.
func WithWildcardParam(paramName string) ResourceOption {
	return func(cfg *resourceConfig) {
		// We don't need to do anything special here since wilduri
		// already handles the wildcard syntax {param*} in the URI pattern.
		// This option is provided for API completeness and documentation.
		if cfg.wildcardParams == nil {
			cfg.wildcardParams = make(map[string]bool)
		}
		cfg.wildcardParams[paramName] = true
	}
}

// WithDefaultParamValue sets a default value for a template parameter.
// If the parameter is not provided in the URI or is empty, this default value will be used.
func WithDefaultParamValue(paramName string, defaultValue interface{}) ResourceOption {
	return func(cfg *resourceConfig) {
		if cfg.defaultParamValues == nil {
			cfg.defaultParamValues = make(map[string]interface{})
		}
		cfg.defaultParamValues[paramName] = defaultValue
	}
}

// WithMultipleURIs registers the same resource under multiple URI patterns.
// This allows a single resource to be accessible via different URI patterns.
func WithMultipleURIs(additionalURIs ...string) ResourceOption {
	return func(cfg *resourceConfig) {
		if cfg.additionalURIs == nil {
			cfg.additionalURIs = make([]string, 0)
		}
		cfg.additionalURIs = append(cfg.additionalURIs, additionalURIs...)
	}
}

// WithCustomKey sets a custom key to identify this resource in the registry.
// By default, the URI is used as the key.
func WithCustomKey(key string) ResourceOption {
	return func(cfg *resourceConfig) {
		cfg.customKey = key
	}
}

// DuplicateHandling defines how to handle duplicate resource registrations
type DuplicateHandling int

const (
	// DuplicateError returns an error when attempting to register a duplicate resource
	DuplicateError DuplicateHandling = iota
	// DuplicateReplace replaces the existing resource with the new one
	DuplicateReplace
	// DuplicateIgnore ignores the new resource and keeps the existing one
	DuplicateIgnore
	// DuplicateWarn logs a warning but replaces the existing resource
	DuplicateWarn
)

// WithDuplicateHandling sets how to handle duplicate resource registrations
func WithDuplicateHandling(handling DuplicateHandling) ResourceOption {
	return func(cfg *resourceConfig) {
		cfg.duplicateHandling = handling
	}
}

// WithReturnTypeConversion configures how return values from resource handlers
// are converted to ResourceContents. This is useful for custom type conversions.
func WithReturnTypeConversion(converter func(interface{}) ([]protocol.ResourceContents, error)) ResourceOption {
	return func(cfg *resourceConfig) {
		cfg.returnConverter = converter
	}
}

// WithContentTypeInference enables or disables automatic content type inference.
// When enabled (default), the system will try to infer the content type based on
// file extensions, content patterns, etc.
func WithContentTypeInference(enabled bool) ResourceOption {
	return func(cfg *resourceConfig) {
		cfg.inferContentType = enabled
	}
}

// WithAsync marks a resource handler as asynchronous.
// Async handlers will be executed in a separate goroutine and can report progress.
func WithAsync(progressTemplate string) ResourceOption {
	return func(cfg *resourceConfig) {
		cfg.async = true
		cfg.progressTemplate = progressTemplate
	}
}

// Note: Resources automatically use the server's logger.
// No need for a separate WithLogger resource option.

// newResourceConfig creates a default resourceConfig.
func newResourceConfig() resourceConfig {
	return resourceConfig{
		Tags:        []string{},
		Annotations: make(map[string]any),
	}
}
