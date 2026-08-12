package rules_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/herdworks/httt/migrate"
	"github.com/herdworks/httt/migrate/rules"
)

func TestTemplateMigratesLoopsAndPreservesAliasFamily(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<article data-ht-template="post:posts,archive.posts;page:page;site:site"></article>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Template{}}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 || !report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got, want := readResource(t, resourcePath), `<article data-ht-each="post in ${ posts, archive.posts }"></article>`; got != want {
		t.Fatalf("resource = %q, want %q", got, want)
	}
	second, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Template{}}}).Run()
	if err != nil || second.Counters.Migrated != 0 {
		t.Fatalf("rerun = %#v, %v", second, err)
	}
}

func TestTemplateMigratesIterationMetadataInRenderedDescendants(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<article ht-template="post:posts" title="${ ht.index }"><span ht-apply>${ ht.count }/${ ht.total }:${ post.title }</span></article>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Template{}}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 || !report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	got := readResource(t, resourcePath)
	for _, want := range []string{`ht-each="post in ${ posts }"`, `${ each.index }`, `${ each.count }/${ each.total }:${ post.title }`} {
		if !strings.Contains(got, want) {
			t.Fatalf("migrated resource missing %q: %s", want, got)
		}
	}
}

func TestTemplateMigratesSemicolonContextMappings(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<article ht-template="page:page.feed.pages; site:site"></article>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Template{}}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 || !report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got, want := readResource(t, resourcePath), `<article ht-each="page in ${ page.feed.pages }"></article>`; got != want {
		t.Fatalf("resource = %q, want %q", got, want)
	}
}

func TestTemplateLeavesRealAliasesUnresolved(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<article ht-template="post:posts;website:site"></article>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Template{}}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Unresolved != 1 || report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got := readResource(t, resourcePath); !strings.Contains(got, "ht-template") {
		t.Fatalf("unexpected resource %q", got)
	}
}

func TestTemplateLeavesSameElementEachUnresolved(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<article ht-each="post in ${ posts }" ht-template="item:items"></article>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Template{}}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Unresolved != 1 || report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got := readResource(t, resourcePath); !strings.Contains(got, "ht-template") || !strings.Contains(got, "ht-each") {
		t.Fatalf("unexpected resource %q", got)
	}
}

func TestTemplateMigratesNestedEachWithoutWarning(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<section ht-template="post:posts"><article ht-each="item in ${ items }"></article></section>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Template{}}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 || report.Counters.Warned != 0 || !report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got := readResource(t, resourcePath); !strings.Contains(got, `ht-each="post in ${ posts }"`) {
		t.Fatalf("unexpected resource %q", got)
	}
}

func TestTemplateMigrationShowsInteractiveProposal(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<article data-ht-template="post:posts"></article>`)
	var output bytes.Buffer
	report, err := (migrate.Migration{
		ThemePath:   themePath,
		Interactive: true,
		Input:       strings.NewReader("y\n"),
		Output:      &output,
		Rules:       []rules.Rule{rules.Template{}},
	}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 || !report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got := output.String(); !strings.Contains(got, "before:") || !strings.Contains(got, "after:") || !strings.Contains(got, "data-ht-template") || !strings.Contains(got, "data-ht-each") {
		t.Fatalf("interactive output = %q", got)
	}
	if got := readResource(t, resourcePath); !strings.Contains(got, "data-ht-each") {
		t.Fatalf("unexpected resource %q", got)
	}
}
