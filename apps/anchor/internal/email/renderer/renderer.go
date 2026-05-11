// Package renderer renders email templates with variable substitution and
// loops. We delegate to Go's text/template (and html/template for HTML
// bodies) so customers get the full Go template language including:
//
//   - Variables:      {{ .name }}
//   - Loops:          {{ range .items }}...{{ end }}
//   - Conditionals:   {{ if .premium }}...{{ end }}
//   - Helpers:        {{ .total | printf "%.2f" }}
//
// HTML bodies are rendered through html/template so user-supplied variables
// are auto-escaped against XSS. The plain-text body uses text/template.
package renderer

import (
	"bytes"
	"errors"
	"fmt"
	"html"
	htmltemplate "html/template"
	"regexp"
	"strings"
	texttemplate "text/template"

	"anchor/internal/domain/email"
)

var (
	reBlockClose = regexp.MustCompile(`(?i)</(p|div|h[1-6]|li|tr|blockquote|section|article|header|footer|main|table)>`)
	reLineBreak  = regexp.MustCompile(`(?i)<br\s*/?>`)
	reStripTags  = regexp.MustCompile(`<[^>]+>`)
	reMultiSpace = regexp.MustCompile(`[ \t]+`)
	reMultiLines = regexp.MustCompile(`\n{3,}`)
)

// htmlToText converts rendered HTML to a readable plain-text fallback.
// It inserts newlines at block boundaries, strips all tags, and unescapes entities.
func htmlToText(rendered string) string {
	s := reBlockClose.ReplaceAllString(rendered, "$0\n")
	s = reLineBreak.ReplaceAllString(s, "\n")
	s = reStripTags.ReplaceAllString(s, "")
	s = html.UnescapeString(s)
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for _, l := range lines {
		l = reMultiSpace.ReplaceAllString(strings.TrimSpace(l), " ")
		out = append(out, l)
	}
	s = strings.Join(out, "\n")
	s = reMultiLines.ReplaceAllString(s, "\n\n")
	return strings.TrimSpace(s)
}

// reVarRef matches top-level variable references: {{ .varName }}.
var reVarRef = regexp.MustCompile(`\{\{-?\s*\.(\w+)`)

// Result is the rendered output of a single template + variable bag.
type Result struct {
	Subject  string
	BodyHTML string
	BodyText string
	// Warnings lists non-fatal issues collected during best-effort rendering.
	Warnings []string
}

// Renderer renders email.TemplateVersion values against a runtime variable
// bag. Rendering is best-effort: unknown or missing variables produce a
// visible [varName] placeholder and a warning rather than an error.
type Renderer struct{}

func New() *Renderer { return &Renderer{} }

// Render executes the version's subject + HTML body + plain-text body
// against the supplied variable bag in best-effort mode.
func (r *Renderer) Render(version *email.TemplateVersion, vars map[string]any) (Result, error) {
	if version == nil {
		return Result{}, errors.New("template version is nil")
	}

	warnings, padded := softValidate(version.Variables, vars, version.Subject, version.BodyHTML, version.BodyText)

	subject, err := r.renderText("subject", version.Subject, padded)
	if err != nil {
		return Result{}, fmt.Errorf("render subject: %w", err)
	}

	bodyHTML, err := r.renderHTML("body_html", version.BodyHTML, padded)
	if err != nil {
		return Result{}, fmt.Errorf("render body_html: %w", err)
	}

	bodyText := version.BodyText
	if bodyText == "" {
		bodyText = htmlToText(bodyHTML)
	} else {
		bodyText, err = r.renderText("body_text", bodyText, padded)
		if err != nil {
			return Result{}, fmt.Errorf("render body_text: %w", err)
		}
	}

	return Result{Subject: subject, BodyHTML: bodyHTML, BodyText: bodyText, Warnings: warnings}, nil
}

// softValidate collects non-fatal warnings and returns a padded vars map:
//   - vars not declared in schema → warned, still passed through
//   - required schema vars missing from vars → warned
//   - top-level template references with no value → padded with "[varName]"
//   - LIST/OBJECT schema vars with no value → padded with empty slice/map
func softValidate(schema []email.VariableSchema, vars map[string]any, templates ...string) ([]string, map[string]any) {
	var warnings []string

	schemaByName := make(map[string]email.VariableSchema, len(schema))
	for _, s := range schema {
		schemaByName[s.Name] = s
	}

	// Warn on vars submitted but not declared in schema.
	for name := range vars {
		if _, ok := schemaByName[name]; !ok {
			warnings = append(warnings, fmt.Sprintf("variable %q is not declared in the template schema", name))
		}
	}

	// Warn on required schema vars that are missing.
	for _, s := range schema {
		if s.Required {
			if _, ok := vars[s.Name]; !ok {
				warnings = append(warnings, fmt.Sprintf("required variable %q is missing", s.Name))
			}
		}
	}

	// Detect all top-level references across all template strings.
	referenced := make(map[string]struct{})
	for _, tmpl := range templates {
		for _, m := range reVarRef.FindAllStringSubmatch(tmpl, -1) {
			referenced[m[1]] = struct{}{}
		}
	}

	// Build padded map: start with caller-supplied values.
	padded := make(map[string]any, len(vars)+len(referenced))
	for k, v := range vars {
		padded[k] = v
	}

	// For referenced vars with no supplied value, insert a visible placeholder.
	// Only act on vars declared in the schema — undeclared refs (e.g. {{ .Name }}
	// inside {{ range .steps }}) are range-element field accessors, not missing
	// top-level variables, and should not be padded or warned about.
	for name := range referenced {
		if _, ok := padded[name]; ok {
			continue
		}
		s, inSchema := schemaByName[name]
		if !inSchema {
			continue
		}
		padded[name] = fmt.Sprintf("[%s]", name)
		switch s.Type {
		case email.VariableTypeString, email.VariableTypeNumber, email.VariableTypeBool:
		case email.VariableTypeList:
			padded[name] = []any{}
		case email.VariableTypeObject:
			padded[name] = map[string]any{}
		}
		warnings = append(warnings, fmt.Sprintf("variable %q has no value; rendered as placeholder", name))
	}

	return warnings, padded
}

func (r *Renderer) renderText(name, src string, vars map[string]any) (string, error) {
	t, err := texttemplate.New(name).Option("missingkey=zero").Parse(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if execErr := t.Execute(&buf, vars); execErr != nil {
		return "", execErr
	}
	return buf.String(), nil
}

func (r *Renderer) renderHTML(name, src string, vars map[string]any) (string, error) {
	t, err := htmltemplate.New(name).Option("missingkey=zero").Parse(src)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if execErr := t.Execute(&buf, vars); execErr != nil {
		return "", execErr
	}
	return buf.String(), nil
}
