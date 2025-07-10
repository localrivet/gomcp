# GoDXT: Go-Based DXT Packaging Tool

## Overview

**GoDXT** is a CLI tool for packaging MCP servers as Desktop Extensions (DXT) files. While focused on Go-based MCP servers, it supports any MCP server implementation by providing universal DXT packaging capabilities.

This addresses [GitHub Issue #12](https://github.com/localrivet/gomcp/issues/12) by providing DXT support as a separate, focused tool rather than adding packaging concerns to the GoMCP library itself.

## Background

Desktop Extensions (.dxt) are zip archives containing a local MCP server and a manifest.json that describes the server and its capabilities. They enable single-click installation of MCP servers for Claude Desktop and other MCP-enabled applications.

**Key Insight**: DXT packaging is about deployment/distribution, not MCP protocol implementation. Therefore, it should be a separate tool that can work with any MCP server, regardless of implementation language.

## Repository Structure

```
godxt/
├── cmd/godxt/              # CLI tool main entry point
├── pkg/
│   ├── manifest/           # Manifest generation and validation
│   ├── packager/           # DXT packaging logic
│   ├── templates/          # Manifest templates
│   ├── builder/            # Cross-platform build helpers
│   └── validator/          # DXT validation and testing
├── templates/              # DXT project templates
│   ├── go/                 # Go MCP server template
│   ├── node/               # Node.js MCP server template
│   ├── python/             # Python MCP server template
│   └── binary/             # Generic binary template
├── examples/               # Complete DXT examples
│   ├── hello-world/        # Basic example
│   ├── filesystem/         # File system server
│   └── database/           # Database connector
├── docs/                   # Documentation
└── README.md
```

## Core Functionality

### 1. Project Initialization
```bash
# Initialize new DXT project
godxt init my-server --type=go
godxt init my-server --type=node
godxt init my-server --type=python
godxt init my-server --type=binary

# Initialize from existing MCP server
godxt init my-server --from-binary ./my-server
godxt init my-server --from-source ./src/
```

**What it creates:**
- `manifest.json` template appropriate for the server type
- Directory structure for DXT packaging
- Example configuration files
- Build scripts (for Go/Node.js/Python projects)
- Documentation template

### 2. Manifest Management
```bash
# Generate manifest template
godxt manifest generate --type=go

# Validate manifest against DXT specification
godxt manifest validate ./manifest.json

# Interactive manifest builder
godxt manifest wizard

# Update manifest from server introspection (if supported)
godxt manifest update --server=./my-server
```

**Manifest Features:**
- Template generation for different server types
- Validation against official DXT schema
- Interactive wizard for complex configurations
- Support for user configuration, environment variables, platform-specific settings

### 3. Building & Packaging
```bash
# Build Go server for multiple platforms
godxt build --platforms=darwin,linux,windows

# Package as DXT
godxt pack ./my-server-project

# Pack with custom manifest
godxt pack ./my-server-project --manifest=./custom-manifest.json

# Pack with icon and screenshots
godxt pack ./my-server-project --icon=./icon.png --screenshots=./screenshots/
```

**Packaging Features:**
- Cross-platform binary building (for Go projects)
- Dependency bundling
- Asset inclusion (icons, screenshots, documentation)
- Manifest validation before packaging
- Automatic file permissions handling

### 4. Testing & Validation
```bash
# Test DXT locally (requires Claude Desktop or compatible client)
godxt test my-server.dxt

# Validate DXT structure and manifest
godxt validate my-server.dxt

# Lint for common issues
godxt lint ./my-server-project
```

**Testing Features:**
- DXT structure validation
- Manifest schema compliance
- Local testing with compatible MCP clients
- Performance and security linting

### 5. Template System
```bash
# List available templates
godxt templates list

# Create project from template
godxt new --template=filesystem-server my-fs-server

# Update templates
godxt templates update
```

## Example Workflows

### Scenario 1: Go Developer with Existing GoMCP Server
```bash
# Developer has built an MCP server using GoMCP
go build -o my-server cmd/server/main.go

# Initialize DXT project
godxt init my-server-dxt --from-binary ./my-server

# Edit manifest.json to configure deployment settings
# (user config, environment variables, descriptions)

# Package as DXT
godxt pack my-server-dxt
# Creates: my-server.dxt

# Test locally
godxt test my-server.dxt
```

### Scenario 2: Creating New Go MCP Server for DXT
```bash
# Start with DXT-ready template
godxt new --template=go-server my-new-server

# Implement MCP server using GoMCP
cd my-new-server
go mod tidy
# Edit server implementation...

# Build and package
godxt build --platforms=darwin,linux,windows
godxt pack .
# Creates: my-new-server.dxt
```

### Scenario 3: Packaging Non-Go MCP Server
```bash
# Works with any MCP server implementation
godxt init my-node-server --type=node --from-source ./my-node-mcp-server/

# Edit manifest for Node.js specifics
# Package with bundled dependencies
godxt pack my-node-server
```

## Key Design Principles

### 1. Separation of Concerns
- **GoMCP**: Focused purely on MCP protocol implementation
- **GoDXT**: Focused purely on DXT packaging and distribution
- Clean boundaries, no cross-dependencies

### 2. Universal Compatibility
- Works with MCP servers in any language
- Supports all DXT features from the specification
- Compatible with Claude Desktop and other DXT-enabled applications

### 3. Developer Experience
- Simple, intuitive CLI commands
- Rich template system for quick starts
- Comprehensive validation and error messages
- Great documentation and examples

### 4. Production Ready
- Cross-platform builds
- Security best practices
- Performance optimization
- Enterprise deployment support

## Technical Implementation Details

### Manifest Generation
- Parse DXT specification for schema validation
- Template system with variable substitution
- Interactive wizard for complex configurations
- Support for all DXT manifest features:
  - User configuration schemas
  - Platform-specific overrides
  - Environment variable templating
  - Security settings

### Packaging Engine
- ZIP archive creation with proper compression
- File permission preservation
- Cross-platform path handling
- Asset optimization (icon resizing, etc.)
- Dependency detection and bundling

### Build System
- Go cross-compilation support
- Node.js/npm integration
- Python virtualenv handling
- Generic binary packaging
- Automated testing integration

### Validation Framework
- JSON schema validation for manifests
- DXT structure verification
- Security linting (file permissions, paths)
- Performance analysis
- Compatibility checking

## Integration with GoMCP Ecosystem

### Documentation
- Add DXT packaging guide to GoMCP docs
- Cross-reference between projects
- Shared examples and tutorials

### Templates
- Include GoMCP-specific templates
- Showcase GoMCP best practices
- Provide ready-to-use server examples

### Community
- Shared GitHub organization
- Coordinated releases
- Compatible versioning

## Relationship to Anthropic's DXT Tools

### Complementary, Not Competing
- **Anthropic's `@anthropic-ai/dxt`**: Official Node.js-based DXT tools
- **GoDXT**: Go-native alternative with additional Go-specific features

### Key Differentiators
- **Go Integration**: Native Go module support, cross-compilation
- **Multi-Language**: Works with any MCP server implementation
- **Developer Experience**: Focused on ease of use for Go developers
- **Template System**: Rich project scaffolding capabilities

### Compatibility
- 100% compatible with official DXT specification
- Generated DXT files work with Claude Desktop
- Manifest format follows official schema
- Interoperable with Anthropic's tools

## Success Metrics

### Developer Adoption
- GoMCP users packaging servers as DXT
- Community contributions to templates
- Documentation and tutorial usage

### Ecosystem Impact
- Increased availability of Go-based MCP servers
- Cross-language MCP server distribution
- Integration with MCP client applications

### Technical Quality
- Reliable DXT generation
- Comprehensive test coverage
- Performance benchmarks
- Security validation

## Next Steps

1. **Repository Creation**: Set up `github.com/localrivet/godxt`
2. **Core Implementation**: CLI framework and basic packaging
3. **Template System**: Go, Node.js, Python, and binary templates
4. **Documentation**: Comprehensive guides and examples
5. **Community Engagement**: Gather feedback from GoMCP users
6. **Integration**: Cross-reference with GoMCP documentation

## Conclusion

GoDXT addresses the DXT packaging need identified in GitHub Issue #12 while maintaining clean separation of concerns. By focusing on packaging and distribution rather than MCP protocol implementation, it provides maximum value to the ecosystem while keeping GoMCP lean and focused.

This approach enables:
- **GoMCP users** to easily distribute their servers as DXT extensions
- **MCP ecosystem** to benefit from more available extensions
- **Claude Desktop users** to access a wider variety of MCP servers
- **Cross-language compatibility** for the entire MCP community

The project scope is well-defined, technically feasible, and addresses a real developer need while following software engineering best practices. 