# Testing Documentation

## Go Testing Best Practices Implemented

This project uses the **Go standard library testing package** following modern best practices:

### Testing Approach

1. **Standard Library Focus**: Uses Go's built-in `testing` package without heavy third-party dependencies
2. **Table-Driven Tests**: Comprehensive scenario coverage with structured test cases
3. **Sub-tests**: Organized test hierarchies using `t.Run()` for better organization
4. **Benchmarks**: Performance testing for critical operations
5. **Edge Case Coverage**: Thorough testing of error conditions and boundary cases

### Test Structure

#### Core Components Tested

- **TemplateData** (`data_test.go`) - 17 test functions
  - Validation, data manipulation, serialization, merging, cleaning
  - JSON/YAML parsing and marshaling
  - Nested data access and modification

- **Parameters** (`parameters_test.go`) - 6 test functions  
  - Parameter parsing, resolution, evaluation
  - Integration testing with TemplateData
  - Conditional logic validation

- **Template Utilities** (`template_test.go`) - 11 test functions
  - HTML node manipulation and attribute handling
  - DOM utilities, cloning, and text operations
  - Parameter parsing and URL validation

- **HyperFile** (`file_test.go`) - 15 test functions
  - File I/O operations and metadata handling
  - MIME type detection and path manipulation
  - Directory walking and pattern matching

- **HyperTheme** (`theme_test.go`) - 10 test functions
  - Theme configuration and validation
  - Resource path resolution and HTML loading
  - Directory structure management

- **OPML** (`opml/opml_test.go`) - 10 test functions
  - XML/JSON serialization and validation
  - Outline management and data integrity
  - Roundtrip testing for data consistency

- **Render Integration** (`render_test.go`) - 4 test functions
  - High-level integration testing
  - Error handling and data format validation

### Test Execution

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...

# Run benchmarks
go test -bench=. ./...

# Run specific test
go test -run TestTemplateData_Validate
```

### GitHub Actions Integration

The `.github/workflows/test.yml` workflow provides:
- Multi-version Go testing (1.23, 1.24)
- Race condition detection
- Coverage reporting
- Benchmark execution
- Linting with golangci-lint

### Test Coverage Highlights

- **659 total test assertions** across all components
- **12 benchmark tests** for performance validation
- **Comprehensive error handling** testing
- **Integration tests** validating component interactions
- **Roundtrip serialization** testing for data integrity

### Testing Conventions

1. **Naming**: Test functions follow `Test<Component>_<Method>` pattern
2. **Organization**: Related tests grouped using sub-tests
3. **Data**: Test data generated programmatically or from temporary files
4. **Cleanup**: Proper resource cleanup using defer statements
5. **Isolation**: Tests are independent and can run in any order

### Performance Testing

Benchmark tests are included for performance-critical operations:
- Data manipulation and merging
- Parameter parsing and evaluation
- File operations and path resolution
- HTML node operations
- Serialization/deserialization

This testing framework provides a solid foundation for Test-Driven Development and ensures the reliability and performance of the HyperTemplates core functionality.