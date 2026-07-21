package document

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"slices"
	"strconv"
	"strings"

	"github.com/andybalholm/cascadia"
	"github.com/herdworks/hypertemplates-go/data"
	htmarkdown "github.com/herdworks/hypertemplates-go/markdown"
	"golang.org/x/net/html"
)

func (document *Document) HTML(elements ...*html.Node) []byte {
	return HTML(elements...)
}

func (document *Document) AttributeData(element *html.Node) data.TemplateData {
	return AttributeData(element)
}

func (document *Document) LayoutData() (out data.TemplateData) {
	out = data.TemplateData{}
	for _, element := range QueryAll(document.Layout, "meta[name^='layout:']") {
		key := strings.Replace(GetAttributeString(element, "name"), "layout:", "", -1)
		value := GetAttributeString(element, "content")
		out.Set(key, value)
	}
	return out
}

func (document *Document) IsAbsoluteHref(href string) bool {
	return IsAbsoluteHref(href)
}

func HTML(elements ...*html.Node) []byte {
	if len(elements) == 0 {
		return []byte{}
	}
	buffer := new(bytes.Buffer)
	for _, element := range elements {
		html.Render(buffer, element)
	}
	return buffer.Bytes()
}

func TextContent(element *html.Node, unescape bool) (text string) {
	var innerText string
	if element.Type == html.TextNode {
		innerText = element.Data
	} else {
		innerText = string(HTML(ChildNodes(element)...))
	}
	if unescape {
		innerText = html.UnescapeString(innerText)
	}
	return innerText
}

func Query(node *html.Node, query ...string) (element *html.Node) {
	if node == nil {
		return nil
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
		return []*html.Node{}
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
		return []*html.Node{}
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

func ForwardAttributes(placeholder *html.Node, fragments []*html.Node, skipKeys ...string) {
	var attributes []html.Attribute
	if len(fragments) == 0 {
		return
	}
	for _, attr := range placeholder.Attr {
		if slices.Contains(skipKeys, attr.Key) {
			continue
		}
		attributes = append(attributes, attr)
	}
	for _, fragment := range fragments {
		if fragment.Type == html.ErrorNode {
			continue
		}
		if fragment.Type == html.TextNode {
			continue
		}
		if fragment.Type == html.CommentNode {
			continue
		}
		if fragment.Type == html.DoctypeNode {
			continue
		}
		for _, attr := range attributes {
			SetAttribute(fragment, attr.Key, attr.Val)
		}
		break
	}
}

// AttributeData returns the HTML element attributes as TemplateData.
func AttributeData(element *html.Node) (out data.TemplateData) {
	out = data.TemplateData{}
	for _, attr := range element.Attr {
		out[attr.Key] = attr.Val
	}
	return out
}

func HasAttribute(element *html.Node, keys ...string) bool {
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

func GetAttributeString(element *html.Node, keys ...string) (value string) {
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

func GetAttributeStrings(element *html.Node, keys ...string) (values []string) {
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

func GetAttributeInt(element *html.Node, keys ...string) (value int) {
	stringValue := GetAttributeString(element, keys...)
	value, _ = strconv.Atoi(stringValue)
	return value
}

func GetAttributes(element *html.Node, keys ...string) (values []string) {
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

func SetAttribute(element *html.Node, key string, value string) {
	if element.Type != html.ElementNode {
		return
	}
	attributeIndex := -1
	for i, attr := range element.Attr {
		if attr.Key != key {
			continue
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

func RemoveElement(element *html.Node) {
	if element.Parent != nil {
		element.Parent.RemoveChild(element)
	} else {
		element.Type = html.CommentNode
		element.Data = ""
		element.Attr = nil
	}
}

func RemoveAttributes(element *html.Node, keys ...string) {
	if element == nil {
		return
	}
	if element.Type != html.ElementNode {
		return
	}
	element.Attr = slices.DeleteFunc(element.Attr, func(attr html.Attribute) bool {
		return slices.Contains(keys, attr.Key)
	})
}

func InnerText(element *html.Node, text string) {
	if element.Type == html.TextNode {
		element.Data = text
		return
	}
	for _, child := range ChildNodes(element) {
		element.RemoveChild(child)
	}
	element.AppendChild(&html.Node{Type: html.TextNode, Data: text})
}

func AppendText(element *html.Node, text string) {
	if element.Type == html.TextNode {
		element.Data += text
		return
	}
	element.AppendChild(&html.Node{Type: html.TextNode, Data: text})
}

func OuterText(element *html.Node, text string) {
	element.Type = html.TextNode
	element.Data = text
}

func InnerHTML(element *html.Node, hypertext []*html.Node) {
	if element.Type != html.ElementNode {
		return
	}
	for _, child := range ChildNodes(element) {
		child.Parent.RemoveChild(child)
	}
	for _, newElement := range hypertext {
		element.AppendChild(newElement)
	}
}

func OuterHTML(element *html.Node, template []*html.Node) {
	for _, e := range template {
		element.Parent.InsertBefore(CloneElement(e), element)
	}
	element.Parent.RemoveChild(element)
}

func CloneElement(element *html.Node) *html.Node {
	clone := &html.Node{
		Type:     element.Type,
		DataAtom: element.DataAtom,
		Data:     element.Data,
		Attr:     make([]html.Attribute, len(element.Attr)),
	}
	copy(clone.Attr, element.Attr)
	for child := element.FirstChild; child != nil; child = child.NextSibling {
		clone.AppendChild(CloneElement(child))
	}
	return clone
}

func ChildNodes(element *html.Node) []*html.Node {
	children := []*html.Node{}
	for child := element.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, child)
	}
	return children
}

func IsAbsoluteHref(href string) bool {
	if strings.HasPrefix(href, "http://") || strings.HasPrefix(href, "https://") || strings.HasPrefix(href, "//") {
		return true
	}
	if strings.HasPrefix(href, "/") {
		return true
	}
	return false
}

func HTMLContent(parent *html.Node, value any) ([]*html.Node, error) {
	return html.ParseFragment(strings.NewReader(fmt.Sprintf("%v", value)), parent)
}

func MarkdownContent(parent *html.Node, config *htmarkdown.HyperMarkConfig, value any) ([]*html.Node, error) {
	md := htmarkdown.New(config)
	if err := md.Parse([]byte(fmt.Sprintf("%v", value))); err != nil {
		return nil, err
	}
	return html.ParseFragment(strings.NewReader(md.Content), parent)
}

func MarkdownValueFunc(parent *html.Node, config *htmarkdown.HyperMarkConfig) func(any) (string, error) {
	return func(value any) (string, error) {
		nodes, err := MarkdownContent(parent, config, value)
		if err != nil {
			return "", err
		}
		return NodesToHTMLString(nodes)
	}
}

func NodesToHTMLString(nodes []*html.Node) (string, error) {
	var buf bytes.Buffer
	for _, node := range nodes {
		if err := html.Render(&buf, node); err != nil {
			return "", err
		}
	}
	return buf.String(), nil
}

func ElementString(element *html.Node) string {
	var buffer bytes.Buffer
	if err := html.Render(&buffer, element); err != nil {
		return ""
	}
	return stdhtml.UnescapeString(buffer.String())
}
