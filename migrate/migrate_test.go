package migrate

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	migrationrules "github.com/herdworks/httt/migrate/rules"
	migrationtypes "github.com/herdworks/httt/migrate/types"
	"golang.org/x/net/html"
)

type Candidate = migrationtypes.Candidate
type AbortMigrationError = migrationtypes.AbortMigrationError
type ManualMigrationRequiredError = migrationtypes.ManualMigrationRequiredError
type MigrationWarning = migrationtypes.MigrationWarning
type Proposal = migrationtypes.Proposal
type Rule = migrationrules.Rule
type ResultStatus = migrationtypes.ResultStatus

const (
	ResultUnresolved = migrationtypes.ResultUnresolved
)

type testRule struct {
	selectors []string
	proposal  func(*Candidate) (*Proposal, error)
	apply     func(*Candidate) error
}

func (rule testRule) Selectors() []string {
	return rule.selectors
}

func (rule testRule) Proposal(candidate *Candidate) (*Proposal, error) {
	if rule.proposal != nil {
		return rule.proposal(candidate)
	}
	if rule.apply == nil {
		return nil, nil
	}
	if err := rule.apply(candidate); err != nil {
		return nil, err
	}
	return &Proposal{After: candidate.Element, Message: "test migration", AutoApply: true}, nil
}

func (rule testRule) Apply(candidate *Candidate) error {
	if rule.apply == nil {
		return nil
	}
	return rule.apply(candidate)
}

func TestNewJobLoadsResourceAndCandidates(t *testing.T) {
	root := t.TempDir()
	path := writeFile(t, root, "fragments/job.html", `<div ht-if="legacy"></div>`)
	rule := testRule{selectors: []string{"[ht-if]"}}
	file := migrationtypes.File{Path: "fragments/job.html", Kind: migrationtypes.FragmentFile}

	job, err := NewJob(root, file, rule)
	if err != nil {
		t.Fatal(err)
	}
	if job == nil || !reflect.DeepEqual(job.Rule, rule) || job.File != file || len(job.Nodes) != 1 || len(job.Candidates) != 1 || job.Modified {
		t.Fatalf("job = %#v", job)
	}
	if got, err := job.Path(); err != nil || got != path {
		t.Fatalf("path = %q, %v; want %q", got, err, path)
	}

	job, err = NewJob(root, file, testRule{selectors: []string{"[ht-not]"}})
	if err != nil {
		t.Fatal(err)
	}
	if job != nil {
		t.Fatalf("job = %#v, want nil for no candidates", job)
	}
}

func TestJobRunUsesOptionsAndCommitsModifiedDOM(t *testing.T) {
	root := t.TempDir()
	path := writeFile(t, root, "fragments/job.html", `<div ht-if="legacy"></div>`)
	rule := testRule{selectors: []string{"[ht-if]"}, apply: func(candidate *Candidate) error {
		removeCandidateAttribute(t, candidate)
		return nil
	}}
	job, err := NewJob(root, migrationtypes.File{Path: "fragments/job.html", Kind: migrationtypes.FragmentFile}, rule)
	if err != nil {
		t.Fatal(err)
	}
	if err := job.Run(JobOptions{Input: bufio.NewReader(strings.NewReader("")), Output: &bytes.Buffer{}}); err != nil {
		t.Fatal(err)
	}
	if !job.Modified || job.Report.Counters.Migrated != 1 {
		t.Fatalf("job = %#v", job)
	}
	if got := readFile(t, path); got != `<div></div>` {
		t.Fatalf("resource = %q", got)
	}
}

