package rules_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/herdworks/hypertemplates-go/migrate"
	"github.com/herdworks/hypertemplates-go/migrate/rules"
)

func TestQueryMigratesStaticURLsAndIsIdempotent(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<a href="/posts?draft=0#top" ht-query="href?tag:page.tag,site.tag;href?page:page.number"></a>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Query{}}}).Run()
	if err != nil || report.Counters.Migrated != 1 || !report.Complete() {
		t.Fatalf("report = %#v, %v", report, err)
	}
	want := `<a href="/posts?draft=0&amp;tag=${ page.tag, site.tag, &#34;&#34; }&amp;page=${ page.number, &#34;&#34; }#top" ht-apply=""></a>`
	if got := readResource(t, resourcePath); got != want {
		t.Fatalf("resource = %q, want %q", got, want)
	}
	before := readResource(t, resourcePath)
	report, err = (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Query{}}}).Run()
	if err != nil || report.Counters.Migrated != 0 || readResource(t, resourcePath) != before {
		t.Fatalf("rerun = %#v, %v", report, err)
	}
}

func TestQueryWarnsForVariableURLAndReportsUnsafeURLs(t *testing.T) {
	t.Run("variable", func(t *testing.T) {
		_, themePath, resourcePath := writeThemeResource(t, `<a href="${ page.url }" data-ht-query="href?tag:page.tag"></a>`)
		var output bytes.Buffer
		report, err := (migrate.Migration{ThemePath: themePath, Interactive: true, Input: strings.NewReader("y\n"), Output: &output, Rules: []rules.Rule{rules.Query{}}}).Run()
		if err != nil || report.Counters.Warned != 1 || !report.Complete() || !strings.Contains(output.String(), "before:") || !strings.Contains(output.String(), "after:") {
			t.Fatalf("report = %#v, output = %q, err = %v", report, output.String(), err)
		}
		if got := readResource(t, resourcePath); !strings.Contains(got, `href="${ page.url }?tag=${ page.tag, &#34;&#34; }"`) || !strings.Contains(got, `data-ht-apply`) {
			t.Fatalf("resource = %q", got)
		}
	})
	t.Run("mixed", func(t *testing.T) {
		source := `<a href="/posts/${ page.id }" ht-query="href?tag:page.tag"></a>`
		_, themePath, resourcePath := writeThemeResource(t, source)
		report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Query{}}}).Run()
		if err != nil || report.Counters.Unresolved != 1 || readResource(t, resourcePath) != source {
			t.Fatalf("report = %#v, err = %v", report, err)
		}
	})
}
