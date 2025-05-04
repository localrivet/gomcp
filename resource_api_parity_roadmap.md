# gomcp Resource API Parity Roadmap with FastMCP - Checklist

This checklist outlines the features and enhancements needed in the `gomcp/server` package's resource handling to achieve a level of ease of use and functionality comparable to the FastMCP Python library's resource features.

**Goal:** To simplify the definition and registration of resources and resource templates in `gomcp`, reduce boilerplate, and provide features available in FastMCP, making it more intuitive for Go developers.

## Checklist

- [x] **Phase 1: Core API Implementation**

  - [x] Define `ResourceOption` function type and `resourceConfig` struct.
  - [x] Implement `With...` Resource Option functions (e.g., `WithHandler`, `WithTextContent`, `WithBinaryContent`, `WithFileContent`, `WithDirectoryListing`, `WithURLContent`, `WithName`, `WithDescription`, `WithMimeType`, `WithTags`, `WithAnnotations`).
  - [x] Modify `server.Server.Resource(uri string, options ...ResourceOption)` to process options and delegate registration.

- [x] **Phase 2: Registry and Content Handling**

  - [x] Update `Registry` to accept `resourceConfig` and store resources/templates.
  - [x] Integrate `github.com/localrivet/wilduri` for URI template parsing and matching.
  - [x] Implement internal logic to handle content/source types (direct content, file path, directory listing, URL).
  - [x] Implement basic metadata inference (Name, Description, MimeType) if not provided.

- [x] **Phase 3: Feature Enhancements**

  - [x] Implement support for wildcard parameters (`{param*}`) in URI template matching.
  - [x] Implement support for using default values for handler function parameters.
  - [x] Implement support for registering the same resource under multiple URI patterns.
  - [x] Implement custom resource keys in the registry.
  - [x] Implement configurable duplicate handling for resource registrations.
  - [x] Enhance return value conversion to `protocol.ResourceContents` for various Go types.
  - [x] Implement support for asynchronous resource handlers and progress reporting.
  - [x] Implement automatic content type inference based on file extensions, content, etc.

- [x] **Documentation**
  - [x] Update documentation to reflect the new `server.Resource` API and available options.
  - [x] Provide clear examples for using the new API with different resource types and options.
  - [x] Document advanced features like wildcard parameters, default values, and asynchronous handlers.
