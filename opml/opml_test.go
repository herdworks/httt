package opml

import (
	"encoding/json"
	"encoding/xml"
	"strings"
	"testing"
)

// TestOPML_Creation tests creating a new OPML document
func TestOPML_Creation(t *testing.T) {
	opml := &OPML{
		Head: Head{
			Title:        "Test OPML",
			OwnerName:    "Test User",
			OwnerEmail:   "test@example.com",
			DateCreated:  "2023-01-01T00:00:00Z",
			DateModified: "2023-01-01T00:00:00Z",
		},
		Body:    Body{},
		Version: "2.0",
	}

	if opml.Head.Title != "Test OPML" {
		t.Errorf("OPML Head.Title = %v, want Test OPML", opml.Head.Title)
	}
	if opml.Version != "2.0" {
		t.Errorf("OPML Version = %v, want 2.0", opml.Version)
	}
}

// TestOPML_Add tests adding outlines to OPML
func TestOPML_Add(t *testing.T) {
	opml := &OPML{
		Body: Body{},
	}

	outline := &Outline{
		Text:        "Test Blog",
		Type:        "rss",
		HTMLURL:     "https://example.com",
		XMLURL:      "https://example.com/feed.xml",
		Description: "A test blog",
	}

	opml.Add(outline)

	if len(opml.Body.Outlines) != 1 {
		t.Errorf("OPML.Add() Body.Outlines length = %v, want 1", len(opml.Body.Outlines))
		return
	}

	addedOutline := opml.Body.Outlines[0]
	if addedOutline.Text != "Test Blog" {
		t.Errorf("Added outline Text = %v, want Test Blog", addedOutline.Text)
	}
	if addedOutline.Type != "rss" {
		t.Errorf("Added outline Type = %v, want rss", addedOutline.Type)
	}
}

// TestOPML_AddRSS tests adding RSS feeds to OPML
func TestOPML_AddRSS(t *testing.T) {
	opml := &OPML{
		Body: Body{},
	}

	outline := &Outline{
		Text:        "RSS Feed",
		HTMLURL:     "https://example.com",
		XMLURL:      "https://example.com/rss.xml",
		Description: "An RSS feed",
	}

	opml.AddRSS(outline)

	if len(opml.Body.Outlines) != 1 {
		t.Errorf("OPML.AddRSS() Body.Outlines length = %v, want 1", len(opml.Body.Outlines))
		return
	}

	addedOutline := opml.Body.Outlines[0]
	if addedOutline.Type != "rss" {
		t.Errorf("AddRSS outline Type = %v, want rss", addedOutline.Type)
	}
	if addedOutline.Version != "RSS" {
		t.Errorf("AddRSS outline Version = %v, want RSS", addedOutline.Version)
	}
}

// TestOPML_Render tests XML rendering of OPML
func TestOPML_Render(t *testing.T) {
	opml := &OPML{
		Head: Head{
			Title:       "Test Feeds",
			OwnerName:   "Test User",
			DateCreated: "2023-01-01T00:00:00Z",
		},
		Body: Body{
			Outlines: []*Outline{
				{
					Text:    "Example Blog",
					Type:    "rss",
					HTMLURL: "https://example.com",
					XMLURL:  "https://example.com/feed.xml",
				},
			},
		},
	}

	rendered, err := opml.Render()
	if err != nil {
		t.Errorf("OPML.Render() error = %v", err)
		return
	}

	// Check that it contains expected XML elements
	expectedElements := []string{
		`<opml version="2.0">`,
		`<head>`,
		`<title>Test Feeds</title>`,
		`<ownerName>Test User</ownerName>`,
		`<body>`,
		`<outline`,
		`text="Example Blog"`,
		`type="rss"`,
		`htmlUrl="https://example.com"`,
		`xmlUrl="https://example.com/feed.xml"`,
	}

	for _, expected := range expectedElements {
		if !strings.Contains(rendered, expected) {
			t.Errorf("OPML.Render() missing expected element: %s", expected)
		}
	}

	// Verify it's valid XML by attempting to unmarshal
	var testOPML OPML
	if err := xml.Unmarshal([]byte(rendered), &testOPML); err != nil {
		t.Errorf("OPML.Render() produced invalid XML: %v", err)
	}
}

