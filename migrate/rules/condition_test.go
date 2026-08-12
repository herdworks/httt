package rules_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/herdworks/httt/migrate"
	"github.com/herdworks/httt/migrate/rules"
)

func TestNotMigratesPresenceComparisonAndAliases(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{
			name:   "presence uses De Morgan repeated attributes",
			source: `<div ht-not="page.draft, site.draft"></div>`,
			want:   `<div ht-if="!${ page.draft }" ht-if="!${ site.draft }"></div>`,
		},
		{
			name:   "comparison inversion",
			source: `<div ht-not="page.kind==post,draft"></div>`,
			want:   `<div ht-if="${ page.kind } != &#34;post&#34;" ht-if="${ page.kind } != &#34;draft&#34;"></div>`,
		},
		{
			name:   "colon style is legacy equality",
			source: `<div ht-not="visible:page.draft"></div>`,
			want:   `<div ht-if="${ visible } != &#34;page.draft&#34;"></div>`,
		},
		{
			name:   "not equal and regex inversion",
			source: `<div data-ht-not="page.kind!=draft;page.path~=/private/"></div>`,
			want:   `<div data-ht-if="${ page.kind } == &#34;draft&#34;" data-ht-if="${ page.path } !~ &#34;/private/&#34;"></div>`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, themePath, resourcePath := writeThemeResource(t, test.source)
			report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Not{}}}).Run()
			if err != nil {
				t.Fatal(err)
			}
			if report.Counters.Migrated != 1 || !report.Complete() {
				t.Fatalf("report = %#v", report)
			}
			if got := readResource(t, resourcePath); got != test.want {
				t.Fatalf("resource = %q, want %q", got, test.want)
			}
		})
	}
}

func TestConditionalMigrationCombinesMixedGatesInSourceOrder(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<div class="x" ht-if="page.title" data-ht-not="page.draft"></div>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Not{}}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 || !report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got, want := readResource(t, resourcePath), `<div class="x" ht-if="page.title" data-ht-if="!${ page.draft }"></div>`; got != want {
		t.Fatalf("mixed resource = %q, want %q", got, want)
	}
}

func TestConditionalMigrationPreservesRepeatedHtNotAndMixedGateOrder(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<div ht-not="page.first" ht-if="${ page.second }" data-ht-not="page.third"></div>`)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Not{}}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 || !report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got, want := readResource(t, resourcePath), `<div ht-if="!${ page.first }" ht-if="${ page.second }" data-ht-if="!${ page.third }"></div>`; got != want {
		t.Fatalf("migrated resource = %q, want %q", got, want)
	}
}

func TestConditionalMigrationIsIdempotent(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<div data-ht-not="page.kind==post,note"></div>`)
	first, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Not{}}}).Run()
	if err != nil || first.Counters.Migrated != 1 || !first.Complete() {
		t.Fatalf("first report = %#v, err=%v", first, err)
	}
	if got, want := readResource(t, resourcePath), `<div data-ht-if="${ page.kind } != &#34;post&#34;" data-ht-if="${ page.kind } != &#34;note&#34;"></div>`; got != want {
		t.Fatalf("migrated resource = %q, want %q", got, want)
	}
	before := readResource(t, resourcePath)
	second, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Not{}}}).Run()
	if err != nil || second.Counters.Migrated != 0 || second.Counters.FilesChanged != 0 || !second.Complete() {
		t.Fatalf("second report = %#v, err=%v", second, err)
	}
	if after := readResource(t, resourcePath); after != before {
		t.Fatalf("idempotent migration changed resource from %q to %q", before, after)
	}
}

func TestConditionalMigrationDoesNotSelectHtIf(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<div data-ht-if="page.title"></div>`)
	report, err := (migrate.Migration{ThemePath: themePath}).Run()
	if err != nil || report.Counters.Migrated != 0 || report.Counters.FilesChanged != 0 || !report.Complete() {
		t.Fatalf("report = %#v, err=%v", report, err)
	}
	if got, want := readResource(t, resourcePath), `<div data-ht-if="page.title"></div>`; got != want {
		t.Fatalf("resource = %q, want %q", got, want)
	}
}

func TestConditionalMigrationInteractiveProposalShowsCompleteElement(t *testing.T) {
	_, themePath, resourcePath := writeThemeResource(t, `<article class="card" ht-not="page.draft"><span>Title</span></article>`)
	var output bytes.Buffer
	report, err := (migrate.Migration{
		ThemePath:   themePath,
		Interactive: true,
		Input:       strings.NewReader("n\n"),
		Output:      &output,
		Rules:       []rules.Rule{rules.Not{}},
	}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Declined != 1 || report.Complete() || report.Counters.FilesChanged != 0 {
		t.Fatalf("report = %#v", report)
	}
	if !strings.Contains(output.String(), "before") || !strings.Contains(output.String(), "after") || !strings.Contains(output.String(), `<article class="card" ht-not="page.draft"><span>Title</span></article>`) || !strings.Contains(output.String(), `!${ page.draft }`) {
		t.Fatalf("interactive output = %q", output.String())
	}
	if got := readResource(t, resourcePath); got != `<article class="card" ht-not="page.draft"><span>Title</span></article>` {
		t.Fatalf("declined resource = %q", got)
	}
}
