package check

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

type failingWriter struct{}

var errWriter = errors.New("write failed")

func (failingWriter) Write([]byte) (int, error) {
	return 0, errWriter
}

func TestHTMLFile(t *testing.T) {
	tests := []struct {
		name    string
		source  string
		options Options
		want    []string
	}{
		{
			name:   "all element text and attributes",
			source: "<html><head><title>${ title }</title></head><body><a href='${ page.url }' rel='example'>Click <span>${ label }</span></a><script>`${ script }`</script><style>${ style }</style><template>${ template }</template></body></html>",
			want: []string{
				findingString("example.html", "<title>${ title }</title>"),
				findingString("example.html", `<a href="${ page.url }" rel="example">Click <span>${ label }</span></a>`),
				findingString("example.html", `<span>${ label }</span>`),
				findingString("example.html", "<script>`${ script }`</script>"),
				findingString("example.html", `<style>${ style }</style>`),
				findingString("example.html", `<template>${ template }</template>`),
			},
		},
		{
			name:   "comments are ignored",
			source: "<!-- ${ comment } --><p>clean</p>",
		},
		{
			name:    "literal element text is ignored but attributes are checked",
			source:  "<pre ht-apply href='${ pre.href }'>${ pre.text }</pre><code data-ht-debug='element' data-value='${ code.value }'>${ code.text }</code>",
			options: Options{Tidy: true},
			want: []string{
				findingString("example.html", `<pre ht-apply="" href="${ pre.href }">${ pre.text }</pre>`),
				findingString("example.html", `<pre ht-apply="" href="${ pre.href }">${ pre.text }</pre>`),
				findingString("example.html", `<code data-ht-debug="element" data-value="${ code.value }">${ code.text }</code>`),
				findingString("example.html", `<code data-ht-debug="element" data-value="${ code.value }">${ code.text }</code>`),
			},
		},
		{
			name:   "nested elements in literal content are checked",
			source: "<code><span>${ nested }</span></code>",
			want: []string{
				findingString("example.html", `<span>${ nested }</span>`),
			},
		},
		{
			name:    "tidy directives are independent",
			source:  "<main ht-apply data-ht-param='${ title }'>${ title }</main>",
			options: Options{Tidy: true},
			want: []string{
				findingString("example.html", `<main ht-apply="" data-ht-param="${ title }">${ title }</main>`),
				findingString("example.html", `<main ht-apply="" data-ht-param="${ title }">${ title }</main>`),
				findingString("example.html", `<main ht-apply="" data-ht-param="${ title }">${ title }</main>`),
				findingString("example.html", `<main ht-apply="" data-ht-param="${ title }">${ title }</main>`),
			},
		},
		{
			name:   "custom skip rule",
			source: "<a href='${ link }'>${ label }</a>",
			options: Options{SkipRules: []SkipRule{
				{Kind: html.ElementNode, Name: "a", Attributes: []string{"href"}, Text: true},
			}},
		},
		{
			name:   "text node skip rule",
			source: "<p>${ label }</p><a href='${ link }'>clean</a>",
			options: Options{SkipRules: []SkipRule{
				{Kind: html.TextNode},
			}},
			want: []string{
				findingString("example.html", `<a href="${ link }">clean</a>`),
			},
		},
		{
			name:   "document node skip rule",
			source: "<p>${ label }</p>",
			options: Options{SkipRules: []SkipRule{
				{Kind: html.DocumentNode},
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			findings, err := htmlFile("example.html", strings.NewReader(test.source), test.options)
			if err != nil {
				t.Fatal(err)
			}
			if got := findingStrings(findings); !slices.Equal(test.want, got) {
				t.Fatalf("findings = %#v, want %#v", got, test.want)
			}
		})
	}
}

func TestCheckDirectory(t *testing.T) {
	root := t.TempDir()
	index := filepath.Join(root, "index.html")
	nested := filepath.Join(root, "nested", "page.html")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, []byte("<p>${ page.title }</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("<p>clean</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "ignored.txt"), []byte("${ ignored }"), 0o644); err != nil {
		t.Fatal(err)
	}

	var output bytes.Buffer
	results, err := Check(root, Options{
		Writer: &output,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("results = %#v", results)
	}
	completed := resultPaths(results)
	slices.Sort(completed)
	if want := []string{index, nested}; !slices.Equal(completed, want) {
		t.Fatalf("completed = %#v, want %#v", completed, want)
	}
	var findings []Finding
	for _, result := range results {
		if result.Err != nil {
			t.Fatal(result.Err)
		}
		findings = append(findings, result.Findings...)
	}
	want := []string{findingString(index, `<p>${ page.title }</p>`)}
	if got := findingStrings(findings); !slices.Equal(want, got) {
		t.Fatalf("findings = %#v, want %#v", got, want)
	}
	if got := output.String(); !strings.Contains(got, `file "`+index+`" errors:`) || !strings.Contains(got, `file "`+nested+`" ok`) {
		t.Fatalf("output = %q", got)
	}

	if _, err = Check(filepath.Join(root, "ignored.txt"), Options{}); err == nil || !strings.Contains(err.Error(), "requires an HTML file or directory") {
		t.Fatalf("non-HTML check error = %v", err)
	}
}

func TestCheckContinuesAfterWriterError(t *testing.T) {
	root := t.TempDir()
	index := filepath.Join(root, "index.html")
	nested := filepath.Join(root, "nested", "page.html")
	if err := os.MkdirAll(filepath.Dir(nested), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(index, []byte("<p>${ page.title }</p>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nested, []byte("<p>clean</p>"), 0o644); err != nil {
		t.Fatal(err)
	}

	results, err := Check(root, Options{Writer: failingWriter{}})
	if !errors.Is(err, errWriter) {
		t.Fatalf("Check error = %v, want %v", err, errWriter)
	}
	completed := resultPaths(results)
	slices.Sort(completed)
	if want := []string{index, nested}; !slices.Equal(completed, want) {
		t.Fatalf("completed = %#v, want %#v", completed, want)
	}
}

func findingString(path, element string) string {
	return `file "` + path + `" error:` + "\n\n" + element
}

func findingStrings(findings []Finding) []string {
	values := make([]string, 0, len(findings))
	for _, finding := range findings {
		values = append(values, finding.Error())
	}
	return values
}

func resultPaths(results []Result) []string {
	paths := make([]string, 0, len(results))
	for _, result := range results {
		paths = append(paths, result.Path)
	}
	return paths
}
