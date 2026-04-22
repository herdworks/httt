package hypertemplates

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/andybalholm/cascadia"
	htformat "github.com/herdworks/hypertemplates-go/format"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Template
//
// A Template is an HTML document.
// Templates may contain templating instructions as HTML5 data attributes.
type Template struct {
	Layout         *html.Node
	Theme          *Theme
	StrictMode     bool // enforce HTML5 custom attribute syntax (data-ht-*)
	TidyMode       bool
	MarkdownConfig *HyperMarkConfig

	// Internal state
	BlockMode bool
}

// TODO: add strict mode toggle (only process data-attributes)
func (template *Template) Process(data *TemplateData, userData *TemplateData) (htmlBytes []byte, err error) {
	if template.Layout == nil {
		return nil, errors.New("template layout empty, nothing to do") // guard
	}
	if template.MarkdownConfig == nil {
		template.MarkdownConfig = &DefaultHyperMarkConfig // TODO: is this still needed?
	}

	// process includes
	if err = template.ResolveIncludes(); err != nil {
		return nil, err
	}

	// process layout template data
	data.Set("layout", template.LayoutData())

	// process conditionals and debuggers, then render template
	for _, element := range template.HyperIfElements() {
		if err = template.EvaluateElement(element, data, true); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperNotElements() {
		if err = template.EvaluateElement(element, data, false); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperDebugElements() {
		if err = template.Debug(element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperTemplateElements() {
		if err = template.RenderTemplate(element, data, userData); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperAttrsElements() {
		if err = template.RenderAttributes(element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperQueryElements() {
		if err = template.RenderQueryParams(element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperParamsElements() {
		if err = template.RenderParams(element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperFormatElements() {
		if err = template.RenderFormatting(element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperContentElements() {
		if err = template.RenderContent(element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperBlockElements() {
		if err = template.RenderBlock(element, userData); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperPipeElements() {
		if err = template.RenderPipe(element); err != nil {
			return nil, err
		}
	}
	for _, element := range template.HyperBaseElements() {
		if err = template.RenderBasedElement(element, data); err != nil {
			return nil, err
		}
	}
	return template.HTML(template.Layout), nil
}

func (template *Template) ResolveIncludes() (err error) {
	for _, element := range template.HyperIncludeElements() {
		if err = template.Include(element); err != nil {
			return errors.Join(ErrInclude, err)
		}
	}
	if len(template.HyperIncludeElements()) > 0 {
		if err = template.ResolveIncludes(); err != nil {
			return errors.Join(ErrInclude, err) // RECURSION!
		}
	}
	return nil
}
func (template *Template) Include(element *html.Node) (err error) {
	var fragments []*html.Node
	path := getAttributeString(element, HyperIncludeAttributeStrings...)
	if fragments, err = template.IncludeFragments(element, path); err != nil {
		return errors.Join(ErrInclude, err)
	}
	template.ForwardAttributes(element, fragments...)
	outerHTML(element, fragments)
	return nil
}
func (template *Template) IncludeFragments(element *html.Node, path string) (fragments []*html.Node, err error) {
	var includePath string = template.Theme.ResourcePath(path)
	var fragmentBytes []byte
	if fragmentBytes, err = os.ReadFile(includePath); err != nil {
		return nil, errors.Join(ErrIncludeFragment, err)
	}
	if len(fragmentBytes) == 0 {
		return []*html.Node{}, nil // TODO: throw an error if fragment fragmentBytes is empty?
	}
	if fragments, err = html.ParseFragment(bytes.NewReader(fragmentBytes), element.Parent); err != nil {
		return nil, err
	}
	return fragments, nil
}
func (template *Template) IncludeFragment(element *html.Node, path string) (fragment *html.Node, err error) {
	var fragments []*html.Node
	if fragments, err = template.IncludeFragments(element, path); err != nil {
		return nil, err
	}
	if len(fragments) == 0 {
		return &html.Node{}, nil // TODO: throw an error if no fragments found?
	}
	for _, fragment := range fragments {
		if fragment.Type == html.ElementNode {
			return fragment, nil // return the first element node found in the fragment
		}
	}
	// TODO: select the first element from fragments that matches the target element tag name?
	return fragments[0], nil // fallback to the first element, which could be a comment node
}

func (template *Template) EvaluateElement(element *html.Node, data *TemplateData, inclusive bool) (err error) {
	paramString := getAttributeString(element, HyperConditionalAttributeStrings...)
	for _, param := range template.ParseParameters(paramString, data) {
		if param.Evaluate() == inclusive {
			removeAttributes(element, "hidden") // remove "hidden" attribute if set
		} else {
			setAttribute(element, "hidden", "true") // hide conditional content
			if template.TidyMode {
				removeElement(element) // remove the element (if possible)
			}
		}
	}
	if template.TidyMode {
		removeAttributes(element, HyperConditionalAttributeStrings...) // cleanup
	}
	return nil
}

func (template *Template) RenderTemplate(element *html.Node, data *TemplateData, userData *TemplateData) (err error) {
	var params Parameters
	var offset int = getAttributeInt(element, HyperTemplateOffsetAttributeStrings...)
	var limit int = getAttributeInt(element, HyperTemplateLimitAttributeStrings...)
	paramString := getAttributeString(element, HyperTemplateAttributeStrings...)
	if params = template.ParseParameters(paramString, data); len(params) == 0 {
		return nil // TODO: do we still need to handle tidy_mode for empty ht-template attributes?
	}
	for index, value := range params[0].ValueSet() {
		index += 1 // one-indexed
		if index <= offset || (limit > 0 && index > offset+limit) {
			continue // skip items outside of the offset/limit range
		}
		count := len(params[0].ValueSet())
		layoutTemplate := cloneElement(element)                            // element becomes the new/nested layout template
		removeAttributes(layoutTemplate, HyperTemplateAttributeStrings...) // remove the ht-template attribute from the new layout template
		nestedData := &TemplateData{}                                      // instantiate a new/scoped TemplateData object for the new layout template
		nestedData.Set(params[0].Target, value)
		for _, p := range params[1:] {
			nestedData.Set(p.Target, p.Value()) // set additional TemplateData values
		}
		nestedData.SetKeyValue("ht.index", index) // set template meta
		nestedData.SetKeyValue("ht.count", count) // set template meta
		nestedTemplate := Template{Layout: layoutTemplate, Theme: template.Theme, TidyMode: template.TidyMode, MarkdownConfig: template.MarkdownConfig}

		// RECURSION — forward userData unchanged
		if _, err = nestedTemplate.Process(nestedData, userData); err != nil {
			return err
		}
		if layoutTemplate.Type == html.CommentNode {
			continue // skip comment nodes
		}
		if layoutTemplate.Type == html.ElementNode {
			// NOTE: when InsertBefore argument #2 (oldChild) is nil, InsertBefore effectively becomes InsertAfter
			element.Parent.InsertBefore(layoutTemplate, element)
		}
	}
	setAttribute(element, "hidden", "true") // hide placeholder template content
	removeAttributes(element, HyperTemplateOffsetAttributeStrings...)
	removeAttributes(element, HyperTemplateLimitAttributeStrings...)
	if template.TidyMode {
		removeElement(element) // cleanup
	}
	return nil
}

func (template *Template) RenderAttributes(element *html.Node, data *TemplateData) (err error) {
	paramString := getAttributeString(element, HyperAttrsAttributeStrings...)
	for _, param := range getAttributeStrings(element, HyperAttrAttributeStrings...) {
		paramString = strings.TrimSpace(paramString) + fmt.Sprintf("; %s", param)
	}
	for _, param := range template.ParseParameters(paramString, data) {
		rawValue := param.Value()
		if rawValue == nil {
			continue // noop when no matching values found
		}
		switch value := rawValue.(type) {
		case TemplateData:
			for k, v := range value {
				setAttribute(element, k, fmt.Sprintf("%v", v))
			}
		case map[string]any:
			for k, v := range value {
				setAttribute(element, k, fmt.Sprintf("%v", v))
			}
		default:
			setAttribute(element, param.Target, fmt.Sprintf("%v", value))
		}
		if template.TidyMode {
			removeAttributes(element, HyperAttrsAttributeStrings...) // cleanup
			removeAttributes(element, HyperAttrAttributeStrings...)  // cleanup
		}
	}
	return nil
}

func (template *Template) RenderBasedElement(element *html.Node, data *TemplateData) (err error) {
	paramString := getAttributeString(element, HyperBaseAttributeStrings...)
	for _, param := range template.ParseParameters(paramString, data) {
		if len(param.Values) == 0 {
			continue // noop when no matching values found
		}
		removeAttributes(element, HyperBaseAttributeStrings...)
		var baseURL *url.URL
		var baseHref string = fmt.Sprintf("%s", param.Value())
		if !(strings.HasSuffix(baseHref, "/") || strings.HasSuffix(baseHref, "index.html")) {
			baseHref += "/" // ensure trailing slash
		}
		if baseURL, err = url.Parse(baseHref); err != nil {
			return nil
		}
		if template.ResolveBasedAttributes(element, param.Target, baseURL) != nil {
			return err
		}
	}
	return nil
}

func (template *Template) ResolveBasedAttributes(element *html.Node, attribute string, baseURL *url.URL) (err error) {
	debug := hasAttribute(element, HyperDebugAttributeStrings...)
	selectors := fmt.Sprintf("[%[1]s]:not([data-ht-template] [%[1]s], [data-ht-template][%[1]s], [data-ht-base] [%[1]s], [data-ht-base][%[1]s])", attribute)
	if !template.StrictMode {
		selectors += fmt.Sprintf(":not([ht-template] [%[1]s], [ht-template][%[1]s], [ht-base] [%[1]s], [ht-base][%[1]s])", attribute)
	}
	for _, el := range QueryRoot(element, selectors) {
		var resolvedHref string
		targetHref := getAttributeString(el, attribute)
		if debug {
			fmt.Printf("base %s='%s' resolving\n", attribute, targetHref)
		}
		if strings.HasPrefix(targetHref, "data:") {
			continue // skip data URIs
		}
		if template.IsAbsoluteHref(targetHref) {
			continue // skip absolute hrefs
		}
		targetURL := baseURL.ResolveReference(&url.URL{Path: targetHref}) // resolve relative hrefs
		if resolvedHref, err = url.PathUnescape(targetURL.String()); err != nil {
			return err // unescape the URL
		}
		if debug {
			fmt.Printf("base %[1]s='%[2]s' => %[1]s='%[3]s' resolved\n", attribute, targetHref, resolvedHref) // debug output
		}
		setAttribute(el, attribute, resolvedHref)
	}
	return nil
}

func (template *Template) RenderFormatting(element *html.Node, data *TemplateData) (err error) {
	if element == nil {
		return nil
	}
	if element.Type != html.ElementNode {
		return nil
	}

	for index, attribute := range element.Attr {
		if slices.Contains(HyperFormatAttributeStrings, attribute.Key) {
			continue
		}
		if !strings.Contains(attribute.Val, "${") {
			continue
		}

		if element.Attr[index].Val, err = htformat.Render("ht-format", attribute.Val, data, nil); err != nil {
			return err
		}
	}

	for child := element.FirstChild; child != nil; child = child.NextSibling {
		if child.Type != html.TextNode {
			continue
		}
		if !strings.Contains(child.Data, "${") {
			continue
		}

		if child.Data, err = htformat.Render("ht-format", child.Data, data, nil); err != nil {
			return err
		}
	}

	if template.TidyMode {
		removeAttributes(element, HyperFormatAttributeStrings...)
	}

	return nil
}

func (template *Template) RenderContent(element *html.Node, data *TemplateData) (err error) {
	paramString := getAttributeString(element, HyperContentAttributeStrings...)
	for _, param := range template.ParseParameters(paramString, data) {
		if len(param.Values) == 0 {
			continue // noop when no matching values found
		}
		switch param.Target {
		case "html":
			htmlString := fmt.Sprintf("%v", param.Value())
			fragment, err := html.ParseFragment(strings.NewReader(htmlString), element)
			if err != nil {
				return err
			}
			innerHTML(element, fragment)
		case "markdown":
			md := HyperMark{}.New(template.MarkdownConfig)
			content := fmt.Sprintf("%v", param.Value())
			if err := md.Parse([]byte(content)); err != nil {
				return err
			}
			fragment, err := html.ParseFragment(strings.NewReader(md.Content), element)
			if err != nil {
				return err
			}
			// TODO: markdown render hooks?
			// See:
			//   https://gohugo.io/render-hooks/
			//   https://github.com/gohugoio/hugo/blob/master/markup/goldmark/render_hooks.go
			innerHTML(element, fragment)
		default:
			innerText(element, fmt.Sprintf("%v", param.Value()))
		}
	}
	if template.TidyMode {
		removeAttributes(element, HyperContentAttributeStrings...) // cleanup
	}
	return nil
}

func (template *Template) RenderParams(element *html.Node, data *TemplateData) (err error) {
	var fragment []*html.Node
	var fragmentDiv *html.Node = &html.Node{Type: html.ElementNode, Data: "div", DataAtom: atom.Div}
	var paramString string = getAttributeString(element, HyperParamsAttributeStrings...)
	outerText(element, getAttributeString(element, "data-default", "default")) // set the default value, if provided
	for _, param := range template.ParseParameters(paramString, data) {
		if len(param.Values) == 0 {
			continue // guard
		}
		if len(fmt.Sprintf("%v", param.Value())) == 0 {
			continue // noop when no matching values found
		}
		switch param.Target {
		case "html":
			htmlString := fmt.Sprintf("%v", param.Value())
			if fragment, err = html.ParseFragment(strings.NewReader(htmlString), fragmentDiv); err != nil {
				return err
			}
			outerHTML(element, fragment)
		case "markdown":
			md := HyperMark{}.New(template.MarkdownConfig)
			mdString := fmt.Sprintf("%v", param.Value())
			if err := md.Parse([]byte(mdString)); err != nil {
				return err
			}
			if fragment, err = html.ParseFragment(strings.NewReader(md.Content), fragmentDiv); err != nil {
				return err
			}
			outerHTML(element, fragment)
		default:
			outerText(element, fmt.Sprintf("%v", param.Value()))
		}
	}
	return nil
}

func (template *Template) RenderBlock(element *html.Node, userData *TemplateData) (err error) {
	var fragment *html.Node
	var fragmentRef string = filepath.Join(template.Theme.Config.BlocksDirectory, element.Data)
	var fragmentData TemplateData
	var fragmentNodes []*html.Node
	if userData != nil {
		fragmentData = userData.Clone() // seed from incoming data so block templates inherit caller context
	} else {
		fragmentData = TemplateData{}
	}
	if fragment, err = template.IncludeFragment(element, fragmentRef); err != nil {
		return err
	}
	template.ForwardAttributes(element, fragment)
	fragmentData.Set("block", template.AttributeData(element))
	fragmentNodes = childNodes(element)
	if len(fragmentNodes) > 0 {
		// pass nested block content as the template data "block.content" property, so that
		// block templates can insert wrapped content into block layouts.
		fragmentData.SetKeyValue("block.content", string(template.HTML(fragmentNodes...)))
	}
	shortcode := Template{Layout: fragment, Theme: template.Theme, TidyMode: template.TidyMode, MarkdownConfig: template.MarkdownConfig, BlockMode: true}
	shortcode.Process(&fragmentData, userData) // RECURSION!
	element.Parent.InsertBefore(shortcode.Layout, element)
	if template.TidyMode {
		removeElement(element) // cleanup
	}
	return nil
}

func (template *Template) RenderPipe(element *html.Node) (err error) {
	var destination *html.Node
	paramString := getAttributeString(element, HyperPipeAttributeStrings...)
	for _, param := range template.ParseParameters(paramString, &TemplateData{}) {
		if destination = Query(template.Layout, param.Keys...); destination == nil {
			continue
		}
		element.Parent.RemoveChild(element) // remove the element from the DOM
		switch param.Target {
		case "text", "css", "js", "javascript":
			appendText(destination, template.innerText(element, true))
		default:
			destination.AppendChild(element) // pipe the element to the destination 😱
		}
	}
	if template.BlockMode {
		return nil // skip cleanup in block mode
	}
	if template.TidyMode {
		removeAttributes(element, HyperPipeAttributeStrings...) // cleanup
	}
	return nil
}

func (template *Template) RenderQueryParams(element *html.Node, data *TemplateData) (err error) {
	debug := hasAttribute(element, HyperDebugAttributeStrings...)
	paramString := getAttributeString(element, HyperQueryAttributeStrings...)
	for _, param := range template.ParseParameters(paramString, data) {
		var attributeName string
		var attributeQuery string
		var hasQuery bool
		if param.Value() == nil {
			continue // guard
		}
		if attributeName, attributeQuery, hasQuery = strings.Cut(param.Target, "?"); !hasQuery {
			continue // guard
		}
		if err = setAttributeQueryParam(element, attributeName, attributeQuery, fmt.Sprintf("%v", param.Value()), debug); err != nil {
			return err // set the query parameter
		}
		removeAttributes(element, HyperQueryAttributeStrings...) // cleanup
	}
	return nil
}

func (template *Template) Debug(element *html.Node, data *TemplateData) (err error) {
	var modes []string
	modesString := strings.ReplaceAll(getAttributeString(element, HyperDebugAttributeStrings...), " ", "") // remove spaces
	if len(modesString) > 0 {
		modes = strings.Split(modesString, ",")
	}
	if len(modes) == 0 {
		return nil // nothing to do, leave the ht-debug attribute in place
	}
	statement := fmt.Appendf([]byte{}, "\n===== ht-debug =====\n")
	if slices.Contains(modes, "element") {
		statement = fmt.Appendf(statement, "%s\n", template.HTML(element))
	}
	if slices.Contains(modes, "data") {
		dataBytes, _ := json.MarshalIndent(&data, "", "\t") // ignore errors
		statement = fmt.Appendf(statement, "%s\n", dataBytes)
	}
	statement = fmt.Appendf(statement, "====================\n")
	fmt.Println(string(statement)) // print debug statement to stdout
	removeAttributes(element, HyperDebugAttributeStrings...)
	return nil
}

// =============================================================================
// Helper types, methods, and functions
// =============================================================================

var HyperIncludeAttributeStrings = []string{"data-ht-include", "ht-include"}

func (template *Template) HyperIncludeElements() []*html.Node {
	selectors := []string{"[data-ht-include]:not([data-ht-template] [data-ht-include])"}
	if template.StrictMode {
		return QueryAll(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-include])"
	selectors = append(selectors, "[ht-include]:not([ht-template] [ht-include]):not([data-ht-template] [ht-include])")
	return QueryAll(template.Layout, selectors...)
}

var HyperConditionalAttributeStrings = []string{"data-ht-if", "ht-if", "data-ht-not", "ht-not"}
var HyperIfAttributeStrings = []string{"data-ht-if", "ht-if"}

func (template *Template) HyperIfElements() []*html.Node {
	selectors := []string{"[data-ht-if]:not([data-ht-template] [data-ht-if], [data-ht-template][data-ht-if])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-if], [ht-template][data-ht-if])"
	selectors = append(selectors, "[ht-if]:not([ht-template] [ht-if], [ht-template][ht-if]):not([data-ht-template] [ht-if], [data-ht-template][ht-if])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperNotAttributeStrings = []string{"data-ht-not", "ht-not"}

func (template *Template) HyperNotElements() []*html.Node {
	selectors := []string{"[data-ht-not]:not([data-ht-template] [data-ht-not], [data-ht-template][data-ht-not])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-not], [ht-template][data-ht-not])"
	selectors = append(selectors, "[ht-not]:not([ht-template] [ht-not], [ht-template][ht-not]):not([data-ht-template] [ht-not], [data-ht-template][ht-not])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperTemplateAttributeStrings = []string{"data-ht-template", "ht-template"}
var HyperTemplateOffsetAttributeStrings = []string{"data-ht-offset", "ht-offset"}
var HyperTemplateLimitAttributeStrings = []string{"data-ht-limit", "ht-limit"}

func (template *Template) HyperTemplateElements() []*html.Node {
	selectors := []string{"[data-ht-template]:not([data-ht-template] [data-ht-template])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-template])"
	selectors = append(selectors, "[ht-template]:not([ht-template] [ht-template]):not([data-ht-template] [ht-template])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperAttrAttributeStrings = []string{"data-ht-attr", "ht-attr"}
var HyperAttrsAttributeStrings = []string{"data-ht-attrs", "ht-attrs"}

func (template *Template) HyperAttrsElements() []*html.Node {
	selectors := []string{"[data-ht-attrs]:not([data-ht-template] [data-ht-attrs], [data-ht-template][data-ht-attrs])"}
	selectors = append(selectors, "[data-ht-attr]:not([data-ht-template] [data-ht-attr], [data-ht-template][data-ht-attr])")
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-attrs], [ht-template][data-ht-attrs])"
	selectors[1] += ":not([ht-template] [data-ht-attr], [ht-template][data-ht-attr])"
	selectors = append(selectors, "[ht-attrs]:not([ht-template] [ht-attrs], [ht-template][ht-attrs]):not([data-ht-template] [ht-attrs], [data-ht-template][ht-attrs])")
	selectors = append(selectors, "[ht-attr]:not([ht-template] [ht-attr], [ht-template][ht-attr]):not([data-ht-template] [ht-attr], [data-ht-template][ht-attr])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperFormatAttributeStrings = []string{"data-ht-format", "ht-format"}

func (template *Template) HyperFormatElements() []*html.Node {
	selectors := []string{"[data-ht-format]:not([data-ht-template] [data-ht-format], [data-ht-template][data-ht-format])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-format], [ht-template][data-ht-format])"
	selectors = append(selectors, "[ht-format]:not([ht-template] [ht-format], [ht-template][ht-format]):not([data-ht-template] [ht-format], [data-ht-template][ht-attrs])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperQueryAttributeStrings = []string{"data-ht-query", "ht-query"}

func (template *Template) HyperQueryElements() []*html.Node {
	selectors := []string{"[data-ht-query]:not([data-ht-template] [data-ht-query], [data-ht-template][data-ht-query])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-query], [ht-template][data-ht-query])"
	selectors = append(selectors, "[ht-query]:not([ht-template] [ht-query], [ht-template][ht-query]):not([data-ht-template] [ht-query], [data-ht-template][ht-attrs])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperBaseAttributeStrings = []string{"data-ht-base", "ht-base"}

func (template *Template) HyperBaseElements() []*html.Node {
	selectors := []string{"[data-ht-base]:not([data-ht-template] [data-ht-base], [data-ht-template][data-ht-base])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-base], [ht-template][data-ht-base])"
	selectors = append(selectors, "[ht-base]:not([ht-template] [ht-base], [ht-template][ht-base]):not([data-ht-template] [ht-base], [data-ht-template][ht-base])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperContentAttributeStrings = []string{"data-ht-content", "ht-content"}

func (template *Template) HyperContentElements() []*html.Node {
	selectors := []string{"[data-ht-content]:not([data-ht-template] [data-ht-content])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-content])"
	selectors = append(selectors, "[ht-content]:not([ht-template] [ht-content]):not([data-ht-template] [ht-content])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperParamsAttributeStrings = []string{"data-ht-param", "ht-param"}

func (template *Template) HyperParamsElements() []*html.Node {
	selectors := []string{"[data-ht-param]:not([data-ht-template] [data-ht-param])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-param])"
	selectors = append(selectors, "[ht-param]:not([ht-template] [ht-param]):not([data-ht-template] [ht-param])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperBlockAttributeStrings = []string{"data-ht-block", "ht-block"}

func (template *Template) HyperBlockElements() []*html.Node {
	selectors := []string{"[data-ht-block]"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors = append(selectors, "[ht-block]")
	return QueryRoot(template.Layout, selectors...)
}

var HyperPipeAttributeStrings = []string{"data-ht-pipe", "ht-pipe"}

func (template *Template) HyperPipeElements() []*html.Node {
	selectors := []string{"[data-ht-pipe]:not([data-ht-template] [data-ht-pipe])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-pipe])"
	selectors = append(selectors, "[ht-pipe]:not([ht-template] [ht-pipe]):not([data-ht-template] [ht-pipe])")
	return QueryRoot(template.Layout, selectors...)
}

var HyperDebugAttributeStrings = []string{"data-ht-debug", "ht-debug"}

func (template *Template) HyperDebugElements() []*html.Node {
	selectors := []string{"[data-ht-debug]:not([data-ht-template] [data-ht-debug], [data-ht-template][data-ht-debug])"}
	if template.StrictMode {
		return QueryRoot(template.Layout, selectors...)
	}
	selectors[0] += ":not([ht-template] [data-ht-debug], [ht-template][data-ht-debug])"
	selectors = append(selectors, "[ht-debug]:not([ht-template] [ht-debug], [ht-template][ht-debug]):not([data-ht-template] [ht-debug], [data-ht-template][ht-debug])")
	return QueryRoot(template.Layout, selectors...)
}

func (template *Template) HTML(elements ...*html.Node) []byte {
	if len(elements) == 0 {
		return []byte{} // guard
	}
	buffer := new(bytes.Buffer)
	for _, element := range elements {
		html.Render(buffer, element)
	}
	return buffer.Bytes()
}

func (template *Template) innerText(element *html.Node, unescape bool) (text string) {
	var innerText string
	if element.Type == html.TextNode {
		innerText = element.Data
	} else {
		innerText = string(template.HTML(childNodes(element)...))
	}
	if unescape {
		innerText = html.UnescapeString(innerText)
	}
	return innerText
}

func Query(node *html.Node, query ...string) (element *html.Node) {
	if node == nil {
		return nil // guard
	}
	for _, q := range query {
		sel, err := cascadia.Parse(q)
		if err != nil {
			return nil
		}
		element = cascadia.Query(node, sel)
		if element != nil {
			break
		}
	}
	return element
}

func QueryAll(node *html.Node, query ...string) (elements []*html.Node) {
	if node == nil {
		return []*html.Node{} // guard
	}
	elements = []*html.Node{}
	for _, q := range query {
		sel, err := cascadia.Parse(q)
		if err != nil {
			return []*html.Node{}
		}
		elements = append(elements, cascadia.QueryAll(node, sel)...)
	}
	return elements
}

func QueryRoot(node *html.Node, query ...string) (elements []*html.Node) {
	if node == nil {
		return []*html.Node{} // guard
	}
	elements = []*html.Node{}
	for _, q := range query {
		sel, err := cascadia.Parse(q)
		if err != nil {
			return []*html.Node{}
		}
		if sel.Match(node) {
			elements = append(elements, node)
		}
		elements = append(elements, cascadia.QueryAll(node, sel)...)
	}
	return elements
}

func (template *Template) ParseParameters(attr string, data *TemplateData) Parameters {
	params := Parameters{}
	if len(attr) == 0 {
		return params
	}
	for _, param := range strings.Split(attr, ";") {
		param = strings.TrimSpace(param)
		params = append(params, template.ParseParameter(param, data))
	}
	return params
}
func (template *Template) ParseParameter(attr string, data *TemplateData) *Parameter {
	param := &Parameter{}
	param.Parse(attr)
	param.Resolve(data)
	return param
}

func (template *Template) ForwardAttributes(placeholder *html.Node, fragments ...*html.Node) {
	var attributes []html.Attribute
	if len(fragments) == 0 {
		return // guard
	}
	for _, attr := range placeholder.Attr {
		if slices.Contains(HyperIncludeAttributeStrings, attr.Key) {
			continue // skip the ht-include attribute
		}
		if slices.Contains(HyperBlockAttributeStrings, attr.Key) {
			continue // skip the ht-block attribute
		}
		attributes = append(attributes, attr)
	}
	for _, fragment := range fragments {
		if fragment.Type == html.ErrorNode {
			continue // skip error nodes
		}
		if fragment.Type == html.TextNode {
			continue // skip text nodes
		}
		if fragment.Type == html.CommentNode {
			continue // skip comment nodes
		}
		if fragment.Type == html.DoctypeNode {
			continue // skip the doctype node
		}
		for _, attr := range attributes {
			setAttribute(fragment, attr.Key, attr.Val)
		}
		break // break after forwarding attribute to the first DocumentNode or ElementNode
	}
}

// AttributeData returns the HTML element attributes as TemplateData
func (template *Template) AttributeData(element *html.Node) (data TemplateData) {
	data = TemplateData{}
	for _, attr := range element.Attr {
		data[attr.Key] = attr.Val
	}
	return data
}

func (template *Template) LayoutData() (data TemplateData) {
	data = TemplateData{}
	for _, element := range QueryAll(template.Layout, "meta[name^='layout:']") {
		key := strings.Replace(getAttributeString(element, "name"), "layout:", "", -1)
		value := getAttributeString(element, "content")
		data.Set(key, value)
	}
	return data
}

func hasAttribute(element *html.Node, keys ...string) bool {
	if element.Type != html.ElementNode {
		return false
	}
	for _, attr := range element.Attr {
		if slices.Contains(keys, attr.Key) {
			return true
		}
	}
	return false
}

func getAttributeString(element *html.Node, keys ...string) (value string) {
	value = ""
	if element.Type != html.ElementNode {
		return value
	}
	for _, attr := range element.Attr {
		if slices.Contains(keys, attr.Key) {
			return attr.Val
		}
	}
	return value
}

func getAttributeStrings(element *html.Node, keys ...string) (values []string) {
	if element.Type != html.ElementNode {
		return values
	}
	for _, attr := range element.Attr {
		if slices.Contains(keys, attr.Key) {
			values = append(values, attr.Val)
		}
	}
	return values
}

func getAttributeInt(element *html.Node, keys ...string) (value int) {
	stringValue := getAttributeString(element, keys...)
	value, _ = strconv.Atoi(stringValue)
	return value
}

// TODO: refactor templating to handle multiple attribute values?! 💥
func getAttributes(element *html.Node, keys ...string) (values []string) {
	if element.Type != html.ElementNode {
		return values
	}
	for _, attr := range element.Attr {
		if !slices.Contains(keys, attr.Key) {
			continue
		}
		values = append(values, attr.Val)
	}
	return values
}

func setAttribute(element *html.Node, key string, value string) {
	if element.Type != html.ElementNode {
		return // guard
	}
	attributeIndex := -1
	for i, attr := range element.Attr {
		if attr.Key != key {
			continue // guard
		}
		attributeIndex = i
		break
	}
	if attributeIndex >= 0 {
		element.Attr[attributeIndex] = html.Attribute{Key: key, Val: value}
	} else {
		element.Attr = append(element.Attr, html.Attribute{Key: key, Val: value})
	}
}

func setAttributeQueryParam(element *html.Node, attribute string, queryParam string, value string, debug bool) (err error) {
	var attributeURL *url.URL
	attributeHref := getAttributeString(element, attribute)
	if attributeURL, err = url.Parse(attributeHref); err != nil {
		return err // parse the attribute URL
	}
	query := attributeURL.Query()          // get the query parameters
	query.Set(queryParam, value)           // set the query parameter
	attributeURL.RawQuery = query.Encode() // update URL with new query parameters
	if debug {
		fmt.Printf("urlquery %s => %s updating\n", attributeHref, attributeURL.String())
	}
	setAttribute(element, attribute, attributeURL.String()) // update the element attribute with the modified URL
	return nil
}

func removeElement(element *html.Node) {
	if element.Parent != nil {
		element.Parent.RemoveChild(element)
	} else {
		element.Type = html.CommentNode // comment out the element
		element.Data = ""               // remove the element data (tag name)
		element.Attr = nil              // remove the element attributes
	}
}

func removeAttributes(element *html.Node, keys ...string) {
	if element == nil {
		return // guard
	}
	if element.Type != html.ElementNode {
		return // guard
	}
	element.Attr = slices.DeleteFunc(element.Attr, func(attr html.Attribute) bool {
		return slices.Contains(keys, attr.Key)
	})
}

func innerText(element *html.Node, text string) {
	if element.Type == html.TextNode {
		element.Data = text
		return
	}
	for _, child := range childNodes(element) {
		element.RemoveChild(child)
	}
	element.AppendChild(&html.Node{Type: html.TextNode, Data: text})
}
func appendText(element *html.Node, text string) {
	if element.Type == html.TextNode {
		element.Data += text
		return
	}
	element.AppendChild(&html.Node{Type: html.TextNode, Data: text})
}
func outerText(element *html.Node, text string) {
	element.Type = html.TextNode
	element.Data = text
}

func innerHTML(element *html.Node, hypertext []*html.Node) {
	if element.Type != html.ElementNode {
		return // guard
	}
	for _, child := range childNodes(element) {
		child.Parent.RemoveChild(child)
	}
	for _, newElement := range hypertext {
		element.AppendChild(newElement)
	}
}

func outerHTML(element *html.Node, template []*html.Node) {
	for _, e := range template {
		element.Parent.InsertBefore(cloneElement(e), element)
	}
	element.Parent.RemoveChild(element)
}

func cloneElement(element *html.Node) *html.Node {
	clone := &html.Node{
		Type:     element.Type,
		DataAtom: element.DataAtom,
		Data:     element.Data,
		Attr:     make([]html.Attribute, len(element.Attr)),
	}
	copy(clone.Attr, element.Attr)
	for child := element.FirstChild; child != nil; child = child.NextSibling {
		clone.AppendChild(cloneElement(child)) // RECURSION!
	}
	return clone
}

func childNodes(element *html.Node) []*html.Node {
	children := []*html.Node{}
	for child := element.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, child)
	}
	return children
}

// func appendChildren(element *html.Node, children []*html.Node) {
// 	for _, child := range children {
// 		element.AppendChild(child)
// 	}
// }

func (template *Template) IsAbsoluteHref(href string) bool {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "//") {
		return true
	}
	if strings.HasPrefix(href, "/") {
		return true
	}
	return false // relative path
}
