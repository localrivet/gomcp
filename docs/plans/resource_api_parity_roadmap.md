# gomcp Resource API Parity Roadmap with FastMCP

This document outlines the features and enhancements needed in the `gomcp/server` package's resource handling to achieve a level of ease of use and functionality comparable to the FastMCP Python library's resource features.

## Goal

To simplify the definition and registration of resources and resource templates in `gomcp`, reduce boilerplate, and provide features available in FastMCP, making it more intuitive for Go developers.

## Features and Enhancements Needed

Based on a comparison with FastMCP's resource documentation, the following areas require implementation or enhancement in `gomcp/server`:

- [ ] **Unified Resource API:** Implement a single function (e.g., `server.Resource(uri string, options ...ResourceOption)`) to handle the registration of both static resources and resource templates, using options to specify content/handler and metadata.
- [ ] **Automatic Metadata Inference:**
  - [ ] Implement inference of the resource/template `Name` from the handler function's name when a handler is provided.
  - [ ] Explore options for inferring the `Description` (e.g., from comments) or strongly encourage providing it via an option.
- [ ] **Comprehensive Return Value Conversion:** Enhance the internal logic to automatically convert a wider range of Go types returned by handlers (e.g., structs, maps, slices, primitive types, `[]byte`) into the appropriate `protocol.ResourceContents` (Text, Blob, JSON serialization). Ensure proper handling of `nil` return values for empty content.
- [ ] **Helper Constructors for Common Resources:** Implement convenience functions for registering common static resource types without requiring a full handler function:
  - [ ] `AddFileResource(uri string, filePath string, options ...ResourceOption)`: Registers a resource that reads content from a local file.
  - [ ] `AddTextResource(uri string, text string, options ...ResourceOption)`: Registers a resource with static text content.
  - [ ] `AddDirectoryResource(uri string, dirPath string, options ...ResourceOption)`: Registers a resource that lists files in a local directory (returning JSON).
  - [ ] `AddHttpResource(uri string, url string, options ...ResourceOption)`: Registers a resource that fetches content from an HTTP(S) URL.
- [ ] **Custom Resource Keys:** Modify the `Registry` and the resource registration API to allow registering resources with a storage key that is different from the resource URI.
- [ ] **Resource Template Enhancements:**
  - [ ] Support wildcard parameters (`{param*}`) in URI patterns for resource templates. This requires enhancing the URI matching and parameter extraction logic.
  - [ ] Support using default values for handler function parameters that are not present in the matched URI template. This requires modifying the argument preparation logic.
  - [ ] Consider an explicit way to register the same handler function with multiple different URI template patterns.
- [ ] **Duplicate Resource Handling Configuration:** Add an option during server initialization or to the Registry to configure the behavior when attempting to register a resource or template with a URI that is already registered (e.g., warn, error, replace, ignore).
- [ ] **Asynchronous Handler Clarity:** While Go's concurrency model differs from Python's `async def`, provide clear documentation or patterns for how resource handlers should perform asynchronous operations without blocking the server.

## Conclusion

Implementing the features outlined above would significantly enhance the developer experience for defining resources and templates in `gomcp`, bringing it closer to parity with the FastMCP library and making it more powerful and flexible for building MCP servers in Go. This document serves as a roadmap for these future development efforts.
