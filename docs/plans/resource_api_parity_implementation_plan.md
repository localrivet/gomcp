# Plan to Achieve gomcp Resource API Parity with FastMCP (Revised)

This document outlines the plan to refactor the resource handling in the `gomcp/server` package to align with the simplified API and features described in the `resource_api_parity_roadmap.md`, leveraging the `github.com/localrivet/wilduri` package for URI template handling.

**Goal:** To simplify the definition and registration of resources and resource templates in `gomcp`, reduce boilerplate, and provide features available in FastMCP, making it more intuitive for Go developers, leveraging the `wilduri` package for URI template handling.

## Revised Plan

**Phase 1: Core API Implementation**

1.  **Define `ResourceOption` and `resourceConfig`**:

    - Create a new file (e.g., `server/resource_options.go`) or add to an existing one.
    - Define the `ResourceOption` function type: `type ResourceOption func(*resourceConfig)`.
    - Define the `resourceConfig` struct with fields for `HandlerFn`, `Content`, `FilePath`, `DirPath`, `URL`, `ContentType`, `Name`, `Description`, `MimeType`, `Tags`, and `Annotations` as outlined in the roadmap.

2.  **Implement `With...` Resource Options**:

    - Implement the functions that return `ResourceOption` closures (e.g., `WithHandler`, `WithTextContent`, `WithBinaryContent`, `WithFileContent`, `WithDirectoryListing`, `WithURLContent`, `WithName`, `WithDescription`, `WithMimeType`, `WithTags`, `WithAnnotations`). These functions will modify the `resourceConfig` struct passed to them.

3.  **Modify `server.Server.Resource`**:
    - Update the existing `Server.Resource` method signature to `func (s *Server) Resource(uri string, options ...ResourceOption) *Server`.
    - Inside this method, create a default `resourceConfig`.
    - Iterate through the provided `options` and apply them to the `resourceConfig`.
    - Analyze the `uri` to determine if it's a static resource URI or a resource template URI pattern (e.g., by attempting to parse it with `wilduri.New`).
    - Based on whether it's static or a template, call the appropriate method on the `s.Registry`, passing the `uri` and the configured `resourceConfig`. This will require modifying the `Registry`.

**Phase 2: Registry and Content Handling (Integrating wilduri)**

4.  **Update `Registry`**:

    - Modify the `Registry` struct and its methods (`RegisterResource`, `AddResourceTemplate`) to accept the `resourceConfig`.
    - The `Registry` will need internal logic to process the `resourceConfig` and store the resource appropriately.
    - For resource templates, use `wilduri.New(uri)` to parse the URI pattern and store the resulting `*wilduri.Template` object along with the handler function.
    - For static resources, store the content source information (content, file path, URL, etc.).

5.  **Implement Internal Content/Source Handling**:

    - Develop the logic within the `Registry` or a new helper component that the `Registry` uses to handle the different content sources specified in `resourceConfig`:
      - For `WithTextContent` and `WithBinaryContent`, store the content directly.
      - For `WithFileContent`, implement logic to read the file content when the resource is requested.
      - For `WithDirectoryListing`, implement logic to generate a directory listing when the resource is requested.
      - For `WithURLContent`, implement logic to fetch content from the URL when the resource is requested.
      - For `WithHandler`, store the handler function for execution.
    - When a resource is requested via its URI, the `Registry`'s lookup logic will first check for a direct static resource match. If not found, it will iterate through registered resource templates and use `wilduri.Template.Match(requestedURI)` to find a matching template and extract URI parameters.

6.  **Implement Metadata Inference**:
    - Add logic within the `Registry` or resource creation process to infer `Name`, `Description`, and `MimeType` if they are not explicitly set in the `resourceConfig` via the `With...` metadata options. Inference rules should follow the roadmap (e.g., name from handler function name, MIME type from content or file extension).

**Phase 3: Feature Parity Enhancements (Leveraging wilduri)**

7.  **Enhance Return Value Conversion**:

    - Modify the logic that executes dynamic resource template handlers to handle a wider range of Go return types (structs, maps, slices, primitives, `[]byte`) and automatically convert them into the appropriate `protocol.ResourceContents` (Text, Blob, JSON).

8.  **Implement Resource Template Enhancements**:

    - **Wildcard Parameters**: The `wilduri` package inherently supports wildcard parameters (`{param*}`). The `Registry`'s matching logic will automatically handle this when using `wilduri.Template.Match`.
    - **Default Values**: Support for using default values for handler function parameters that are not present in the matched URI template still needs to be implemented in `gomcp`'s argument preparation logic before calling the handler function.
    - **Multiple URI Patterns**: The design of the `Server.Resource` function already allows registering the same handler function with multiple different URI template patterns by calling `Server.Resource` multiple times with the same handler but different URIs.

9.  **Implement Custom Resource Keys**:

    - Modify the `Registry`'s internal storage mechanism to allow associating a resource with a storage key that is different from its URI, potentially adding a `WithKey(key string)` option.

10. **Implement Duplicate Resource Handling Configuration**:

    - Add an option during `Server` initialization or to the `Registry` to configure the behavior when a duplicate URI is registered (warn, error, replace, ignore). Implement the corresponding logic in the `Registry`.

11. **Document Asynchronous Handlers**:

    - Add documentation or examples demonstrating how resource handlers should perform asynchronous operations using Go's concurrency features (goroutines, channels) without blocking the server's main loop.

12. **Implement Internal Static Resource Handlers**:
    - Ensure the internal logic for serving content from files, URLs, and directory listings (implemented in Phase 2) is robust and handles potential errors (file not found, network issues, permissions).

## Revised Plan Flow

```mermaid
graph TD
    A[User Code] --> B{server.Resource(uri, options...)};
    B --> C[Create resourceConfig];
    C --> D[Apply ResourceOptions];
    D --> E[Configured resourceConfig];
    E --> F[server.Registry];
    F --> G{Parse URI with wilduri.New};
    G -- Static URI --> H[Register Static Resource];
    G -- Template URI --> I[Register Resource Template (Store wilduri.Template)];
    H --> J[Store Content/Source Info];
    I --> K[Store Handler Function];
    SubGraph Resource Request
        L[Incoming Resource URI] --> M{Registry Lookup};
        M -- Static Match --> N[Serve Static Content];
        M -- No Static Match --> O{Iterate Templates};
        O --> P{wilduri.Template.Match(URI)};
        P -- Match Found --> Q[Extract Params with wilduri];
        Q --> R[Prepare Handler Args (Handle Defaults)];
        R --> S[Execute Handler];
        S --> T[Convert Result to protocol.ResourceContents];
        N --> U[Return ResourceContents];
        T --> U;
    End
    F --> V[Metadata Inference Logic];
    F --> W[Duplicate Handling Logic];
    F --> X[Custom Key Logic];
    S --> Y[Enhanced Return Conversion Logic];
```
