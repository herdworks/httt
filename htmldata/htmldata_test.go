package htmldata

import (
	"bytes"
	"testing"

	"golang.org/x/net/html"
)

const sampleHTML = `<!doctype html>
<html>
  <head>
    <title>Hello World</title>
    <meta name="description" content="An example page.">
    <meta name="created_at" content="2024-01-01">
    <meta name="updated_at" content="2024-06-01">
  </head>
  <body><p>Body</p></body>
</html>`

func TestUnmarshalTemplateData(t *testing.T) {
	d, err := UnmarshalTemplateData([]byte(sampleHTML))
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	checks := map[string]any{
		"title":       "Hello World",
		"description": "An example page.",
		"created_at":  "2024-01-01",
		"updated_at":  "2024-06-01",
	}
	for k, want := range checks {
		if got := d.Get(k); got != want {
			t.Errorf("%s = %v, want %v", k, got, want)
		}
	}
}

func TestExtractTemplateDataNilNode(t *testing.T) {
	d := ExtractTemplateData(nil)
	if d == nil {
		t.Fatalf("expected non-nil empty map for nil node")
	}
	if len(d) != 0 {
		t.Errorf("expected empty map, got %v", d)
	}
}

func TestExtractTemplateDataNodes(t *testing.T) {
	first, _ := html.Parse(bytes.NewReader([]byte(`<html><head><title>A</title></head></html>`)))
	second, _ := html.Parse(bytes.NewReader([]byte(`<html><head><title>B</title></head></html>`)))
	d := ExtractTemplateDataNodes([]*html.Node{first, second})
	if d.Get("title") != "B" {
		t.Errorf("merge order: title = %v, want B (later wins)", d.Get("title"))
	}
}

func TestMissingFieldsAreOmitted(t *testing.T) {
	d, err := UnmarshalTemplateData([]byte(`<html><head><title>Only Title</title></head></html>`))
	if err != nil {
		t.Fatalf("UnmarshalTemplateData: %v", err)
	}
	if _, ok := d["description"]; ok {
		t.Errorf("expected description to be absent")
	}
	if d.Get("title") != "Only Title" {
		t.Errorf("title not extracted")
	}
}
