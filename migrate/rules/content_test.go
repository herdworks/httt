package rules_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/herdworks/httt/migrate"
	"github.com/herdworks/httt/migrate/rules"
)

func TestContentMigratesLegacyContent(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "default ordered keypaths and text fallback",
			source: `<title class="card" ht-content="page.title, site.title">Placeholder</title>`,
			want:   `<title class="card" ht-apply="">${ page.title, site.title, &#34;Placeholder&#34; }</title>`,
		},
		{
			name:   "html empty fallback",
			source: `<article ht-content="html:page.body"></article>`,
			want:   `<article ht-apply="">${ page.body, &#34;&#34; }</article>`,
		},
		{
			name:   "markdown data alias and multiline fallback",
			source: "<section data-ht-content=\"markdown:page.summary, post.summary\">First\nSecond</section>",
			want:   `<section data-ht-apply="">${ markdown(page.summary, post.summary, &#34;First\nSecond&#34;) }</section>`,
		},
		{
			name:   "existing apply marker is retained",
			source: `<p ht-apply ht-content="page.title">Fallback</p>`,
			want:   `<p ht-apply="">${ page.title, &#34;Fallback&#34; }</p>`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, themePath, resourcePath := writeThemeResource(t, test.source)
			report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Content{}}}).Run()
			if err != nil {
				t.Fatal(err)
			}
			if report.Counters.Migrated != 1 || report.Counters.Unresolved != 0 || !report.Complete() {
				t.Fatalf("report = %#v", report)
			}
			if got := readResource(t, resourcePath); got != test.want {
				t.Fatalf("resource = %q, want %q", got, test.want)
			}
		})
	}
}

func TestContentRequiresManualMigrationForUnsafeLegacyShapes(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "duplicate aliases", source: `<div ht-content="page.title" data-ht-content="site.title"></div>`},
		{name: "multiple declarations", source: `<div ht-content="page.title;markdown:page.summary"></div>`},
		{name: "unsupported mode", source: `<div ht-content="json:page.body"></div>`},
		{name: "condition", source: `<div ht-content="page.title==Published"></div>`},
		{name: "empty source", source: `<div ht-content></div>`},
		{name: "empty fallback keypath", source: `<div ht-content="page.title,"></div>`},
		{name: "child element", source: `<div ht-content="page.title"><em>Fallback</em></div>`},
		{name: "comment child", source: `<div ht-content="page.title"><!-- Fallback --></div>`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, themePath, resourcePath := writeThemeResource(t, test.source)
			report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Content{}}}).Run()
			if err != nil {
				t.Fatal(err)
			}
			if report.Counters.Unresolved != 1 || report.Counters.FilesChanged != 0 || report.Complete() {
				t.Fatalf("report = %#v", report)
			}
			if got := readResource(t, resourcePath); got != test.source {
				t.Fatalf("resource changed to %q", got)
			}
		})
	}
}

func TestContentMigrationIsIdempotentAndShowsInteractiveProposal(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<article data-ht-content="page.title">Fallback</article>`)
	var output bytes.Buffer
	report, err := (migrate.Migration{
		ThemePath:   themePath,
		Interactive: true,
		Input:       strings.NewReader("y\n"),
		Output:      &output,
		Rules:       []rules.Rule{rules.Content{}},
	}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 || !report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got := output.String(); !strings.Contains(got, "before:") || !strings.Contains(got, "after:") || !strings.Contains(got, `data-ht-content`) || !strings.Contains(got, `data-ht-apply`) {
		t.Fatalf("interactive output = %q", got)
	}
	before := readResource(t, resourcePath)
	second, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Content{}}}).Run()
	if err != nil || second.Counters.Migrated != 0 || !second.Complete() {
		t.Fatalf("rerun = %#v, %v", second, err)
	}
	if got := readResource(t, resourcePath); got != before {
		t.Fatalf("idempotent migration changed resource from %q to %q", before, got)
	}
}
