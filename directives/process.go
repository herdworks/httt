package directives

import (
	"errors"
	"io"
	"os"

	htdata "github.com/herdworks/hypertemplates-go/data"
	htdocument "github.com/herdworks/hypertemplates-go/document"
)

func Process(document *htdocument.Document, data *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) ([]byte, error) {
	return process(document, data, userData, w)
}

func process(document *htdocument.Document, data *htdata.TemplateData, userData *htdata.TemplateData, w io.Writer) (htmlBytes []byte, err error) {
	if w == nil {
		w = os.Stdout
	}

	if document.Layout == nil {
		return nil, errors.New("template layout empty, nothing to do")
	}
	document.EnsureMarkdownConfig()

	if err = ResolveIncludes(document); err != nil {
		return nil, err
	}

	if err = htdocument.IncludeBuiltins(document); err != nil {
		return nil, err
	}

	data.Set("layout", document.LayoutData())

	for _, element := range HyperIfElements(document) {
		if err = EvaluateElement(document, element, data, true, w); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperNotElements(document) {
		if err = EvaluateElement(document, element, data, false, w); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperDebugElements(document) {
		if err = Debug(document, element, data, w); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperTemplateElements(document) {
		if err = RenderTemplate(document, element, data, userData, w); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperValueElements(document) {
		if err = RenderValues(document, element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperAttrsElements(document) {
		if err = RenderAttributes(document, element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperQueryElements(document) {
		if err = RenderQueryParams(document, element, data, w); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperParamsElements(document) {
		if err = RenderParams(document, element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperContentElements(document) {
		if err = RenderContent(document, element, data); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperBlockElements(document) {
		if err = RenderBlock(document, element, userData, w); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperPipeElements(document) {
		if err = RenderPipe(document, element); err != nil {
			return nil, err
		}
	}
	for _, element := range HyperBaseElements(document) {
		if err = RenderBasedElement(document, element, data, w); err != nil {
			return nil, err
		}
	}
	return document.HTML(document.Layout), nil
}