func TestApplyFailureDiscardsJobAndLaterRulesContinue(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	path := writeFile(t, root, "fragments/continue.html", `<div ht-if="legacy" ht-not="legacy"></div>`)
	failing := testRule{selectors: []string{"[ht-if]"}, apply: func(candidate *Candidate) error {
		removeCandidateAttributeNamed(t, candidate, "ht-if")
		return fmt.Errorf("apply failed")
	}}
	succeeding := testRule{selectors: []string{"[ht-not]"}, apply: func(candidate *Candidate) error {
		removeCandidateAttributeNamed(t, candidate, "ht-not")
		return nil
	}}

	report, err := (Migration{ThemePath: themePath, Rules: []Rule{failing, succeeding}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Failed != 1 || report.Counters.Migrated != 1 || report.Counters.FilesChanged != 1 {
		t.Fatalf("report = %#v", report)
	}
	if got := readFile(t, path); got != `<div ht-if="legacy"></div>` {
		t.Fatalf("resource = %q", got)
	}
}

func TestDeclinedProposalDoesNotCallApply(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	path := writeFile(t, root, "fragments/decline.html", `<div ht-if="legacy"></div>`)
	applyCalls := 0
	rule := testRule{
		selectors: []string{"[ht-if]"},
		proposal: func(candidate *Candidate) (*Proposal, error) {
			removeCandidateAttribute(t, candidate)
			return &Proposal{After: candidate.Element, Message: "remove legacy directive", AutoApply: true}, nil
		},
		apply: func(candidate *Candidate) error {
			applyCalls++
			removeCandidateAttribute(t, candidate)
			return nil
		},
	}

	report, err := (Migration{
		ThemePath:   themePath,
		Interactive: true,
		Input:       strings.NewReader("n\n"),
		Output:      &bytes.Buffer{},
		Rules:       []Rule{rule},
	}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if applyCalls != 0 || report.Counters.Declined != 1 || report.Counters.FilesChanged != 0 || len(report.Results) != 0 {
		t.Fatalf("apply calls = %d, report = %#v", applyCalls, report)
	}
	if got := readFile(t, path); got != `<div ht-if="legacy"></div>` {
		t.Fatalf("resource changed to %q", got)
	}
}

func TestMigrationWarningKeepsAppliedProposal(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	path := writeFile(t, root, "fragments/warning.html", `<div ht-if="legacy"></div>`)
	rule := testRule{
		selectors: []string{"[ht-if]"},
		proposal: func(candidate *Candidate) (*Proposal, error) {
			removeCandidateAttribute(t, candidate)
			return &Proposal{After: candidate.Element, Message: "remove legacy directive", AutoApply: true}, nil
		},
		apply: func(candidate *Candidate) error {
			removeCandidateAttribute(t, candidate)
			return &MigrationWarning{Message: "verify migrated semantics"}
		},
	}

	report, err := (Migration{ThemePath: themePath, Rules: []Rule{rule}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Warned != 1 || report.Counters.FilesChanged != 1 || !report.Complete() {
		t.Fatalf("report = %#v", report)
	}
	if got := readFile(t, path); got != `<div></div>` {
		t.Fatalf("resource = %q", got)
	}
}

func TestAbortMigrationErrorDiscardsCurrentPass(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	path := writeFile(t, root, "fragments/abort.html", `<div ht-if="one"></div><p ht-if="two"></p>`)
	applyCalls := 0
	rule := testRule{
		selectors: []string{"[ht-if]"},
		proposal: func(candidate *Candidate) (*Proposal, error) {
			removeCandidateAttribute(t, candidate)
			return &Proposal{After: candidate.Element, Message: "remove legacy directive", AutoApply: true}, nil
		},
		apply: func(candidate *Candidate) error {
			applyCalls++
			removeCandidateAttribute(t, candidate)
			if applyCalls == 2 {
				return &AbortMigrationError{Err: fmt.Errorf("stop test migration")}
			}
			return nil
		},
	}

	report, err := (Migration{ThemePath: themePath, Rules: []Rule{rule}}).Run()
	var abort *AbortMigrationError
	if !errors.As(err, &abort) {
		t.Fatalf("error = %v, want AbortMigrationError", err)
	}
	if report.Counters.Migrated != 0 || report.Counters.FilesChanged != 0 || len(report.Results) != 0 {
		t.Fatalf("report retained current-pass outcomes: %#v", report)
	}
	if got := readFile(t, path); got != `<div ht-if="one"></div><p ht-if="two"></p>` {
		t.Fatalf("resource changed to %q", got)
	}
}

func TestApplyIsAuthoritative(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	path := writeFile(t, root, "fragments/mismatch.html", `<div ht-if="legacy"></div>`)
	rule := testRule{
		selectors: []string{"[ht-if]"},
		proposal: func(candidate *Candidate) (*Proposal, error) {
			removeCandidateAttribute(t, candidate)
			return &Proposal{After: candidate.Element, Message: "remove legacy directive", AutoApply: true}, nil
		},
		apply: func(candidate *Candidate) error {
			removeCandidateAttribute(t, candidate)
			candidate.Element.Attr = append(candidate.Element.Attr, html.Attribute{Key: "unexpected"})
			return nil
		},
	}

	report, err := (Migration{ThemePath: themePath, Rules: []Rule{rule}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 || report.Counters.Failed != 0 || report.Counters.FilesChanged != 1 {
		t.Fatalf("report = %#v", report)
	}
	if got := readFile(t, path); got != `<div unexpected=""></div>` {
		t.Fatalf("resource changed to %q", got)
	}
}

func TestApplyErrorDiscardsJob(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	path := writeFile(t, root, "fragments/error.html", `<div ht-if="legacy"></div>`)
	rule := testRule{
		selectors: []string{"[ht-if]"},
		proposal: func(candidate *Candidate) (*Proposal, error) {
			removeCandidateAttribute(t, candidate)
			return &Proposal{After: candidate.Element, Message: "remove legacy directive", AutoApply: true}, nil
		},
		apply: func(candidate *Candidate) error {
			removeCandidateAttribute(t, candidate)
			return fmt.Errorf("apply failed")
		},
	}

	report, err := (Migration{ThemePath: themePath, Rules: []Rule{rule}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Failed != 1 || report.Counters.FilesChanged != 0 {
		t.Fatalf("report = %#v", report)
	}
	if got := readFile(t, path); got != `<div ht-if="legacy"></div>` {
		t.Fatalf("resource changed to %q", got)
	}
}

func TestMigrationProcessesRulesAcrossLayoutsThenFragments(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	writeFile(t, root, "layouts/b.html", `<html><body><b ht-not="b"></b><b ht-if="b"></b></body></html>`)
	writeFile(t, root, "layouts/a.html", `<html><body><a ht-not="a"></a><a ht-if="a"></a></body></html>`)
	writeFile(t, root, "blocks/z.html", `<z ht-if="z"></z>`)
	writeFile(t, root, "fragments/m.html", `<m ht-if="m"></m>`)

	manual := func(*Candidate) (*Proposal, error) {
		return nil, &ManualMigrationRequiredError{Message: "test manual migration"}
	}
	report, err := (Migration{ThemePath: themePath, Rules: []Rule{
		testRule{selectors: []string{"[ht-if]"}, proposal: manual},
		testRule{selectors: []string{"[ht-not]"}, proposal: manual},
	}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, item := range report.Results {
		if item.Status == ResultUnresolved && (item.Selector == "[ht-if]" || item.Selector == "[ht-not]") {
			got = append(got, item.Selector+" "+item.File)
		}
	}
	want := []string{
		"[ht-if] layouts/a.html",
		"[ht-if] layouts/b.html",
		"[ht-if] blocks/z.html",
		"[ht-if] fragments/m.html",
		"[ht-not] layouts/a.html",
		"[ht-not] layouts/b.html",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("processing order = %#v, want %#v", got, want)
	}
	if report.Counters.FilesScanned != 4 {
		t.Fatalf("files scanned = %d, want 4", report.Counters.FilesScanned)
	}
}

func TestMigrationUsesCSSSelectorsAndElementCandidates(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	writeFile(t, root, "fragments/selectors.html", `<section><p ht-if ht-not></p></section><p ht-if></p>`)
	rule := testRule{
		selectors: []string{"section > p[ht-if]", "[ht-not]"},
		proposal: func(*Candidate) (*Proposal, error) {
			return nil, &ManualMigrationRequiredError{Message: "manual migration required"}
		},
	}

	report, err := (Migration{ThemePath: themePath, Rules: []Rule{rule}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Unresolved != 1 || len(report.Results) != 1 {
		t.Fatalf("report = %#v", report)
	}
	if got, want := report.Results[0].Selector, "section > p[ht-if], [ht-not]"; got != want {
		t.Fatalf("selector = %q, want %q", got, want)
	}
}

func TestMigrationRejectsInvalidCSSSelector(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	_, err := (Migration{
		ThemePath: themePath,
		Rules:     []Rule{testRule{selectors: []string{"["}}},
	}).Run()
	if err == nil || !strings.Contains(err.Error(), "invalid migration selector") {
		t.Fatalf("error = %v", err)
	}
}

func TestDiscoverUsesConfiguredDirectoriesAndLayoutPrecedence(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{"config":{"layouts_dir":"templates","blocks_dir":"components","fragments_dir":"templates/fragments"}}`)
	writeFile(t, root, "templates/layout.html", `<html></html>`)
	writeFile(t, root, "templates/fragments/overlap.html", `<div></div>`)
	writeFile(t, root, "components/block.html", `<div></div>`)
	writeFile(t, root, "components/ignored.HTML", `<div></div>`)
	writeFile(t, root, "components/ignored.html.bak", `<div></div>`)

	theme, err := loadTheme(themePath)
	if err != nil {
		t.Fatal(err)
	}
	registry, err := migrationtypes.DiscoverFiles(theme)
	if err != nil {
		t.Fatal(err)
	}
	var layouts []string
	for _, resource := range registry.Layouts {
		layouts = append(layouts, resource.Path)
	}
	var fragments []string
	for _, resource := range registry.Fragments {
		fragments = append(fragments, resource.Path)
	}
	if want := []string{"templates/fragments/overlap.html", "templates/layout.html"}; !reflect.DeepEqual(layouts, want) {
		t.Fatalf("layouts = %#v, want %#v", layouts, want)
	}
	if want := []string{"components/block.html"}; !reflect.DeepEqual(fragments, want) {
		t.Fatalf("fragments = %#v, want %#v", fragments, want)
	}
}

func TestLoadThemeUsesParseAndValidate(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{"name":"Loaded","config":{"layouts_dir":"templates"}}`)
	theme, err := loadTheme(themePath)
	if err != nil {
		t.Fatal(err)
	}
	if theme.Name != "Loaded" || theme.LayoutsDirName() != "templates" || theme.URI.Path != themePath {
		t.Fatalf("theme = %#v", theme)
	}
	invalidPath := writeFile(t, root, "invalid.json", `{"config":{"layouts_dir":"../outside"}}`)
	if _, err := loadTheme(invalidPath); err == nil || !strings.Contains(err.Error(), "validate theme configuration") {
		t.Fatalf("invalid config error = %v", err)
	}
	malformedPath := writeFile(t, root, "malformed.json", `{`)
	if _, err := loadTheme(malformedPath); err == nil || !strings.Contains(err.Error(), "parse theme configuration") {
		t.Fatalf("malformed config error = %v", err)
	}
}

func TestMigrationWritesEachRuleFilePassForLaterRules(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	path := writeFile(t, root, "fragments/passes.html", `<div ht-if="one" ht-not="two"></div>`)
	first := testRule{selectors: []string{"[ht-if]"}, apply: func(candidate *Candidate) error {
		removeCandidateAttribute(t, candidate)
		candidate.Element.Attr = append(candidate.Element.Attr, html.Attribute{Key: "data-first", Val: "yes"})
		return nil
	}}
	second := testRule{selectors: []string{"[ht-not]"}, apply: func(candidate *Candidate) error {
		if !hasAttribute(candidate.Element, "data-first") {
			return fmt.Errorf("earlier pass was not visible")
		}
		removeCandidateAttribute(t, candidate)
		return nil
	}}

	report, err := (Migration{ThemePath: themePath, Rules: []Rule{first, second}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 2 || report.Counters.FilesChanged != 1 {
		t.Fatalf("counters = %#v", report.Counters)
	}
	if got, want := readFile(t, path), `<div data-first="yes"></div>`; got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}
}

func TestCurrentAndManualProposalsDoNotRewrite(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	currentPath := writeFile(t, root, "fragments/current.html", `<div ht-if="current"></div>`)
	manualPath := writeFile(t, root, "fragments/manual.html", `<div ht-not="manual"></div>`)
	rules := []Rule{
		testRule{selectors: []string{"[ht-if]"}},
		testRule{selectors: []string{"[ht-not]"}, proposal: func(*Candidate) (*Proposal, error) {
			return nil, &ManualMigrationRequiredError{Message: "manual migration required"}
		}},
	}

	report, err := (Migration{ThemePath: themePath, Rules: rules}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Unresolved != 1 || report.Counters.FilesChanged != 0 || len(report.Results) != 1 {
		t.Fatalf("report = %#v", report)
	}
	if readFile(t, currentPath) != `<div ht-if="current"></div>` || readFile(t, manualPath) != `<div ht-not="manual"></div>` {
		t.Fatal("non-proposal outcomes rewrote source")
	}
}

func TestDebugMigrationReportsLegacyDataModesWithoutMutatingCurrentSyntax(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	source := `<div ht-debug="data"></div><p data-ht-debug="element,data"></p><a ht-debug="element"></a><b data-ht-debug="${ page.title }"></b>`
	path := writeFile(t, root, "fragments/debug.html", source)

	report, err := (Migration{
		ThemePath: themePath,
		Rules:     []Rule{migrationrules.Debug{}},
	}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Unresolved != 2 || report.Counters.OptionalFound != 0 || report.Counters.FilesChanged != 0 {
		t.Fatalf("counters = %#v", report.Counters)
	}
	if len(report.Results) != 2 {
		t.Fatalf("results = %#v", report.Results)
	}
	if got, want := readFile(t, path), source; got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}
}

func TestCandidateReplacesFragmentRoot(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	path := writeFile(t, root, "fragments/root.html", `<old ht-if="legacy"><span></span></old>`)
	rule := testRule{selectors: []string{"[ht-if]"}, apply: func(candidate *Candidate) error {
		replacement := &html.Node{Type: html.ElementNode, Data: "new"}
		if err := candidate.ReplaceElement(replacement); err != nil {
			return err
		}
		return nil
	}}

	if _, err := (Migration{ThemePath: themePath, Rules: []Rule{rule}}).Run(); err != nil {
		t.Fatal(err)
	}
	if got, want := readFile(t, path), `<new></new>`; got != want {
		t.Fatalf("source = %q, want %q", got, want)
	}
}

func TestOutputDirectoryCopiesCompleteThemeAndMigratesCopy(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	writeFile(t, root, "fragments/header.html", `<header ht-if="legacy"></header>`)
	cssPath := writeFile(t, root, "static/site.css", `body { color: red; }`)
	if err := os.Chmod(cssPath, 0o751); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "data/site.json", `{"title":"Test"}`)
	writeFile(t, root, "types/post.json", `{"name":"Post"}`)
	writeFile(t, root, "layouts/default.html", `<html><body></body></html>`)
	writeFile(t, root, "blocks/card.html", `<article></article>`)
	writeFile(t, root, "plugins/format.js", `export default function format() {}`)
	outputRoot := filepath.Join(t.TempDir(), "migrated")
	rule := testRule{selectors: []string{"[ht-if]"}, apply: func(candidate *Candidate) error {
		removeCandidateAttribute(t, candidate)
		return nil
	}}

	report, err := (Migration{ThemePath: themePath, OutputDir: outputRoot, Rules: []Rule{rule}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if report.Counters.Migrated != 1 {
		t.Fatalf("report = %#v", report)
	}
	if got := readFile(t, filepath.Join(root, "fragments/header.html")); got != `<header ht-if="legacy"></header>` {
		t.Fatalf("source theme changed: %q", got)
	}
	if got, want := readFile(t, filepath.Join(outputRoot, "fragments/header.html")), `<header></header>`; got != want {
		t.Fatalf("output fragment = %q, want %q", got, want)
	}
	for _, relative := range []string{"theme.json", "static/site.css", "data/site.json", "types/post.json", "layouts/default.html", "blocks/card.html", "plugins/format.js"} {
		if _, err := os.Stat(filepath.Join(outputRoot, relative)); err != nil {
			t.Fatalf("copied file %s: %v", relative, err)
		}
	}
	info, err := os.Stat(filepath.Join(outputRoot, "static/site.css"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o751 {
		t.Fatalf("copied mode = %v", info.Mode().Perm())
	}
}

func TestOutputDirectoryMustBeEmptyAndDisjoint(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	t.Run("non-empty", func(t *testing.T) {
		output := t.TempDir()
		writeFile(t, output, "existing", "x")
		if _, err := (Migration{ThemePath: themePath, OutputDir: output}).Run(); err == nil || !strings.Contains(err.Error(), "not empty") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("nested", func(t *testing.T) {
		if _, err := (Migration{ThemePath: themePath, OutputDir: filepath.Join(root, "output")}).Run(); err == nil || !strings.Contains(err.Error(), "disjoint") {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("symlink", func(t *testing.T) {
		target := t.TempDir()
		link := filepath.Join(t.TempDir(), "output")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		if _, err := (Migration{ThemePath: themePath, OutputDir: link}).Run(); err == nil || !strings.Contains(err.Error(), "is a symlink") {
			t.Fatalf("error = %v", err)
		}
	})
}

func TestOutputDirectoryCopiesSourceSymlinkContents(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	outside := writeFile(t, t.TempDir(), "outside.css", "external")
	if err := os.MkdirAll(filepath.Join(root, "static"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "static/link.css")); err != nil {
		t.Fatal(err)
	}
	outputRoot := filepath.Join(t.TempDir(), "migrated")

	report, err := (Migration{ThemePath: themePath, OutputDir: outputRoot, Rules: []Rule{}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if !report.Complete() || report.Counters.Unresolved != 0 {
		t.Fatalf("report = %#v", report)
	}
	outputPath := filepath.Join(outputRoot, "static/link.css")
	if got := readFile(t, outputPath); got != "external" {
		t.Fatalf("copied symlink contents = %q", got)
	}
	if info, err := os.Lstat(outputPath); err != nil || info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("output path is not a regular copy: info=%v err=%v", info, err)
	}
}

func TestInPlaceMigrationFollowsSourceFileSymlink(t *testing.T) {
	root := t.TempDir()
	themePath := writeTheme(t, root, `{}`)
	outside := writeFile(t, t.TempDir(), "outside.html", `<div ht-if="hidden"></div>`)
	if err := os.MkdirAll(filepath.Join(root, "fragments"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "fragments/link.html")); err != nil {
		t.Fatal(err)
	}

	rule := testRule{selectors: []string{"[ht-if]"}, apply: func(candidate *Candidate) error {
		removeCandidateAttribute(t, candidate)
		return nil
	}}
	report, err := (Migration{ThemePath: themePath, Rules: []Rule{rule}}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if !report.Complete() || report.Counters.Migrated != 1 || report.Counters.FilesChanged != 1 {
		t.Fatalf("report = %#v", report)
	}
	if got := readFile(t, outside); got != `<div></div>` {
		t.Fatalf("symlink target = %q", got)
	}
}

func removeCandidateAttribute(t *testing.T, candidate *Candidate) {
	t.Helper()
	attribute := candidateDirectiveAttribute(t, candidate)
	for index := range candidate.Element.Attr {
		if &candidate.Element.Attr[index] != attribute {
			continue
		}
		candidate.Element.Attr = append(candidate.Element.Attr[:index], candidate.Element.Attr[index+1:]...)
		return
	}
	t.Fatal("candidate directive attribute not found")
}

func removeCandidateAttributeNamed(t *testing.T, candidate *Candidate, name string) {
	t.Helper()
	if candidate == nil || candidate.Element == nil {
		t.Fatal("candidate element is nil")
	}
	for index, attribute := range candidate.Element.Attr {
		if strings.EqualFold(attribute.Key, name) {
			candidate.Element.Attr = append(candidate.Element.Attr[:index], candidate.Element.Attr[index+1:]...)
			return
		}
	}
	t.Fatalf("candidate attribute %q not found", name)
}

func candidateDirectiveAttribute(t *testing.T, candidate *Candidate) *html.Attribute {
	t.Helper()
	if candidate == nil || candidate.Element == nil {
		t.Fatal("candidate element is nil")
	}
	for index := range candidate.Element.Attr {
		name := strings.ToLower(candidate.Element.Attr[index].Key)
		if strings.HasPrefix(name, "ht-") || strings.HasPrefix(name, "data-ht-") {
			return &candidate.Element.Attr[index]
		}
	}
	t.Fatal("candidate directive attribute not found")
	return nil
}

func hasAttribute(element *html.Node, name string) bool {
	for _, attribute := range element.Attr {
		if attribute.Key == name {
			return true
		}
	}
	return false
}

func writeTheme(t *testing.T, root, body string) string {
	t.Helper()
	return writeFile(t, root, "theme.json", body)
}

func writeFile(t *testing.T, root, relative, body string) string {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
		t.Fatal(err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(contents)
}
