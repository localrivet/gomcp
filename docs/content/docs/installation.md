---
title: Installation
weight: 20
---

To use the `gomcp` library in your Go project, you can add it as a dependency using `go get`:

```bash
go get github.com/localrivet/gomcp
```

Then, import it in your Go code:

```go
import (
	"github.com/localrivet/gomcp/client"   // For building clients
	"github.com/localrivet/gomcp/server"   // For building servers
	"github.com/localrivet/gomcp/protocol" // For message types
	// ... and specific transport packages as needed
)
```
