package scripting

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const pluginNameDirective = "ht:plugin"

// PluginName returns a plugin name declared in a leading JavaScript metadata
// comment.
//
// Supported forms are:
//
//	// ht:plugin upper
//	// ht:plugin: upper
//	// ht:plugin=upper
//	/* ht:plugin upper */
//
// Only leading comments and whitespace are scanned. Directives that appear
// after executable JavaScript are ignored.
func PluginName(source string) (string, bool, error) {
	var name string
	found := false

	apply := func(next string) error {
		if !validPluginName(next) {
			return fmt.Errorf("scripting: invalid plugin name %q", next)
		}
		if found {
			return fmt.Errorf("scripting: duplicate plugin name %q", next)
		}
		name = next
		found = true
		return nil
	}

	for offset := 0; offset < len(source); {
		offset = skipSpace(source, offset)
		if offset >= len(source) {
			break
		}

		switch {
		case strings.HasPrefix(source[offset:], "//"):
			end := strings.IndexByte(source[offset:], '\n')
			if end < 0 {
				end = len(source) - offset
			}
			comment := source[offset+2 : offset+end]
			next, ok, err := parsePluginNameComment(comment)
			if err != nil {
				return "", false, err
			}
			if ok {
				if err := apply(next); err != nil {
					return "", false, err
				}
			}
			offset += end

		case strings.HasPrefix(source[offset:], "/*"):
			end := strings.Index(source[offset+2:], "*/")
			if end < 0 {
				return "", false, fmt.Errorf("scripting: unterminated metadata comment")
			}
			comment := source[offset+2 : offset+2+end]
			for _, line := range strings.Split(comment, "\n") {
				next, ok, err := parsePluginNameComment(line)
				if err != nil {
					return "", false, err
				}
				if ok {
					if err := apply(next); err != nil {
						return "", false, err
					}
				}
			}
			offset += 2 + end + 2

		default:
			return name, found, nil
		}
	}

	return name, found, nil
}

func parsePluginNameComment(comment string) (string, bool, error) {
	line := strings.TrimSpace(comment)
	line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
	line = strings.TrimPrefix(line, "@")
	if !strings.HasPrefix(line, pluginNameDirective) {
		return "", false, nil
	}

	rest := line[len(pluginNameDirective):]
	if rest == "" {
		return "", false, fmt.Errorf("scripting: missing plugin name")
	}

	first, _ := utf8.DecodeRuneInString(rest)
	switch {
	case first == ':' || first == '=':
		rest = rest[1:]
	case unicode.IsSpace(first):
	default:
		return "", false, nil
	}

	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", false, fmt.Errorf("scripting: missing plugin name")
	}
	if len(fields) > 1 {
		return "", false, fmt.Errorf("scripting: invalid plugin name directive %q", strings.TrimSpace(comment))
	}

	return fields[0], true, nil
}

func validPluginName(name string) bool {
	if name == "" {
		return false
	}

	for index, r := range name {
		if index == 0 {
			if r != '_' && !unicode.IsLetter(r) {
				return false
			}
			continue
		}
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			return false
		}
	}

	return true
}

func skipSpace(source string, offset int) int {
	for offset < len(source) {
		r, size := utf8.DecodeRuneInString(source[offset:])
		if !unicode.IsSpace(r) {
			break
		}
		offset += size
	}
	return offset
}
