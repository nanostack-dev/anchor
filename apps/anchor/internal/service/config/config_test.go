package config_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"anchor/internal/service/config"
)

// The framework's config validator reports every zero-valued field as missing
// unless the field is tagged `optional:"true"`. A bool whose safe value is
// false is therefore unsatisfiable without the tag: the environments that
// correctly leave it unset fail to boot, while any test config that sets it to
// true passes — which is exactly how a boot-breaking change reaches production
// with a green pipeline.
//
// This walks every bool in CoreConfig and requires the tag, so the next such
// field is caught here rather than by a crash-looping container.
func TestBoolConfigFieldsAreOptional(t *testing.T) {
	t.Parallel()

	assertBoolsOptional(t, reflect.TypeFor[config.CoreConfig](), "")
}

func assertBoolsOptional(t *testing.T, structType reflect.Type, parent string) {
	t.Helper()

	for field := range structType.Fields() {
		name := field.Tag.Get("yaml")
		if name == "" {
			name = field.Name
		}
		if parent != "" {
			name = parent + "." + name
		}

		fieldType := field.Type
		if fieldType.Kind() == reflect.Pointer {
			fieldType = fieldType.Elem()
		}

		if fieldType.Kind() == reflect.Struct {
			assertBoolsOptional(t, fieldType, name)

			continue
		}

		if fieldType.Kind() == reflect.Bool {
			assert.Equal(t, "true", field.Tag.Get("optional"),
				"bool config field %q must be tagged `optional:\"true\"`; "+
					"otherwise a false value is reported as missing and the app cannot boot", name)
		}
	}
}
