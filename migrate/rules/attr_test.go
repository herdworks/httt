package rules_test

import (
	"strings"
	"testing"

	"github.com/herdworks/hypertemplates-go/migrate"
	"github.com/herdworks/hypertemplates-go/migrate/rules"
)

func TestAttrMigratesTargetedDeclarations(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "ordered sources and empty literal fallback",
			source: `<a href="" ht-attr="href:page.url, site.url"></a>`,
			want:   `<a href="${ page.url, site.url, &#34;&#34; }" ht-apply=""></a>`,
		},
		{
			name:   "data alias family",
			source: `<a data-ht-attr="href:page.url"></a>`,
			want:   `<a href="${ page.url }" data-ht-apply=""></a>`,
		},
		{
			name:   "first source controls mixed alias family",
			source: `<a ht-attr="href:page.url" data-ht-attr="title:page.title"></a>`,
			want:   `<a href="${ page.url }" title="${ page.title }" ht-apply=""></a>`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, themePath, resourcePath := writeThemeResource(t, test.source)
			report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Attr{}}}).Run()
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

func TestAttrRequiresManualMigrationForUnsafeDeclarations(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{name: "targetless map expansion", source: `<a ht-attr="page.attributes"></a>`},
		{name: "existing template variable", source: `<a href="${ page.previous }" ht-attr="href:page.url"></a>`},
		{name: "existing get action", source: `<a href="${ get(page.previous, site.previous) }" ht-attr="href:page.url"></a>`},
		{name: "mixed template target", source: `<a href="/users/${ page.id }" ht-attr="href:page.url"></a>`},
		{name: "duplicate target", source: `<a ht-attr="href:page.url; href:site.url"></a>`},
		{name: "conditional declaration", source: `<a ht-attr="href:page.url==published"></a>`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, themePath, resourcePath := writeThemeResource(t, test.source)
			report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Attr{}}}).Run()
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

func TestAttrsReportsLegacyPluralSyntax(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<a ht-attrs="href:page.url"></a>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Attrs{}}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Unresolved != 1 || report.Counters.FilesChanged != 0 || report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got := readResource(t, resourcePath); !strings.Contains(got, `ht-attrs="href:page.url"`) {
		t.Fatalf("resource changed to %q", got)
	}
}
