package engine

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var referencePattern = regexp.MustCompile(`\{\{\s*([A-Za-z0-9_.\-]+)\s*\}\}`)

// Scope is what a run can read: `event.*`, `workflow.*` and the output of
// every step that already ran under `steps.<id>.*`.
type Scope map[string]any

func NewScope(workflowID, workflowName, eventID, eventType string, eventData map[string]string) Scope {
	data := make(map[string]any, len(eventData))
	for key, value := range eventData {
		data[key] = value
	}
	return Scope{
		"workflow": map[string]any{"id": workflowID, keyName: workflowName},
		"event":    map[string]any{"id": eventID, "type": eventType, "data": data},
		"steps":    map[string]any{},
	}
}

func (s Scope) SetStepOutput(stepID string, output map[string]any) {
	steps, _ := s["steps"].(map[string]any)
	steps[stepID] = output
}

func (s Scope) Lookup(path string) (any, bool) {
	var current any = map[string]any(s)
	for segment := range strings.SplitSeq(path, ".") {
		switch node := current.(type) {
		case map[string]any:
			value, ok := node[segment]
			if !ok {
				return nil, false
			}
			current = value
		case map[string]string:
			value, ok := node[segment]
			if !ok {
				return nil, false
			}
			current = value
		default:
			return nil, false
		}
	}
	if current == nil {
		return nil, false
	}
	return current, true
}

// Render resolves every `{{ path }}` in raw. A string that is exactly one
// reference keeps the referenced value's type, so a JSON object can be passed
// on as is.
func (s Scope) Render(raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if match := referencePattern.FindStringSubmatch(trimmed); match != nil && match[0] == trimmed {
		value, ok := s.Lookup(match[1])
		if !ok {
			return nil, unresolvedReferenceError(match[1])
		}
		return value, nil
	}
	var missing string
	rendered := referencePattern.ReplaceAllStringFunc(raw, func(token string) string {
		path := referencePattern.FindStringSubmatch(token)[1]
		value, ok := s.Lookup(path)
		if !ok {
			if missing == "" {
				missing = path
			}
			return ""
		}
		return Stringify(value)
	})
	if missing != "" {
		return nil, unresolvedReferenceError(missing)
	}
	return rendered, nil
}

func (s Scope) RenderString(raw string) (string, error) {
	value, err := s.Render(raw)
	if err != nil {
		return "", err
	}
	return Stringify(value), nil
}

func Stringify(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case int:
		return strconv.Itoa(typed)
	case int64:
		return strconv.FormatInt(typed, 10)
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(encoded)
	}
}

// References lists every path that raw refers to.
func References(raw string) []string {
	matches := referencePattern.FindAllStringSubmatch(raw, -1)
	paths := make([]string, 0, len(matches))
	for _, match := range matches {
		paths = append(paths, match[1])
	}
	return paths
}
