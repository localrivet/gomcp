# Installation

This guide provides detailed instructions for installing and setting up GOMCP in your Go projects.

## Requirements

- Go 1.20 or later
- Git (for source installation)

## Installing from Go Modules (Recommended)

The simplest way to install GOMCP is using Go modules:

```bash
go get github.com/localrivet/gomcp
```

Then import the packages you need in your code:

```go
import (
    "github.com/localrivet/gomcp/client"
    "github.com/localrivet/gomcp/server"
)
```

## Installing from Source

For the latest development version or to contribute to GOMCP:

```bash
git clone https://github.com/localrivet/gomcp.git
cd gomcp
go install ./...
```

## Verifying Installation

Create a simple program to verify your installation:

```go
package main

import (
    "fmt"

    "github.com/localrivet/gomcp"
)

func main() {
    fmt.Printf("GOMCP Version: %s\n", gomcp.Version)
}
```

## Versioning

GOMCP follows semantic versioning. You can specify a particular version in your go.mod file:

```
require github.com/localrivet/gomcp v1.0.0
```

## Next Steps

- [Getting Started Guide](../getting-started/README.md)
- [Tutorials](../tutorials/README.md)
- [Examples](../examples/README.md)
