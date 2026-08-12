package rules_test

import (
	"testing"

	"github.com/herdworks/httt/migrate"
	"github.com/herdworks/httt/migrate/rules"
)

func TestPipeReportsEveryPrePullSyntaxForManualMigration(t *testing.T) {
	for _, source := range []string{
		`<style ht-pipe="css:style#components"></style>`,
		`<div data-ht-pipe="main"></div>`,
		`<style ht-pipe='to "style#components" as css'></style>`,
	} {
		_, themePath, resourcePath := writeThemeResource(t, source)
		report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Pipe{}}}).Run()
		if err != nil || report.Counters.Unresolved != 1 || readResource(t, resourcePath) != source {
			t.Fatalf("source = %q; report = %#v, err = %v", source, report, err)
		}
	}
}

func TestPipeSkipsCurrentPullSyntax(t *testing.T) {
	source := `<style ht-pipe='from "style.component" as css'></style>`
	_, themePath, resourcePath := writeThemeResource(t, source)
	report, err := (migrate.Migration{ThemePath: themePath, Rules: []rules.Rule{rules.Pipe{}}}).Run()
	if err != nil || report.Counters.Unresolved != 0 || !report.Complete() || readResource(t, resourcePath) != source {
		t.Fatalf("report = %#v, err = %v", report, err)
	}
}
