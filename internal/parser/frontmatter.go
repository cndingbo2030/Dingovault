package parser

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SplitFrontmatter returns YAML bytes and markdown body when the file starts with a
// YAML frontmatter block (--- ... ---). Otherwise ok is false and body is the full src.
func SplitFrontmatter(src []byte) (yamlBytes []byte, body []byte, ok bool) {
	s := trimUTF8BOM(src)
	var rest []byte
	switch {
	case bytes.HasPrefix(s, []byte("---\r\n")):
		rest = s[len("---\r\n"):]
	case bytes.HasPrefix(s, []byte("---\n")):
		rest = s[len("---\n"):]
	default:
		return nil, src, false
	}

	pos := bytes.Index(rest, []byte("\n---"))
	if pos < 0 {
		return nil, src, false
	}
	yamlPart := rest[:pos]
	j := pos + 1 // at first '-' of closing fence
	if j+2 >= len(rest) || rest[j] != '-' || rest[j+1] != '-' || rest[j+2] != '-' {
		return nil, src, false
	}
	k := j + 3
	for k < len(rest) && rest[k] == '-' {
		k++
	}
	if k >= len(rest) {
		return yamlPart, nil, true
	}
	if rest[k] == '\r' {
		k++
	}
	if k >= len(rest) || rest[k] != '\n' {
		return nil, src, false
	}
	bodyStart := k + 1
	return yamlPart, rest[bodyStart:], true
}

func trimUTF8BOM(b []byte) []byte {
	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		return b[3:]
	}
	return b
}

// StripWikilinkBrackets trims optional Obsidian-style [[ ... ]] around a title.
func StripWikilinkBrackets(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[[") && strings.HasSuffix(s, "]]") {
		s = strings.TrimSuffix(strings.TrimPrefix(s, "[["), "]]")
	}
	return strings.TrimSpace(s)
}

// NormalizeAliasKey lowercases and strips wikilink brackets for alias lookup.
func NormalizeAliasKey(s string) string {
	return strings.ToLower(StripWikilinkBrackets(s))
}

// ParseFrontmatterYAML parses YAML frontmatter into flat string properties and extracted aliases.
// Known alias keys: alias, aliases (string, slice, or nested wikilink strings).
func ParseFrontmatterYAML(yamlBytes []byte) (props map[string]string, aliases []string, err error) {
	yamlBytes = trimUTF8BOM(yamlBytes)
	if len(strings.TrimSpace(string(yamlBytes))) == 0 {
		return map[string]string{}, nil, nil
	}
	var root map[string]any
	if err := yaml.Unmarshal(yamlBytes, &root); err != nil {
		return nil, nil, fmt.Errorf("yaml: %w", err)
	}
	if root == nil {
		return map[string]string{}, nil, nil
	}

	props = make(map[string]string)
	aliasSet := make(map[string]struct{})

	for k, v := range root {
		lk := strings.ToLower(strings.TrimSpace(k))
		if lk == "alias" || lk == "aliases" {
			collectAliases(v, aliasSet)
			// Still store raw in props for querying
			if s, ok := scalarToPropString(v); ok {
				props[k] = s
			} else if b, err := json.Marshal(v); err == nil {
				props[k] = string(b)
			}
			continue
		}
		if s, ok := scalarToPropString(v); ok {
			props[k] = s
			continue
		}
		if b, err := json.Marshal(v); err == nil {
			props[k] = string(b)
		}
	}

	for a := range aliasSet {
		if a != "" {
			aliases = append(aliases, a)
		}
	}
	return props, aliases, nil
}

func scalarToPropString(v any) (string, bool) {
	switch x := v.(type) {
	case string:
		return x, true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	case int:
		return fmt.Sprintf("%d", x), true
	case int64:
		return fmt.Sprintf("%d", x), true
	case float64:
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", x), "0"), "."), true
	default:
		return "", false
	}
}

func collectAliases(v any, out map[string]struct{}) {
	switch x := v.(type) {
	case string:
		a := StripWikilinkBrackets(x)
		if a != "" {
			out[a] = struct{}{}
		}
	case []any:
		for _, it := range x {
			collectAliases(it, out)
		}
	case nil:
	default:
		if s, ok := scalarToPropString(v); ok {
			collectAliases(s, out)
		}
	}
}
