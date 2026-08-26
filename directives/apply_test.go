package directives

import (
	"bytes"
	"strings"
	"testing"

	htdocument "github.com/herdworks/httt/document"
	"golang.org/x/net/html"
)

func TestApplyWithNilDataUsesFallback(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><p ht-apply>${ page.title, "Fallback" }</p></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	output, err := Process(&htdocument.Document{Layout: layout}, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`>Fallback</p>`)) {
		t.Fatalf("nil-data fallback output: %s", output)
	}
}
