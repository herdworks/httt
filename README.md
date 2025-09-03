# HyperTemplates Go

The `herdworks/hypertemplates-go` package is the core HTML templating engine for HyperTemplates, designed to deliver high-performance, semantic HTML generation with guaranteed valid HTML5 output and reproducible results.

## Why HyperTemplates?

HyperTemplates is a pure-HTML templating engine that prioritizes:

- **Performance**: Optimized for both mobile devices (minimal overhead) and large-scale websites (raw performance)
- **Reliability**: Guarantees valid HTML5 output and reproducible results
- **Semantic HTML**: Rewards developers for writing semantic HTML with powerful templating features
- **CSS Management**: Provides advanced tools for managing inline CSS and style elements

Unlike traditional templating engines that mix programming logic with markup, HyperTemplates keeps your HTML clean and semantic while providing powerful data-driven content generation.

## Architecture

The HyperTemplates engine consists of several core components:

- **TemplateData**: Dynamic data management with validation, merging, and serialization support
- **Parameters**: Template parameter parsing, resolution, and conditional evaluation
- **Template utilities**: HTML node manipulation, attribute handling, and DOM operations
- **HyperFile**: File I/O, MIME detection, and directory management
- **HyperTheme**: Configuration and resource management for template themes
- **OPML**: XML/JSON serialization for outline and RSS feed management

### Dependencies

HyperTemplates uses minimal, well-established dependencies:

- `golang.org/x/net` - HTML parsing and manipulation
- `gopkg.in/yaml.v3` - YAML configuration support
- `github.com/yuin/goldmark` ecosystem - Markdown processing
- `github.com/alecthomas/chroma/v2` - Syntax highlighting

## Quick Start

### Installation

```bash
go get github.com/herdworks/hypertemplates-go
```

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/herdworks/hypertemplates-go"
)

func main() {
    // Create template data
    data := hypertemplates.TemplateData{
        "title": "My Page",
        "author": "John Doe",
        "content": "Hello, World!",
    }
    
    // Validate required fields
    required := []string{"title", "author"}
    if err := data.Validate(required); err != nil {
        panic(err)
    }
    
    // Use the data in your templates
    fmt.Printf("Title: %v\n", data.Get("title"))
}
```

### Theme Configuration

```go
// Load a theme
theme := &hypertemplates.HyperTheme{
    URI: "/path/to/theme",
}

if err := theme.Validate(); err != nil {
    panic(err)
}

// Use theme resources
htmlContent, err := theme.ResourceHTML("layout")
if err != nil {
    panic(err)
}
```

## Testing

HyperTemplates includes a comprehensive test suite following Go best practices:

```bash
# Run all tests
go test ./...

# Run tests with race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./...

# View test coverage
go test -cover ./...
```

See [TESTING.md](TESTING.md) for detailed testing documentation.

## Development

### Setup

1. Clone the repository:
   ```bash
   git clone https://github.com/herdworks/hypertemplates-go.git
   cd hypertemplates-go
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run tests to verify setup:
   ```bash
   go test ./...
   ```

### Code Standards

This project follows strict Go coding standards:

- Code must be formatted with `gofmt`
- All code must pass `go vet`
- Use table-driven tests for comprehensive coverage
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Error messages should be lowercase without punctuation

## Troubleshooting

### Common Issues

**Build failures with missing dependencies:**
```bash
go mod tidy
go mod download
```

**Test failures:**
```bash
# Clean module cache and retry
go clean -modcache
go mod download
go test ./...
```

**Import errors:**
Ensure you're using the correct import path:
```go
import "github.com/herdworks/hypertemplates-go"
```

**Performance issues:**
- Review your template data structures for unnecessary complexity
- Use benchmarks to identify bottlenecks: `go test -bench=.`
- Consider using streaming approaches for large datasets

### Getting Help

- Visit [HyperTemplates Documentation](https://hypertemplates.net)
- Join the [HyperTexting Community](https://hypertexting.community)
- Check [Issues](https://github.com/herdworks/hypertemplates-go/issues) for known problems

## Resources

- [HyperTemplates Website](https://makehypertext.com)
- [Documentation](https://hypertemplates.net)
- [HyperTexting Guide](https://hypertexting.com)
- [Community](https://hypertexting.community)
