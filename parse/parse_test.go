package parse

import (
	"strings"
	"testing"
)

func TestUnmarshalJSON(t *testing.T) {
	d, err := Unmarshal([]byte(`{"a":1}`), ".json")
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := d.Get("a"); got != float64(1) {
		t.Errorf("a = %v, want 1", got)
	}
}

func TestUnmarshalYAML(t *testing.T) {
	d, err := Unmarshal([]byte("a: 1\nb: x\n"), ".yaml")
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if d.Get("a") != 1 || d.Get("b") != "x" {
		t.Errorf("got %v", d)
	}
}

func TestUnmarshalMarkdown(t *testing.T) {
	d, err := Unmarshal([]byte("---\ntitle: Hi\n---\n\nbody\n"), ".md")
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if d.Get("title") != "Hi" {
		t.Errorf("title = %v, want Hi", d.Get("title"))
	}
	if !strings.Contains(d.Get("content").(string), "<p") {
		t.Errorf("content not rendered")
	}
}

func TestUnmarshalHTML(t *testing.T) {
	d, err := Unmarshal([]byte(`<html><head><title>Hi</title></head></html>`), ".html")
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if d.Get("title") != "Hi" {
		t.Errorf("title = %v, want Hi", d.Get("title"))
	}
}

func TestUnsupportedExtension(t *testing.T) {
	if _, err := Unmarshal([]byte(""), ".xyz"); err == nil {
		t.Errorf("expected error for unknown extension")
	}
}

func TestExtensionNormalization(t *testing.T) {
	cases := []string{".json", "json", ".JSON", "JSON"}
	for _, ext := range cases {
		if _, err := Unmarshal([]byte(`{}`), ext); err != nil {
			t.Errorf("ext %q: %v", ext, err)
		}
	}
}

func TestUnmarshalCSV(t *testing.T) {
	d, err := Unmarshal([]byte("name,age\nAlice,30\nBob,40\n"), ".csv")
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := d.Len("rows"); got != 2 {
		t.Fatalf("rows length = %d, want 2", got)
	}
	if got := d.Get("rows.1.name"); got != "Alice" {
		t.Errorf("rows.1.name = %v, want Alice", got)
	}
}

func TestUnmarshalTSV(t *testing.T) {
	d, err := Unmarshal([]byte("name\tage\nAlice\t30\n"), ".tsv")
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := d.Get("rows.1.age"); got != "30" {
		t.Errorf("rows.1.age = %v, want \"30\" (delimiter not honored)", got)
	}
}

func TestUnmarshalCSVHeaderValidationError(t *testing.T) {
	_, err := Unmarshal([]byte(",age\nAlice,30\n"), ".csv")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "header") {
		t.Errorf("error %q does not mention header", err)
	}
}

func TestReadFile(t *testing.T) {
	d, err := ReadFile("testdata/keypaths.yaml")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if got := d.Get("site.title"); got != "HyperTemplates Testing" {
		t.Errorf("site.title = %v", got)
	}
}
