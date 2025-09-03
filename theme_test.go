package hypertemplates

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"testing"
)

// TestHyperTheme_Creation tests creating a new HyperTheme
func TestHyperTheme_Creation(t *testing.T) {
	uri, _ := url.Parse("/path/to/theme")
	theme := &HyperTheme{
		URI:         uri,
		Name:        "Test Theme",
		Description: "A test theme",
		Version:     "1.0.0",
	}

	if theme.Name != "Test Theme" {
		t.Errorf("HyperTheme Name = %v, want Test Theme", theme.Name)
	}
	if theme.Version != "1.0.0" {
		t.Errorf("HyperTheme Version = %v, want 1.0.0", theme.Version)
	}
}

// TestHyperTheme_Validate tests theme validation
func TestHyperTheme_Validate(t *testing.T) {
	tests := []struct {
		name    string
		theme   *HyperTheme
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid theme",
			theme: &HyperTheme{
				URI: &url.URL{Path: "/valid/theme"},
			},
			wantErr: false,
		},
		{
			name: "missing URI",
			theme: &HyperTheme{
				Name: "Test",
			},
			wantErr: true,
			errMsg:  "website.theme.uri is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.theme.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("HyperTheme.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("HyperTheme.Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestHyperTheme_DirectoryMethods tests directory path methods
func TestHyperTheme_DirectoryMethods(t *testing.T) {
	theme := &HyperTheme{
		URI: &url.URL{Path: "/path/to/theme/theme.json"},
		Config: HyperThemeConfig{
			ContentTypesDirectory: "custom-types",
			BlocksDirectory:       "custom-blocks",
			LayoutsDirectory:      "custom-layouts",
			StaticDirectory:       "custom-static",
		},
	}

	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "RootDir",
			method:   theme.RootDir,
			expected: "/path/to/theme",
		},
		{
			name:     "ContentTypesDir",
			method:   theme.ContentTypesDir,
			expected: "/path/to/theme/custom-types",
		},
		{
			name:     "ElementsDir",
			method:   theme.ElementsDir,
			expected: "/path/to/theme/custom-blocks",
		},
		{
			name:     "LayoutsDir",
			method:   theme.LayoutsDir,
			expected: "/path/to/theme/custom-layouts",
		},
		{
			name:     "StaticDir",
			method:   theme.StaticDir,
			expected: "/path/to/theme/custom-static",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method()
			if got != tt.expected {
				t.Errorf("HyperTheme.%s() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

// TestHyperTheme_DefaultDirectories tests default directory configuration
func TestHyperTheme_DefaultDirectories(t *testing.T) {
	theme := &HyperTheme{
		URI:    &url.URL{Path: "/path/to/theme/theme.json"},
		Config: HyperThemeConfig{}, // Empty config to test defaults
	}

	tests := []struct {
		name     string
		method   func() string
		expected string
	}{
		{
			name:     "ContentTypesDir default",
			method:   theme.ContentTypesDir,
			expected: "/path/to/theme/types",
		},
		{
			name:     "ElementsDir default",
			method:   theme.ElementsDir,
			expected: "/path/to/theme/blocks",
		},
		{
			name:     "LayoutsDir default",
			method:   theme.LayoutsDir,
			expected: "/path/to/theme/layouts",
		},
		{
			name:     "StaticDir default",
			method:   theme.StaticDir,
			expected: "/path/to/theme/static",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.method()
			if got != tt.expected {
				t.Errorf("HyperTheme.%s() = %v, want %v", tt.name, got, tt.expected)
			}
		})
	}
}

// TestHyperTheme_ResolveConfig tests configuration resolution
func TestHyperTheme_ResolveConfig(t *testing.T) {
	theme := &HyperTheme{
		Config: HyperThemeConfig{}, // Empty config
	}

	theme.ResolveConfig()

	expectedDefaults := map[string]string{
		"ContentTypesDirectory": "types",
		"BlocksDirectory":       "blocks",
		"LayoutsDirectory":      "layouts",
		"StaticDirectory":       "static",
	}

	if theme.Config.ContentTypesDirectory != expectedDefaults["ContentTypesDirectory"] {
		t.Errorf("ResolveConfig() ContentTypesDirectory = %v, want %v", 
			theme.Config.ContentTypesDirectory, expectedDefaults["ContentTypesDirectory"])
	}
	if theme.Config.BlocksDirectory != expectedDefaults["BlocksDirectory"] {
		t.Errorf("ResolveConfig() BlocksDirectory = %v, want %v", 
			theme.Config.BlocksDirectory, expectedDefaults["BlocksDirectory"])
	}
	if theme.Config.LayoutsDirectory != expectedDefaults["LayoutsDirectory"] {
		t.Errorf("ResolveConfig() LayoutsDirectory = %v, want %v", 
			theme.Config.LayoutsDirectory, expectedDefaults["LayoutsDirectory"])
	}
	if theme.Config.StaticDirectory != expectedDefaults["StaticDirectory"] {
		t.Errorf("ResolveConfig() StaticDirectory = %v, want %v", 
			theme.Config.StaticDirectory, expectedDefaults["StaticDirectory"])
	}
}

// TestHyperTheme_ResourcePath tests resource path generation
func TestHyperTheme_ResourcePath(t *testing.T) {
	theme := &HyperTheme{
		URI: &url.URL{Path: "/path/to/theme/theme.json"},
	}

	tests := []struct {
		name         string
		resourceRef  string
		expected     string
	}{
		{
			name:        "resource with extension",
			resourceRef: "layouts/default.html",
			expected:    "/path/to/theme/layouts/default.html",
		},
		{
			name:        "resource without extension",
			resourceRef: "layouts/default",
			expected:    "/path/to/theme/layouts/default.html",
		},
		{
			name:        "nested resource",
			resourceRef: "blocks/components/header.html",
			expected:    "/path/to/theme/blocks/components/header.html",
		},
		{
			name:        "root resource",
			resourceRef: "index",
			expected:    "/path/to/theme/index.html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := theme.ResourcePath(tt.resourceRef)
			if got != tt.expected {
				t.Errorf("HyperTheme.ResourcePath(%v) = %v, want %v", tt.resourceRef, got, tt.expected)
			}
		})
	}
}

// TestHyperTheme_ResourceExists tests resource existence checking
func TestHyperTheme_ResourceExists(t *testing.T) {
	// Create temporary theme directory
	tmpDir, err := os.MkdirTemp("", "theme_resource_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.html")
	if err := os.WriteFile(testFile, []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	theme := &HyperTheme{
		URI: &url.URL{Path: filepath.Join(tmpDir, "theme.json")},
	}

	tests := []struct {
		name           string
		path           string
		expectedExists bool
		expectedError  bool
	}{
		{
			name:           "existing file",
			path:           testFile,
			expectedExists: true,
			expectedError:  false,
		},
		{
			name:           "non-existing file",
			path:           filepath.Join(tmpDir, "nonexistent.html"),
			expectedExists: false,
			expectedError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exists, err := theme.ResourceExists(tt.path)
			if (err != nil) != tt.expectedError {
				t.Errorf("HyperTheme.ResourceExists() error = %v, expectedError %v", err, tt.expectedError)
			}
			if exists != tt.expectedExists {
				t.Errorf("HyperTheme.ResourceExists() = %v, want %v", exists, tt.expectedExists)
			}
		})
	}
}

// TestHyperTheme_ResourceHTML tests HTML resource loading
func TestHyperTheme_ResourceHTML(t *testing.T) {
	// Create temporary theme directory
	tmpDir, err := os.MkdirTemp("", "theme_html_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test HTML file
	testFile := filepath.Join(tmpDir, "test.html")
	htmlContent := `<!DOCTYPE html><html><head><title>Test</title></head><body><h1>Hello</h1></body></html>`
	if err := os.WriteFile(testFile, []byte(htmlContent), 0644); err != nil {
		t.Fatalf("Failed to create test HTML file: %v", err)
	}

	// Create non-HTML file
	nonHtmlFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(nonHtmlFile, []byte("text content"), 0644); err != nil {
		t.Fatalf("Failed to create test text file: %v", err)
	}

	theme := &HyperTheme{
		URI: &url.URL{Path: filepath.Join(tmpDir, "theme.json")},
	}

	tests := []struct {
		name        string
		resourceRef string
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid HTML file",
			resourceRef: "test.html",
			wantErr:     false,
		},
		{
			name:        "HTML file without extension",
			resourceRef: "test",
			wantErr:     false,
		},
		{
			name:        "non-HTML file",
			resourceRef: "test.txt",
			wantErr:     true,
			errContains: "is not an HTML file",
		},
		{
			name:        "non-existent file",
			resourceRef: "nonexistent.html",
			wantErr:     true,
			errContains: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node, err := theme.ResourceHTML(tt.resourceRef)
			if (err != nil) != tt.wantErr {
				t.Errorf("HyperTheme.ResourceHTML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("HyperTheme.ResourceHTML() error = %v, expected to contain %v", err.Error(), tt.errContains)
				}
			}
			if !tt.wantErr && node == nil {
				t.Error("HyperTheme.ResourceHTML() returned nil node for valid resource")
			}
		})
	}
}

// TestHyperTheme_Parse tests theme configuration parsing
func TestHyperTheme_Parse(t *testing.T) {
	// Create temporary theme directory
	tmpDir, err := os.MkdirTemp("", "theme_parse_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create theme configuration file
	themeConfigPath := filepath.Join(tmpDir, "theme.json")
	themeConfig := map[string]any{
		"name":        "Test Theme",
		"description": "A test theme for parsing",
		"version":     "1.0.0",
		"config": map[string]string{
			"layouts_dir": "custom-layouts",
			"static_dir":  "custom-static",
		},
	}
	configBytes, _ := json.Marshal(themeConfig)
	if err := os.WriteFile(themeConfigPath, configBytes, 0644); err != nil {
		t.Fatalf("Failed to create theme config file: %v", err)
	}

	// Test parsing
	theme := &HyperTheme{
		URI: &url.URL{Path: themeConfigPath},
	}

	err = theme.Parse()
	if err != nil {
		t.Errorf("HyperTheme.Parse() error = %v", err)
		return
	}

	// Verify parsed values
	if theme.Name != "Test Theme" {
		t.Errorf("Parse() Name = %v, want Test Theme", theme.Name)
	}
	if theme.Description != "A test theme for parsing" {
		t.Errorf("Parse() Description = %v, want 'A test theme for parsing'", theme.Description)
	}
	if theme.Version != "1.0.0" {
		t.Errorf("Parse() Version = %v, want 1.0.0", theme.Version)
	}
}

// TestHyperTheme_ParseNonExistentFile tests parsing with non-existent file
func TestHyperTheme_ParseNonExistentFile(t *testing.T) {
	theme := &HyperTheme{
		URI: &url.URL{Path: "/nonexistent/theme.json"},
	}

	// Should not return an error, but should print a warning
	err := theme.Parse()
	if err != nil {
		t.Errorf("HyperTheme.Parse() with non-existent file should not return error, got: %v", err)
	}
}

// Helper function to check if a string contains a substring
func containsString(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && 
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}

// Benchmark tests for HyperTheme operations
func BenchmarkHyperTheme_ResourcePath(b *testing.B) {
	theme := &HyperTheme{
		URI: &url.URL{Path: "/path/to/theme/theme.json"},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		theme.ResourcePath("layouts/default")
	}
}

func BenchmarkHyperTheme_ResolveConfig(b *testing.B) {
	theme := &HyperTheme{
		Config: HyperThemeConfig{},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		theme.ResolveConfig()
	}
}