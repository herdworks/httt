package format

// Syntax selects the surface syntax the parser accepts for template actions.
type Syntax int

const (
	// SyntaxDefault recognizes ${ ... } actions.
	SyntaxDefault Syntax = iota

	// SyntaxCSS recognizes htfmt("...") and htfmt('...') calls.
	// The quoted argument is the action body and follows the same grammar
	// as a default-syntax action (bare keypaths, comma-separated fallbacks,
	// pipelines to reserved or user functions).
	SyntaxCSS
)
