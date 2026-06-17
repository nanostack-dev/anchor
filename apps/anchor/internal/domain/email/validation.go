package email

import "strings"

// MissingRequiredVariables returns the names of variables that the schema marks
// required but that have no usable value in vars: absent, nil, or a string that
// is empty/whitespace-only. Names are returned in schema declaration order so
// error messages are stable.
//
// Sends must reference a template and a variable map; this is the gate that
// turns an unfilled required variable into a hard validation error rather than
// a best-effort placeholder render.
func MissingRequiredVariables(schema []VariableSchema, vars map[string]any) []string {
	var missing []string
	for _, s := range schema {
		if !s.Required {
			continue
		}
		v, ok := vars[s.Name]
		if !ok || v == nil || isBlankString(v) {
			missing = append(missing, s.Name)
		}
	}
	return missing
}

// isBlankString reports whether v is a string that is empty or whitespace-only.
// Non-string values are never blank.
func isBlankString(v any) bool {
	s, ok := v.(string)
	return ok && strings.TrimSpace(s) == ""
}
