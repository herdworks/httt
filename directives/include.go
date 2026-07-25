package directives

import (
	"bytes"
	"errors"
	"os"
	"slices"

	htdocument "github.com/herdworks/hypertemplates-go/document"
	"golang.org/x/net/html"
)

var (
	ErrInclude         = errors.New("ht-include error")
	ErrIncludeFragment = errors.New("ht-include fragment error")
)

var HyperIncludeAttributeStrings = []string{"data-ht-include", "ht-include"}

func ResolveIncludes(document *htdocument.Document) (err error) {
	for _, element := range HyperIncludeElements(document) {
		if err = Include(document, element); err != nil {
			return errors.Join(ErrInclude, err)
		}
	}
	if len(HyperIncludeElements(document)) > 0 {
		if err = ResolveIncludes(document); err != nil {
			return errors.Join(ErrInclude, err)
		}
	}
	return nil
}

func Include(document *htdocument.Document, element *html.Node) (err error) {
	var fragments []*html.Node
	path := htdocument.GetAttribute(element, HyperIncludeAttributeStrings...).Val
	if fragments, err = IncludeFragments(document, element, path); err != nil {
		return errors.Join(ErrInclude, err)
	}
	htdocument.ForwardAttributes(element, fragments, forwardedAttributeSkipKeys()...)
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

func HyperIncludeElements(document *htdocument.Document) []*html.Node {
	selectors := []string{"[data-ht-include]:not([data-ht-each] [data-ht-include], [data-ht-each][data-ht-include])"}
	if document.StrictMode {
		return htdocument.QueryAll(document.Layout, selectors...)
	}
	selectors[0] += ":not([ht-each] [data-ht-include], [ht-each][data-ht-include])"
	selectors = append(selectors, "[ht-include]:not([ht-each] [ht-include], [ht-each][ht-include]):not([data-ht-each] [ht-include], [data-ht-each][ht-include])")
	return htdocument.QueryAll(document.Layout, selectors...)
}

func forwardedAttributeSkipKeys() []string {
	keys := slices.Clone(HyperIncludeAttributeStrings)
	keys = append(keys, HyperBlockAttributeStrings...)
	return keys
}
