// Package check validates rendered HTML for remaining template artifacts.
package check

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/net/html"

	"github.com/herdworks/httt/directives"
	"github.com/herdworks/httt/files"
)

// Document checks one parsed HTML file.
type Document struct {
	path            string
	root            *html.Node
	skipRules       []SkipRule
	knownAttributes map[string]bool
	findings        []Finding
}

// Options configures rendered-output validation. A nil SkipRules slice uses
// DefaultSkipRules; a non-nil empty slice disables all default skips.
type Options struct {
	SkipRules []SkipRule
	Tidy      bool
	Writer    io.Writer
}

// SkipRule controls which parts of matching HTML nodes are not checked.
// Attributes names attribute values to skip; Text skips direct text-node
// children. A matching non-element node is skipped entirely.
type SkipRule struct {
	Kind       html.NodeType
	Name       string
	Attributes []string
	Text       bool
}

// Finding identifies one template artifact and its owning rendered element.
type Finding struct {
	Path    string
	Element string
}

// Result is the completed outcome of checking one HTML file.
type Result struct {
	Path     string
	Findings []Finding
	Err      error
}

func (document *Document) check() error {
	return document.walk(document.root)
}

func (document *Document) walk(node *html.Node) error {
	if node == nil || document.skipNode(node) {
		return nil
	}
	if node.Type == html.ElementNode {
		if err := document.checkAttributes(node); err != nil {
			return err
		}
	}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.TextNode && (document.skipText(node) || document.skipNode(child)) {
			continue
		}
		if child.Type == html.TextNode {
			if err := document.checkValue(child.Data, node); err != nil {
				return err
			}
			continue
		}
		if err := document.walk(child); err != nil {
			return err
		}
	}
	return nil
}

func (document *Document) checkAttributes(element *html.Node) error {
	for _, attribute := range element.Attr {
		if document.knownAttributes[strings.ToLower(attribute.Key)] {
			if err := document.addFinding(element); err != nil {
				return err
			}
		}
		if document.skipAttribute(element, attribute.Key) {
			continue
		}
		if err := document.checkValue(attribute.Val, element); err != nil {
			return err
		}
	}
	return nil
}

func (document *Document) checkValue(value string, element *html.Node) error {
	for marker := strings.Index(value, "${"); marker >= 0; {
		if err := document.addFinding(element); err != nil {
			return err
		}
		next := marker + 2
		following := strings.Index(value[next:], "${")
		if following < 0 {
			return nil
		}
		marker = next + following
	}
	return nil
}

func (document *Document) addFinding(element *html.Node) error {
	var output bytes.Buffer
	if err := html.Render(&output, element); err != nil {
		return err
	}
	document.findings = append(document.findings, Finding{
		Path:    document.path,
		Element: output.String(),
	})
	return nil
}

func (document *Document) skipNode(node *html.Node) bool {
	if node.Type == html.ElementNode {
		return false
	}
	for _, rule := range document.skipRules {
		if matchesSkipRule(rule, node) {
			return true
		}
	}
	return false
}

func (document *Document) skipAttribute(element *html.Node, attribute string) bool {
	for _, rule := range document.skipRules {
		if !matchesSkipRule(rule, element) {
			continue
		}
		for _, skipped := range rule.Attributes {
			if strings.EqualFold(attribute, skipped) {
				return true
			}
		}
	}
	return false
}

func (document *Document) skipText(element *html.Node) bool {
	for _, rule := range document.skipRules {
		if rule.Text && matchesSkipRule(rule, element) {
			return true
		}
	}
	return false
}

// Error formats the finding as a complete source-file diagnostic.
func (finding Finding) Error() string {
	return fmt.Sprintf("file %q error:\n\n%s", finding.Path, finding.Element)
}

// DefaultSkipRules returns the standard literal-content exclusions.
func DefaultSkipRules() []SkipRule {
	return []SkipRule{
		{Kind: html.CommentNode},
		{Kind: html.ElementNode, Name: "pre", Text: true},
		{Kind: html.ElementNode, Name: "code", Text: true},
	}
}

// WriteResult writes one completed file status and any associated findings.
func WriteResult(writer io.Writer, result Result) error {
	if result.Err != nil {
		_, err := fmt.Fprintf(writer, "file %q error:\n\n%s\n\n", result.Path, result.Err)
		return err
	}
	if len(result.Findings) == 0 {
		_, err := fmt.Fprintf(writer, "file %q ok\n", result.Path)
		return err
	}
	if _, err := fmt.Fprintf(writer, "file %q errors:\n\n", result.Path); err != nil {
		return err
	}
	for _, finding := range result.Findings {
		if _, err := fmt.Fprintf(writer, "%s\n\n", finding.Element); err != nil {
			return err
		}
	}
	return nil
}

// Check scans every selected HTML file concurrently and returns each completed
// result in completion order. Per-file failures are recorded in Result.Err.
func Check(path string, options Options) ([]Result, error) {
	paths, err := paths(path)
	if err != nil {
		return nil, err
	}
	results := make(chan Result)
	var group sync.WaitGroup
	for _, path := range paths {
		group.Add(1)
		go func() {
			defer group.Done()
			findings, err := file(path, options)
			results <- Result{Path: path, Findings: findings, Err: err}
		}()
	}
	go func() {
		group.Wait()
		close(results)
	}()

	var completed []Result
	var writeErr error
	for result := range results {
		completed = append(completed, result)
		if options.Writer != nil {
			if err := WriteResult(options.Writer, result); err != nil && writeErr == nil {
				writeErr = err
			}
		}
	}
	return completed, writeErr
}

func paths(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		if !info.Mode().IsRegular() || filepath.Ext(path) != ".html" {
			return nil, fmt.Errorf("check requires an HTML file or directory, got %q", path)
		}
		return []string{path}, nil
	}

	relativePaths, err := files.List(path, true)
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(relativePaths))
	for _, relativePath := range relativePaths {
		if filepath.Ext(relativePath) == ".html" {
			paths = append(paths, filepath.Join(path, filepath.FromSlash(relativePath)))
		}
	}
	return paths, nil
}

func file(path string, options Options) ([]Finding, error) {
	input, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer input.Close()
	return htmlFile(path, input, options)
}

func htmlFile(path string, source io.Reader, options Options) ([]Finding, error) {
	root, err := html.Parse(source)
	if err != nil {
		return nil, fmt.Errorf("parse %q: %w", path, err)
	}
	if options.SkipRules == nil {
		options.SkipRules = DefaultSkipRules()
	}
	document := Document{
		path:            path,
		root:            root,
		skipRules:       options.SkipRules,
		knownAttributes: directiveAttributes(options.Tidy),
	}
	if err := document.check(); err != nil {
		return nil, err
	}
	return document.findings, nil
}

func directiveAttributes(tidy bool) map[string]bool {
	if !tidy {
		return nil
	}
	attributes := map[string]bool{}
	for _, attribute := range directives.AttributeNames(false, true) {
		attributes[attribute] = true
	}
	return attributes
}

func matchesSkipRule(rule SkipRule, node *html.Node) bool {
	return node.Type == rule.Kind && (rule.Name == "" || strings.EqualFold(node.Data, rule.Name))
}