// TestOPML_JSON tests JSON serialization of OPML
func TestOPML_JSON(t *testing.T) {
	opml := &OPML{
		Head: Head{
			Title:     "Test JSON",
			OwnerName: "Test User",
		},
		Body: Body{
			Outlines: []*Outline{
				{
					Text:    "Test Outline",
					Type:    "rss",
					HTMLURL: "https://example.com",
					XMLURL:  "https://example.com/feed.xml",
				},
			},
		},
		Version: "2.0",
	}

	jsonBytes := opml.JSON()
	if len(jsonBytes) == 0 {
		t.Error("OPML.JSON() returned empty bytes")
		return
	}

	// Verify it's valid JSON by attempting to unmarshal
	var testOPML OPML
	if err := json.Unmarshal(jsonBytes, &testOPML); err != nil {
		t.Errorf("OPML.JSON() produced invalid JSON: %v", err)
		return
	}

	// Verify content
	if testOPML.Head.Title != "Test JSON" {
		t.Errorf("JSON unmarshaled Title = %v, want Test JSON", testOPML.Head.Title)
	}
	if len(testOPML.Body.Outlines) != 1 {
		t.Errorf("JSON unmarshaled Outlines length = %v, want 1", len(testOPML.Body.Outlines))
	}
}

// TestOPML_Validate tests OPML validation
func TestOPML_Validate(t *testing.T) {
	tests := []struct {
		name     string
		opml     *OPML
		wantErr  bool
		errMsg   string
	}{
		{
			name: "valid OPML",
			opml: &OPML{
				Body: Body{
					Outlines: []*Outline{
						{
							Text:    "Valid Outline",
							Type:    "rss",
							XMLURL:  "https://example.com/feed.xml",
							HTMLURL: "https://example.com",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing type",
			opml: &OPML{
				Body: Body{
					Outlines: []*Outline{
						{
							Text:   "Invalid Outline",
							XMLURL: "https://example.com/feed.xml",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "outline.type is required",
		},
		{
			name: "missing text",
			opml: &OPML{
				Body: Body{
					Outlines: []*Outline{
						{
							Type:   "rss",
							XMLURL: "https://example.com/feed.xml",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "outline.text is required",
		},
		{
			name: "missing xmlUrl",
			opml: &OPML{
				Body: Body{
					Outlines: []*Outline{
						{
							Text: "Invalid Outline",
							Type: "rss",
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "outline.xmlUrl is required",
		},
		{
			name: "empty outlines",
			opml: &OPML{
				Body: Body{
					Outlines: []*Outline{},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opml.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("OPML.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("OPML.Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestOutline_Validate tests individual outline validation
func TestOutline_Validate(t *testing.T) {
	tests := []struct {
		name    string
		outline *Outline
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid outline",
			outline: &Outline{
				Text:    "Valid Blog",
				Type:    "rss",
				XMLURL:  "https://example.com/feed.xml",
				HTMLURL: "https://example.com",
			},
			wantErr: false,
		},
		{
			name: "missing type",
			outline: &Outline{
				Text:   "Blog",
				XMLURL: "https://example.com/feed.xml",
			},
			wantErr: true,
			errMsg:  "outline.type is required",
		},
		{
			name: "missing text",
			outline: &Outline{
				Type:   "rss",
				XMLURL: "https://example.com/feed.xml",
			},
			wantErr: true,
			errMsg:  "outline.text is required",
		},
		{
			name: "missing xmlUrl",
			outline: &Outline{
				Text: "Blog",
				Type: "rss",
			},
			wantErr: true,
			errMsg:  "outline.xmlUrl is required",
		},
		{
			name: "empty strings",
			outline: &Outline{
				Text:   "",
				Type:   "",
				XMLURL: "",
			},
			wantErr: true,
			errMsg:  "outline.type is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.outline.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Outline.Validate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err.Error() != tt.errMsg {
				t.Errorf("Outline.Validate() error = %v, want %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// TestOPML_XMLMarshalUnmarshal tests XML serialization roundtrip
func TestOPML_XMLMarshalUnmarshal(t *testing.T) {
	original := &OPML{
		XMLName: xml.Name{Local: "opml"},
		Head: Head{
			Title:        "Test OPML Document",
			OwnerName:    "John Doe",
			OwnerEmail:   "john@example.com",
			DateCreated:  "2023-01-01T00:00:00Z",
			DateModified: "2023-01-02T00:00:00Z",
		},
		Body: Body{
			Outlines: []*Outline{
				{
					Text:        "Tech Blog",
					Type:        "rss",
					Title:       "Technology Blog",
					Description: "Latest tech news",
					HTMLURL:     "https://techblog.example.com",
					XMLURL:      "https://techblog.example.com/feed.xml",
					Category:    "technology",
					FaviconURL:  "https://techblog.example.com/favicon.ico",
				},
				{
					Text:    "News Feed",
					Type:    "rss",
					HTMLURL: "https://news.example.com",
					XMLURL:  "https://news.example.com/rss.xml",
					Outlines: []Outline{
						{
							Text:    "Local News",
							Type:    "rss",
							HTMLURL: "https://local.news.example.com",
							XMLURL:  "https://local.news.example.com/feed.xml",
						},
					},
				},
			},
		},
		Version: "2.0",
	}

	// Marshal to XML
	xmlBytes, err := xml.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal OPML to XML: %v", err)
	}

	// Unmarshal from XML
	var unmarshaled OPML
	if err := xml.Unmarshal(xmlBytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal OPML from XML: %v", err)
	}

	// Compare key fields
	if unmarshaled.Head.Title != original.Head.Title {
		t.Errorf("XML roundtrip Head.Title = %v, want %v", unmarshaled.Head.Title, original.Head.Title)
	}
	if unmarshaled.Version != original.Version {
		t.Errorf("XML roundtrip Version = %v, want %v", unmarshaled.Version, original.Version)
	}
	if len(unmarshaled.Body.Outlines) != len(original.Body.Outlines) {
		t.Errorf("XML roundtrip Outlines length = %v, want %v", len(unmarshaled.Body.Outlines), len(original.Body.Outlines))
		return
	}

	// Check first outline
	if unmarshaled.Body.Outlines[0].Text != original.Body.Outlines[0].Text {
		t.Errorf("XML roundtrip first outline Text = %v, want %v", 
			unmarshaled.Body.Outlines[0].Text, original.Body.Outlines[0].Text)
	}
	if unmarshaled.Body.Outlines[0].FaviconURL != original.Body.Outlines[0].FaviconURL {
		t.Errorf("XML roundtrip first outline FaviconURL = %v, want %v", 
			unmarshaled.Body.Outlines[0].FaviconURL, original.Body.Outlines[0].FaviconURL)
	}

	// Check nested outlines
	if len(unmarshaled.Body.Outlines[1].Outlines) != len(original.Body.Outlines[1].Outlines) {
		t.Errorf("XML roundtrip nested outlines length = %v, want %v", 
			len(unmarshaled.Body.Outlines[1].Outlines), len(original.Body.Outlines[1].Outlines))
	}
}

// TestOPML_JSONMarshalUnmarshal tests JSON serialization roundtrip
func TestOPML_JSONMarshalUnmarshal(t *testing.T) {
	original := &OPML{
		Head: Head{
			Title:      "JSON Test",
			OwnerName:  "Jane Doe",
			OwnerEmail: "jane@example.com",
		},
		Body: Body{
			Outlines: []*Outline{
				{
					Text:       "Sample Feed",
					Type:       "rss",
					HTMLURL:    "https://sample.example.com",
					XMLURL:     "https://sample.example.com/feed.xml",
					AudioURL:   "https://sample.example.com/podcast.mp3",
					VideoURL:   "https://sample.example.com/video.mp4",
					IsComment:  false,
					IsBreakpoint: true,
				},
			},
		},
		Version: "2.0",
	}

	// Marshal to JSON
	jsonBytes, err := json.MarshalIndent(original, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal OPML to JSON: %v", err)
	}

	// Unmarshal from JSON
	var unmarshaled OPML
	if err := json.Unmarshal(jsonBytes, &unmarshaled); err != nil {
		t.Fatalf("Failed to unmarshal OPML from JSON: %v", err)
	}

	// Compare key fields
	if unmarshaled.Head.Title != original.Head.Title {
		t.Errorf("JSON roundtrip Head.Title = %v, want %v", unmarshaled.Head.Title, original.Head.Title)
	}
	if len(unmarshaled.Body.Outlines) != len(original.Body.Outlines) {
		t.Errorf("JSON roundtrip Outlines length = %v, want %v", len(unmarshaled.Body.Outlines), len(original.Body.Outlines))
		return
	}

	// Check outline properties including custom attributes
	outline := unmarshaled.Body.Outlines[0]
	originalOutline := original.Body.Outlines[0]
	
	if outline.AudioURL != originalOutline.AudioURL {
		t.Errorf("JSON roundtrip AudioURL = %v, want %v", outline.AudioURL, originalOutline.AudioURL)
	}
	if outline.VideoURL != originalOutline.VideoURL {
		t.Errorf("JSON roundtrip VideoURL = %v, want %v", outline.VideoURL, originalOutline.VideoURL)
	}
	if outline.IsBreakpoint != originalOutline.IsBreakpoint {
		t.Errorf("JSON roundtrip IsBreakpoint = %v, want %v", outline.IsBreakpoint, originalOutline.IsBreakpoint)
	}
}

// TestOPML_EmptyHandling tests handling of empty/nil values
func TestOPML_EmptyHandling(t *testing.T) {
	// Test with minimal OPML
	minimal := &OPML{}
	
	// Should not panic with empty structure
	rendered, err := minimal.Render()
	if err != nil {
		t.Errorf("Empty OPML render error: %v", err)
	}
	if !strings.Contains(rendered, `version="2.0"`) {
		t.Error("Empty OPML should have default version 2.0")
	}

	// JSON should handle empty structure
	jsonBytes := minimal.JSON()
	if len(jsonBytes) == 0 {
		t.Error("Empty OPML JSON() should not return empty bytes")
	}

	// Validation should pass with empty outlines
	if err := minimal.Validate(); err != nil {
		t.Errorf("Empty OPML validation should pass: %v", err)
	}
}

// Benchmark tests for performance-critical OPML operations
func BenchmarkOPML_Render(b *testing.B) {
	opml := &OPML{
		Head: Head{Title: "Benchmark Test"},
		Body: Body{
			Outlines: []*Outline{
				{Text: "Feed 1", Type: "rss", XMLURL: "https://example1.com/feed.xml"},
				{Text: "Feed 2", Type: "rss", XMLURL: "https://example2.com/feed.xml"},
				{Text: "Feed 3", Type: "rss", XMLURL: "https://example3.com/feed.xml"},
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opml.Render()
	}
}

func BenchmarkOPML_JSON(b *testing.B) {
	opml := &OPML{
		Head: Head{Title: "Benchmark Test"},
		Body: Body{
			Outlines: []*Outline{
				{Text: "Feed 1", Type: "rss", XMLURL: "https://example1.com/feed.xml"},
				{Text: "Feed 2", Type: "rss", XMLURL: "https://example2.com/feed.xml"},
				{Text: "Feed 3", Type: "rss", XMLURL: "https://example3.com/feed.xml"},
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opml.JSON()
	}
}

func BenchmarkOPML_Validate(b *testing.B) {
	opml := &OPML{
		Body: Body{
			Outlines: []*Outline{
				{Text: "Feed 1", Type: "rss", XMLURL: "https://example1.com/feed.xml"},
				{Text: "Feed 2", Type: "rss", XMLURL: "https://example2.com/feed.xml"},
				{Text: "Feed 3", Type: "rss", XMLURL: "https://example3.com/feed.xml"},
			},
		},
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		opml.Validate()
	}
}