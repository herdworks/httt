package hypertemplates

import (
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHyperFile_File tests opening a file
func TestHyperFile_File(t *testing.T) {
	// Create a temporary test file
	tmpFile, err := os.CreateTemp("", "hyperfile_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Write test content
	testContent := "Hello, HyperFile!"
	if _, err := tmpFile.WriteString(testContent); err != nil {
		t.Fatalf("Failed to write to temp file: %v", err)
	}
	tmpFile.Close() // Close so HyperFile can open it

	// Create HyperFile
	uri, _ := url.Parse(tmpFile.Name())
	hyperFile := &HyperFile{URI: uri}

	// Test opening the file
	file := hyperFile.File()
	if file == nil {
		t.Error("HyperFile.File() returned nil")
		return
	}
	defer file.Close()

	// Read content and verify
	content, err := io.ReadAll(file)
	if err != nil {
		t.Errorf("Failed to read file content: %v", err)
	}
	if string(content) != testContent {
		t.Errorf("File content = %v, want %v", string(content), testContent)
	}
}

// TestHyperFile_Bytes tests reading file as bytes
func TestHyperFile_Bytes(t *testing.T) {
	// Create a temporary test file
	tmpFile, err := os.CreateTemp("", "hyperfile_bytes_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testContent := "Test content for bytes"
	if err := os.WriteFile(tmpFile.Name(), []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Create HyperFile
	uri, _ := url.Parse(tmpFile.Name())
	hyperFile := &HyperFile{URI: uri}

	// Test reading bytes
	content, err := hyperFile.Bytes()
	if err != nil {
		t.Errorf("HyperFile.Bytes() error = %v", err)
	}
	if string(content) != testContent {
		t.Errorf("HyperFile.Bytes() = %v, want %v", string(content), testContent)
	}
}

// TestHyperFile_Size tests getting file size
func TestHyperFile_Size(t *testing.T) {
	// Create a temporary test file
	tmpFile, err := os.CreateTemp("", "hyperfile_size_test_*.txt")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())

	testContent := "Test content for size measurement"
	if err := os.WriteFile(tmpFile.Name(), []byte(testContent), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	// Create HyperFile
	uri, _ := url.Parse(tmpFile.Name())
	hyperFile := &HyperFile{URI: uri}

	// Test size
	size := hyperFile.Size()
	expectedSize := int64(len(testContent))
	if size != expectedSize {
		t.Errorf("HyperFile.Size() = %v, want %v", size, expectedSize)
	}
}

// TestHyperFile_Extension tests getting file extension
func TestHyperFile_Extension(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		expected  string
	}{
		{
			name:     "HTML file",
			path:     "/path/to/file.html",
			expected: ".html",
		},
		{
			name:     "CSS file",
			path:     "/path/to/style.css",
			expected: ".css",
		},
		{
			name:     "No extension",
			path:     "/path/to/file",
			expected: "",
		},
		{
			name:     "Multiple dots",
			path:     "/path/to/file.min.js",
			expected: ".js",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, _ := url.Parse(tt.path)
			hyperFile := &HyperFile{URI: uri}
			
			got := hyperFile.Extension()
			if got != tt.expected {
				t.Errorf("HyperFile.Extension() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestHyperFile_ExtensionType tests MIME type detection by extension
func TestHyperFile_ExtensionType(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		expected  string
	}{
		{
			name:     "HTML file",
			path:     "/path/to/file.html",
			expected: "text/html",
		},
		{
			name:     "CSS file",
			path:     "/path/to/style.css",
			expected: "text/css",
		},
		{
			name:     "JavaScript file",
			path:     "/path/to/script.js",
			expected: "application/javascript",
		},
		{
			name:     "JSON file",
			path:     "/path/to/data.json",
			expected: "application/json",
		},
		{
			name:     "PNG image",
			path:     "/path/to/image.png",
			expected: "image/png",
		},
		{
			name:     "JPEG image",
			path:     "/path/to/image.jpg",
			expected: "image/jpeg",
		},
		{
			name:     "JPEG image (alternative)",
			path:     "/path/to/image.jpeg",
			expected: "image/jpeg",
		},
		{
			name:     "SVG image",
			path:     "/path/to/icon.svg",
			expected: "image/svg+xml",
		},
		{
			name:     "MP3 audio",
			path:     "/path/to/audio.mp3",
			expected: "audio/mpeg",
		},
		{
			name:     "MP4 video",
			path:     "/path/to/video.mp4",
			expected: "video/mp4",
		},
		{
			name:     "Unknown extension",
			path:     "/path/to/file.unknown",
			expected: "application/octet-stream",
		},
		{
			name:     "No extension",
			path:     "/path/to/file",
			expected: "application/octet-stream",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uri, _ := url.Parse(tt.path)
			hyperFile := &HyperFile{URI: uri}
			
			got := hyperFile.ExtensionType()
			if got != tt.expected {
				t.Errorf("HyperFile.ExtensionType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestHyperFile_PathMethods tests various path manipulation methods
func TestHyperFile_PathMethods(t *testing.T) {
	// Create a temporary directory structure for testing
	tmpDir, err := os.MkdirTemp("", "hyperfile_path_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory and file
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	testFile := filepath.Join(subDir, "test.html")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create HyperFile
	uri, _ := url.Parse(testFile)
	hyperFile := &HyperFile{
		URI:  uri,
		Base: tmpDir,
	}

	// Test AbsBase
	absBase := hyperFile.AbsBase()
	expectedAbsBase, _ := filepath.Abs(tmpDir)
	if !strings.HasPrefix(absBase, expectedAbsBase) {
		t.Errorf("HyperFile.AbsBase() = %v, expected to start with %v", absBase, expectedAbsBase)
	}

	// Test AbsPath
	absPath := hyperFile.AbsPath()
	expectedAbsPath, _ := filepath.Abs(testFile)
	if absPath != expectedAbsPath {
		t.Errorf("HyperFile.AbsPath() = %v, want %v", absPath, expectedAbsPath)
	}

	// Test SubPath (relative to base)
	subPath := hyperFile.SubPath()
	expectedSubPath := filepath.Join("subdir", "test.html")
	if subPath != expectedSubPath {
		t.Errorf("HyperFile.SubPath() = %v, want %v", subPath, expectedSubPath)
	}

	// Test DestinationAbsPath
	prefix := "/dest"
	destPath := hyperFile.DestinationAbsPath(prefix)
	expectedDestPath, _ := filepath.Abs(filepath.Join(prefix, expectedSubPath))
	if destPath != expectedDestPath {
		t.Errorf("HyperFile.DestinationAbsPath() = %v, want %v", destPath, expectedDestPath)
	}
}

// TestHyperFile_Metadata tests metadata operations
func TestHyperFile_Metadata(t *testing.T) {
	hyperFile := &HyperFile{
		URI: &url.URL{Path: "/test/file.html"},
	}

	// Test initial metadata (should be nil)
	metadata := hyperFile.GetMetadata()
	if metadata != nil {
		t.Error("Initial metadata should be nil")
	}

	// Test setting metadata
	hyperFile.SetMetadata("Content-Type", "text/html")
	hyperFile.SetMetadata("Cache-Control", "max-age=3600")

	// Test getting metadata
	metadata = hyperFile.GetMetadata()
	if metadata == nil {
		t.Error("Metadata should not be nil after setting values")
		return
	}

	if metadata["Content-Type"] != "text/html" {
		t.Errorf("Metadata Content-Type = %v, want text/html", metadata["Content-Type"])
	}
	if metadata["Cache-Control"] != "max-age=3600" {
		t.Errorf("Metadata Cache-Control = %v, want max-age=3600", metadata["Cache-Control"])
	}
}

// TestHyperFile_ContentType tests Content-Type metadata retrieval
func TestHyperFile_ContentType(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		expected string
	}{
		{
			name:     "no metadata",
			metadata: nil,
			expected: "application/octet-stream",
		},
		{
			name:     "Content-Type metadata",
			metadata: map[string]string{"Content-Type": "text/html"},
			expected: "text/html",
		},
		{
			name:     "content-type metadata (lowercase)",
			metadata: map[string]string{"content-type": "application/json"},
			expected: "application/json",
		},
		{
			name:     "both forms present (Content-Type takes precedence)",
			metadata: map[string]string{
				"Content-Type": "text/html",
				"content-type": "application/json",
			},
			expected: "text/html",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hyperFile := &HyperFile{
				URI:        &url.URL{Path: "/test/file.html"},
				Properties: tt.metadata,
			}

			got := hyperFile.ContentType()
			if got != tt.expected {
				t.Errorf("HyperFile.ContentType() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestHyperFile_Href tests URL generation
func TestHyperFile_Href(t *testing.T) {
	tests := []struct {
		name     string
		file     *HyperFile
		href     string
		expected string
	}{
		{
			name: "simple file",
			file: &HyperFile{
				URI:  &url.URL{Path: "/base/subdir/file.html"},
				Base: "/base",
			},
			href:     "https://example.com",
			expected: "https://example.com/subdir/file.html",
		},
		{
			name: "file with query params",
			file: &HyperFile{
				URI:  &url.URL{Path: "/base/page.html", RawQuery: "param=value"},
				Base: "/base",
			},
			href:     "https://example.com",
			expected: "https://example.com/page.html?param=value", // Base URL doesn't include paths
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.file.Href(tt.href)
			if got != tt.expected {
				t.Errorf("HyperFile.Href() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// TestHyperFiles_Sort tests sorting functionality
func TestHyperFiles_Sort(t *testing.T) {
	files := HyperFiles{
		{URI: &url.URL{Path: "/base/z/file.html"}},
		{URI: &url.URL{Path: "/base/a/file.html"}},
		{URI: &url.URL{Path: "/base/m/file.html"}},
	}

	files.Sort(false)

	// Check if sorted by path
	if files[0].URI.Path >= files[1].URI.Path || files[1].URI.Path >= files[2].URI.Path {
		t.Error("Files are not sorted correctly by path")
	}
}

// TestHyperFiles_FileNames tests getting file names
func TestHyperFiles_FileNames(t *testing.T) {
	files := HyperFiles{
		{URI: &url.URL{Path: "/path/to/file1.html"}},
		{URI: &url.URL{Path: "/another/path/file2.css"}},
		{URI: &url.URL{Path: "/file3.js"}},
	}

	names := files.FileNames()
	expected := []string{"file1.html", "file2.css", "file3.js"}

	if len(names) != len(expected) {
		t.Errorf("FileNames() length = %v, want %v", len(names), len(expected))
		return
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("FileNames()[%d] = %v, want %v", i, name, expected[i])
		}
	}
}

// TestHyperFiles_ColumnValues tests column value extraction
func TestHyperFiles_ColumnValues(t *testing.T) {
	// Create temp files for testing
	tmpDir, err := os.MkdirTemp("", "hyperfiles_column_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2_longer"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	files := HyperFiles{
		{URI: &url.URL{Path: file1}, Base: tmpDir},
		{URI: &url.URL{Path: file2}, Base: tmpDir},
	}

	tests := []struct {
		name     string
		field    string
		checkLen bool
	}{
		{
			name:     "Size column",
			field:    "Size",
			checkLen: true,
		},
		{
			name:     "Path column",
			field:    "Path",
			checkLen: true,
		},
		{
			name:     "Source column",
			field:    "Source",
			checkLen: true,
		},
		{
			name:     "Unknown column",
			field:    "Unknown",
			checkLen: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			values := files.ColumnValues(tt.field)
			if tt.checkLen && len(values) != len(files) {
				t.Errorf("ColumnValues(%v) length = %v, want %v", tt.field, len(values), len(files))
			}
			if !tt.checkLen && len(values) != 0 {
				t.Errorf("ColumnValues(%v) should return empty slice for unknown field", tt.field)
			}
		})
	}
}

// TestGetPaths tests directory walking functionality
func TestGetPaths(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "getpaths_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create files and subdirectories
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(subDir, "file2.txt")
	if err := os.WriteFile(file1, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Test non-recursive
	paths := GetPaths(tmpDir, false)
	if len(paths) != 1 {
		t.Errorf("GetPaths(false) returned %d paths, want 1", len(paths))
	}

	// Test recursive
	pathsRecursive := GetPaths(tmpDir, true)
	if len(pathsRecursive) != 2 {
		t.Errorf("GetPaths(true) returned %d paths, want 2", len(pathsRecursive))
	}

	// Test that paths are sorted
	for i := 1; i < len(pathsRecursive); i++ {
		if pathsRecursive[i-1] > pathsRecursive[i] {
			t.Error("Paths are not sorted")
			break
		}
	}
}

// TestGetFiles tests file collection functionality
func TestGetFiles(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "getfiles_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.html")
	if err := os.WriteFile(testFile, []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Test GetFiles
	files := GetFiles(tmpDir, false)
	if len(files) != 1 {
		t.Errorf("GetFiles() returned %d files, want 1", len(files))
		return
	}

	// Verify file properties
	file := files[0]
	if file.URI == nil {
		t.Error("File URI is nil")
	}
	if file.Base == "" {
		t.Error("File Base is empty")
	}
	if file.Extension() != ".html" {
		t.Errorf("File extension = %v, want .html", file.Extension())
	}
}

// TestGetFilesByPattern tests pattern-based file filtering
func TestGetFilesByPattern(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "getfiles_pattern_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	htmlFile := filepath.Join(tmpDir, "test.html")
	cssFile := filepath.Join(tmpDir, "style.css")
	jsFile := filepath.Join(tmpDir, "script.js")
	
	for _, file := range []string{htmlFile, cssFile, jsFile} {
		if err := os.WriteFile(file, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", file, err)
		}
	}

	tests := []struct {
		name         string
		include      string
		exclude      string
		expectedLen  int
	}{
		{
			name:        "include HTML files",
			include:     `\.html$`,
			exclude:     "",
			expectedLen: 1,
		},
		{
			name:        "exclude CSS files",
			include:     "",
			exclude:     `\.css$`,
			expectedLen: 2, // HTML and JS files
		},
		{
			name:        "include JS, exclude nothing",
			include:     `\.js$`,
			exclude:     "",
			expectedLen: 1,
		},
		{
			name:        "no filters",
			include:     "",
			exclude:     "",
			expectedLen: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := GetFilesByPattern(tmpDir, false, tt.include, tt.exclude)
			if len(files) != tt.expectedLen {
				t.Errorf("GetFilesByPattern() returned %d files, want %d", len(files), tt.expectedLen)
			}
		})
	}
}

// Benchmark tests for performance-critical file operations
func BenchmarkHyperFile_Extension(b *testing.B) {
	uri, _ := url.Parse("/path/to/file.html")
	hyperFile := &HyperFile{URI: uri}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hyperFile.Extension()
	}
}

func BenchmarkHyperFile_ExtensionType(b *testing.B) {
	uri, _ := url.Parse("/path/to/file.html")
	hyperFile := &HyperFile{URI: uri}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hyperFile.ExtensionType()
	}
}

func BenchmarkHyperFiles_Sort(b *testing.B) {
	files := make(HyperFiles, 100)
	for i := range files {
		uri, _ := url.Parse("/path/" + string(rune('z'-i%26)) + "/file.html")
		files[i] = HyperFile{URI: uri}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		files.Sort(false)
	}
}