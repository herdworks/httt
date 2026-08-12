package rules

import (
	"slices"
	"strings"

	migrationtypes "github.com/herdworks/httt/migrate/types"
	"golang.org/x/net/html"
)

func containsOtherDirective(candidate *migrationtypes.Candidate, owned ...string) bool {
	var found bool
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if found {
			return
		}
		if node.Type == html.ElementNode {
			for _, attribute := range node.Attr {
				name := strings.ToLower(attribute.Key)
				if node == candidate.Element && slices.Contains(owned, name) {
					continue
				}
				if isDirectiveName(name) {
					found = true
					return
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(candidate.Element)
	return found
}

func isDirectiveName(name string) bool {
	name = strings.ToLower(name)
	return strings.HasPrefix(name, "ht-") || strings.HasPrefix(name, "data-ht-")
}
