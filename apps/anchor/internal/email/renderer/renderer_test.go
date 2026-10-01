package renderer_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"anchor/internal/domain/email"
	"anchor/internal/email/renderer"
)

const inviterGreeting = `{{ if .inviter_name }}{{ .inviter_name }} invited you{{ else }}You are invited{{ end }}`

func renderSubject(t *testing.T, variable email.VariableSchema, vars map[string]any) renderer.Result {
	t.Helper()
	result, err := renderer.New().Render(&email.TemplateVersion{
		Subject:   inviterGreeting,
		BodyHTML:  "<p>" + inviterGreeting + "</p>",
		Variables: []email.VariableSchema{variable},
	}, vars)
	require.NoError(t, err)
	return result
}

func TestRenderTreatsAMissingOptionalVariableAsEmpty(t *testing.T) {
	result := renderSubject(t,
		email.VariableSchema{Name: "inviter_name", Type: email.VariableTypeString},
		map[string]any{},
	)

	assert.Equal(t, "You are invited", result.Subject)
	assert.Empty(t, result.Warnings)
}

func TestRenderPadsAMissingRequiredVariableWithAPlaceholder(t *testing.T) {
	result := renderSubject(t,
		email.VariableSchema{Name: "inviter_name", Type: email.VariableTypeString, Required: true},
		map[string]any{},
	)

	assert.Equal(t, "[inviter_name] invited you", result.Subject)
	assert.Contains(t, result.Warnings, `variable "inviter_name" has no value; rendered as placeholder`)
}

func TestRenderUsesASuppliedOptionalVariable(t *testing.T) {
	result := renderSubject(t,
		email.VariableSchema{Name: "inviter_name", Type: email.VariableTypeString},
		map[string]any{"inviter_name": "Ada"},
	)

	assert.Equal(t, "Ada invited you", result.Subject)
}
