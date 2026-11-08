//go:build contract

package contract_test

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// selectPath evaluates the JSONPath subset the gate flows use (RFC 9535 as
// echopoint-runner runs it through theory/jsonpath): .name, [n], [-n], [*],
// ['name'], [?@.a.b=='lit'], [?@.a] and [?!@.a]. Like the runner, one match is
// returned as the value, several as a slice, and none is an error.
func selectPath(path string, doc any) (any, error) {
	if !strings.HasPrefix(path, "$") {
		return nil, fmt.Errorf("JSONPath %q must start with $", path)
	}
	nodes := []any{doc}
	rest := path[1:]
	for rest != "" {
		var err error
		nodes, rest, err = step(nodes, rest)
		if err != nil {
			return nil, fmt.Errorf("JSONPath %q: %w", path, err)
		}
	}
	switch len(nodes) {
	case 0:
		return nil, fmt.Errorf("JSONPath %q did not match any nodes", path)
	case 1:
		return nodes[0], nil
	default:
		return nodes, nil
	}
}

func step(nodes []any, rest string) ([]any, string, error) {
	switch rest[0] {
	case '.':
		end := strings.IndexAny(rest[1:], ".[")
		name := rest[1:]
		next := ""
		if end >= 0 {
			name = rest[1 : end+1]
			next = rest[end+1:]
		}
		if name == "*" {
			return wildcard(nodes), next, nil
		}
		return member(nodes, name), next, nil
	case '[':
		end := closingBracket(rest)
		if end < 0 {
			return nil, "", errors.New("unclosed [")
		}
		selector := strings.TrimSpace(rest[1:end])
		next := rest[end+1:]
		out, err := bracket(nodes, selector)
		return out, next, err
	default:
		return nil, "", fmt.Errorf("unexpected %q", rest)
	}
}

// closingBracket finds the ] that closes the [ at rest[0], skipping quoted text.
func closingBracket(rest string) int {
	quote := byte(0)
	for i := 1; i < len(rest); i++ {
		c := rest[i]
		switch {
		case quote != 0 && c == quote:
			quote = 0
		case quote != 0:
		case c == '\'' || c == '"':
			quote = c
		case c == ']':
			return i
		}
	}
	return -1
}

func bracket(nodes []any, selector string) ([]any, error) {
	switch {
	case selector == "*":
		return wildcard(nodes), nil
	case strings.HasPrefix(selector, "?"):
		return filter(nodes, strings.TrimSpace(selector[1:]))
	case strings.HasPrefix(selector, "'") || strings.HasPrefix(selector, `"`):
		return member(nodes, strings.Trim(selector, `'"`)), nil
	default:
		index, err := strconv.Atoi(selector)
		if err != nil {
			return nil, fmt.Errorf("unsupported selector [%s]", selector)
		}
		var out []any
		for _, node := range nodes {
			arr, ok := node.([]any)
			if !ok {
				continue
			}
			i := index
			if i < 0 {
				i += len(arr)
			}
			if i >= 0 && i < len(arr) {
				out = append(out, arr[i])
			}
		}
		return out, nil
	}
}

func member(nodes []any, name string) []any {
	var out []any
	for _, node := range nodes {
		if obj, ok := node.(map[string]any); ok {
			if v, found := obj[name]; found {
				out = append(out, v)
			}
		}
	}
	return out
}

func wildcard(nodes []any) []any {
	var out []any
	for _, node := range nodes {
		switch t := node.(type) {
		case []any:
			out = append(out, t...)
		case map[string]any:
			for _, v := range t {
				out = append(out, v)
			}
		}
	}
	return out
}

// filter supports @.path == literal, @.path (exists) and !@.path (absent).
func filter(nodes []any, expr string) ([]any, error) {
	negate := strings.HasPrefix(expr, "!")
	expr = strings.TrimPrefix(expr, "!")
	left, literal, hasCompare := strings.Cut(expr, "==")
	left = strings.TrimSpace(left)
	if !strings.HasPrefix(left, "@") {
		return nil, fmt.Errorf("unsupported filter %q", expr)
	}
	var want any
	if hasCompare {
		var err error
		want, err = parseLiteral(strings.TrimSpace(literal))
		if err != nil {
			return nil, err
		}
	}
	var out []any
	for _, candidate := range wildcard(nodes) {
		got, err := selectPath("$"+left[1:], candidate)
		found := err == nil
		keep := found
		if hasCompare {
			keep = found && fmt.Sprint(got) == fmt.Sprint(want)
		}
		if keep != negate {
			out = append(out, candidate)
		}
	}
	return out, nil
}

func parseLiteral(s string) (any, error) {
	if len(s) >= 2 && (s[0] == '\'' || s[0] == '"') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1], nil
	}
	switch s {
	case "true":
		return true, nil
	case "false":
		return false, nil
	case "null":
		return nil, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil, fmt.Errorf("unsupported literal %q", s)
	}
	return f, nil
}
