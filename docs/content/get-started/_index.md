---
title: 'Installation'
weight: 20
---

This guide will walk you through how to add the `gomcp` library to your Go project.

### Prerequisites

Before you begin, ensure you have the following installed:

- **Go:** You need a recent version of Go installed on your system. Go version 1.18 or later is recommended as it includes support for generics, which are used in some parts of the `gomcp` library. You can download and install Go from the official Go website: [https://golang.org/dl/](https://golang.org/dl/)
- **A Go Project:** You should have an existing Go project or create a new one. If you're creating a new project, initialize a Go module:
  ```bash
  go mod init your_module_name
  ```

### Installing GoMCP

To add the `gomcp` library to your Go project, open your terminal or command prompt, navigate to your project's root directory, and run the following command:

```bash
go get github.com/localrivet/gomcp
```

This command will download the `gomcp` library and its dependencies and add them to your project's `go.mod` file.

### Importing the Library

Once installed, you can import the necessary packages in your Go code to start building MCP servers or clients:

```go
import (
	"github.com/localrivet/gomcp/client"   // For building MCP clients
	"github.com/localrivet/gomcp/server"   // For building MCP servers
	"github.com/localrivet/gomcp/protocol" // For MCP message types and structures
	// You may also need to import specific transport packages, e.g.:
	// "github.com/localrivet/gomcp/transport/stdio"
	// "github.com/localrivet/gomcp/transport/sse"
	// "github.com/localrivet/gomcp/transport/websocket"
)
```
