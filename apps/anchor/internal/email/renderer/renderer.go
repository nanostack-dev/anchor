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
	htmltemplate "html/template"
	"maps"
	"regexp"
	"strconv"
	texttemplate "text/template"

	"anchor/internal/domain/email"

	"github.com/nanostack-dev/nanostack-framework/pkg/functional"
)

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
// bag. Rendering is best-effort: a missing required variable produces a
// visible [varName] placeholder and a warning rather than an error, and a
// missing optional variable renders empty.
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
//   - required scalar references with no value → padded with "[varName]"
//   - optional scalar references with no value → padded with "", so
//     {{ if .var }} stays false
//   - LIST/OBJECT schema vars with no value → padded with empty slice/map
func softValidate(schema []email.VariableSchema, vars map[string]any, templates ...string) ([]string, map[string]any) {
	var warnings []string

	schemaByName := functional.Slice(schema).ToMap(func(s email.VariableSchema) string { return s.Name })

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
	maps.Copy(padded, vars)

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
		switch s.Type {
		case email.VariableTypeList:
			padded[name] = []any{}
		case email.VariableTypeObject:
			padded[name] = map[string]any{}
		case email.VariableTypeString, email.VariableTypeNumber, email.VariableTypeBool:
			if !s.Required {
				padded[name] = ""
				continue
			}
			padded[name] = fmt.Sprintf("[%s]", name)
			warnings = append(warnings, fmt.Sprintf("variable %q has no value; rendered as placeholder", name))
		}
	}

	return warnings, padded
}

const (
	// maxRenderedBytes bounds one rendered part: a template can double its output with nested defines.
	maxRenderedBytes = 256 << 10
	// maxRangeCount bounds a range over an integer literal, which can loop without writing anything.
	maxRangeCount = 10_000
)

var (
	errRenderTooLarge   = fmt.Errorf("the rendered part exceeds %d bytes", maxRenderedBytes)
	errRangeCountTooBig = fmt.Errorf("a range over a number is limited to %d iterations", maxRangeCount)
	integerRange        = regexp.MustCompile(`\{\{-?\s*range\s+(?:\$\w+\s*:?=\s*)?(\d+)\s*-?\}\}`)
)

type cappedBuffer struct{ bytes.Buffer }

func (b *cappedBuffer) Write(p []byte) (int, error) {
	if b.Len()+len(p) > maxRenderedBytes {
		return 0, errRenderTooLarge
	}
	return b.Buffer.Write(p)
}

func checkIntegerRanges(src string) error {
	for _, match := range integerRange.FindAllStringSubmatch(src, -1) {
		if count, err := strconv.Atoi(match[1]); err != nil || count > maxRangeCount {
			return errRangeCountTooBig
		}
	}
	return nil
}

func (r *Renderer) renderText(name, src string, vars map[string]any) (string, error) {
	if err := checkIntegerRanges(src); err != nil {
		return "", err
	}
	t, err := texttemplate.New(name).Option("missingkey=zero").Parse(src)
	if err != nil {
		return "", err
	}
	var buf cappedBuffer
	if execErr := t.Execute(&buf, vars); execErr != nil {
		return "", execErr
	}
	return buf.String(), nil
}

func (r *Renderer) renderHTML(name, src string, vars map[string]any) (string, error) {
	if err := checkIntegerRanges(src); err != nil {
		return "", err
	}
	t, err := htmltemplate.New(name).Option("missingkey=zero").Parse(src)
	if err != nil {
		return "", err
	}
	var buf cappedBuffer
	if execErr := t.Execute(&buf, vars); execErr != nil {
		return "", execErr
	}
	return buf.String(), nil
}
