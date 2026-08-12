// Package migrate applies stateless template transformations to configured
// theme resources. Idempotency is a critical migration contract: applying a
// completed migration job again must make no further changes. A rule that
// cannot meet that contract must be redesigned or omitted.
package migrate

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"

	htdocument "github.com/herdworks/httt/document"
	migrationrules "github.com/herdworks/httt/migrate/rules"
	migrationtypes "github.com/herdworks/httt/migrate/types"
	httheme "github.com/herdworks/httt/theme"
)

// Migration describes one configured theme migration job.
type Migration struct {
	ThemePath   string
	OutputDir   string
	Interactive bool
	Input       io.Reader
	Output      io.Writer
	Rules       []migrationrules.Rule
	status      migrationtypes.Status
	changed     map[string]bool
}

// DefaultRules is the ordered migration rule set used when a job supplies no rules.
var DefaultRules = []migrationrules.Rule{
	migrationrules.Not{},
	migrationrules.Template{},
	migrationrules.Attr{},
	migrationrules.Attrs{},
	migrationrules.Param{},
	migrationrules.Content{},
	migrationrules.Query{},
	migrationrules.Pipe{},
	migrationrules.Debug{},
}

// ErrInterrupted is returned when an interactive migration is quit or reaches EOF.
var ErrInterrupted = errors.New("migration interrupted")

// Run performs the configured theme migration.
func (migration Migration) Run() (migrationtypes.Status, error) {
	if migration.Input == nil {
		migration.Input = os.Stdin
	}
	if migration.Output == nil {
		migration.Output = os.Stdout
	}
	if migration.Rules == nil {
		migration.Rules = DefaultRules
	}
	migration.status = migrationtypes.Status{}
	migration.changed = map[string]bool{}
	stdin := bufio.NewReader(migration.Input)

	theme, err := loadTheme(migration.ThemePath)
	if err != nil {
		return migration.status, err
	}
	theme, err = migration.prepareOutput(theme)
	if err != nil {
		return migration.status, err
	}
	registry, err := migrationtypes.DiscoverFiles(theme)
	if err != nil {
		return migration.status, err
	}
	migration.status.Counters.FilesScanned = len(registry.Layouts) + len(registry.Fragments)

	for _, rule := range migration.Rules {
		if err := migrationrules.Validate(rule); err != nil {
			return migration.status, err
		}
		for _, resource := range registry.Ordered() {
			job, err := NewJob(theme.RootDir(), resource, rule)
			if err != nil {
				migration.status.Add(migrationtypes.Result{Status: migrationtypes.ResultFailed, File: resource.Path, Message: err.Error()})
				continue
			}
			if job == nil {
				continue
			}
			err = job.Run(JobOptions{Interactive: migration.Interactive, Input: stdin, Output: migration.Output})
			if errors.Is(err, ErrInterrupted) {
				migration.status.Interrupted = true
				return migration.status, ErrInterrupted
			}
			if err != nil {
				return migration.status, err
			}
			migration.status.Merge(job.Report)
			if job.Modified && !migration.changed[resource.Path] {
				migration.changed[resource.Path] = true
				migration.status.Counters.FilesChanged++
			}
		}
	}
	return migration.status, nil
}

func readAnswer(stdin *bufio.Reader) (string, error) {
	line, err := stdin.ReadString('\n')
	if err != nil && len(line) == 0 {
		return "", err
	}
	return strings.TrimSpace(line), err
}

func migrationCandidates(nodes []*html.Node, resource migrationtypes.File, selectors []string) []*migrationtypes.Candidate {
	var candidates []*migrationtypes.Candidate
	seen := map[*html.Node]bool{}
	for _, root := range nodes {
		for _, element := range htdocument.QueryRoot(root, selectors...) {
			if seen[element] {
				continue
			}
			seen[element] = true
			candidates = append(candidates, &migrationtypes.Candidate{Element: element, File: resource.Path})
		}
	}
	return candidates
}

func loadTheme(rawPath string) (httheme.Theme, error) {
	var theme httheme.Theme
	if rawPath == "" {
		return theme, fmt.Errorf("theme configuration path is required")
	}
	themePath, err := filepath.Abs(rawPath)
	if err != nil {
		return theme, fmt.Errorf("resolve theme configuration %q: %w", rawPath, err)
	}
	info, err := os.Stat(themePath)
	if err != nil {
		return theme, fmt.Errorf("stat theme configuration %q: %w", themePath, err)
	}
	if !info.Mode().IsRegular() {
		return theme, fmt.Errorf("theme configuration %q is not a regular file", themePath)
	}
	theme.URI = &url.URL{Path: themePath}
	if err := theme.Parse(io.Discard); err != nil {
		return theme, fmt.Errorf("parse theme configuration %q: %w", themePath, err)
	}
	if err := theme.Validate(); err != nil {
		return theme, fmt.Errorf("validate theme configuration %q: %w", themePath, err)
	}
	return theme, nil
}
