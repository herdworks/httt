package directives

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"slices"

	htdata "github.com/herdworks/httt/data"
	htdocument "github.com/herdworks/httt/document"
	"golang.org/x/net/html"
)

// IncludeDirective replaces ht-include placeholders with theme fragments.
type IncludeDirective struct{}

var _ Directive = IncludeDirective{} // make the compiler check Directive interface conformance

var (
	ErrInclude         = errors.New("ht-include error")
	ErrIncludeFragment = errors.New("ht-include fragment error")
)

func (IncludeDirective) Attributes(strict bool) []string {
	attributes := []string{"data-ht-include"}
	if !strict {
		attributes = append(attributes, "ht-include")
	}
	return attributes
}

func (d IncludeDirective) Selectors(strict bool) []string {
	// Keep includes on scope roots so fragments expand before their nested
	// rendering pass; only descendants of ht-each and ht-with are deferred.
	attributes := d.Attributes(strict)
	selectors := make([]string, 0, len(attributes))
	boundaries := defaultBoundaryAttributes(strict)
	for _, attribute := range attributes {
		selectors = append(selectors, fmt.Sprintf("[%s]%s", attribute, selectorBoundaries(boundaries, false)))
	}
	return selectors
}

func (d IncludeDirective) Elements(document *htdocument.Document) []*html.Node {
	if document == nil {
		return nil
	}
	return htdocument.QueryAll(document.Layout, d.Selectors(document.StrictMode)...)
}

// NextElements returns the next include depth after the current batch has
// been replaced.
func (d IncludeDirective) NextElements(document *htdocument.Document) []*html.Node {
	return d.Elements(document)
}

func (d IncludeDirective) Process(document *htdocument.Document, element *html.Node, _ *htdata.TemplateData, _ *htdata.TemplateData, _ io.Writer) error {
	if element == nil || element.Parent == nil {
		return nil
	}
	attribute := htdocument.GetAttribute(element, d.Attributes(false)...)
	path := attribute.Val
	fragments, err := IncludeFragments(document, element, path)
	if err != nil {
		return fmt.Errorf("ht-include: error evaluating %q=%q:\n%s\n%w", attribute.Key, attribute.Val, htdocument.ElementString(element), errors.Join(ErrInclude, err))
	}
	htdocument.ForwardAttributes(element, fragments, (EachDirective{}).withAttributes(false), forwardedAttributeSkipKeys()...)
	htdocument.OuterHTML(element, fragments)
	return nil
}
func IncludeFragments(document *htdocument.Document, element *html.Node, path string) (fragments []*html.Node, err error) {
	var fragmentBytes []byte
	var found bool
	if fragmentBytes, found, err = document.Theme.ResourceBytes(path); err != nil {
		return nil, errors.Join(ErrIncludeFragment, err)
	}
	if !found {
		return nil, errors.Join(ErrIncludeFragment, &os.PathError{
			Op:   "open",
			Path: document.Theme.ResourcePath(path),
			Err:  os.ErrNotExist,
		})
	}
	if len(fragmentBytes) == 0 {
		return []*html.Node{}, nil
	}
	if fragments, err = html.ParseFragment(bytes.NewReader(fragmentBytes), element.Parent); err != nil {
		return nil, err
	}
	return fragments, nil
}

func IncludeFragment(document *htdocument.Document, element *html.Node, path string) (fragment *html.Node, err error) {
	var fragments []*html.Node
	if fragments, err = IncludeFragments(document, element, path); err != nil {
		return nil, err
	}
	if len(fragments) == 0 {
		return &html.Node{}, nil
	}
	for _, fragment := range fragments {
		if fragment.Type == html.ElementNode {
			return fragment, nil
		}
	}
	return fragments[0], nil
}

func forwardedAttributeSkipKeys() []string {
	keys := slices.Clone(IncludeDirective{}.Attributes(false))
	keys = append(keys, BlockDirective{}.Attributes(false)...)
	return keys
}
