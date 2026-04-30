package scripting

import (
	"fmt"

	"github.com/dop251/goja/ast"
	"github.com/dop251/goja/parser"
	"github.com/dop251/goja/token"
)

const pluginSelectorName = "plugin"

// PluginSelector returns the public plugin name selected by a JavaScript plugin
// file. Plugin files must declare exactly one top-level `const plugin = name`
// binding. The selector value must be an identifier that refers to a top-level
// callable binding in the same file.
func PluginSelector(source string) (string, error) {
	program, err := parser.ParseFile(nil, "plugin.js", source, 0)
	if err != nil {
		return "", fmt.Errorf("scripting: parse plugin function: %w", err)
	}

	callables := map[string]bool{}
	var selector string
	selectorFound := false

	for _, statement := range program.Body {
		switch stmt := statement.(type) {
		case *ast.FunctionDeclaration:
			if stmt.Function != nil && stmt.Function.Name != nil {
				callables[stmt.Function.Name.Name.String()] = true
			}
		case *ast.LexicalDeclaration:
			for _, binding := range stmt.List {
				name, ok := bindingIdentifierName(binding)
				if !ok {
					continue
				}
				if name == pluginSelectorName {
					if stmt.Token != token.CONST {
						return "", fmt.Errorf("scripting: plugin selector must be declared with const")
					}
					if selectorFound {
						return "", fmt.Errorf("scripting: duplicate plugin selector")
					}
					next, err := selectorIdentifierName(binding.Initializer)
					if err != nil {
						return "", err
					}
					selector = next
					selectorFound = true
					continue
				}
				if stmt.Token == token.CONST && initializerIsCallable(binding.Initializer) {
					callables[name] = true
				}
			}
		case *ast.VariableStatement:
			for _, binding := range stmt.List {
				if name, ok := bindingIdentifierName(binding); ok && name == pluginSelectorName {
					return "", fmt.Errorf("scripting: plugin selector must be declared with const")
				}
			}
		}
	}

	if !selectorFound {
		return "", fmt.Errorf("scripting: missing plugin selector const plugin = name")
	}
	if reservedPluginName(selector) {
		return "", fmt.Errorf("scripting: reserved plugin name %q", selector)
	}
	if !callables[selector] {
		return "", fmt.Errorf("scripting: plugin selector %q does not refer to a top-level callable binding", selector)
	}
	return selector, nil
}

func bindingIdentifierName(binding *ast.Binding) (string, bool) {
	if binding == nil {
		return "", false
	}
	identifier, ok := binding.Target.(*ast.Identifier)
	if !ok {
		return "", false
	}
	return identifier.Name.String(), true
}

func selectorIdentifierName(initializer ast.Expression) (string, error) {
	if initializer == nil {
		return "", fmt.Errorf("scripting: plugin selector must reference a callable identifier")
	}
	identifier, ok := initializer.(*ast.Identifier)
	if !ok {
		return "", fmt.Errorf("scripting: plugin selector must reference a callable identifier")
	}
	return identifier.Name.String(), nil
}

func initializerIsCallable(initializer ast.Expression) bool {
	switch initializer.(type) {
	case *ast.FunctionLiteral, *ast.ArrowFunctionLiteral:
		return true
	default:
		return false
	}
}

func reservedPluginName(name string) bool {
	switch name {
	case "get", pluginSelectorName:
		return true
	default:
		return false
	}
}
