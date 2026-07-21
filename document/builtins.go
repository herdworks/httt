package document

import (
	"bytes"

	"github.com/herdworks/hypertemplates-go/theme"
	"golang.org/x/net/html"
)

// IncludeBuiltins injects the default builtin fragments into doc.
func IncludeBuiltins(document *Document) error {
	return includeBuiltinSlots(document, theme.DefaultBuiltins())
}

func includeBuiltinSlots(document *Document, slots []theme.Builtin) error {
	if document.BlockMode {
		return nil
	}
	for _, slot := range slots {
		if err := includeBuiltinSlot(document, slot); err != nil {
			return err
		}
	}
	return nil
}

func includeBuiltinSlot(document *Document, slot theme.Builtin) error {
	fragmentBytes, found, err := resolveBuiltinFragment(document, slot.Fragment)
	if err != nil {
		return err
	}
	if !found {
		return nil
	}
	if len(fragmentBytes) == 0 {
		return nil
	}

	target := Query(document.Layout, slot.Selector)
	if target == nil {
		return nil
	}

	parseParent := target
	switch slot.Mode {
	case "before", "after":
		if target.Parent == nil {
			return nil
		}
		parseParent = target.Parent
	}

	fragments, err := html.ParseFragment(bytes.NewReader(fragmentBytes), parseParent)
	if err != nil {
		return err
	}

	switch slot.Mode {
	case "prepend":
		anchor := target.FirstChild
		for _, node := range fragments {
			if anchor == nil {
				target.AppendChild(node)
			} else {
				target.InsertBefore(node, anchor)
			}
		}
	case "append":
		for _, node := range fragments {
			target.AppendChild(node)
		}
	case "before":
		for _, node := range fragments {
			target.Parent.InsertBefore(node, target)
		}
	case "after":
		anchor := target.NextSibling
		for _, node := range fragments {
			if anchor == nil {
				target.Parent.AppendChild(node)
			} else {
				target.Parent.InsertBefore(node, anchor)
			}
		}
	}
	return nil
}

func resolveBuiltinFragment(document *Document, ref string) ([]byte, bool, error) {
	if document.Theme != nil {
		return document.Theme.ResourceBytes(ref)
	}
	return theme.ReadBuiltinResource(ref)
}
