package engine

import (
	"context"
	"encoding/json"
	"strings"

	"anchor/internal/domain/workflow"
)

type ParamType string

const (
	ParamText            ParamType = "text"
	ParamEmail           ParamType = "email"
	ParamJSON            ParamType = "json"
	ParamOrganization    ParamType = "organization"
	ParamProductUser     ParamType = "product_user"
	ParamRole            ParamType = "role"
	ParamLicenseTemplate ParamType = "license_template"
	ParamEmailTemplate   ParamType = "email_template"
)

type ParamSpec struct {
	Name        string
	Label       string
	Description string
	Type        ParamType
	Required    bool
}

type OutputSpec struct {
	Name        string
	Description string
}

type ActionSpec struct {
	Type        workflow.ActionType
	Name        string
	Description string
	Group       string
	Writes      bool
	Params      []ParamSpec
	Outputs     []OutputSpec
}

func (a ActionSpec) Param(name string) (ParamSpec, bool) {
	for _, param := range a.Params {
		if param.Name == name {
			return param, true
		}
	}
	return ParamSpec{}, false
}

// Env is what every action runs as: the Product the workflow belongs to.
// Nothing a template resolves can move an action to another Product.
type Env struct {
	TenantID  string
	ProductID string
	RunID     string
	StepID    string
}

type Params map[string]any

func (p Params) String(name string) string {
	return strings.TrimSpace(Stringify(p[name]))
}

func (p Params) OptionalString(name string) *string {
	value := p.String(name)
	if value == "" {
		return nil
	}
	return &value
}

func (p Params) Object(name string) (map[string]any, bool, error) {
	raw, ok := p[name]
	if !ok || raw == nil {
		return nil, false, nil
	}
	if object, isObject := raw.(map[string]any); isObject {
		return object, true, nil
	}
	text := strings.TrimSpace(Stringify(raw))
	if text == "" {
		return nil, false, nil
	}
	var object map[string]any
	if err := json.Unmarshal([]byte(text), &object); err != nil {
		return nil, false, invalidJSONParamError(name, err)
	}
	return object, true, nil
}

type runFunc func(ctx context.Context, env Env, params Params) (map[string]any, error)

type action struct {
	spec ActionSpec
	run  runFunc
}
