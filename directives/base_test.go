package directives

import (
	"bytes"
	"strings"
	"testing"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"github.com/herdworks/httt/variable"
	"golang.org/x/net/html"
)

func TestBaseUsesTypedThreeTokenDeclarations(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><div ht-base="href on ${ page.base }" ht-base="src on ${ page.base }"><a href="post">post</a><img src="image.png"></div></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"page": htdata.TemplateData{"base": "/blog/"}}
	output, err := Process(&htdocument.Document{Layout: layout}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`href="/blog/post"`)) || !bytes.Contains(output, []byte(`src="/blog/image.png"`)) || bytes.Contains(output, []byte("ht-base")) {
		t.Fatalf("output = %s", output)
	}
}

func TestBaseRejectsNakedAndDuplicateTargetsWithoutLookup(t *testing.T) {
	tests := []struct {
		source string
		want   string
	}{
		{
			source: `<html><body><div ht-base="href:page.base"></div></body></html>`,
			want:   "invalid \"ht-base\" directive (expected \"<attribute> on ${ ... }\", got \"href:page.base\")\n<div ht-base=\"href:page.base\"></div>",
		},
		{
			source: `<html><body><div ht-base="href on ${ page.base }" data-ht-base="href on ${ page.other }"></div></body></html>`,
		},
	}
	for _, test := range tests {
		layout, err := html.Parse(strings.NewReader(test.source))
		if err != nil {
			t.Fatal(err)
		}
		data := htdata.TemplateData{"page": htdata.TemplateData{"base": "/blog/"}}
		if _, err = Process(&htdocument.Document{Layout: layout}, &data, nil, nil); err == nil {
			t.Fatalf("Process(%q) succeeded", test.source)
		} else if test.want != "" && err.Error() != test.want {
			t.Errorf("error = %q, want %q", err, test.want)
		}
	}
}

func TestBaseRejectsInvalidDeclarationShapes(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "quoted target", source: `<div ht-base="'href' on ${ page.base }"></div>`},
		{name: "uppercase operator", source: `<div ht-base="href ON ${ page.base }"></div>`},
		{name: "semicolon declarations", source: `<div ht-base="href on ${ page.base };src on ${ page.base }"></div>`},
		{name: "trailing token", source: `<div ht-base="href on ${ page.base } extra"></div>`},
		{name: "incomplete variable", source: `<div ht-base="href on ${ page.base"></div>`},
		{name: "variable suffix", source: `<div ht-base="href on ${ page.base }suffix"></div>`},
		{name: "missing value", source: `<div ht-base="href on ${ page.missing }"></div>`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			layout, err := html.Parse(strings.NewReader(`<html><body>` + test.source + `</body></html>`))
			if err != nil {
				t.Fatal(err)
			}
			data := htdata.TemplateData{"page": htdata.TemplateData{"base": "/blog/"}}
			if _, err = Process(&htdocument.Document{Layout: layout}, &data, nil, nil); err == nil {
				t.Fatalf("Process(%q) succeeded", test.source)
			}
		})
	}
}

func TestBaseStrictModeUsesDataAliasAndRemovesBothAliases(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><div ht-base="href on ${ page.ignored }" data-ht-base="src on ${ page.base }"><a href="post">post</a><img src="image.png"></div></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	data := htdata.TemplateData{"page": htdata.TemplateData{"base": "/blog/"}}
	output, err := Process(&htdocument.Document{Layout: layout, StrictMode: true}, &data, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`href="post"`)) || !bytes.Contains(output, []byte(`src="/blog/image.png"`)) || bytes.Contains(output, []byte("ht-base")) {
		t.Fatalf("output = %s", output)
	}
}

func TestBaseRendersVariablePlugins(t *testing.T) {
	layout, err := html.Parse(strings.NewReader(`<html><body><div ht-base="href on ${ base() }"><a href="post">post</a></div></body></html>`))
	if err != nil {
		t.Fatal(err)
	}
	base := func(...any) (any, error) { return "/plugin/", nil }
	output, err := Process(&htdocument.Document{Layout: layout, Plugins: variable.Plugins{"base": {Name: "base", Func: base}}}, &htdata.TemplateData{}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output, []byte(`href="/plugin/post"`)) {
		t.Fatalf("output = %s", output)
	}
}
